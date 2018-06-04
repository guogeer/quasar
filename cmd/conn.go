package cmd

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"github.com/guogeer/husky/log"
	"io"
	"net"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 32 << 10 // 32K
	sendQueueSize  = 16 << 10
)

const (
	RawMessage   = 0x01
	CloseMessage = 0xf0
	PingMessage  = 0xf1
	PongMessage  = 0xf2
	AuthMessage  = 0xf3
)

const (
	StateClosed = iota
	StateConecting
	StateConnected
	StateClosing
)

type TCPConn struct {
	rwc     net.Conn
	ssid    string
	send    chan []byte
	isClose bool
}

func (c *TCPConn) Close() {
	if c.isClose == true {
		return
	}
	c.isClose = true
	close(c.send)
}

func (c *TCPConn) RemoteAddr() string {
	return c.rwc.RemoteAddr().String()
}

func (c *TCPConn) ReadMessage() (mt uint8, buf []byte, err error) {
	var head [3]byte
	// read message
	if _, err = io.ReadFull(c.rwc, head[:3]); err != nil {
		return
	}

	// 0x01~0x0f 表示版本
	// 0xf0 写队列尾部标识
	// 0xf1 PING
	// 0xf2 PONG
	n := int(binary.BigEndian.Uint16(head[1:3]))

	// log.Infof("read message %x %d", code, n)
	// 消息
	mt = uint8(head[0])
	switch mt {
	case PingMessage, PongMessage, CloseMessage:
		return
	case AuthMessage, RawMessage:
		if n > 0 && n < maxMessageSize {
			buf = make([]byte, n)
			if _, err = io.ReadFull(c.rwc, buf); err == nil {
				return
			}
		}
	}
	err = errors.New("invalid data")
	return
}

func (c *TCPConn) WriteJSON(name string, i interface{}) {
	// 4K缓存
	s, err := MarshalJSON(i)
	if err != nil {
		return
	}
	// 消息格式
	pkg := &Package{Id: name, Data: s}
	buf, err := json.Marshal(pkg)
	if err != nil {
		return
	}
	c.Write(buf)
}

func (c *TCPConn) Write(data []byte) {
	if c.isClose == true {
		return
	}
	buf := NewMessageBytes(RawMessage, data)
	c.writeMessage(buf)
}

func (c *TCPConn) writeMessage(msg []byte) {
	select {
	case c.send <- msg:
	default:
		log.Errorf("write too busy %s", string(msg))
	}
}

type Handler func(*Context, interface{})

type cmdEntry struct {
	h     Handler
	type_ reflect.Type
}

type CmdSet struct {
	services map[string]bool // 内部服务
	e        map[string]*cmdEntry
	mu       sync.RWMutex
}

var defaultCmdSet = NewCmdSet()

func GetCmdSet() *CmdSet {
	return defaultCmdSet
}

func NewCmdSet() *CmdSet {
	return &CmdSet{
		services: make(map[string]bool),
		e:        make(map[string]*cmdEntry),
	}
}

func (s *CmdSet) RemoveService(name string) {
	s.mu.Lock()
	s.services[name] = false
	s.mu.Unlock()
}

func (s *CmdSet) RegisterService(name string) {
	s.mu.Lock()
	s.services[name] = true
	s.mu.Unlock()
}

// 恢复服务
func (s *CmdSet) RecoverService(name string) {
	s.mu.Lock()
	if _, ok := s.services[name]; ok {
		s.services[name] = true
	}
	s.mu.Unlock()
}

func (s *CmdSet) Bind(name string, h Handler, i interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.e[name]; ok {
		log.Warnf("%s exist", name)
	}
	type_ := reflect.TypeOf(i)
	s.e[name] = &cmdEntry{h: h, type_: type_}
}

func (s *CmdSet) Parse(name string, data []byte) (Handler, interface{}, error) {
	s.mu.RLock()
	e, ok := s.e[name]
	s.mu.RUnlock()
	if ok == false {
		return nil, nil, errors.New("unkown message id")
	}

	// unmarshal argument
	args := reflect.New(e.type_.Elem()).Interface()
	if err := json.Unmarshal(data, args); err != nil {
		return nil, nil, err
	}
	return e.h, args, nil
}

func (s *CmdSet) Handle(ctx *Context, name string, data []byte) error {
	// 空数据使用默认JSON格式数据
	if data == nil || len(data) == 0 {
		data = []byte("{}")
	}

	var serverName string
	if subs := strings.SplitN(name, ".", 2); len(subs) > 1 {
		serverName, name = subs[0], subs[1]
	}
	if ctx.isGateway == true {
		// 网关转发的消息ID仅允许包含字母、数字
		if match, err := regexp.MatchString("^[A-Za-z0-9]*$", name); err == nil && !match {
			return errors.New("invalid message id")
		}
	}
	// router
	if len(serverName) > 0 {
		if ctx.isGateway == true {
			s.mu.RLock()
			isRegister := s.services[serverName]
			s.mu.RUnlock()
			// 网关仅允许转发已注册的逻辑服务器
			if isRegister == false {
				return errors.New("gateway try to route invalid service")
			}
		}

		if ss := GetSession(ctx.Ssid); ss != nil {
			ss.Route(serverName, name, data)
		}
		return nil
	}

	// unmarshal argument
	h, args, err := s.Parse(name, data)
	if err != nil {
		return err
	}
	Enqueue(ctx, h, args)
	return nil
}

func BindWithName(name string, h Handler, args interface{}) {
	GetCmdSet().Bind(name, h, args)
}

func RegisterServiceInGateway(name string) {
	GetCmdSet().RegisterService(name)
}

func Bind(h Handler, args interface{}) {
	name := runtime.FuncForPC(reflect.ValueOf(h).Pointer()).Name()
	n := strings.LastIndexByte(name, '.')
	if n >= 0 {
		name = name[n+1:]
	}
	// log.Debug("method name =", name)
	BindWithName(name, h, args)
}

func closeConn(ctx *Context, i interface{}) {
	ctx.Out.Close()
}

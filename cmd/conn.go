package cmd

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"github.com/guogeer/quasar/log"
	"io"
	"net"
	"reflect"
	"regexp"
	"sync"
	"time"
)

// 协议格式，前4个字节
// BYTE0：消息类型，BYTE1-3：消息长度

var errInvalidMessageID = errors.New("invalid message ID")

const (
	writeWait       = 10 * time.Second
	pongWait        = 60 * time.Second
	pingPeriod      = (pongWait * 9) / 10
	maxMessageSize  = 96 << 10 // 96K
	sendQueueSize   = 16 << 10
	messageHeadSize = 4
)

const (
	RawMessage   = 0x01
	CloseMessage = 0xf0
	PingMessage  = 0xf1
	PongMessage  = 0xf2
	AuthMessage  = 0xf3
)

type Conn interface {
	Write([]byte) error
	WriteJSON(string, interface{}) error
	RemoteAddr() string
	Close()
}

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
	var head [messageHeadSize]byte
	// read message
	if _, err = io.ReadFull(c.rwc, head[:]); err != nil {
		return
	}

	n := int(binary.BigEndian.Uint16(head[1:]))

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

func (c *TCPConn) NewMessageBytes(mt int, data []byte) ([]byte, error) {
	if len(data) > maxMessageSize {
		return nil, errTooLargeMessage
	}
	buf := make([]byte, len(data)+messageHeadSize)

	// 协议头
	buf[0] = byte(mt)
	binary.BigEndian.PutUint16(buf[1:messageHeadSize], uint16(len(data)))
	// 协议数据
	copy(buf[messageHeadSize:], data)
	return buf, nil
}

func (c *TCPConn) WriteJSON(name string, i interface{}) error {
	// 消息格式
	pkg := &Package{Id: name, Body: i}
	buf, err := defaultRawParser.Encode(pkg)
	if err != nil {
		return err
	}
	return c.Write(buf)
}

func (c *TCPConn) Write(data []byte) error {
	if c.isClose == true {
		return errors.New("connection is closed")
	}
	select {
	case c.send <- data:
	default:
		return errors.New("write too busy")
	}
	return nil
}

func (c *TCPConn) writeMsg(mt int, msg []byte) (int, error) {
	buf, err := c.NewMessageBytes(mt, msg)
	if err != nil {
		return 0, err
	}
	return c.rwc.Write(buf)
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

var defaultCmdSet = &CmdSet{
	services: make(map[string]bool),
	e:        make(map[string]*cmdEntry),
}

func (s *CmdSet) RemoveService(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.services[name] = false
}

func (s *CmdSet) RegisterService(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.services[name] = true
}

// 恢复已有的服务
func (s *CmdSet) RecoverService(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.services[name]; ok {
		s.services[name] = true
	}
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

func (s *CmdSet) Handle(ctx *Context, messageID string, data []byte) error {
	// 空数据使用默认JSON格式数据
	if data == nil || len(data) == 0 {
		data = []byte("{}")
	}

	serverName, name := routeMessage("", messageID)
	// 网关转发的消息ID仅允许包含字母、数字
	if ctx.isGateway == true {
		match, err := regexp.MatchString("^[A-Za-z0-9]+$", name)
		if err == nil && !match {
			return errors.New("invalid message id")
		}
	}

	s.mu.RLock()
	e := s.e[name]
	isService := s.services[serverName]
	s.mu.RUnlock()
	// router
	if len(serverName) > 0 {
		// 网关仅允许转发已注册的逻辑服务器
		if ctx.isGateway == true && isService == false {
			ctx.Out.WriteJSON("ServerClose", map[string]interface{}{"ServerName": serverName})
			return errors.New("gateway try to route invalid service")
		}

		if ss := GetSession(ctx.Ssid); ss != nil {
			ss.Route(serverName, name, data)
		}
		return nil
	}

	if e == nil {
		return errInvalidMessageID
	}

	// unmarshal argument
	args := reflect.New(e.type_.Elem()).Interface()
	if err := json.Unmarshal(data, args); err != nil {
		return err
	}

	msg := &Message{id: name, ctx: ctx, h: e.h, args: args}
	defaultMessageQueue.Enqueue(msg)
	return nil
}

func funcClose(ctx *Context, i interface{}) {
	ctx.Out.Close()
}

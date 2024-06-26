package cmd

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/guogeer/quasar/v2/log"
)

// 协议格式，前4个字节
// BYTE0：消息类型，BYTE1-3：消息长度

const (
	pongWait        = 60 * time.Second // 60s
	pingPeriod      = (pongWait * 9) / 10
	maxMessageSize  = 1 << 20 // 1M
	sendQueueSize   = 32 << 10
	messageHeadSize = 4
)

const (
	RawMessage  = 0x01
	PingMessage = 0xf1
	PongMessage = 0xf2
)

type Conn interface {
	Write([]byte) error
	WriteJSON(string, any) error
	RemoteAddr() string
	Close()
}

type TCPConn struct {
	rwc     net.Conn
	ssid    string
	send    chan []byte
	pong    chan bool
	isClose bool
	mu      sync.RWMutex
}

func (c *TCPConn) Close() {
	c.rwc.Close()

	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.isClose {
		c.isClose = true
		close(c.send)
	}
}

func (c *TCPConn) RemoteAddr() string {
	return c.rwc.RemoteAddr().String()
}

func (c *TCPConn) ReadMessage() (uint8, []byte, error) {
	var head [messageHeadSize]byte
	// read message
	if _, err := io.ReadFull(c.rwc, head[:]); err != nil {
		return 0, nil, err
	}

	// 消息
	mt := uint8(head[0])
	n := int(binary.BigEndian.Uint16(head[1:]))
	switch mt {
	case PingMessage:
		c.pong <- true

		c.rwc.SetReadDeadline(time.Now().Add(pongWait))
		return mt, nil, nil
	case PongMessage:
		return mt, nil, nil
	case RawMessage:
		if n > 0 && n < maxMessageSize {
			buf := make([]byte, n)
			if _, err := io.ReadFull(c.rwc, buf); err != nil {
				return 0, nil, err
			}
			return mt, buf, nil
		}
	}
	return 0, nil, errors.New("invalid data")
}

func (c *TCPConn) WriteJSON(name string, i any) error {
	// 消息格式
	pkg := &Package{Id: name, Body: i}
	buf, err := EncodePackage(pkg)
	if err != nil {
		return err
	}
	return c.Write(buf)
}

func (c *TCPConn) Write(data []byte) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.isClose {
		return errors.New("connection is closed")
	}
	select {
	case c.send <- data:
	default:
		return errors.New("write too busy")
	}
	return nil
}

func (c *TCPConn) writeMsg(mt int, data []byte) (int, error) {
	if len(data) > maxMessageSize {
		return 0, errTooLargeMessage
	}
	buf := make([]byte, len(data)+messageHeadSize)

	// 协议头
	buf[0] = byte(mt)
	binary.BigEndian.PutUint16(buf[1:messageHeadSize], uint16(len(data)))
	// 协议数据
	copy(buf[messageHeadSize:], data)
	return c.rwc.Write(buf)
}

type Handler func(*Context, any)

type cmdEntry struct {
	name       string
	h          Handler
	type_      reflect.Type
	inQueue    bool // 请求入消息队列处理
	isPrivate  bool // 内部消息，不对外开放
	serverName string
}

type bindOption struct {
	isPrivate  bool
	inQueue    bool
	serverName string
}

type bindOptionFunc func(opt *bindOption)

func WithoutQueue() bindOptionFunc {
	return func(opt *bindOption) {
		opt.inQueue = false
	}
}

func WithPrivate() bindOptionFunc {
	return func(opt *bindOption) {
		opt.isPrivate = true
	}
}

func WithServer(name string) bindOptionFunc {
	return func(opt *bindOption) {
		opt.serverName = name
	}
}

type CmdSet struct {
	table map[string]*cmdEntry
	mu    sync.RWMutex

	hook Handler // 调用顺序：hook->bind
}

var defaultCmdSet = &CmdSet{
	table: make(map[string]*cmdEntry),
}

func (s *CmdSet) Bind(name string, h Handler, i any, opt ...bindOptionFunc) {
	name = strings.ToLower(name)

	var type_ reflect.Type
	if i != nil {
		type_ = reflect.TypeOf(i)
	}

	optResult := &bindOption{inQueue: true}
	for _, fn := range opt {
		fn(optResult)
	}

	e := &cmdEntry{name: name, h: h, type_: type_, inQueue: !optResult.inQueue, isPrivate: optResult.isPrivate, serverName: optResult.serverName}

	s.mu.Lock()
	defer s.mu.Unlock()

	matchName := name
	if e.serverName != "" {
		matchName = strings.Join([]string{e.serverName, name}, ".")
	}
	if _, ok := s.table[matchName]; ok {
		panic("cmd " + matchName + " redefined")
	}
	s.table[matchName] = e
}

func (s *CmdSet) Hook(h Handler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.hook != nil {
		log.Warn("cmd hook is existed")
	}
	s.hook = h
}

func (s *CmdSet) Handle(ctx *Context, msgId string, data []byte) error {
	msgId = strings.ToLower(msgId)

	ctx.MsgId = msgId
	// 空数据使用默认JSON格式数据
	if len(data) == 0 {
		data = []byte("{}")
	}

	serverName, name := splitMsgId(msgId)
	s.mu.RLock()
	e, ok := s.table[name]
	if !ok {
		e = s.table[strings.Join([]string{ctx.ServerName, name}, ".")]
	}
	hook := s.hook
	s.mu.RUnlock()
	// 转发消息
	if len(serverName) > 0 {
		if ss := GetSession(ctx.Ssid); ss != nil {
			ss.routeContext(ctx, name, data)
		}
		return nil
	}

	if e == nil {
		return errors.New("invalid message id")
	}
	if e.isPrivate && ctx.ServerName != "" {
		return errors.New("not allow message id")
	}

	var args any
	if e.type_ != nil {
		args = reflect.New(e.type_.Elem()).Interface()
		if err := json.Unmarshal(data, args); err != nil {
			return err
		}
	}

	// 消息入队处理
	if e.inQueue {
		msg := &msgTask{id: name, ctx: ctx, h: e.h, args: args, hook: hook}
		defaultMsgQueue.q <- msg
	} else {
		// 消息直接处理。入网关转发数据时
		if hook != nil {
			hook(ctx, args)
		}
		if !ctx.isFail {
			e.h(ctx, args)
		}
	}

	return nil
}

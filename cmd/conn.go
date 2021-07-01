package cmd

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
	"reflect"
	"sync"
	"time"

	"github.com/guogeer/quasar/log"
)

// 协议格式，前4个字节
// BYTE0：消息类型，BYTE1-3：消息长度

const (
	pongWait        = 60 * time.Second
	pingPeriod      = (pongWait * 9) / 10
	maxMessageSize  = 1 << 20 // 1M
	sendQueueSize   = 32 << 10
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
			buf = newBuf(n)
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
	buf := newBuf(len(data) + messageHeadSize)

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

func (c *TCPConn) writeMsg(mt int, msg []byte) (int, error) {
	buf, err := c.NewMessageBytes(mt, msg)
	saveBuf(msg)
	if err != nil {
		return 0, err
	}
	return c.rwc.Write(buf)
}

type Handler func(*Context, interface{})

type cmdEntry struct {
	h           Handler
	type_       reflect.Type
	isPushQueue bool // 请求入消息队列处理
}

type CmdSet struct {
	e  map[string]*cmdEntry
	mu sync.RWMutex

	hook Handler // 调用顺序：hook->bind
}

var defaultCmdSet = &CmdSet{
	e: make(map[string]*cmdEntry),
}

func (s *CmdSet) Bind(name string, h Handler, i interface{}, isPushQueue bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.e[name]; ok {
		log.Warnf("%s exist", name)
	}
	type_ := reflect.TypeOf(i)
	s.e[name] = &cmdEntry{h: h, type_: type_, isPushQueue: isPushQueue}
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
	ctx.MsgId = msgId
	// 空数据使用默认JSON格式数据
	if len(data) == 0 {
		data = []byte("{}")
	}

	serverName, name := routeMessage("", msgId)
	s.mu.RLock()
	e := s.e[name]
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

	// unmarshal argument
	args := reflect.New(e.type_.Elem()).Interface()
	if err := json.Unmarshal(data, args); err != nil {
		return err
	}

	// 消息入队处理
	if e.isPushQueue {
		msg := &Message{id: name, ctx: ctx, h: e.h, args: args, hook: hook}
		defaultMessageQueue.Enqueue(msg)
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

var (
	bufPool256, bufPool1024 sync.Pool
)

// 缓存
func newBuf(n int) []byte {
	var p interface{}
	if n <= 256 {
		p = bufPool256.Get()
	} else if n <= 1024 {
		p = bufPool1024.Get()
	}

	var buf []byte
	if p != nil {
		buf = *(p.(*[]byte))
	}
	if buf == nil {
		buf = make([]byte, n)
	}
	return buf[:n]
}

func saveBuf(buf []byte) {
	if cap(buf) == 256 {
		bufPool256.Put(&buf)
	} else if cap(buf) == 1024 {
		bufPool1024.Put(&buf)
	}
}

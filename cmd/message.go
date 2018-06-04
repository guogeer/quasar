package cmd

import (
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/buger/jsonparser"
	"third/env"
	"third/log"
	"time"
)

type Conn interface {
	Write([]byte)
	WriteJSON(string, interface{})
	RemoteAddr() string
	Close()
}

type Context struct {
	Out       Conn   // 连接
	Ssid      string // 发送方会话ID
	Version   int    // 协议版本，当前未生效
	isGateway bool   // 网关
}

type Message struct {
	h    Handler
	ctx  *Context
	args interface{}
}

type SafeQueue struct {
	q chan interface{}
}

func NewSafeQueue(size int) *SafeQueue {
	return &SafeQueue{q: make(chan interface{}, size)}
}

func (safeQueue *SafeQueue) Enqueue(i interface{}) {
	safeQueue.q <- i
}

func (safeQueue *SafeQueue) Dequeue() interface{} {
	select {
	case msg, ok := <-safeQueue.q:
		if ok == false {
			log.Debug("use closed queue in fault")
		}
		return msg
	case <-time.After(40 * time.Millisecond):
		return nil
	}
	return nil
}

var defaultMessageQueue = NewSafeQueue(16 << 10)

func GetMessageQueue() *SafeQueue {
	return defaultMessageQueue
}

func RunOnce() {
	for i := 0; i < 64; i++ {
		front := GetMessageQueue().Dequeue()
		if front == nil {
			break
		}
		msg := front.(*Message)
		msg.h(msg.ctx, msg.args)
	}
}

func Enqueue(ctx *Context, h Handler, args interface{}) {
	GetMessageQueue().Enqueue(&Message{ctx: ctx, h: h, args: args})
}

type Package struct {
	Id       string          `json:",omitempty"`    // 消息ID
	Data     json.RawMessage `json:",omitempty"`    // 数据,object类型
	Sign     string          `json:",omitempty"`    // 签名
	Ssid     string          `json:",omitempty"`    // 会话ID
	Version  int             `json:"Ver,omitempty"` // 版本
	SendTime int64           `json:",omitempty"`    // 发送的时间戳
}

type PackageParser interface {
	Encode(*Package) ([]byte, error)
	Decode([]byte) (*Package, error)
}

var gRawParser PackageParser = &rawParser{}
var gHashParser PackageParser = &hashParser{tempSign: "12345678"}
var gAuthParser PackageParser = &authParser{tempSign: "a9542bb104fe3f4d562e1d275e03f5ba"}

// 自定义客户端发送的数据处理
func SetParser(parser PackageParser) {
	gHashParser = parser
}

// 服务器内部身份认证
type authParser struct {
	tempSign string
}

func (parser *authParser) Encode(pkg *Package) ([]byte, error) {
	pkg.Sign = parser.tempSign
	pkg.SendTime = time.Now().Unix()

	buf, err := json.Marshal(pkg)
	if err != nil {
		return nil, err
	}
	if _, err := parser.Signature(buf); err != nil {
		return nil, err
	}
	return NewMessageBytes(AuthMessage, buf), nil
}

func (parser *authParser) Decode(buf []byte) (*Package, error) {
	pkg := &Package{}
	if err := json.Unmarshal(buf, pkg); err != nil {
		return nil, err
	}

	ts := int64(pkg.SendTime)
	ts0 := time.Now().Unix()
	if ts > ts0 || ts+5 < ts0 {
		return nil, errors.New("package expire")
	}

	sign, err := parser.Signature(buf)
	if err != nil {
		return nil, err
	}
	if pkg.Sign != sign {
		return nil, errors.New("invalid sign")
	}
	return pkg, nil
}

func (parser *authParser) Signature(data []byte) (string, error) {
	key := env.Config().ProductKey
	buf := make([]byte, len(key)+len(data))
	copy(buf[:], []byte(key))
	copy(buf[len(key):], data)

	tempSign := parser.tempSign
	_, _, n, err := jsonparser.Get(data, "Sign")
	if err != nil {
		return "", err
	}
	if signLen := len(tempSign) + 1; n >= signLen {
		copy(buf[len(key)+n-signLen:], tempSign)
	}

	sum := md5.Sum(buf)
	hexSum := hex.EncodeToString(sum[:])
	if signLen := len(tempSign) + 1; n >= signLen {
		copy(data[n-signLen:n], hexSum)
	}
	return string(hexSum), nil

}

type rawParser struct{}

func (parser *rawParser) Encode(pkg *Package) ([]byte, error) {
	buf, err := json.Marshal(pkg)
	if err != nil {
		return nil, err
	}
	return NewMessageBytes(RawMessage, buf), nil
}

func (parser *rawParser) Decode(buf []byte) (*Package, error) {
	pkg := &Package{}
	if err := json.Unmarshal(buf, pkg); err != nil {
		return nil, err
	}
	return pkg, nil
}

// 哈希
type hashParser struct{ tempSign string }

func (parser *hashParser) Encode(pkg *Package) ([]byte, error) {
	pkg.Sign = parser.tempSign

	buf, err := json.Marshal(pkg)
	if err != nil {
		return nil, err
	}
	if _, err := parser.Signature(buf); err != nil {
		return nil, err
	}
	return buf, nil
}

func (parser *hashParser) Decode(buf []byte) (*Package, error) {
	pkg := &Package{}
	if err := json.Unmarshal(buf, pkg); err != nil {
		return nil, err
	}

	sign, err := parser.Signature(buf)
	if err != nil {
		return nil, err
	}
	if pkg.Sign != sign {
		return nil, errors.New("invalid sign")
	}
	return pkg, nil

}

func (parser *hashParser) Signature(data []byte) (string, error) {
	key := env.Config().ProductKey
	ref := []int{0, 3, 4, 8, 10, 11, 13, 14}
	buf := make([]byte, len(key)+len(data))
	copy(buf[:], []byte(key))
	copy(buf[len(key):], data)

	tempSign := parser.tempSign
	_, _, n, err := jsonparser.Get(data, "Sign")
	if err != nil {
		return "", nil
	}
	if signLen := len(tempSign) + 1; n >= signLen {
		copy(buf[len(key)+n-signLen:], tempSign)
	}

	sign := make([]byte, len(ref))
	sum := md5.Sum(buf)
	hexSum := hex.EncodeToString(sum[:])
	for k, v := range ref {
		sign[k] = hexSum[v]
	}
	if signLen := len(tempSign) + 1; n >= signLen {
		copy(data[n-signLen:n], sign)
	}
	return string(sign), nil
}

func Encode(pkg *Package) ([]byte, error) {
	return gHashParser.Encode(pkg)
}

func Decode(buf []byte) (*Package, error) {
	return gHashParser.Decode(buf)
}

func MarshalJSON(i interface{}) ([]byte, error) {
	switch i.(type) {
	case []byte:
		return i.([]byte), nil
	case string:
		return []byte(i.(string)), nil
	}
	return json.Marshal(i)
}

func NewMessageBytes(mt int, data []byte) []byte {
	buf := make([]byte, len(data)+3)
	// 协议头
	copy(buf, []byte{byte(mt), 0x0, 0x0})
	binary.BigEndian.PutUint16(buf[1:3], uint16(len(data)))
	// 协议数据
	copy(buf[3:], data)
	return buf
}

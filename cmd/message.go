package cmd

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/buger/jsonparser"
	"github.com/guogeer/husky/log"
	"strings"
	"time"
)

var (
	ErrInvalidSign     = errors.New("invalid sign")
	errPackageExpire   = errors.New("package expire")
	errTooLargeMessage = errors.New("too large message")
)

type Conn interface {
	Write([]byte) error
	WriteJSON(string, interface{}) error
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

	Body  interface{} `json:"-"` // 传入的参数
	IsRaw bool        `json:"-"`
}

type PackageParser interface {
	Encode(*Package) ([]byte, error)
	Decode([]byte) (*Package, error)
}

func (pkg *Package) beforeEncode(parser PackageParser) (err error) {
	body := pkg.Body
	if body == nil {
		return
	}
	pkg.Data, err = marshalJSON(body)
	return
}

var defaultRawParser PackageParser = &hashParser{}
var defaultHashParser PackageParser = &hashParser{
	ref:      []int{0, 3, 4, 8, 10, 11, 13, 14},
	tempSign: "12345678",
}
var defaultAuthParser PackageParser = &hashParser{
	secs:     5,
	tempSign: "a9542bb104fe3f4d562e1d275e03f5ba",
}

// 哈希
type hashParser struct {
	secs     int
	ref      []int
	key      string
	tempSign string
}

func (parser *hashParser) Encode(pkg *Package) ([]byte, error) {
	pkg.Sign = parser.tempSign
	if secs := parser.secs; secs > 0 {
		pkg.SendTime = time.Now().Unix()
	}

	if err := pkg.beforeEncode(parser); err != nil {
		return nil, err
	}
	buf, err := json.Marshal(pkg)
	if err != nil {
		return nil, err
	}
	if parser.tempSign != "" {
		if _, err := parser.Signature(buf); err != nil {
			return nil, err
		}
	}
	return buf, nil
}

func (parser *hashParser) Decode(buf []byte) (*Package, error) {
	pkg := &Package{}
	if err := json.Unmarshal(buf, pkg); err != nil {
		return nil, err
	}
	if secs := int64(parser.secs); secs > 0 {
		ts := pkg.SendTime
		ts0 := time.Now().Unix()
		if ts > ts0 || ts+secs < ts0 {
			return nil, errPackageExpire
		}
	}

	sign, err := parser.Signature(buf)
	if err != nil {
		return nil, ErrInvalidSign
	}
	if pkg.Sign != sign {
		return pkg, ErrInvalidSign
	}
	return pkg, nil

}

func (parser *hashParser) Signature(data []byte) (string, error) {
	ref, key := parser.ref, parser.key
	if key == "" {
		return "", nil
	}
	buf := make([]byte, len(key)+len(data))
	copy(buf[:], []byte(key))
	copy(buf[len(key):], data)

	tempSign := parser.tempSign
	_, _, n, err := jsonparser.Get(data, "Sign")
	if err != nil {
		return "", err
	}
	signLen := len(tempSign) + 1
	if n < signLen {
		return "", ErrInvalidSign
	}
	copy(buf[len(key)+n-signLen:], tempSign)

	sum := md5.Sum(buf)
	sign := hex.EncodeToString(sum[:])
	if len(ref) == len(tempSign) {
		sign2 := make([]byte, len(ref))
		for k, v := range ref {
			sign2[k] = sign[v]
		}
		sign = string(sign2)
	}
	if signLen := len(tempSign) + 1; n >= signLen {
		copy(data[n-signLen:n], sign)
	}
	return sign, nil
}

func Encode(pkg *Package) ([]byte, error) {
	parser := defaultHashParser
	if pkg.IsRaw == true {
		parser = defaultRawParser
	}
	return parser.Encode(pkg)
}

func Decode(buf []byte) (*Package, error) {
	return defaultHashParser.Decode(buf)
}

func Encode2(name string, i interface{}) ([]byte, error) {
	return Encode(&Package{Id: name, Body: i})
}

func marshalJSON(i interface{}) ([]byte, error) {
	switch i.(type) {
	case []byte:
		return i.([]byte), nil
	case string:
		return []byte(i.(string)), nil
	}
	return json.Marshal(i)
}

func routeMessage(server, message string) (string, string) {
	if server != "" {
		message = server + "." + message
	}
	if subs := strings.SplitN(message, ".", 2); len(subs) > 1 {
		server, message = subs[0], subs[1]
	}
	return server, message
}

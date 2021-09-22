package cmd

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/buger/jsonparser"
	"github.com/guogeer/quasar/log"
)

var (
	ErrInvalidSign     = errors.New("invalid sign")
	errPackageExpire   = errors.New("package expire")
	errTooLargeMessage = errors.New("too large message")
)

type M map[string]interface{}

type Context struct {
	Out         Conn   // 连接
	MsgId       string // 消息ID
	Ssid        string // 发送方会话ID
	Version     int    // 协议版本，当前未生效
	ServerName  string // 请求的协议头
	ClientAddr  string // 客户端地址
	MatchServer string // 多个服务合并后的唯一serverName
	isFail      bool   // 失败处理后，不需要继续处理
}

// 失败后不再处理后续消息
func (ctx *Context) Fail() {
	ctx.isFail = true
}

type msgTask struct {
	id   string
	h    Handler
	hook Handler
	ctx  *Context
	args interface{}
}

type msgQueue struct {
	q chan *msgTask
}

func newMsgQueue(size int) *msgQueue {
	return &msgQueue{q: make(chan *msgTask, size)}
}

var defaultMsgQueue = newMsgQueue(8 << 10)

// 统计消息平均负载&访问频率等
type messageStat struct {
	id   string
	d    time.Duration // 耗时
	call int           // 调用次数
}

func (stat *messageStat) merge(stat2 *messageStat) {
	if stat2 != nil {
		stat.d += stat2.d
		stat.call += stat2.call
	}
}

var (
	lastPrintTime time.Time // 10分钟打印一次
	messageStats  map[string]messageStat
)

func RunOnce() {
	var t1, t2 time.Time
	var stats map[string]messageStat
	if enableDebug {
		stats = map[string]messageStat{}
	}
	mq := defaultMsgQueue
	for i := 0; i < 256; i++ {
		var msg *msgTask
		select {
		case msg = <-mq.q:
		case <-time.After(40 * time.Microsecond):
		}
		if msg == nil {
			break
		}
		if enableDebug {
			t1 = time.Now()
		}
		if msg.hook != nil {
			msg.hook(msg.ctx, msg.args)
		}
		if !msg.ctx.isFail {
			msg.h(msg.ctx, msg.args)
		}

		if enableDebug {
			t2 = time.Now()
			stat := stats[msg.id]
			stat.merge(&messageStat{d: t2.Sub(t1), call: 1})
			stats[msg.id] = stat
		}
	}
	if enableDebug {
		if lastPrintTime.IsZero() {
			lastPrintTime = time.Now()
		}
		if len(messageStats) == 0 {
			messageStats = map[string]messageStat{}
		}

		for id, stat := range stats {
			stat2 := messageStats[id]
			stat2.merge(&stat)
			messageStats[id] = stat2
		}
		tpc := make([]messageStat, 0, 256) // cost time per call
		cps := make([]messageStat, 0, 256) // call per second
		for id, stat := range messageStats {
			stat.id = id
			tpc = append(tpc, stat)
			cps = append(cps, stat)
		}

		d := time.Since(lastPrintTime)
		if d >= 10*time.Minute {
			log.Debug("=========== message stats start  ============")
			sort.SliceStable(tpc, func(i, j int) bool {
				return tpc[i].d.Seconds()/float64(tpc[i].call) > tpc[j].d.Seconds()/float64(tpc[j].call)
			})
			sort.SliceStable(cps, func(i, j int) bool { return cps[i].call > cps[j].call })
			for i := 0; i < 10 && i < len(messageStats); i++ {
				stat1, stat2 := tpc[i], cps[i]
				log.Debugf("cost time per call: %s %.2fms, call per second %s %.2f", stat1.id, stat1.d.Seconds()*1000/float64(stat1.call), stat2.id, float64(stat2.call)/d.Seconds())
			}
			log.Debug("=========== message stats end  ============")

			// 清理旧数据
			messageStats = nil
			lastPrintTime = time.Time{}
		}
	}
}

type Package struct {
	Id         string          `json:",omitempty"`    // 消息ID
	Data       json.RawMessage `json:",omitempty"`    // 数据,object类型
	Sign       string          `json:",omitempty"`    // 签名
	Ssid       string          `json:",omitempty"`    // 会话ID
	Version    int             `json:"Ver,omitempty"` // 版本
	ExpireTs   int64           `json:",omitempty"`    // 过期时间戳
	ServerName string          `json:",omitempty"`    // 请求的协议头
	ClientAddr string          `json:",omitempty"`    // 客户端地址

	Body interface{} `json:"-"` // 解析成Data
}

// 服务内部协议
var rawParser = &hashParser{}

// 服务器内建立连接时将检验第一个包的数据
var authParser = &hashParser{
	key:      "420e57b017066b44e05ea1577f6e2e12",
	tempSign: "a9542bb104fe3f4d562e1d275e03f5ba",
}

// 外网客户端协议
var clientParser = &hashParser{
	ref:      []int{0, 3, 4, 8, 10, 11, 13, 14},
	key:      "helloworld!",
	tempSign: "12345678",
}

// 协议使用哈希值检验
type hashParser struct {
	ref      []int
	key      string
	tempSign string
}

func (parser *hashParser) Encode(pkg *Package) ([]byte, error) {
	if pkg.Body != nil {
		data, err := marshalJSON(pkg.Body)
		if err != nil {
			return nil, err
		}
		pkg.Data = data
	}

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
	if ts := pkg.ExpireTs; ts > 0 && ts < time.Now().Unix() {
		return nil, errPackageExpire
	}

	sign, err := parser.Signature(buf)
	if err != nil {
		return pkg, ErrInvalidSign
	}
	if sign != "" && pkg.Sign != sign {
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
	copy(buf, key)
	copy(buf[len(key):], data)
	// buf = append([]byte(key), data...)
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
	copy(data[n-signLen:n], sign)
	return sign, nil
}

func Encode(name string, i interface{}) ([]byte, error) {
	buf, _ := marshalJSON(i)
	pkg := &Package{Id: name, Data: buf}
	return clientParser.Encode(pkg)
}

func Decode(buf []byte) (*Package, error) {
	return clientParser.Decode(buf)
}

func EncodePackage(pkg *Package) ([]byte, error) {
	return rawParser.Encode(pkg)
}

func marshalJSON(i interface{}) ([]byte, error) {
	switch v := i.(type) {
	case []byte:
		return v, nil
	case string:
		return []byte(v), nil
	}
	return json.Marshal(i)
}

func splitMsgId(msgId string) (string, string) {
	var serverName string
	if subs := strings.SplitN(msgId, ".", 2); len(subs) > 1 {
		serverName, msgId = subs[0], subs[1]
	}
	return serverName, msgId
}

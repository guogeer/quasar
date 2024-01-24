package cmd

import (
	"encoding/json"
	"net"
	"reflect"
	"runtime"
	"strings"
	"time"

	"quasar/config"
)

var (
	enableDebug       = false
	defaultRouterAddr = "127.0.0.1:9003"
)

func init() {
	conf := config.Config()
	// 服务器内部数据校验KEY
	if conf.ProductKey != "" {
		authCodec.key = conf.ProductKey
		clientCodec.key = conf.ProductKey
	}
	if conf.EnableDebug {
		enableDebug = true
	}
	if addr := conf.Server("router").Addr; addr != "" {
		defaultRouterAddr = addr
	}
}

func Bind(name string, h Handler, args any) wrapper {
	return defaultCmdSet.Bind(name, h, args)
}

func Hook(h Handler) {
	defaultCmdSet.Hook(h)
}

// 绑定，函数名作为消息ID
// 注：客户端发送的消息ID仅允许包含字母、数字
func BindFunc(h Handler, args any) wrapper {
	name := runtime.FuncForPC(reflect.ValueOf(h).Pointer()).Name()
	n := strings.LastIndexByte(name, '.')
	if n >= 0 {
		name = name[n+1:]
	}
	// log.Debug("method name =", name)
	return Bind(name, h, args)
}

func Handle(ctx *Context, name string, data []byte) error {
	return defaultCmdSet.Handle(ctx, name, data)
}

type ServiceConfig struct {
	Id        string `json:"id,omitempty"`        // 服务ID。可为空
	Name      string `json:"name,omitempty"`      // 服务名。存在多个时采用逗号,隔开
	Addr      string `json:"addr,omitempty"`      // 地址
	MinWeight int    `json:"minWeight,omitempty"` // 最小的负载
	MaxWeight int    `json:"maxWeight,omitempty"` // 最大的负载
}

type cmdArgs struct {
	Name string `json:"name,omitempty"`
	Addr string `json:"addr,omitempty"`
}

// 忽略nil类型nil/slice/pointer
type M map[string]any

// 忽略空值nil
func (m M) MarshalJSON() ([]byte, error) {
	copyM := map[string]any{}
	for k, v := range m {
		if v != nil {
			switch ref := reflect.ValueOf(v); ref.Kind() {
			case reflect.Ptr, reflect.Slice, reflect.Map, reflect.Interface:
				if !ref.IsNil() {
					copyM[k] = v
				}
			default:
				copyM[k] = v
			}
		}
	}
	return json.Marshal(copyM)
}

type forwardArgs struct {
	ServerName string
	MsgId      string
	MsgData    json.RawMessage
}

// 消息通过router转发
// name = "*"：向所有非网关服务转发消息
func Forward(name string, msgId string, i any) {
	buf, err := marshalJSON(i)
	if err != nil {
		return
	}

	args := &forwardArgs{
		ServerName: name,
		MsgId:      msgId,
		MsgData:    buf,
	}
	Route("router", "C2S_Route", args)
}

// 同步请求
func Request(serverName, msgId string, in any) ([]byte, error) {
	addr, err := RequestServerAddr(serverName)
	if err != nil {
		return nil, err
	}
	rwc, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	defer rwc.Close()

	c := &TCPConn{rwc: rwc}
	buf, err := authCodec.Encode(&Package{Id: msgId, Body: in, Ts: time.Now().Unix()})
	if err != nil {
		return nil, err
	}
	if _, err := c.writeMsg(RawMessage, buf); err != nil {
		return nil, err
	}

	_, buf, err = c.ReadMessage()
	if err != nil {
		return nil, err
	}
	pkg, err := rawCodec.Decode(buf)
	if err != nil {
		return nil, err
	}
	return pkg.Data, nil
}

// 向路由请求服务器地址
func RequestServerAddr(name string) (string, error) {
	if name == "router" {
		return defaultRouterAddr, nil
	}

	req := cmdArgs{Name: name}
	buf, err := Request("router", "C2S_GetServerAddr", req)
	if err != nil {
		return "", err
	}
	args := &cmdArgs{}
	if err := json.Unmarshal(buf, args); err != nil {
		return "", err
	}
	return args.Addr, nil
}

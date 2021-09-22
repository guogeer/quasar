package cmd

import (
	"encoding/json"
	"net"
	"reflect"
	"runtime"
	"strings"

	"github.com/guogeer/quasar/config"
	"github.com/guogeer/quasar/internal"
)

var (
	enableDebug       = false
	defaultRouterAddr = "127.0.0.1:9003"
)

func init() {
	conf := config.Config()
	// 服务器内部数据校验KEY
	if conf.ProductKey != "" {
		authParser.key = conf.ProductKey
		clientParser.key = conf.ProductKey
	}
	if conf.EnableDebug {
		enableDebug = true
	}
	if addr := conf.Server("router").Addr; addr != "" {
		defaultRouterAddr = addr
	}
}

func BindWithName(name string, h Handler, args interface{}) {
	defaultCmdSet.Bind(name, h, args, true)
}

func Hook(h Handler) {
	defaultCmdSet.Hook(h)
}

// 消息不入队列直接处理
func BindWithoutQueue(name string, h Handler, args interface{}) {
	defaultCmdSet.Bind(name, h, args, false)
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

func Handle(ctx *Context, name string, data []byte) error {
	return defaultCmdSet.Handle(ctx, name, data)
}

type ServiceConfig struct {
	Id        string `json:",omitempty"` // 服务ID。可为空
	Name      string `json:",omitempty"` // 服务名。存在多个时采用逗号,隔开
	Addr      string `json:",omitempty"` // 地址
	MinWeight int    `json:",omitempty"` // 最小的负载
	MaxWeight int    `json:",omitempty"` // 最大的负载
}

type cmdArgs struct {
	Name string
	Addr string
}

// 消息通过router转发
// name = "*"：向所有非网关服务转发消息
func Forward(name string, msgId string, i interface{}) {
	buf, err := marshalJSON(i)
	if err != nil {
		return
	}

	args := &internal.ForwardArgs{
		ServerName: name,
		MsgId:      msgId,
		MsgData:    buf,
	}
	Route("router", "C2S_Route", args)
}

// 同步请求
func Request(serverName, msgId string, in interface{}) ([]byte, error) {
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
	buf, err := authParser.Encode(&Package{Id: msgId, Body: in})
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
	pkg, err := rawParser.Decode(buf)
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

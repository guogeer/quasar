package cmd

import (
	"encoding/json"
	"errors"
	"net"
	"reflect"
	"runtime"
	"strings"

	"github.com/guogeer/quasar/config"
)

var (
	enableDebug       = false
	defaultRouterAddr = "127.0.0.1:9003"

	errInvalidAddr = errors.New("request empty address")
)

func init() {
	cfg := config.Config()
	sign, productKey := cfg.Sign, cfg.ProductKey
	// 服务器内部数据校验KEY
	if h := defaultAuthParser; sign != "" {
		h.key = sign
	}
	// 客户端与服务器数据校验KEY
	if h := defaultHashParser; productKey != "" {
		h.key = productKey
	}
	defaultRawParser.compressPackage = cfg.CompressPackage
	if cfg.EnableDebug {
		enableDebug = true
	}
	if addr := config.Config().Server("router").Addr; addr != "" {
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

func Route(serverName, messageId string, data interface{}) {
	defaultClientManage.Route3(serverName, messageId, data)
}

func RegisterService(config *ServiceConfig) {
	defaultClientManage.RegisterService(config)
}

type ServiceConfig struct {
	ServerName string      `json:",omitempty"`
	ServerAddr string      `json:",omitempty"`
	ServerData interface{} `json:",omitempty"`
	ServerType string      `json:",omitempty"` // center,gateway etc
	IsRandPort bool        `json:",omitempty"` // Deprecated: 服务合并后指定端口，不再需要随机端口
	ServerList []string    `json:",omitempty"`
	MinWeight  int         `json:",omitempty"` // 最小的负载
	MaxWeight  int         `json:",omitempty"` // 最大的负载
}

type cmdArgs ServiceConfig

type ForwardArgs struct {
	ServerList []string
	Name       string
	Data       json.RawMessage
}

// 消息通过router转发
func Forward(servers interface{}, messageId string, i interface{}) {
	buf, err := marshalJSON(i)
	if err != nil {
		return
	}

	var serverList []string
	switch v := servers.(type) {
	case string:
		serverList = []string{v}
	case []string:
		serverList = v
	}
	if len(serverList) == 0 {
		return
	}

	args := &ForwardArgs{
		ServerList: serverList,
		Name:       messageId,
		Data:       buf,
	}
	Route("router", "C2S_Route", args)
}

// 同步请求
func Request(serverName, msgId string, in interface{}) ([]byte, error) {
	var addr string
	if serverName == "router" {
		addr = defaultRouterAddr
	} else {
		addr, _ = RequestServerAddr(serverName)
	}
	if addr == "" {
		return nil, errInvalidAddr
	}
	rwc, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	defer rwc.Close()

	c := &TCPConn{rwc: rwc}
	// 第一个包发送校验数据
	firstPackage, _ := defaultAuthParser.Encode(&Package{})
	if _, err := c.writeMsg(AuthMessage, firstPackage); err != nil {
		return nil, err
	}
	req := &Package{Id: msgId, Body: in}
	buf, err := defaultRawParser.Encode(req)
	if err != nil {
		return nil, err
	}
	if _, err := c.writeMsg(RawMessage, buf); err != nil {
		return nil, err
	}

	// read message, ignore heart beat message
	for i := 0; i < 8; i++ {
		mt, buf, err := c.ReadMessage()
		if err != nil {
			return nil, err
		}
		if mt == RawMessage {
			pkg, err := defaultRawParser.Decode(buf)
			if err != nil {
				return nil, err
			}
			return pkg.Data, err
		}
	}
	return nil, errors.New("unkown error")
}

// 向路由请求服务器地址
func RequestServerAddr(name string) (string, error) {
	if name == "router" {
		return defaultRouterAddr, nil
	}

	req := cmdArgs{ServerName: name}
	buf, err := Request("router", "C2S_GetServerAddr", req)
	if err != nil {
		return "", err
	}
	args := &cmdArgs{}
	if err := json.Unmarshal(buf, args); err != nil {
		return "", err
	}
	return args.ServerAddr, nil
}

package cmd

import (
	"encoding/json"
	"errors"
	"github.com/guogeer/husky/config"
	"net"
	"reflect"
	"runtime"
	"strings"
)

var defaultRouter string

func init() {
	defaultRouter = config.Config().Server("router").Addr
	if defaultRouter == "" {
		panic("NOTE router address is empty")
	}

	// 服务器内部数据校验KEY
	if h, ok := defaultAuthParser.(*hashParser); ok {
		h.key = config.Config().Sign
	}
	// 客户端与服务器数据校验KEY
	if h, ok := defaultHashParser.(*hashParser); ok {
		h.key = config.Config().ProductKey
	}

	BindWithName("C2S_RegisterOk", funcRegisterOk, (*cmdArgs)(nil))

	// 某些情况下需要发送一个包去探路，这个包可能会发送失败
	BindWithName("FUNC_Test", funcTest, (*cmdArgs)(nil))
	// 断线后自动重连
	BindWithName("CMD_AutoConnect", funcAutoConnect, (*cmdArgs)(nil))
	BindWithName("CMD_Close", funcClose, (*cmdArgs)(nil))
}

func BindWithName(name string, h Handler, args interface{}) {
	defaultCmdSet.Bind(name, h, args)
}

func RegisterServiceInGateway(name string) {
	defaultCmdSet.RegisterService(name)
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

func Handle(ctx *Context, name string, args interface{}) {
	b, err := MarshalJSON(args)
	if err != nil {
		return
	}
	defaultCmdSet.Handle(ctx, name, b)
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
}

type cmdArgs ServiceConfig

type ForwardArgs struct {
	ServerList []string
	Name       string
	Data       json.RawMessage
}

// 消息通过router转发
func Forward(servers interface{}, messageId string, i interface{}) {
	buf, err := MarshalJSON(i)
	if err != nil {
		return
	}

	var serverList []string
	switch servers.(type) {
	case string:
		serverList = []string{servers.(string)}
	case []string:
		serverList = servers.([]string)
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

// TODO 当前仅支持router
func Request(serverName string, messageId string, in interface{}) ([]byte, error) {
	addr := defaultRouter
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
	data, err := MarshalJSON(in)
	if err != nil {
		return nil, err
	}
	req := &Package{Id: messageId, Data: data}
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

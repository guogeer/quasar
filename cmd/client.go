package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/guogeer/husky/env"
	"github.com/guogeer/husky/log"
	"github.com/guogeer/husky/util"
	"net"
	"strings"
	"sync"
	"time"
)

type Client struct {
	name string
	*TCPConn

	registerArgs interface{}
}

func NewClient(name string) *Client {
	client := &Client{
		name: name,
		TCPConn: &TCPConn{
			send: make(chan []byte, sendQueueSize),
		},
	}
	return client
}

func (c *Client) ServerName() string {
	return c.name
}

func (c *Client) start() {
	doneCtx, cancel := context.WithCancel(context.Background())
	go func() {
		defer func() {
			c.rwc.Close() // 关闭连接

			// 关闭后，自动重连，并消息通知
			ctx := &Context{Ssid: c.ssid, Out: c}
			Enqueue(ctx, closeClient, nil)
			GetCmdSet().Handle(&Context{Out: c}, "FUNC_ServerClose", nil)
		}()

		// 第一个包发送校验数据
		firstPackage, err := gAuthParser.Encode(&Package{})
		if err != nil {
			return
		}
		if _, err := c.rwc.Write(firstPackage); err != nil {
			return
		}
		for {
			select {
			case buf, ok := <-c.send:
				if ok == false {
					return
				}
				if _, err := c.rwc.Write(buf); err != nil {
					log.Debugf("write %v", err)
					return
				}
			case <-doneCtx.Done():
				return
			}
		}
	}()

	// 读关闭通知
	defer cancel()
	for {
		// read message head
		mt, buf, err := c.ReadMessage()
		if err != nil {
			log.Debugf("read %v", err)
			return
		}
		switch mt {
		case PingMessage:
			c.TCPConn.writeMessage([]byte{PongMessage, 0x00, 0x00})
		case PongMessage:
		case RawMessage:
			// log.Info("read", string(buf[:n]))
			pkg, err := gRawParser.Decode(buf)
			if err != nil {
				return
			}

			id, ssid, data := pkg.Id, pkg.Ssid, pkg.Data
			err = GetCmdSet().Handle(&Context{Out: c, Ssid: ssid}, id, data)
			if err != nil {
				log.Infof("handle message[%s] %v", id, err)
			}
		}
	}
}

type ClientManage struct {
	clients map[string]*Client
	mu      sync.RWMutex
}

var defaultClientManage = &ClientManage{clients: make(map[string]*Client)}

func GetClientManage() *ClientManage {
	return defaultClientManage
}

func (cm *ClientManage) Route(serverName string, data []byte) {
	// 已存在的连接不会被删除
	cm.mu.RLock()
	client, ok := cm.clients[serverName]
	cm.mu.RUnlock()

	if ok == false {
		cm.mu.Lock()
		if _, ok2 := cm.clients[serverName]; !ok2 {
			client = NewClient(serverName)
			cm.clients[serverName] = client
		}
		client = cm.clients[serverName]
		cm.mu.Unlock()
		cm.queryServerAddr(serverName)
	}

	if err := client.Write(data); err != nil {
		log.Errorf("route %s data %d error: %v", serverName, len(data), err)
	}
}

func (cm *ClientManage) queryServerAddr(serverName string) {
	if serverName == ServerRouter {
		addr := env.Config().Server(ServerRouter).Addr
		cm.Dial(serverName, addr)
	} else if len(serverName) > 0 {
		data := map[string]interface{}{"ServerName": serverName}
		cm.Route3(ServerRouter, "C2S_GetServerAddr", data)
	}
}

func (cm *ClientManage) Dial(serverName, serverAddr string) {
	cm.mu.RLock()
	c := cm.clients[serverName]
	cm.mu.RUnlock()

	go func() {
		try := 0
		for {
			rwc, err := net.Dial("tcp", serverAddr)
			if err == nil {
				c.rwc = rwc
				break
			}

			// 重新查询地址
			if try > 10 {
				Enqueue(&Context{Out: c}, closeClient, nil)
				return

			}
			// 间隔时间
			var ms int
			var intervals = []int{100, 400, 1600, 3200, 5000}
			if n := len(intervals); try < n {
				ms = intervals[try]
			} else {
				ms = intervals[n-1]
			}
			log.Infof("connect %v, retry %d after %dms", err, try, ms)
			time.Sleep(time.Duration(ms) * time.Millisecond)
			try++
		}

		GetCmdSet().RecoverService(c.ServerName()) // 恢复服务
		c.start()
	}()
}

func (cm *ClientManage) Route3(serverName, messageId string, i interface{}) {
	data, err := MarshalJSON(i)
	if err != nil {
		return
	}
	if len(serverName) > 0 {
		messageId = serverName + "." + messageId
	}
	if subs := strings.SplitN(messageId, ".", 2); len(subs) > 1 {
		serverName, messageId = subs[0], subs[1]
	}

	msg, err := json.Marshal(&Package{Id: messageId, Data: data})
	if err != nil {
		return
	}
	cm.Route(serverName, msg)
}

func (cm *ClientManage) RegisterService(args *ServiceConfig) {
	cm.Route3(ServerRouter, "C2S_Register", args)
	cm.mu.Lock()
	client := cm.clients[ServerRouter]
	client.registerArgs = args
	cm.mu.Unlock()
}

func Route(serverName, messageId string, data interface{}) {
	GetClientManage().Route3(serverName, messageId, data)
}

func RegisterService(config *ServiceConfig) {
	GetClientManage().RegisterService(config)
}

type ServiceConfig struct {
	ServerName string
	ServerAddr string
	ServerType string // center,gateway etc
	ServerData interface{}
}

type Args struct {
	ServerName string
	ServerAddr string
	ServerData interface{}
}

func init() {
	Bind(S2C_GetServerAddr, (*Args)(nil))
	Bind(C2S_RegisterOk, (*Args)(nil))

	Bind(FUNC_Test, (*Args)(nil))
}

func S2C_GetServerAddr(ctx *Context, iArgs interface{}) {
	args := iArgs.(*Args)
	addr := args.ServerAddr
	name := args.ServerName
	if _, _, err := net.SplitHostPort(addr); err != nil {
		log.Warnf("server %s not exist", name)
		// 5s retry
		util.NewTimer(func() { GetClientManage().queryServerAddr(name) }, 5*time.Second)
		return
	}
	GetClientManage().Dial(name, addr)
}

func FUNC_Test(ctx *Context, iArgs interface{}) {
	// empty
}

func C2S_RegisterOk(ctx *Context, iArgs interface{}) {
	// TODO
}

// 触发客户端重连
func closeClient(ctx *Context, iArgs interface{}) {
	client := ctx.Out.(*Client)
	// ctx.Out.Close()

	cm := GetClientManage()
	name := client.ServerName()
	registerArgs := client.registerArgs

	client2 := &Client{
		name: name,
		TCPConn: &TCPConn{
			send: client.send,
		},
	}

	client2.registerArgs = registerArgs
	GetCmdSet().RemoveService(name)

	cm.mu.Lock()
	cm.clients[name] = client2
	cm.mu.Unlock()

	if registerArgs != nil && name == ServerRouter {
		cm.Route3(name, "C2S_Register", registerArgs)
	}
	GetClientManage().queryServerAddr(name)
}

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
func Request(serverName string, messageId string, i interface{}) (interface{}, error) {
	addr := env.Config().Server(serverName).Addr
	rwc, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	defer rwc.Close()

	c := &TCPConn{rwc: rwc}
	// 第一个包发送校验数据
	firstPackage, err := gAuthParser.Encode(&Package{})
	if _, err := c.rwc.Write(firstPackage); err != nil {
		return nil, err
	}
	data, err := MarshalJSON(i)
	if err != nil {
		return nil, err
	}
	buf, err := gRawParser.Encode(&Package{Id: messageId, Data: data})
	if err != nil {
		return nil, err
	}
	if _, err := c.rwc.Write(buf); err != nil {
		return nil, err
	}

	// read message, ignore heart beat message
	for i := 0; i < 8; i++ {
		mt, buf, err := c.ReadMessage()
		if err != nil {
			return nil, err
		}
		if mt == RawMessage {
			pkg, err := gRawParser.Decode(buf)
			if err != nil {
				return nil, err
			}

			_, args, err := GetCmdSet().Parse(pkg.Id, pkg.Data)
			return args, err
		}
	}
	return nil, errors.New("unkown error")
}

// 向路由请求服务器地址
func RequestServerAddr(name string) (string, error) {
	data := map[string]interface{}{"ServerName": name}
	i, err := Request(ServerRouter, "C2S_GetServerAddr", data)
	if err != nil {
		log.Error(err)
		return "", err
	}

	args := i.(*Args)
	return args.ServerAddr, nil
}

package cmd

import (
	"context"
	"github.com/guogeer/quasar/log"
	"net"
	"sync"
	"time"
)

type Client struct {
	name string
	*TCPConn

	reg interface{}
}

func newClient(name string) *Client {
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
	defaultCmdSet.RecoverService(c.name) // 恢复服务

	doneCtx, cancel := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(pingPeriod)
		defer func() {
			ticker.Stop() // 关闭定时器
			c.rwc.Close() // 关闭连接

			// 关闭后，自动重连，并消息通知
			defaultCmdSet.Handle(&Context{Out: c}, "CMD_AutoConnect", nil)
			defaultCmdSet.Handle(&Context{Out: c}, "FUNC_ServerClose", nil)
		}()

		// 第一个包发送校验数据
		firstPackage, err := defaultAuthParser.Encode(&Package{})
		if err != nil {
			return
		}
		if _, err := c.writeMsg(AuthMessage, firstPackage); err != nil {
			return
		}
		for {
			select {
			case buf, ok := <-c.send:
				if ok == false {
					return
				}
				if _, err := c.writeMsg(RawMessage, buf); err != nil {
					return
				}
			case <-ticker.C: // heart beat
				if _, err := c.writeMsg(PingMessage, nil); err != nil {
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
		case PingMessage, PongMessage:
		case RawMessage:
			// log.Info("read", string(buf[:n]))
			pkg, err := defaultRawParser.Decode(buf)
			if err != nil {
				return
			}

			id, ssid, data := pkg.Id, pkg.Ssid, pkg.Data
			err = defaultCmdSet.Handle(&Context{Out: c, Ssid: ssid}, id, data)
			if err != nil {
				log.Errorf("handle message[%s] %v", id, err)
			}
		}
	}
}

type clientManage struct {
	clients map[string]*Client // 已存在的连接不会被删除
	mu      sync.RWMutex
}

var defaultClientManage = &clientManage{
	clients: make(map[string]*Client),
}

func (cm *clientManage) Route(serverName string, data []byte) {
	if serverName == "" {
		return
	}

	cm.mu.RLock()
	client, ok := cm.clients[serverName]
	cm.mu.RUnlock()

	if ok == false {
		cm.mu.Lock()
		_, ok2 := cm.clients[serverName]
		if ok2 == false {
			client = newClient(serverName)
			cm.clients[serverName] = client
		}
		client = cm.clients[serverName]
		cm.mu.Unlock()
		// 防止重复连接
		if ok2 == false {
			cm.connect(serverName)
		}
	}

	if err := client.Write(data); err != nil {
		log.Errorf("route %s data %d error: %v", serverName, len(data), err)
	}
}

// 第一步向路由查询地址
// 第二步建立连接
func (cm *clientManage) connect(serverName string) {
	cm.mu.RLock()
	client := cm.clients[serverName]
	cm.mu.RUnlock()

	go func() {
		addr := defaultRouterAddr
		for try, ms := range []int{100, 400, 1600, 3200, 5000} {
			if serverName != "router" {
				addr2, err := RequestServerAddr(serverName)
				if err != nil {
					log.Errorf("connect %s %v", serverName, err)
				}
				addr = addr2
			}
			if addr == "" {
				continue
			}
			rwc, err := net.Dial("tcp", addr)
			if err == nil {
				client.rwc = rwc
				client.start()
				return
			}

			// 间隔时间
			log.Infof("connect %v, retry %d after %dms", err, try, ms)
			time.Sleep(time.Duration(ms) * time.Millisecond)
		}
		defaultCmdSet.Handle(&Context{Out: client}, "CMD_AutoConnect", nil)
	}()
}

func (cm *clientManage) Route3(serverName, messageId string, i interface{}) {
	serverName, messageId = routeMessage(serverName, messageId)
	msg, err := Encode(&Package{Id: messageId, Body: i, IsRaw: true})
	if err != nil {
		return
	}
	cm.Route(serverName, msg)
}

func (cm *clientManage) RegisterService(args *ServiceConfig) {
	cm.Route3(ServerRouter, "C2S_Register", args)
	cm.mu.Lock()
	client := cm.clients["router"]
	client.reg = args
	cm.mu.Unlock()
}

func funcTest(ctx *Context, iArgs interface{}) {
	// empty
}

func funcRegisterOk(ctx *Context, iArgs interface{}) {
	// TODO
}

// Client自动重连
func funcAutoConnect(ctx *Context, iArgs interface{}) {
	client := ctx.Out.(*Client)
	// ctx.Out.Close()

	cm := defaultClientManage
	reg, name := client.reg, client.name
	defaultCmdSet.RemoveService(name)
	if reg != nil && name == ServerRouter {
		cm.Route3(name, "C2S_Register", reg)
	}
	cm.connect(name)
}

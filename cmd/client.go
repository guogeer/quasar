package cmd

import (
	"context"
	"net"
	"sync"
	"time"

	"quasar/log"
)

var clients sync.Map // 已存在的连接不会被删除

type Client struct {
	TCPConn

	serverId string
	conf     ServiceConfig // 向路由注册的参数
}

func newClient(serverId string) *Client {
	client := &Client{
		serverId: serverId,
		TCPConn: TCPConn{
			send: make(chan []byte, sendQueueSize),
		},
	}
	return client
}

func (client *Client) connect() {
	serverId := client.serverId

	internalMillis := []int{100, 400, 1600, 3200, 5000}
	for retry := 0; true; retry++ {
		// 间隔时间
		ms := internalMillis[len(internalMillis)-1]
		if retry < len(internalMillis) {
			ms = internalMillis[retry]
		}

		// 第一步向路由查询地址
		addr, err := RequestServerAddr(serverId)
		if err != nil {
			log.Infof("connect %s %v", serverId, err)
		}

		// 第二步建立连接
		if addr != "" {
			rwc, err := net.Dial("tcp", addr)
			if err == nil {
				client.rwc = rwc
				break
			}
		}
		// 断线后等待一定时候后再重连
		time.Sleep(time.Duration(ms) * time.Millisecond)
		log.Debugf("connect server %s, retry %d after %dms", serverId, retry, ms)
	}
	client.start()
}

func (c *Client) start() {
	doneCtx, cancel := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(pingPeriod)
		defer func() {
			ticker.Stop() // 关闭定时器
			c.rwc.Close() // 关闭连接

			// 关闭后，自动重连，并消息通知
			c.autoConnect()
		}()

		// 第一个包发送校验数据
		pkg := &Package{
			Ts: time.Now().Unix(),
		}
		firstMsg, _ := authParser.Encode(pkg)
		if _, err := c.writeMsg(RawMessage, firstMsg); err != nil {
			return
		}
		for {
			select {
			case buf, ok := <-c.send:
				if !ok {
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
			log.Debug(err)
			return
		}
		if mt == RawMessage {
			pkg, err := rawParser.Decode(buf)
			if err != nil {
				log.Debug(err)
				return
			}

			id, ssid, data := pkg.Id, pkg.Ssid, pkg.Data
			err = defaultCmdSet.Handle(&Context{Out: c, Ssid: ssid}, id, data)
			if err != nil {
				log.Debugf("handle message[%s] %v", id, err)
			}
		}
	}
}

func routeMsg(serverId string, data []byte) {
	if serverId == "" {
		panic("route empty server")
	}

	client, ok := clients.Load(serverId)
	if !ok {
		newClient := newClient(serverId)
		client, ok = clients.LoadOrStore(serverId, newClient)
		// 防止重复连接
		if ok {
			close(newClient.send)
		} else {
			go func() {
				client.(*Client).connect()
			}()
		}
	}

	if err := client.(*Client).Write(data); err != nil {
		log.Errorf("server %s write %s error: %v", serverId, data, err)
	}
}

func Route(serverId, msgId string, i any) {
	pkg := &Package{Id: msgId, Body: i}
	buf, err := EncodePackage(pkg)
	if err != nil {
		return
	}
	routeMsg(serverId, buf)
}

// 向router注册服务
func RegisterService(conf *ServiceConfig) {
	if conf.Id == "" {
		conf.Id = conf.Name
	}
	if conf.Id == "" {
		panic("empty server id")
	}
	Route("router", "C2S_Register", conf)

	client, _ := clients.Load("router")
	client.(*Client).conf = *conf
}

// Client自动重连
func (client *Client) autoConnect() {
	if client.serverId == "router" {
		RegisterService(&client.conf)
	}
	go func() {
		client.connect()
	}()
}

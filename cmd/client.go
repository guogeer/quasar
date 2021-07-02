package cmd

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/guogeer/quasar/log"
)

type Client struct {
	*TCPConn

	name   string
	params interface{} // 连接成功后发送的第一个请求
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

func (client *Client) connect() {
	serverName := client.name
	go func() {
		intervals := []int{100, 400, 1600, 3200, 5000}
		for retry := 0; true; retry++ {
			// 间隔时间
			ms := intervals[len(intervals)-1]
			if retry < len(intervals) {
				ms = intervals[retry]
			}
			// 断线后等待一定时候后再重连
			time.Sleep(time.Duration(ms) * time.Millisecond)

			// 第一步向路由查询地址
			addr, err := RequestServerAddr(serverName)
			if err != nil {
				log.Errorf("connect %s %v", serverName, err)
			}

			// 第二步建立连接
			if addr != "" {
				rwc, err := net.Dial("tcp", addr)
				if err == nil {
					client.rwc = rwc
					break
				}
			}
			log.Infof("connect server %s, retry %d after %dms", serverName, retry, ms)
		}
		client.start()
	}()
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
			defaultCmdSet.Handle(&Context{Out: c}, "FUNC_ServerClose", nil)
		}()

		// 第一个包发送校验数据
		pkg := &Package{
			SignType: "md5",
			ExpireTs: time.Now().Add(5 * time.Second).Unix(),
		}
		firstMsg, _ := pkg.Encode()
		if _, err := c.writeMsg(AuthMessage, firstMsg); err != nil {
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
			log.Debugf("read %v", err)
			return
		}
		if mt == RawMessage {
			pkg, err := defaultRawParser.Decode(buf)
			if err != nil {
				return
			}

			id, ssid, data := pkg.Id, pkg.Ssid, pkg.Data
			err = defaultCmdSet.Handle(&Context{Out: c, Ssid: ssid}, id, data)
			if err != nil {
				log.Debugf("handle message[%s] %v", id, err)
			}
		}
		saveBuf(buf)
	}
}

var (
	clients  = map[string]*Client{} // 已存在的连接不会被删除
	clientMu sync.RWMutex
)

func routeMsg(serverName string, data []byte) {
	if serverName == "" {
		panic("route empty server name")
	}

	clientMu.RLock()
	client, ok := clients[serverName]
	clientMu.RUnlock()

	if !ok {
		clientMu.Lock()
		_, rok := clients[serverName]
		if !rok {
			clients[serverName] = newClient(serverName)
		}
		client = clients[serverName]
		clientMu.Unlock()
		// 防止重复连接
		if !rok {
			client.connect()
		}
	}

	if err := client.Write(data); err != nil {
		log.Errorf("server %s write %s error: %v", serverName, data, err)
	}
}

func Route(serverName, messageId string, i interface{}) {
	serverName, messageId = splitMsgId(serverName + "." + messageId)

	pkg := &Package{Id: messageId, Body: i}
	msg, err := pkg.Encode()
	if err != nil {
		return
	}
	routeMsg(serverName, msg)
}

func RegisterService(params *ServiceConfig) {
	Route("router", "C2S_Register", params)
	clientMu.Lock()
	client := clients["router"]
	client.params = params
	clientMu.Unlock()
}

// Client自动重连
func (client *Client) autoConnect() {
	params, name := client.params, client.name
	if params != nil && name == "router" {
		Route(name, "C2S_Register", params)
	}
	client.connect()
}

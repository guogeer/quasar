package main

// 网关

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"quasar/cmd"
	"quasar/log"
	"quasar/utils"

	"github.com/gorilla/websocket"
)

const (
	clientPackageSpeedPer2s = 96 // 2 second

	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 96 << 10 // 96K
	sendQueueSize  = 16 << 10
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type WsConn struct {
	ws      *websocket.Conn
	ssid    string
	send    chan []byte
	isClose bool
	mu      sync.RWMutex
}

func init() {
	http.HandleFunc("/ws", serveWs)
}

func (c *WsConn) RemoteAddr() string {
	return c.ws.RemoteAddr().String()
}

func (c *WsConn) Close() {
	c.ws.Close()

	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.isClose {
		c.isClose = true
		close(c.send)
	}
}

func (c *WsConn) WriteJSON(name string, i any) error {
	// 消息格式
	pkg := &cmd.Package{Id: name, Body: i}
	buf, err := cmd.EncodePackage(pkg)
	if err != nil {
		return err
	}
	return c.Write(buf)
}

func (c *WsConn) Write(data []byte) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.isClose {
		return errors.New("write to closed chan")
	}

	c.send <- data
	return nil
}

func (c *WsConn) writeMessage(mt int, payload []byte) error {
	c.ws.SetWriteDeadline(time.Now().Add(writeWait))
	return c.ws.WriteMessage(mt, payload)
}

func serveWs(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	ssid := utils.GUID()
	c := &WsConn{
		ssid: ssid,
		ws:   ws,
		send: make(chan []byte, 1<<10),
	}
	cmd.AddSession(&cmd.Session{Id: ssid, Out: c})

	doneCtx, cancel := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(pingPeriod)
		defer func() {
			c.Close()
			ticker.Stop() // 关闭定时器
			cmd.RemoveSession(c.ssid)

			ctx := &cmd.Context{Ssid: c.ssid, Out: c}
			cmd.Handle(ctx, "FUNC_Close", nil)
		}()

		for {
			select {
			case buf, ok := <-c.send:
				if !ok {
					return
				}
				if err := c.writeMessage(websocket.TextMessage, buf); err != nil {
					log.Debug("write message", err)
					return
				}
			case <-ticker.C:
				if err := c.writeMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			case <-doneCtx.Done():
				return
			}
		}
	}()

	c.ws.SetReadLimit(4 << 10)
	c.ws.SetReadDeadline(time.Now().Add(pongWait))
	c.ws.SetPongHandler(func(string) error {
		c.ws.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	defer cancel()

	var deadline time.Time
	var recvPackageCounter int
	var oldServer, oldMatchServer string

	remoteAddr := c.ws.RemoteAddr().String()
	matchMsg, _ := regexp.Compile("^[A-Za-z0-9]+$")
	for {
		_, message, err := c.ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
				log.Debugf("websocket close, %v", err)
			}
			return
		}

		pkg, err := cmd.Decode(message)
		if err != nil {
			log.Warn(err)
			return
		}

		// 网络限流
		recvPackageCounter++
		if recvPackageCounter >= clientPackageSpeedPer2s {
			recvPackageCounter = 0
			if time.Now().Before(deadline) {
				log.Errorf("client %s send %s too busy", remoteAddr, pkg.Id)
				time.Sleep(5 * time.Second)
				return // 消息发送过快，直接关闭链接
			}
			deadline = time.Now().Add(2 * time.Second)
		}

		// 网关转发的消息ID仅允许包含字母、数字
		var serverName, matchServer string
		if servers := strings.SplitN(pkg.Id, ".", 2); len(servers) > 1 {
			serverName = servers[0]
			if !matchMsg.MatchString(servers[1]) {
				log.Warnf("invalid message id %s", pkg.Id)
				continue
			}
			matchServer = oldMatchServer
			// 请求的新服务
			if serverName != oldServer {
				matchServer = matchBestServer(c.ssid, serverName)
				if matchServer != serverName && matchServer != "" {
					oldServer, oldMatchServer = serverName, matchServer
				}
			}
			// log.Debugf("serverName:%s matchServer:%s oldServer:%s oldMatchServer:%s", serverName, matchServer, oldServer, oldMatchServer)
			// 服务有效
			var isAlive bool
			if matchServer != "" {
				serverStateMu.RLock()
				if _, ok := serverStates[matchServer]; ok {
					isAlive = true
				}
				serverStateMu.RUnlock()
			}

			// 无效的服务
			if !isAlive {
				c.WriteJSON("serverClose", cmd.M{"serverId": servers[0], "cause": "not alive"})
				continue
			}
		}

		ctx := &cmd.Context{
			Out:         c,
			Ssid:        c.ssid,
			ClientAddr:  c.RemoteAddr(),
			MatchServer: matchServer,
			ServerName:  serverName,
		}
		if err := cmd.Handle(ctx, pkg.Id, pkg.Data); err != nil {
			log.Warnf("handle client %s %v", remoteAddr, err)
		}
	}
}

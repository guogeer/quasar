// 2019-07-17 支持大协议数据压缩

package gateway

// 网关

import (
	"context"
	"math/rand"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/guogeer/quasar/cmd"
	"github.com/guogeer/quasar/log"
	"github.com/guogeer/quasar/util"
)

const (
	clientPackageSpeedPer2s = 512 // 2 second

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
	ws   *websocket.Conn
	ssid string
	send chan []byte
	// args       interface{}
	isClose bool
}

func init() {
	http.HandleFunc("/ws", serveWs)
}

func (c *WsConn) RemoteAddr() string {
	return c.ws.RemoteAddr().String()
}

func (c *WsConn) Close() {
	if c.isClose {
		return
	}

	c.isClose = true
	close(c.send)
}

func (c *WsConn) WriteJSON(name string, i interface{}) error {
	// 消息格式
	pkg := &cmd.Package{Id: name, Body: i, IsZip: true}
	buf, err := pkg.Encode()
	if err != nil {
		return err
	}
	return c.Write(buf)
}

func (c *WsConn) Write(data []byte) error {
	if c.isClose {
		return nil
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
		log.Errorf("%v", err)
		return
	}
	ssid := util.GUID()
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
			// c.writeMessage(websocket.CloseMessage, []byte{})
			c.ws.Close()
			ticker.Stop() // 关闭定时器

			ctx := &cmd.Context{Ssid: c.ssid, Out: c}
			cmd.Handle(ctx, "CMD_Close", nil)
			cmd.Handle(ctx, "FUNC_Close", nil)
			cmd.RemoveSession(c.ssid)
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
	c.ws.SetPongHandler(func(string) error { c.ws.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	defer cancel()

	var deadline time.Time
	var oldServer, oldMatchServer string

	recvPackageCounter := -1
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
		if recvPackageCounter == -1 && rand.Intn(7) == 0 {
			recvPackageCounter = 0
			deadline = time.Now().Add(2 * time.Second)
		}
		if recvPackageCounter >= 0 {
			recvPackageCounter++
			if time.Now().After(deadline) {
				recvPackageCounter = -1
			}
			if recvPackageCounter >= clientPackageSpeedPer2s {
				log.Errorf("client %s send %s too busy", remoteAddr, pkg.Id)
				time.Sleep(2 * time.Second)
			}
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
			if serverName != oldServer {
				matchServer = matchBestServer(c.ssid, serverName)
				if matchServer != serverName {
					oldMatchServer = matchServer
				}
			}
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
				c.WriteJSON("ServerClose", map[string]interface{}{"ServerName": serverName})
				time.Sleep(2 * time.Second)
				continue
			}
		}

		ctx := &cmd.Context{
			Out:         c,
			Ssid:        c.ssid,
			ClientAddr:  c.RemoteAddr(),
			MatchServer: matchServer,
			ToServer:    serverName,
		}
		if err := cmd.Handle(ctx, pkg.Id, pkg.Data); err != nil {
			log.Warnf("handle client %s %v", remoteAddr, err)
		}
	}
}

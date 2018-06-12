package cmd

// 网关

import (
	"context"
	"encoding/json"
	"github.com/gorilla/websocket"
	"github.com/guogeer/husky/log"
	"github.com/guogeer/husky/util"
	"net/http"
	"time"
)

const (
	clientPackageSpeedPerSecond = 64 // second
)

var (
	heartBeatMessage = []byte(`{"Id":"HeartBeat","Data":{}}`)
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

func (c *WsConn) RemoteAddr() string {
	return c.ws.RemoteAddr().String()
}

func (c *WsConn) Close() {
	if c.isClose == true {
		return
	}

	c.isClose = true
	close(c.send)
}

func (c *WsConn) WriteJSON(name string, i interface{}) {
	s, err := MarshalJSON(i)
	if err != nil {
		log.Debug(err)
		return
	}

	// 消息格式
	pkg := &Package{Id: name, Data: s}
	buf, err := json.Marshal(pkg)
	if err != nil {
		log.Debug(err)
		return
	}
	c.Write(buf)
}

func (c *WsConn) Write(data []byte) {
	if c.isClose {
		return
	}

	select {
	case c.send <- data:
	default:
		log.Error("write time out")
	}
}

func (c *WsConn) writeMessage(mt int, payload []byte) error {
	c.ws.SetWriteDeadline(time.Now().Add(writeWait))
	return c.ws.WriteMessage(mt, payload)
}

func ServeWs(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Errorf("%v", err)
		return
	}
	id := util.GUID()
	c := &WsConn{
		ssid: id,
		ws:   ws,
		send: make(chan []byte, 1<<10),
	}
	addSession(&Session{Id: id, Out: c})

	doneCtx, cancel := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(pingPeriod)
		defer func() {
			// c.writeMessage(websocket.CloseMessage, []byte{})
			c.ws.Close()
			ticker.Stop() // 关闭定时器

			Enqueue(&Context{Ssid: c.ssid, Out: c}, closeConn, nil)
			GetCmdSet().Handle(&Context{Ssid: c.ssid, Out: c}, "FUNC_Close", nil)
			removeSession(c.ssid)
		}()

		for {
			select {
			case buf, ok := <-c.send:
				if ok == false {
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

	var recvPackageCounter int
	var lastClearTime = time.Now()
	for {
		_, message, err := c.ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
				log.Infof("websocket close, %v", err)
			}
			return
		}

		pkg, err := Decode(message)
		if err != nil {
			log.Error(err)
			return
		}

		id, data := pkg.Id, pkg.Data

		recvPackageCounter++
		if recvPackageCounter >= clientPackageSpeedPerSecond {
			now := time.Now()
			if lastClearTime.Add(1 * time.Second).After(now) {
				log.Errorf("client %s send too busy", c.ws.RemoteAddr().String())
				return
			}
			lastClearTime = now
			recvPackageCounter = 0
		}
		// log.Info("read", c.ssid)
		err = GetCmdSet().Handle(&Context{Out: c, Ssid: c.ssid, isGateway: true}, id, data)
		if err != nil {
			log.Debugf("handle %v", err)
			return
		}
	}
}

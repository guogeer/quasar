// 2019-07-17 支持大协议数据压缩

package cmd

// 网关

import (
	"context"
	"errors"
	"github.com/gorilla/websocket"
	"github.com/guogeer/quasar/log"
	"github.com/guogeer/quasar/util"
	"math/rand"
	"net/http"
	"time"
)

const (
	clientPackageSpeedPer2s = 32 // 2 second
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

func (c *WsConn) WriteJSON(name string, i interface{}) error {
	// 消息格式
	pkg := &Package{Id: name, Body: i, IsZip: true}
	buf, err := defaultRawParser.Encode(pkg)
	if err != nil {
		return err
	}
	return c.Write(buf)
}

func (c *WsConn) Write(data []byte) error {
	if c.isClose {
		return nil
	}

	select {
	case c.send <- data:
		return nil
	}
	return errors.New("write too busy")
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

			ctx := &Context{Ssid: c.ssid, Out: c}
			defaultCmdSet.Handle(ctx, "CMD_Close", nil)
			defaultCmdSet.Handle(ctx, "FUNC_Close", nil)
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

	var deadline time.Time
	var recvPackageCounter = -1
	var remoteAddr = c.ws.RemoteAddr().String()
	for {
		_, message, err := c.ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
				log.Debugf("websocket close, %v", err)
			}
			return
		}

		pkg, err := Decode(message)
		if err != nil {
			log.Warn(err)
			return
		}

		id, data := pkg.Id, pkg.Data
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
				log.Errorf("client %s send %s too busy", remoteAddr, id)
				time.Sleep(2 * time.Second)
			}
		}
		// log.Info("read", c.ssid)
		ctx := &Context{Out: c, Ssid: c.ssid, isGateway: true}
		err = defaultCmdSet.Handle(ctx, id, data)
		if err != nil {
			log.Warnf("handle client %s %v", remoteAddr, err)
		}
	}
}

package cmd

import (
	"bytes"
	"encoding/json"
	"github.com/gorilla/websocket"
	"github.com/guogeer/husky/log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type testArgs struct {
	N int
	S string
}

var clientMsg = &testArgs{N: 100, S: "SEND"}
var serverMsg = &testArgs{N: 200, S: "RECV"}

func TestMain(m *testing.M) {
	log.SetLevelByTag("FATAL")
	m.Run()
}

func testEqual(a, b interface{}) bool {
	b1, _ := json.Marshal(a)
	b2, _ := json.Marshal(b)
	return bytes.Compare(b1, b2) == 0
}

func testEcho(ctx *Context, iArgs interface{}) {
	if !testEqual(iArgs, clientMsg) {
		panic("server handle invalid message")
	}
	ctx.Out.WriteJSON("Echo", serverMsg)
}

func TestRecvClientPackage(t *testing.T) {
	BindWithName("Echo", testEcho, (*testArgs)(nil))
	http.HandleFunc("/ws", ServeWs)
	srv := httptest.NewServer(nil)
	defer srv.Close()

	// srv.URL like http://127.0.0.1:port
	// url like ws://127.0.0.1:port/ws
	url := "ws" + srv.URL[4:] + "/ws"
	ws, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Error(err)
	}
	defer ws.Close()

	for counter := 0; counter < 100*clientPackageSpeedPer2s; counter++ {
		t1 := time.Now()
		b, _ := Encode2("Echo", clientMsg)
		ws.WriteMessage(websocket.TextMessage, b)
		waitAndRunOnce(1, 120*time.Second)

		_, buf, err := ws.ReadMessage()
		if err != nil {
			t.Error(err)
		}
		pkg, _ := Decode(buf)
		if !testEqual(json.RawMessage(pkg.Data), serverMsg) {
			panic("client recv invald message")
		}
		t2 := time.Now()
		if t1.Add(time.Second).Before(t2) {
			t.Log("Success")
			return
		}
	}
	t.Error("limit client request fail")
}

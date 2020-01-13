package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/guogeer/quasar/log"
	"github.com/guogeer/quasar/util"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type testArgs struct {
	N int
	S string
}

var bigPackage []byte
var clientMsg = &testArgs{N: 100, S: "SEND"}
var serverMsg = &testArgs{N: 200, S: "RECV"}

func TestMain(m *testing.M) {
	log.SetLevel("FATAL")
	body := map[string]string{}
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("line_%d", i)
		body[key] = fmt.Sprintf("%010d", rand.Intn(1234567890))
	}
	defaultRawParser.compressPackage = 8 * 1024
	bigPackage, _ = json.Marshal(body)

	m.Run()
}

func testEcho(ctx *Context, iArgs interface{}) {
	if !util.EqualJSON(iArgs, clientMsg) {
		panic("server handle invalid message")
	}
	ctx.Out.WriteJSON("Echo", serverMsg)
}

func BenchmarkCompressZip(b *testing.B) {
	for i := 0; i < b.N; i++ {
		pkg := &Package{Id: "Test", Body: bigPackage, IsZip: true}
		b2, _ := defaultRawParser.Encode(pkg)
		if i == 0 {
			b.Logf("compress result: %d -> %d %d", len(bigPackage), len(b2), b.N)
		}
	}
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
		b, _ := Encode("Echo", clientMsg)
		ws.WriteMessage(websocket.TextMessage, b)
		waitAndRunOnce(1, 120*time.Second)

		_, buf, err := ws.ReadMessage()
		if err != nil {
			t.Error(err)
		}
		pkg, err := Decode(buf)
		if !util.EqualJSON(json.RawMessage(pkg.Data), serverMsg) {
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

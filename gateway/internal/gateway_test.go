package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/guogeer/quasar/cmd"
	"github.com/guogeer/quasar/log"
	"github.com/guogeer/quasar/util"
)

type testArgs struct {
	N int
	S string
}

var bigPackage []byte

func TestMain(m *testing.M) {
	log.SetLevel("FATAL")
	body := map[string]string{}
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("line_%d", i)
		body[key] = fmt.Sprintf("%010d", rand.Intn(1234567890))
	}
	// defaultRawParser.compressPackage = 8 * 1024
	bigPackage, _ = json.Marshal(body)

	cmd.BindWithName("Echo", testEcho, (*testArgs)(nil))
	m.Run()
}

func testEcho(ctx *cmd.Context, data interface{}) {
	ctx.Out.WriteJSON("Echo", data)
}

func BenchmarkCompressZip(b *testing.B) {
	for i := 0; i < b.N; i++ {
		pkg := &cmd.Package{Id: "Test", Body: bigPackage, IsZip: true, SignType: "raw"}
		b2, _ := pkg.Encode()
		if i == 0 {
			b.Logf("compress result: %d -> %d %d", len(bigPackage), len(b2), b.N)
		}
	}
}

func TestRecvClientPackage(t *testing.T) {
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

	const maxSendMsgNum = 99
	go func() {
		for counter := 0; counter < maxSendMsgNum; counter++ {
			b, _ := cmd.Encode("Echo", &testArgs{N: counter, S: "hello world"})
			ws.WriteMessage(websocket.TextMessage, b)
		}
	}()

	doneCtx, cancel := context.WithCancel(context.Background())
	go func() {
		for i := 0; i < maxSendMsgNum; i++ {
			_, buf, err := ws.ReadMessage()
			if err != nil {
				t.Error(err)
			}
			pkg, _ := cmd.Decode(buf)
			if pkg.Id != "Echo" {
				t.Error("recv invalid client message id")
			}
			if !util.EqualJSON(json.RawMessage(pkg.Data), &testArgs{N: i, S: "hello world"}) {
				panic("recv invalid client message data")
			}
		}
		cancel()
	}()
	// 服务器接收处理
	for i := 0; true; i++ {
		cmd.RunOnce()

		select {
		case <-doneCtx.Done():
			return
		default:
		}
	}
}

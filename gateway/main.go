package main

import (
	"flag"
	"fmt"
	"github.com/guogeer/husky/cmd"
	"github.com/guogeer/husky/log"
	"github.com/guogeer/husky/util"
	"net/http"
	"runtime"
	_ "third/env"
)

var port = flag.Int("port", 8201, "gateway server port")
var proxy = flag.String("proxy", "", "gateway server proxy addr")

func main() {
	flag.Parse()
	log.Infof("start gateway, listen %d", *port)
	addr := fmt.Sprintf("%s:%d", *proxy, *port)
	cfg := &cmd.ServiceConfig{
		ServerName: "ws_gateway",
		ServerAddr: addr,
		ServerType: "gateway",
	}
	cmd.RegisterService(cfg)

	addr = fmt.Sprintf(":%d", *port)
	http.HandleFunc("/ws", cmd.ServeWs)
	go func() {
		if err := http.ListenAndServe(addr, nil); err != nil {
			log.Fatal(err)
		}
	}()

	defer func() {
		if err := recover(); err != nil {
			const size = 64 << 10
			buf := make([]byte, size)
			buf = buf[:runtime.Stack(buf, false)]
			log.Error(err)
			log.Errorf("%s", buf)
		}
	}()

	for {
		util.TickTimerRun()
		// handle message
		cmd.RunOnce()
	}
}

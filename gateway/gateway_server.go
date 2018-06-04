package main

import (
	"flag"
	"fmt"
	_ "gateway"
	"net/http"
	"runtime"
	"third/cmd"
	"third/log"
	"third/util"
)

var port = flag.Int("port", 8201, "gateway server port")
var proxy = flag.String("proxy", "", "gateway server proxy addr")

func main() {
	flag.Parse()
	log.Infof("start gateway, listen %d", *port)
	addr := fmt.Sprintf("%s:%d", *proxy, *port)
	cmd.RegisterService("ws_gateway", addr, nil)

	addr = fmt.Sprintf(":%d", *port)
	http.HandleFunc("/ws", cmd.ServeWs)
	go func() { http.ListenAndServe(addr, nil) }()

	defer func() {
		if err := recover(); err != nil {
			const size = 64 << 10
			buf := make([]byte, size)
			buf = buf[:runtime.Stack(buf, false)]
			log.Error(err)
			log.Errorf("%s", buf)
		}
	}()

	log.Infof("start ....")
	for {
		util.TickTimerRun()
		// handle message
		cmd.RunOnce()
	}
}

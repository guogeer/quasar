package main

import (
	"flag"
	"fmt"
	"net"
	"runtime"
	"strconv"

	"quasar/cmd"
	"quasar/config"
	"quasar/log"
	_ "quasar/router/internal"
	"quasar/util"
)

var port = flag.Int("port", 9003, "router server port")

func main() {
	flag.Parse()

	addr := config.Config().Server("router").Addr
	_, portStr, _ := net.SplitHostPort(addr)
	if portStr != "" {
		*port, _ = strconv.Atoi(portStr)
	}
	log.Infof("start router server, listen %d", *port)
	go func() {
		cmd.ListenAndServe(fmt.Sprintf(":%d", *port))
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
		util.GetTimerSet().RunOnce()
		// handle message
		cmd.RunOnce()
	}
}

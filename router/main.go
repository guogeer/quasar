package main

import (
	"flag"
	"fmt"
	"net"
	"runtime"
	"strconv"

	"github.com/guogeer/quasar/v2/cmd"
	"github.com/guogeer/quasar/v2/config"
	"github.com/guogeer/quasar/v2/log"
	"github.com/guogeer/quasar/v2/utils"
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
		utils.GetTimerSet().RunOnce()
		// handle message
		cmd.RunOnce()
	}
}

package main

import (
	"fmt"
	"net"
	"runtime"
	"strconv"

	"github.com/guogeer/quasar/cmd"
	"github.com/guogeer/quasar/config"
	"github.com/guogeer/quasar/log"
	_ "github.com/guogeer/quasar/router/internal"
	"github.com/guogeer/quasar/util"
)

func main() {
	addr := config.Config().Server("router").Addr
	_, portStr, _ := net.SplitHostPort(addr)
	port, _ := strconv.Atoi(portStr)
	log.Infof("start router server, listen %d", port)
	go func() { cmd.ListenAndServe(fmt.Sprintf(":%d", port)) }()

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

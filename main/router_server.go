package main

import (
	"fmt"
	// "game"
	"github.com/guogeer/husky/cmd"
	"github.com/guogeer/husky/env"
	"github.com/guogeer/husky/log"
	_ "github.com/guogeer/husky/router"
	"github.com/guogeer/husky/util"
	"net"
	"runtime"
	"strconv"
)

func main() {
	addr := env.Config().Server("router").Addr
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

	log.Infof("start ....")
	for {
		util.TickTimerRun()
		// handle message
		cmd.RunOnce()
	}
}

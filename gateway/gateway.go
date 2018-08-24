package main

import (
	"github.com/guogeer/husky/cmd"
	"github.com/guogeer/husky/util"
	"time"
)

var (
	gSessionLocation = map[string]string{}
)

type serverStatus struct {
	Weight int
}

// update current online
func concurrent() {
	counter := cmd.GetSessionManage().Count()
	data := serverStatus{Weight: counter}
	cmd.Route(cmd.ServerRouter, "C2S_Concurrent", data)
}

func init() {
	util.NewPeriodTimer(concurrent, "2001-01-01", 10*time.Second)
}

package gateway

import (
	"sync"
	"time"

	"github.com/guogeer/quasar/cmd"
	"github.com/guogeer/quasar/util"
)

var (
	sessionLocations sync.Map
	regServices      sync.Map

	serverStates  = map[string]*serverState{}
	serverStateMu sync.RWMutex
)

type serverState struct {
	MinOnline  int
	MaxOnline  int
	CurOnline  int
	Weight     int
	ServerName string
	ServerList []string
}

func init() {
	util.NewPeriodTimer(concurrent, time.Now(), 10*time.Second)
}

// update current online
func concurrent() {
	counter := cmd.GetSessionManage().Count()
	data := serverState{Weight: counter}
	cmd.Route("router", "C2S_Concurrent", data)
}

//
// 匹配最佳的服务
// 匹配规则：
// 1、ServerName == name时直接选中
// 2、优先匹配最小ServerName且人数小于MinOnline
// 3、匹配Weight最小
//
func matchBestServer(ssid, name string) string {
	serverStateMu.RLock()
	defer serverStateMu.RUnlock()

	state, ok := serverStates[name]
	if ok {
		return state.ServerName
	}

	if v, ok := sessionLocations.Load(ssid); ok && v != "" {
		return v.(string)
	}

	matchServers := map[string]bool{}
	for _, server := range serverStates {
		for _, child := range server.ServerList {
			if server.ServerName == child {
				matchServers[server.ServerName] = true
			}
		}
	}

	var matchName string
	for server := range matchServers {
		state := serverStates[server]
		if state.Weight < state.MinOnline && matchName < state.ServerName {
			matchName = server
		}
	}
	if matchName != "" {
		return matchName
	}
	for server := range matchServers {
		state := serverStates[server]
		if state.Weight < state.MaxOnline && (matchName == "" || state.Weight < serverStates[matchName].Weight) {
			matchName = server
		}
	}
	return matchName
}

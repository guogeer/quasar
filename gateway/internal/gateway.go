package gateway

import (
	"sync"
	"time"

	"github.com/guogeer/quasar/cmd"
	"github.com/guogeer/quasar/util"
)

var (
	sessionLocations sync.Map
	regServices      sync.Map // Deprecated

	serverStates  = map[string]*serverState{}
	serverStateMu sync.RWMutex
)

type serverState struct {
	MinWeight  int
	MaxWeight  int
	Weight     int
	ServerName string
	ServerList []string
}

type sessionLocation struct {
	MatchServer string // 服务的ID
	ServerName  string // 客户端请求的协议头
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
// 3、匹配Weight最小
//
func matchBestServer(ssid, name string) string {
	serverStateMu.RLock()
	defer serverStateMu.RUnlock()

	state, ok := serverStates[name]
	if ok {
		return state.ServerName
	}

	matchServers := map[string]bool{}
	for _, server := range serverStates {
		for _, child := range server.ServerList {
			if name == child {
				matchServers[server.ServerName] = true
			}
		}
	}

	if v, ok := sessionLocations.Load(ssid); ok {
		loc := v.(*sessionLocation)
		if matchServers[loc.ServerName] {
			return v.(string)
		}
	}

	var matchName string
	for server := range matchServers {
		state := serverStates[server]
		if state.Weight < state.MinWeight && matchName < state.ServerName {
			matchName = server
		}
	}
	if matchName != "" {
		return matchName
	}
	for server := range matchServers {
		state := serverStates[server]
		if (state.MaxWeight == 0 || state.Weight < state.MaxWeight) &&
			(matchName == "" || state.Weight < serverStates[matchName].Weight) {
			matchName = server
		}
	}
	return matchName
}

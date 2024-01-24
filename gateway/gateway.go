package main

import (
	"strings"
	"sync"
	"time"

	"quasar/cmd"
	"quasar/util"
)

var (
	sessionLocations sync.Map // 连接会话的位置。[ssid:serverId]

	serverStates  = map[string]*serverState{} // 服务负载。[serverId:serverState]
	serverStateMu sync.RWMutex
)

type serverState struct {
	Id        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	Weight    int    `json:"weight,omitempty"`
	MaxWeight int    `json:"maxWeight,omitempty"`
	MinWeight int    `json:"minWeight,omitempty"`
}

type sessionLocation struct {
	MatchServer string `json:"matchServer,omitempty"` // 服务的ID
	ServerName  string `json:"serverName,omitempty"`  // 客户端请求的协议头
}

func init() {
	util.NewPeriodTimer(concurrent, time.Now(), 10*time.Second)
}

// update current online
func concurrent() {
	counter := cmd.CountSession()
	data := serverState{Weight: counter}
	cmd.Route("router", "C2S_Concurrent", data)

	cmd.Route("router", "C2S_QueryServerState", cmd.M{})
}

// 匹配最佳的服务
// 匹配规则：
// 1、serverId == name时直接选中
// 2、优先匹配最小serverId且人数小于MinOnline
// 3、匹配Weight最小
func matchBestServer(ssid, name string) string {
	serverStateMu.RLock()
	defer serverStateMu.RUnlock()

	state, ok := serverStates[name]
	if ok {
		return state.Name
	}

	matchServers := map[string]bool{}
	for _, server := range serverStates {
		for _, serverName := range strings.Split(server.Name, ",") {
			if name == serverName {
				matchServers[server.Id] = true
			}
		}
	}

	if v, ok := sessionLocations.Load(ssid); ok {
		loc := v.(*sessionLocation)
		if matchServers[loc.ServerName] {
			return loc.ServerName
		}
	}

	var matchServer string
	for serverId := range matchServers {
		state := serverStates[serverId]
		if state.Weight < state.MinWeight && matchServer < state.Id {
			matchServer = serverId
		}
	}
	if matchServer != "" {
		return matchServer
	}
	for server := range matchServers {
		state := serverStates[server]
		if (state.MaxWeight == 0 || state.Weight < state.MaxWeight) &&
			(matchServer == "" || state.Weight < serverStates[matchServer].Weight) {
			matchServer = server
		}
	}
	return matchServer
}

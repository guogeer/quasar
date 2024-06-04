package main

import (
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/guogeer/quasar/cmd"
	"github.com/guogeer/quasar/utils"
)

var (
	sessionLocations sync.Map // 连接会话的位置。[ssid:serverId]

	serverStates  = map[string]serverState{} // 服务负载。[serverId:serverState]
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
	MatchServerId string `json:"matchServerId,omitempty"` // 服务的ID
	ServerName    string `json:"serverName,omitempty"`    // 客户端请求的协议头
}

func init() {
	utils.NewPeriodTimer(concurrent, time.Now(), 10*time.Second)
}

// update current online
func concurrent() {
	counter := cmd.CountSession()
	data := serverState{Weight: counter}
	cmd.Route("router", "c2s_concurrent", data)

	cmd.Route("router", "c2s_queryServerState", cmd.M{})
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
		if slices.Contains(strings.Split(server.Name, ","), name) {
			matchServers[server.Id] = true
		}
	}

	if v, ok := sessionLocations.Load(ssid); ok {
		loc := v.(*sessionLocation)
		if matchServers[loc.ServerName] {
			return loc.ServerName
		}
	}

	var matchServerId string
	for serverId := range matchServers {
		state := serverStates[serverId]
		if state.Weight < state.MinWeight && matchServerId < state.Id {
			matchServerId = serverId
		}
	}
	if matchServerId != "" {
		return matchServerId
	}
	for server := range matchServers {
		state := serverStates[server]
		if (state.MaxWeight == 0 || state.Weight < state.MaxWeight) &&
			(matchServerId == "" || state.Weight < serverStates[matchServerId].Weight) {
			matchServerId = server
		}
	}
	return matchServerId
}

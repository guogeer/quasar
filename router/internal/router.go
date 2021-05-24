package router

import (
	"encoding/json"
	"time"

	"github.com/guogeer/quasar/cmd"
	"github.com/guogeer/quasar/util"
)

const (
	serverGateway = "gateway" // 网关服
	serverCenter  = "center"  // 世界服
	serverEntry   = "entry"   // 入口，需同步gw地址
)

func init() {
	util.NewPeriodTimer(syncServerState, time.Now(), 10*time.Second)
}

type Server struct {
	out cmd.Conn

	name       string
	typ        string
	addr       string   // 地址
	serverList []string // 子服务
	IsRandPort bool

	minWeight int // 最大负载
	maxWeight int // 最小负载
	weight    int // 当前负载

	data json.RawMessage
}

var (
	servers  = map[string]*Server{}
	gateways = map[string]*Server{}
)

// 查找最新的gw地址
func getBestGateway() string {
	var addr string
	var weight int
	for host, gw := range gateways {
		if len(addr) == 0 || gw.weight < weight {
			addr = host
			weight = gw.weight
		}
	}
	return addr
}

// 匹配服务
func matchBestServer(name string) string {
	if server, ok := servers[name]; ok {
		return server.addr
	}
	for _, server := range servers {
		for _, serverName := range server.serverList {
			if serverName == name {
				return server.addr
			}
		}
	}
	return ""
}

func getServer(name string) *Server {
	if server, ok := servers[name]; ok {
		return server
	}
	return nil
}

func removeServer(out cmd.Conn) *Server {
	for addr, server := range gateways {
		if server.out == out {
			delete(gateways, addr)
			return server
		}
	}
	for name, server := range servers {
		if server.out == out {
			delete(servers, name)
			return server
		}
	}
	return nil
}

// 查找链接的服务
func findServerByConn(out cmd.Conn) *Server {
	for _, server := range gateways {
		if server.out == out {
			return server
		}
	}
	for _, server := range servers {
		if server.out == out {
			return server
		}
	}
	return nil
}

func addServer(server *Server) {
	name := server.name
	addr := server.addr
	if server.typ == serverGateway {
		gateways[addr] = server
	} else {
		servers[name] = server
		syncServerState()
	}
	// 立即同步网关地址
	if server.typ == serverEntry {
		syncBestGateway()
	}
}

type serverState struct {
	MinWeight  int
	MaxWeight  int
	Weight     int
	ServerName string
	ServerList []string
}

// 向gw同步server服务负载
func syncServerState() {
	var states []serverState
	for _, server := range servers {
		states = append(states, serverState{
			MinWeight:  server.minWeight,
			MaxWeight:  server.maxWeight,
			Weight:     server.weight,
			ServerName: server.name,
			ServerList: server.serverList,
		})
	}
	for _, gw := range gateways {
		gw.out.WriteJSON("FUNC_SyncServerState", map[string]interface{}{
			"Servers": states,
		})
	}
}

// 同步gw地址
func syncBestGateway() {
	for _, srv := range servers {
		var isUpdateGateway bool
		if srv.typ == serverEntry {
			isUpdateGateway = true
		}
		// Deprecated: remove at v1.7.x
		if srv.name == "login" {
			isUpdateGateway = true
		}

		if isUpdateGateway {
			addr := getBestGateway()
			response := map[string]interface{}{"Address": addr}
			srv.out.WriteJSON("S2C_GetBestGateway", response)
		}
	}
}

package main

import (
	"strings"

	"github.com/guogeer/quasar/cmd"
)

var servers = map[string]*Server{}

type Server struct {
	out cmd.Conn

	id   string
	name string // 服务。若存在多个采用逗号,隔开
	addr string // 地址

	minWeight int // 最大负载
	maxWeight int // 最小负载
	weight    int // 当前负载
}

func (server *Server) IsGateway() bool {
	return server.name == "gateway"
}

// 匹配最佳gw地址
// 优先选择低负载
// 负载相同，选择ID更小
func matchBestGateway() string {
	var matchServer *Server
	for _, server := range servers {
		if server.IsGateway() {
			if matchServer == nil || server.weight < matchServer.weight {
				matchServer = server
			} else if server.weight == matchServer.weight && server.id < matchServer.id {
				matchServer = server
			}
		}
	}
	if matchServer == nil {
		return ""
	}
	return matchServer.addr
}

// 匹配服务
func matchBestServer(name string) string {
	if server, ok := servers[name]; ok {
		return server.addr
	}
	for _, server := range servers {
		for _, serverName := range strings.Split(server.name, ",") {
			if serverName == name {
				return server.addr
			}
		}
	}
	return ""
}

func removeServer(out cmd.Conn) *Server {
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
	for _, server := range servers {
		if server.out == out {
			return server
		}
	}
	return nil
}

// 增加新服，不可覆盖已有的服
func addServer(server *Server) {
	if _, ok := servers[server.id]; !ok {
		servers[server.id] = server
	}
}

type serverState struct {
	MinWeight int    `json:"minWeight,omitempty"`
	MaxWeight int    `json:"maxWeight,omitempty"`
	Weight    int    `json:"weight,omitempty"`
	Id        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
}

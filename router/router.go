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

// 匹配负载最低的gw地址
func matchBestGateway() string {
	var addr string
	var weight int
	for _, server := range servers {
		if server.name == "gateway" && (weight == 0 || server.weight < weight) {
			addr, weight = server.addr, server.weight
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
	MinWeight int
	MaxWeight int
	Weight    int
	Id        string
	Name      string
}

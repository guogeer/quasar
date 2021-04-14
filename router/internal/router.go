package router

import (
	"github.com/guogeer/quasar/cmd"
	// "github.com/guogeer/quasar/log"
	"encoding/json"
)

const (
	serverGateway = "gateway" // 网关服
	serverCenter  = "center"  // 世界服
)

type Server struct {
	out             cmd.Conn
	weight          int
	name, addr, typ string
	IsRandPort      bool
	serverList      []string

	data json.RawMessage
}

type Router struct {
	servers  map[string]*Server
	gateways map[string]*Server
}

var gRouter = &Router{
	servers:  make(map[string]*Server),
	gateways: make(map[string]*Server),
	// SubGameList: make(map[string]cmd.Writer),
}

func (r *Router) GetBestGateway() string {
	var (
		addr   string
		weight int
	)
	for host, gw := range r.gateways {
		if len(addr) == 0 || gw.weight < weight {
			addr = host
			weight = gw.weight
			// log.Debug("best", addr, gw.weight, weight)
		}
	}
	return addr
}

func (r *Router) MatchBestServer(name string) string {
	if server, ok := r.servers[name]; ok {
		return server.addr
	}
	for _, server := range r.servers {
		for _, serverName := range server.serverList {
			if serverName == name {
				return server.addr
			}
		}
	}
	return ""
}

func (r *Router) GetServer(name string) *Server {
	if server, ok := r.servers[name]; ok {
		return server
	}
	return nil
}

func (r *Router) Remove(out cmd.Conn) *Server {
	for addr, server := range r.gateways {
		if server.out == out {
			delete(r.gateways, addr)
			return server
		}
	}
	for name, server := range r.servers {
		if server.out == out {
			delete(r.servers, name)
			return server
		}
	}
	return nil
}

// 查找链接的服务
func (r *Router) findConnServer(out cmd.Conn) *Server {
	for _, server := range r.gateways {
		if server.out == out {
			return server
		}
	}
	for _, server := range r.servers {
		if server.out == out {
			return server
		}
	}
	return nil
}

func (r *Router) AddServer(server *Server) {
	name := server.name
	addr := server.addr
	if server.typ == serverGateway {
		r.gateways[addr] = server
	} else {
		r.servers[name] = server
	}
}

package router

import (
	"github.com/guogeer/husky/cmd"
	// "github.com/guogeer/husky/log"
	"encoding/json"
)

type Server struct {
	out        cmd.Conn
	name, addr string
	weight     int
	isCenter   bool // center server

	data json.RawMessage
}

type Router struct {
	ServerList  map[string]*Server
	GatewayList map[string]*Server
	// SubGameList map[string]SubGame
}

var defaultRouter = &Router{
	ServerList:  make(map[string]*Server),
	GatewayList: make(map[string]*Server),
	// SubGameList: make(map[string]cmd.Writer),
}

func GetRouter() *Router {
	return defaultRouter
}

func (r *Router) GetBestGateway() string {
	var (
		addr   string
		weight int
	)
	for host, gw := range r.GatewayList {
		if len(addr) == 0 || gw.weight < weight {
			addr = host
			weight = gw.weight
			// log.Debug("best", addr, gw.weight, weight)
		}
	}
	return addr
}

func (r *Router) GetServerAddr(name string) string {
	var addr string
	if server, ok := r.ServerList[name]; ok {
		addr = server.addr
	}
	return addr
}

func (r *Router) GetServer(name string) *Server {
	if server, ok := r.ServerList[name]; ok {
		return server
	}
	return nil
}

func (r *Router) Remove(out cmd.Conn) {
	for addr, server := range r.GatewayList {
		if server.out == out {
			delete(r.GatewayList, addr)
			break
		}
	}
	for name, server := range r.ServerList {
		if server.out == out {
			delete(r.ServerList, name)
			break
		}
	}
}

func (r *Router) AddServer(server *Server) {
	name := server.name
	addr := server.addr
	if cmd.IsGateway(name) {
		r.GatewayList[addr] = server
	} else {
		r.ServerList[name] = server
	}
}

func (r *Router) Broadcast(pkg *cmd.Package) {
	for _, gw := range r.GatewayList {
		gw.out.WriteJSON("FUNC_Broadcast", pkg)
	}
}

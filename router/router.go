package main

import (
	"github.com/guogeer/husky/cmd"
	// "github.com/guogeer/husky/log"
	"encoding/json"
)

type Server struct {
	out             cmd.Conn
	weight          int
	name, addr, typ string

	data json.RawMessage
}

type Router struct {
	servers  map[string]*Server
	gateways map[string]*Server
}

var defaultRouter = &Router{
	servers:  make(map[string]*Server),
	gateways: make(map[string]*Server),
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
	for host, gw := range r.gateways {
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
	if server, ok := r.servers[name]; ok {
		addr = server.addr
	}
	return addr
}

func (r *Router) GetServer(name string) *Server {
	if server, ok := r.servers[name]; ok {
		return server
	}
	return nil
}

func (r *Router) Remove(out cmd.Conn) {
	for addr, server := range r.gateways {
		if server.out == out {
			delete(r.gateways, addr)
			break
		}
	}
	for name, server := range r.servers {
		if server.out == out {
			delete(r.servers, name)
			break
		}
	}
}

func (r *Router) AddServer(server *Server) {
	name := server.name
	addr := server.addr
	if server.typ == "gateway" {
		r.gateways[addr] = server
	} else {
		r.servers[name] = server
	}
}

func (r *Router) Broadcast(pkg *cmd.Package) {
	for _, gw := range r.gateways {
		gw.out.WriteJSON("FUNC_Broadcast", pkg)
	}
}

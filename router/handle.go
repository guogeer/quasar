package main

import (
	"encoding/json"
	"net"

	"github.com/guogeer/quasar/cmd"
	"github.com/guogeer/quasar/log"
)

type routeArgs struct {
	cmd.ServiceConfig

	Weight int `json:"weight,omitempty"`
}

type forwardArgs struct {
	ServerId   string          `json:"serverId,omitempty"`
	ServerName string          `json:"serverName,omitempty"`
	MsgId      string          `json:"msgId,omitempty"`
	MsgData    json.RawMessage `json:"msgData,omitempty"`
}

func init() {
	cmd.BindFunc(C2S_Register, (*routeArgs)(nil), cmd.WithPrivate())
	cmd.BindFunc(C2S_GetServerAddr, (*routeArgs)(nil), cmd.WithPrivate())
	cmd.BindFunc(C2S_Concurrent, (*routeArgs)(nil), cmd.WithPrivate())
	cmd.BindFunc(C2S_Route, (*forwardArgs)(nil), cmd.WithPrivate())
	cmd.BindFunc(C2S_QueryServerState, (*routeArgs)(nil), cmd.WithPrivate())
	cmd.BindFunc(C2S_GetBestGateway, (*routeArgs)(nil), cmd.WithPrivate())

	cmd.BindFunc(C2S_Broadcast, (*cmd.Package)(nil), cmd.WithPrivate())
	cmd.BindFunc(FUNC_Close, (*routeArgs)(nil), cmd.WithPrivate())
}

// ServerAddr == "" 无服务
func C2S_Register(ctx *cmd.Context, data any) {
	args := data.(*routeArgs)
	host, port, _ := net.SplitHostPort(args.Addr)
	if host == "" {
		host, _, _ = net.SplitHostPort(ctx.Out.RemoteAddr())
	}

	var addr string
	if port != "" {
		addr = host + ":" + port
	}
	log.Infof("register server:%s %s addr:%s", args.Id, args.Name, addr)

	newServer := &Server{
		out:  ctx.Out,
		id:   args.Id,
		name: args.Name,
		addr: addr,
	}
	addServer(newServer)

	for _, server := range servers {
		if server.IsGateway() {
			server.out.WriteJSON("s2c_register", struct{}{})
		}
	}
}

func C2S_GetServerAddr(ctx *cmd.Context, data any) {
	args := data.(*routeArgs)
	name := args.Name
	addr := matchBestServer(name)
	log.Infof("get server:%s addr:%s", name, addr)
	ctx.Out.WriteJSON("s2c_getServerAddr", cmd.M{"name": name, "addr": addr})
}

func C2S_Broadcast(ctx *cmd.Context, data any) {
	pkg := data.(*cmd.Package)
	for _, server := range servers {
		if server.IsGateway() {
			server.out.WriteJSON("func_broadcast", pkg)
		}
	}
}

// 更新网关负载
func C2S_Concurrent(ctx *cmd.Context, data any) {
	args := data.(*routeArgs)

	server := findServerByConn(ctx.Out)
	if server == nil {
		return
	}
	log.Debugf("concurrent %v %v", server.id, args.Weight)

	server.weight = args.Weight
}

func C2S_Route(ctx *cmd.Context, data any) {
	args := data.(*forwardArgs)

	var matchServers []string
	for id := range servers {
		if args.ServerName == "*" || args.ServerName == servers[id].name {
			matchServers = append(matchServers, id)
		}
	}
	if args.ServerId != "" {
		if _, ok := servers[args.ServerId]; ok {
			matchServers = []string{args.ServerId}
		}
	}

	for _, id := range matchServers {
		if server, ok := servers[id]; ok {
			server.out.WriteJSON(args.MsgId, args.MsgData)
		}
	}
}

func FUNC_Close(ctx *cmd.Context, data any) {
	// args := data.(*Args)
	closedServer := findServerByConn(ctx.Out)
	if closedServer == nil {
		return
	}
	log.Infof("server %s lose connection", closedServer.id)

	removeServer(ctx.Out)
}

// 同步服务状态，需主动查询
func C2S_QueryServerState(ctx *cmd.Context, data any) {
	var states []serverState
	for _, server := range servers {
		states = append(states, serverState{
			Id:        server.id,
			Name:      server.name,
			Weight:    server.weight,
			MinWeight: server.minWeight,
			MaxWeight: server.maxWeight,
		})
		// log.Debug("query server state", server.id, server.weight)
	}
	ctx.Out.WriteJSON("s2c_queryServerState", cmd.M{"servers": states})
}

func C2S_GetBestGateway(ctx *cmd.Context, data any) {
	addr := matchBestGateway()
	ctx.Out.WriteJSON("s2c_getBestGateway", cmd.M{"addr": addr})
}

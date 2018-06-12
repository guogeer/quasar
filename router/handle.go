package main

import (
	"encoding/json"
	"github.com/guogeer/husky/cmd"
	"github.com/guogeer/husky/log"
	"net"
)

type Args struct {
	ServerName string
	ServerAddr string
	ServerData json.RawMessage
	ServerType string

	Nickname string
	Type     int
	Info     string
	// SubGame    SubGame
	Weight int
}

func init() {
	cmd.Bind(C2S_Register, (*Args)(nil))
	cmd.Bind(C2S_GetServerAddr, (*Args)(nil))
	cmd.Bind(C2S_Concurrent, (*Args)(nil))
	cmd.Bind(C2S_Route, (*cmd.ForwardArgs)(nil))

	cmd.Bind(C2S_Broadcast, (*cmd.Package)(nil))
	cmd.Bind(FUNC_Close, (*Args)(nil))
}

// ServerAddr == "" 无服务
func C2S_Register(ctx *cmd.Context, data interface{}) {
	args := data.(*Args)
	host, port, _ := net.SplitHostPort(args.ServerAddr)
	if host == "" {
		host, _, _ = net.SplitHostPort(ctx.Out.RemoteAddr())
	}

	addr := ""
	if port != "" {
		addr = host + ":" + port
	}
	log.Info("register ...", args.ServerName, addr)

	// check addr register
	oldServer := GetRouter().GetServerByAddr(addr)
	if addr != "" && oldServer != nil && oldServer.name != args.ServerName {
		log.Errorf("server %s attemp to register addr %s of %s", args.ServerName, addr, oldServer.name)
		return
	}
	// center server
	if args.ServerType == "center" {
		for _, server := range GetRouter().servers {
			ctx.Out.WriteJSON("S2C_AddGame", map[string]interface{}{
				"Name": server.name,
				"Data": server.data,
			})
		}
	}
	for _, server := range GetRouter().servers {
		if server.typ == "center" && server.name != args.ServerName {
			server.out.WriteJSON("S2C_AddGame", map[string]interface{}{
				"Name": args.ServerName,
				"Data": server.data,
			})
		}
	}

	newServer := &Server{
		out:  ctx.Out,
		name: args.ServerName,
		addr: addr,
		data: args.ServerData,
		typ:  args.ServerType,
	}
	GetRouter().AddServer(newServer)
	// 向网关注册服务
	if args.ServerType == "gateway" {
		for _, server := range GetRouter().servers {
			ctx.Out.WriteJSON("FUNC_RegisterServiceInGateway", map[string]interface{}{
				"Name": server.name,
			})
		}
	} else if len(args.ServerAddr) > 0 {
		log.Info("route server", args.ServerName)
		for _, gw := range GetRouter().gateways {
			gw.out.WriteJSON("FUNC_RegisterServiceInGateway", map[string]interface{}{
				"Name": args.ServerName,
			})
		}
	}
}

func C2S_GetServerAddr(ctx *cmd.Context, data interface{}) {
	args := data.(*Args)
	name := args.ServerName
	addr := GetRouter().GetServerAddr(name)
	log.Debug("get addr", name, addr)
	ctx.Out.WriteJSON("S2C_GetServerAddr", map[string]string{"ServerName": name, "ServerAddr": addr})
}

func C2S_Broadcast(ctx *cmd.Context, data interface{}) {
	pkg := data.(*cmd.Package)
	GetRouter().Broadcast(pkg)
}

// 更新网关负载
func C2S_Concurrent(ctx *cmd.Context, data interface{}) {
	args := data.(*Args)
	for _, gw := range GetRouter().gateways {
		if gw.out == ctx.Out {
			// log.Debug("test ", gw.addr, gw.weight)
			gw.weight = args.Weight
		}
	}

	addr := GetRouter().GetBestGateway()
	// log.Debug("concurrent", addr, args.Weight)
	if s := GetRouter().GetServer("login"); s != nil {
		s.out.WriteJSON("S2C_GetBestGateway", map[string]interface{}{"Address": addr})
	}
}

func C2S_Route(ctx *cmd.Context, data interface{}) {
	args := data.(*cmd.ForwardArgs)
	servers := args.ServerList
	if len(servers) == 1 && servers[0] == "*" {
		prefixMap := make(map[string]bool)
		for _, server := range GetRouter().servers {
			prefixMap[server.name] = true
		}
		servers = servers[:0]
		for s := range prefixMap {
			servers = append(servers, s)
		}
	}

	for _, name := range servers {
		if s := GetRouter().GetServer(name); s != nil {
			s.out.WriteJSON(args.Name, args.Data)
		}
	}
}

func FUNC_Close(ctx *cmd.Context, data interface{}) {
	// args := data.(*Args)
	GetRouter().Remove(ctx.Out)
}

package router

import (
	"encoding/json"
	"net"

	"github.com/guogeer/quasar/cmd"
	"github.com/guogeer/quasar/log"
)

type Args struct {
	cmd.ServiceConfig
	ServerData json.RawMessage

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
	log.Infof("register server:%s %v addr:%s", args.ServerName, args.ServerList, addr)
	ctx.Out.WriteJSON("C2S_RegisterOk", struct{}{})

	newServer := &Server{
		out:        ctx.Out,
		name:       args.ServerName,
		addr:       addr,
		data:       args.ServerData,
		typ:        args.ServerType,
		IsRandPort: args.IsRandPort,
		serverList: args.ServerList,
	}
	addServer(newServer)
	// center server
	if newServer.typ == serverCenter {
		for _, server := range servers {
			ctx.Out.WriteJSON("S2C_AddGame", map[string]interface{}{
				"Name": server.name,
				"Data": server.data,
			})
		}
	}
	for _, server := range servers {
		if server.typ == serverCenter && server.name != newServer.name {
			server.out.WriteJSON("S2C_AddGame", map[string]interface{}{
				"Name": newServer.name,
				"Data": newServer.data,
			})
		}
	}

	// Deprecated: use FUNC_SyncServerState
	// 向网关注册服务
	if newServer.typ == serverGateway {
		for _, server := range servers {
			ctx.Out.WriteJSON("FUNC_RegisterServiceInGateway", map[string]interface{}{
				"Name":       server.name,
				"ServerList": server.serverList,
			})
		}
	} else if newServer.addr != "" {
		for _, gw := range gateways {
			gw.out.WriteJSON("FUNC_RegisterServiceInGateway", map[string]interface{}{
				"Name":       newServer.name,
				"ServerList": newServer.serverList,
			})
		}
	}
}

func C2S_GetServerAddr(ctx *cmd.Context, data interface{}) {
	args := data.(*Args)
	name := args.ServerName
	addr := matchBestServer(name)
	log.Infof("get server:%s addr:%s", name, addr)
	response := map[string]string{"ServerName": name, "ServerAddr": addr}
	ctx.Out.WriteJSON("S2C_GetServerAddr", response)
}

func C2S_Broadcast(ctx *cmd.Context, data interface{}) {
	pkg := data.(*cmd.Package)
	for _, gw := range gateways {
		gw.out.WriteJSON("FUNC_Broadcast", pkg)
	}
}

// 更新网关负载
func C2S_Concurrent(ctx *cmd.Context, data interface{}) {
	args := data.(*Args)

	server := findServerByConn(ctx.Out)
	if server == nil {
		return
	}
	log.Debug("concurrent", server.name, args.Weight)

	server.weight = args.Weight
	if server.typ == serverGateway {
		syncBestGateway()
	}
}

func C2S_Route(ctx *cmd.Context, data interface{}) {
	args := data.(*cmd.ForwardArgs)
	serverList := args.ServerList
	if len(serverList) == 1 && serverList[0] == "*" {
		prefixMap := make(map[string]bool)
		for _, server := range servers {
			prefixMap[server.name] = true
		}
		serverList = serverList[:0]
		for s := range prefixMap {
			serverList = append(serverList, s)
		}
	}

	for _, name := range serverList {
		if s := getServer(name); s != nil {
			s.out.WriteJSON(args.Name, args.Data)
		}
	}
}

func FUNC_Close(ctx *cmd.Context, data interface{}) {
	// args := data.(*Args)
	server := findServerByConn(ctx.Out)
	if server == nil {
		return
	}
	log.Infof("server %s lose connection", server.name)

	removeServer(ctx.Out)
	switch server.typ {
	case serverGateway:
		syncBestGateway()
	default:
		syncServerState()
	}
}

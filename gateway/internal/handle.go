package gateway

import (
	"encoding/json"
	"net"

	"github.com/guogeer/quasar/cmd"
	"github.com/guogeer/quasar/log"
)

type Args struct {
	Id, ServerName string

	UId  int
	Data json.RawMessage

	Name       string
	ServerList []string
	Servers    []*serverState
}

func init() {
	cmd.BindWithoutQueue("FUNC_Route", FUNC_Route, (*Args)(nil))

	cmd.Bind(FUNC_Broadcast, (*Args)(nil))
	cmd.Bind(FUNC_ServerClose, (*Args)(nil))
	cmd.Bind(FUNC_HelloGateway, (*Args)(nil))
	cmd.Bind(FUNC_Close, (*Args)(nil))
	cmd.Bind(FUNC_RegisterServiceInGateway, (*Args)(nil))
	cmd.Bind(FUNC_SyncServerState, (*Args)(nil))

	cmd.Bind(HeartBeat, (*Args)(nil))
}

func FUNC_Close(ctx *cmd.Context, data interface{}) {
	log.Debugf("session close %s", ctx.Ssid)
	if v, ok := sessionLocations.Load(ctx.Ssid); ok {
		serverName := v.(string)
		ss := &cmd.Session{Id: ctx.Ssid, Out: ctx.Out}
		ss.Route(serverName, "Close", struct{}{})

	}
	sessionLocations.Delete(ctx.Ssid)
}

func FUNC_HelloGateway(ctx *cmd.Context, data interface{}) {
	log.Debugf("session locate %s", ctx.Ssid)
	args := data.(*Args)
	uid := args.UId

	ip := "UNKNOW"
	if ss := cmd.GetSession(ctx.Ssid); ss != nil {
		addr := ss.Out.RemoteAddr()
		log.Debug("hello gateway", addr)
		sessionLocations.Store(ctx.Ssid, args.ServerName)
		if host, _, err := net.SplitHostPort(addr); err == nil {
			ip = host
		}
	}
	ss := &cmd.Session{Id: ctx.Ssid, Out: ctx.Out}
	ss.WriteJSON("FUNC_HelloGateway", map[string]interface{}{"UId": uid, "IP": ip})
}

// 直接转发消息到客户端
func FUNC_Route(ctx *cmd.Context, data interface{}) {
	args := data.(*Args)
	// log.Info("route", ctx.Ssid)
	if ss := cmd.GetSession(ctx.Ssid); ss != nil {
		// client := ctx.Out.(*cmd.Client)
		// id := fmt.Sprintf("%s.%s", client.ServerName(), args.Id)
		ss.Out.WriteJSON(args.Id, args.Data)
	}
}

func FUNC_Broadcast(ctx *cmd.Context, data interface{}) {
	args := data.(*Args)
	for _, ss := range cmd.GetSessionList() {
		ss.Out.WriteJSON(args.Id, args.Data)
	}
}

func FUNC_ServerClose(ctx *cmd.Context, data interface{}) {
	client := ctx.Out.(*cmd.Client)
	for _, ss := range cmd.GetSessionList() {
		// 2020-11-24 仅通知在当前服务的连接
		if v, ok := sessionLocations.Load(ss.Id); ok && v == client.ServerName() {
			ss.Out.WriteJSON("ServerClose", map[string]string{"ServerName": client.ServerName()})
		}
	}
	regServices.Store(client.ServerName(), false)
}

func HeartBeat(ctx *cmd.Context, data interface{}) {
	ctx.Out.WriteJSON("HeartBeat", struct{}{})
}

// Deprecated: use FUNC_SyncServerState
func FUNC_RegisterServiceInGateway(ctx *cmd.Context, data interface{}) {
	args := data.(*Args)
	for _, name := range args.ServerList {
		regServices.Store(name, true)
	}
}

// 同步服务负载
func FUNC_SyncServerState(ctx *cmd.Context, data interface{}) {
	args := data.(*Args)

	serverStateMu.Lock()
	defer serverStateMu.Unlock()
	for _, state := range args.Servers {
		serverStates[state.ServerName] = state
	}
}

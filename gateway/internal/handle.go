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
}

func init() {
	cmd.Bind(FUNC_Route, (*Args)(nil))
	cmd.Bind(FUNC_Broadcast, (*Args)(nil))
	cmd.Bind(FUNC_ServerClose, (*Args)(nil))
	cmd.Bind(FUNC_HelloGateway, (*Args)(nil))
	cmd.Bind(FUNC_Close, (*Args)(nil))
	cmd.Bind(FUNC_RegisterServiceInGateway, (*Args)(nil))

	cmd.Bind(HeartBeat, (*Args)(nil))
}

func FUNC_Close(ctx *cmd.Context, data interface{}) {
	log.Debugf("session close %s", ctx.Ssid)
	if v, ok := gSessionLocations.Load(ctx.Ssid); ok {
		serverName := v.(string)
		ss := &cmd.Session{Id: ctx.Ssid, Out: ctx.Out}
		ss.Route(serverName, "Close", struct{}{})

	}
	gSessionLocations.Delete(ctx.Ssid)
}

func FUNC_HelloGateway(ctx *cmd.Context, data interface{}) {
	log.Debugf("session locate %s", ctx.Ssid)
	args := data.(*Args)
	uid := args.UId

	ip := "UNKNOW"
	if ss := cmd.GetSession(ctx.Ssid); ss != nil {
		addr := ss.Out.RemoteAddr()
		log.Debug("hello gateway", addr)
		gSessionLocations.Store(ctx.Ssid, args.ServerName)
		if host, _, err := net.SplitHostPort(addr); err == nil {
			ip = host
		}
	}
	ss := &cmd.Session{Id: ctx.Ssid, Out: ctx.Out}
	ss.WriteJSON("FUNC_HelloGateway", map[string]interface{}{"UId": uid, "IP": ip})
}

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
		if v, ok := gSessionLocations.Load(ss.Id); ok && v == client.ServerName() {
			ss.Out.WriteJSON("ServerClose", map[string]string{"ServerName": client.ServerName()})
		}
	}
	gServices.Store(client.ServerName(), false)
}

func HeartBeat(ctx *cmd.Context, data interface{}) {
	ctx.Out.WriteJSON("HeartBeat", struct{}{})
}

func FUNC_RegisterServiceInGateway(ctx *cmd.Context, data interface{}) {
	args := data.(*Args)
	for _, name := range args.ServerList {
		gServices.Store(name, true)
	}
}

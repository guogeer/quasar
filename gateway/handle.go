package main

import (
	"encoding/json"

	"github.com/guogeer/quasar/cmd"
	"github.com/guogeer/quasar/log"
)

type Args struct {
	Id          string
	ServerName  string
	MatchServer string

	Data json.RawMessage

	Name    string
	Servers []*serverState
}

func init() {
	cmd.BindWithoutQueue("FUNC_Route", FUNC_Route, (*Args)(nil))
	cmd.BindWithoutQueue("HeartBeat", HeartBeat, (*Args)(nil))

	cmd.Bind(FUNC_Broadcast, (*Args)(nil))
	cmd.Bind(FUNC_SwitchServer, (*Args)(nil))
	cmd.Bind(FUNC_Close, (*Args)(nil))
	cmd.Bind(S2C_ServerClose, (*Args)(nil))
	cmd.Bind(S2C_QueryServerState, (*Args)(nil))
}

func FUNC_Close(ctx *cmd.Context, data interface{}) {
	log.Debugf("session close %s", ctx.Ssid)
	if v, ok := sessionLocations.Load(ctx.Ssid); ok {
		loc := v.(*sessionLocation)
		ss := &cmd.Session{Id: ctx.Ssid, Out: ctx.Out}
		ss.Route(loc.ServerName, "Close", struct{}{})

	}
	sessionLocations.Delete(ctx.Ssid)
}

func FUNC_SwitchServer(ctx *cmd.Context, data interface{}) {
	args := data.(*Args)
	log.Debugf("session ssid:%s switch server name:%s", ctx.Ssid, args.ServerName)
	loc := &sessionLocation{ServerName: args.ServerName, MatchServer: args.MatchServer}
	sessionLocations.Store(ctx.Ssid, loc)

	// 新连接未关联业务服时断线，会丢失Close消息
	if cmd.GetSession(ctx.Ssid) == nil {
		FUNC_Close(ctx, args)
	}
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

func S2C_ServerClose(ctx *cmd.Context, data interface{}) {
	client := ctx.Out.(*cmd.Client)
	for _, ss := range cmd.GetSessionList() {
		// 2020-11-24 仅通知在当前服务的连接
		if v, ok := sessionLocations.Load(ss.Id); ok {
			loc := v.(*sessionLocation)
			if loc.MatchServer == client.ServerName() {
				ss.Out.WriteJSON("ServerClose", map[string]string{"ServerName": loc.ServerName})
			}
		}
	}
	cmd.Forward("router", "C2S_QueryServerState", cmd.T{})
}

func HeartBeat(ctx *cmd.Context, data interface{}) {
	ctx.Out.WriteJSON("HeartBeat", struct{}{})
}

// 同步服务负载
func S2C_QueryServerState(ctx *cmd.Context, data interface{}) {
	args := data.(*Args)

	serverStateMu.Lock()
	defer serverStateMu.Unlock()
	serverStates = map[string]*serverState{}
	for _, state := range args.Servers {
		serverStates[state.Name] = state
	}
}

package main

import (
	"encoding/json"

	"github.com/guogeer/quasar/cmd"
	"github.com/guogeer/quasar/log"
)

type gatewayArgs struct {
	Id   string
	Data json.RawMessage

	ServerId    string
	ServerName  string
	MatchServer string // 匹配的ServerId

	Name    string
	Servers []*serverState
}

func init() {
	cmd.BindWithoutQueue("FUNC_Route", FUNC_Route, (*gatewayArgs)(nil))
	cmd.BindWithoutQueue("HeartBeat", HeartBeat, (*gatewayArgs)(nil))

	cmd.Bind(FUNC_Broadcast, (*gatewayArgs)(nil))
	cmd.Bind(FUNC_SwitchServer, (*gatewayArgs)(nil))
	cmd.Bind(FUNC_Close, (*gatewayArgs)(nil))
	cmd.Bind(S2C_ServerClose, (*gatewayArgs)(nil))
	cmd.Bind(S2C_QueryServerState, (*gatewayArgs)(nil))
	cmd.Bind(S2C_Register, (*gatewayArgs)(nil))
}

func FUNC_Close(ctx *cmd.Context, data interface{}) {
	log.Debugf("session close %s", ctx.Ssid)
	if v, ok := sessionLocations.Load(ctx.Ssid); ok {
		loc := v.(*sessionLocation)
		ss := &cmd.Session{Id: ctx.Ssid, Out: ctx.Out}
		ss.Route(loc.MatchServer, "Close", struct{}{})
	}
	sessionLocations.Delete(ctx.Ssid)
}

func FUNC_SwitchServer(ctx *cmd.Context, data interface{}) {
	args := data.(*gatewayArgs)
	log.Debugf("session ssid:%s switch request server:%s,match server:%s", ctx.Ssid, args.ServerName, args.MatchServer)
	loc := &sessionLocation{ServerName: args.ServerName, MatchServer: args.MatchServer}
	sessionLocations.Store(ctx.Ssid, loc)

	// 新连接未关联业务服时断线，会丢失Close消息
	if cmd.GetSession(ctx.Ssid) == nil {
		FUNC_Close(ctx, args)
	}
}

// 直接转发消息到客户端
func FUNC_Route(ctx *cmd.Context, data interface{}) {
	args := data.(*gatewayArgs)
	if ss := cmd.GetSession(ctx.Ssid); ss != nil {
		ss.Out.WriteJSON(args.Id, args.Data)
	}
}

func FUNC_Broadcast(ctx *cmd.Context, data interface{}) {
	args := data.(*gatewayArgs)
	for _, ss := range cmd.GetSessionList() {
		ss.Out.WriteJSON(args.Id, args.Data)
	}
}

func S2C_Register(ctx *cmd.Context, data interface{}) {
	cmd.Route("router", "C2S_QueryServerState", cmd.M{})
}

func S2C_ServerClose(ctx *cmd.Context, data interface{}) {
	args := data.(*gatewayArgs)
	// 2020-11-24 仅通知在当前服务的连接
	for _, ss := range cmd.GetSessionList() {
		if v, ok := sessionLocations.Load(ss.Id); ok {
			loc := v.(*sessionLocation)
			if loc.MatchServer == args.ServerId {
				ss.Out.WriteJSON("ServerClose", cmd.M{"ServerName": loc.ServerName})
			}
		}
	}
	cmd.Route("router", "C2S_QueryServerState", cmd.M{})
}

func HeartBeat(ctx *cmd.Context, data interface{}) {
	ctx.Out.WriteJSON("HeartBeat", cmd.M{})
}

// 同步服务负载
func S2C_QueryServerState(ctx *cmd.Context, data interface{}) {
	args := data.(*gatewayArgs)

	serverStateMu.Lock()
	defer serverStateMu.Unlock()
	serverStates = map[string]*serverState{}
	for _, state := range args.Servers {
		serverStates[state.Id] = state
	}
}

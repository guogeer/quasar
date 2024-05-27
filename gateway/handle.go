package main

import (
	"encoding/json"

	"github.com/guogeer/quasar/cmd"
	"github.com/guogeer/quasar/log"
)

type gatewayArgs struct {
	Id   string          `json:"id,omitempty"`
	Data json.RawMessage `json:"data,omitempty"`

	ServerId      string `json:"serverId,omitempty"`
	ServerName    string `json:"serverName,omitempty"`
	MatchServerId string `json:"matchServerId,omitempty"` // 匹配的ServerId

	Name    string        `json:"name,omitempty"`
	Servers []serverState `json:"servers,omitempty"`
}

func init() {
	cmd.BindFunc(FUNC_Route, (*gatewayArgs)(nil)).SetPrivate()
	cmd.BindFunc(HeartBeat, (*gatewayArgs)(nil)).SetNoQueue()
	cmd.BindFunc(FUNC_Broadcast, (*gatewayArgs)(nil)).SetPrivate()
	cmd.BindFunc(FUNC_SwitchServer, (*gatewayArgs)(nil)).SetPrivate()
	cmd.BindFunc(FUNC_Close, (*gatewayArgs)(nil)).SetPrivate()
	cmd.BindFunc(serverClose, (*gatewayArgs)(nil)).SetPrivate()
	cmd.BindFunc(S2C_QueryServerState, (*gatewayArgs)(nil)).SetPrivate()
	cmd.BindFunc(S2C_Register, (*gatewayArgs)(nil)).SetPrivate()
}

func closeSession(ss *cmd.Session) {
	log.Debugf("session close %s", ss.Id)
	if v, ok := sessionLocations.Load(ss.Id); ok {
		loc := v.(*sessionLocation)
		ss.Route(loc.MatchServerId, "close", struct{}{})
	}
	sessionLocations.Delete(ss.Id)
}

func FUNC_Close(ctx *cmd.Context, data any) {
	log.Debugf("session close %s", ctx.Ssid)
	closeSession(&cmd.Session{Id: ctx.Ssid, Out: ctx.Out})
}

func FUNC_SwitchServer(ctx *cmd.Context, data any) {
	args := data.(*gatewayArgs)
	log.Debugf("session ssid:%s switch request server:%s,match server:%s", ctx.Ssid, args.ServerName, args.MatchServerId)
	loc := &sessionLocation{ServerName: args.ServerName, MatchServerId: args.MatchServerId}
	sessionLocations.Store(ctx.Ssid, loc)

	// 新连接未关联业务服时断线，会丢失close消息
	if cmd.GetSession(ctx.Ssid) == nil {
		closeSession(&cmd.Session{Id: ctx.Ssid, Out: ctx.Out})
	}
}

// 直接转发消息到客户端
func FUNC_Route(ctx *cmd.Context, data any) {
	args := data.(*gatewayArgs)
	if ss := cmd.GetSession(ctx.Ssid); ss != nil {
		ss.Out.WriteJSON(args.Id, args.Data)
	}
}

func FUNC_Broadcast(ctx *cmd.Context, data any) {
	args := data.(*gatewayArgs)
	for _, ss := range cmd.GetSessionList() {
		ss.Out.WriteJSON(args.Id, args.Data)
	}
}

func S2C_Register(ctx *cmd.Context, data any) {
	cmd.Route("router", "c2s_queryServerState", cmd.M{})
}

func serverClose(ctx *cmd.Context, data any) {
	args := data.(*gatewayArgs)
	// 2020-11-24 仅通知在当前服务的连接
	for _, ss := range cmd.GetSessionList() {
		if v, ok := sessionLocations.Load(ss.Id); ok {
			loc := v.(*sessionLocation)
			if loc.MatchServerId == args.ServerId {
				ss.Out.WriteJSON("serverClose", cmd.M{"serverId": loc.ServerName, "cause": "server crash"})
			}
		}
	}
	cmd.Route("router", "c2s_queryServerState", cmd.M{})
}

func HeartBeat(ctx *cmd.Context, data any) {
	ctx.Out.WriteJSON("heartBeat", cmd.M{})
}

// 同步服务负载
func S2C_QueryServerState(ctx *cmd.Context, data any) {
	args := data.(*gatewayArgs)

	serverStateMu.Lock()
	defer serverStateMu.Unlock()
	serverStates = map[string]serverState{}
	for _, state := range args.Servers {
		serverStates[state.Id] = state
	}
}

package cmd

import (
	"sync"
)

type Session struct {
	Id  string
	Out Conn
}

func (ss *Session) GetServerName() string {
	if client, ok := ss.Out.(*Client); ok {
		return client.serverName
	}
	return ""
}

func (ss *Session) routeContext(ctx *Context, name string, i interface{}) {
	pkg := &Package{
		Id:         name,
		Body:       i,
		Ssid:       ss.Id,
		ServerName: ctx.ServerName,
		ClientAddr: ctx.ClientAddr,
	}
	buf, err := EncodePackage(pkg)
	if err != nil {
		return
	}
	routeMsg(ctx.MatchServer, buf)
}

func (ss *Session) Route(serverName, name string, i interface{}) {
	pkg := &Package{
		Id:         name,
		Ssid:       ss.Id,
		ServerName: serverName,
	}
	buf, err := EncodePackage(pkg)
	if err != nil {
		return
	}
	routeMsg(serverName, buf)
}

func (ss *Session) WriteJSON(name string, i interface{}) {
	pkg := &Package{Id: name, Body: i, Ssid: ss.Id}
	buf, err := EncodePackage(pkg)
	if err != nil {
		return
	}
	ss.Out.Write(buf)
}

var (
	sessions  = map[string]*Session{}
	sessionMu sync.RWMutex
)

func AddSession(s *Session) {
	sessionMu.Lock()
	defer sessionMu.Unlock()
	if _, ok := sessions[s.Id]; ok {
		panic("add same session " + s.Id)
	}
	sessions[s.Id] = s
}

func RemoveSession(id string) {
	sessionMu.Lock()
	defer sessionMu.Unlock()
	delete(sessions, id)
}

func GetSession(id string) *Session {
	sessionMu.RLock()
	defer sessionMu.RUnlock()
	return sessions[id]
}

func GetSessionList() []*Session {
	sessionMu.RLock()
	defer sessionMu.RUnlock()

	var all []*Session
	for _, ss := range sessions {
		all = append(all, ss)
	}
	return all
}

func CountSession() int {
	sessionMu.RLock()
	defer sessionMu.RUnlock()
	return len(sessions)
}

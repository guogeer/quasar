package cmd

import (
	"github.com/guogeer/husky/log"
	"sync"
)

type Session struct {
	Id  string
	Out Conn
}

func (ss *Session) GetServerName() string {
	client := ss.Out.(*Client)
	return client.name
}

func (ss *Session) Route(serverName, name string, i interface{}) {
	pkg := &Package{Id: name, Body: i, Ssid: ss.Id, IsRaw: true}
	buf, err := Encode(pkg)
	if err != nil {
		return
	}
	defaultClientManage.Route(serverName, buf)
}

func (ss *Session) WriteJSON(name string, i interface{}) {
	pkg := &Package{Id: name, Body: i, Ssid: ss.Id, IsRaw: true}
	buf, err := Encode(pkg)
	if err != nil {
		return
	}
	ss.Out.Write(buf)
}

type SessionManage struct {
	sessions map[string]*Session
	mu       sync.RWMutex
}

var defaultSessionManage = &SessionManage{sessions: make(map[string]*Session)}

func GetSessionManage() *SessionManage {
	return defaultSessionManage
}

func (sm *SessionManage) Add(s *Session) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if _, ok := sm.sessions[s.Id]; ok {
		log.Warnf("session %s exist", s.Id)
		return
	}
	sm.sessions[s.Id] = s
}

func (sm *SessionManage) Del(id string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.sessions, id)
}

func (sm *SessionManage) Get(id string) *Session {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	s, ok := sm.sessions[id]
	if !ok {
		// log.Debugf("session [%s] not exist", id)
	}
	return s
}

func (sm *SessionManage) GetList() []*Session {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	var active []*Session
	for _, ss := range sm.sessions {
		active = append(active, ss)
	}
	return active
}

func (sm *SessionManage) Count() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.sessions)
}

func addSession(s *Session) {
	defaultSessionManage.Add(s)
}

func removeSession(id string) {
	defaultSessionManage.Del(id)
}

func GetSession(id string) *Session {
	return defaultSessionManage.Get(id)
}

func GetSessionList() []*Session {
	return defaultSessionManage.GetList()
}

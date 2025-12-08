package ws

import (
	"sync"

	"github.com/ether/etherpad-go/lib/models/ws"
)

type SessionStore struct {
	sessions map[string]*ws.Session
	sync     sync.RWMutex
}

// NewSessionStore
// @param refresh *int Number of milliseconds to refresh the session
//
// /*
func NewSessionStore() SessionStore {
	return SessionStore{
		sessions: make(map[string]*ws.Session),
	}
}

func (s *SessionStore) initSession(sessionId string) {
	s.sync.Lock()
	s.sessions[sessionId] = &ws.Session{}
	s.sync.Unlock()
}

func (s *SessionStore) addHandleClientInformation(sessionId string, padId string, token string) *ws.Session {
	s.sync.Lock()
	s.sessions[sessionId] = &ws.Session{
		Auth: &ws.SessionAuth{
			Token:     token,
			PadId:     padId,
			SessionId: sessionId,
		},
	}
	s.sync.Unlock()
	return s.sessions[sessionId]
}

func (s *SessionStore) addPadReadOnlyIds(sessionId, padId string, readOnlyPadId string, readOnly bool) {
	s.sync.Lock()
	s.sessions[sessionId].ReadOnlyPadId = readOnlyPadId
	s.sessions[sessionId].PadId = padId
	s.sessions[sessionId].ReadOnly = readOnly
	s.sync.Unlock()
}

func (s *SessionStore) addFinalInformation(sessionId, padId, readOnlyPadId string, readonly bool) {
	s.sync.Lock()
	var session = s.sessions[sessionId]

	session.PadId = padId
	session.ReadOnlyPadId = readOnlyPadId
	session.ReadOnly = readonly
	s.sync.Unlock()
}

func (s *SessionStore) removeSession(sessionId string) {
	s.sync.Lock()
	delete(s.sessions, sessionId)
	s.sync.Unlock()
}

func (s *SessionStore) hasSession(sessionId string) bool {
	s.sync.RLock()
	_, ok := s.sessions[sessionId]
	s.sync.RUnlock()
	return ok
}

func (s *SessionStore) getSession(sessionId string) *ws.Session {
	s.sync.RLock()
	defer s.sync.RUnlock()
	return s.sessions[sessionId]
}

func (s *SessionStore) resetSession(sessionId string) {
	s.sync.Lock()
	s.sessions[sessionId] = &ws.Session{}
	s.sync.Unlock()
}

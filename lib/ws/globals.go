package ws

import "github.com/ether/etherpad-go/lib/models/ws"

type SessionStore struct {
	sessions map[string]*ws.Session
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
	s.sessions[sessionId] = &ws.Session{}
}

func (s *SessionStore) addHandleClientInformation(sessionId string, padId string, token string) *ws.Session {
	s.sessions[sessionId] = &ws.Session{
		Auth: &ws.SessionAuth{
			Token:     token,
			PadId:     padId,
			SessionId: sessionId,
		},
	}
	return s.sessions[sessionId]
}

func (s *SessionStore) addPadReadOnlyIds(sessionId, padId string, readOnlyPadId string, readOnly bool) {
	s.sessions[sessionId].ReadOnlyPadId = readOnlyPadId
	s.sessions[sessionId].PadId = padId
	s.sessions[sessionId].ReadOnly = readOnly
}

func (s *SessionStore) addFinalInformation(sessionId, padId, readOnlyPadId string, readonly bool) {
	var session = s.sessions[sessionId]

	session.PadId = padId
	session.ReadOnlyPadId = readOnlyPadId
	session.ReadOnly = readonly

}

func (s *SessionStore) removeSession(sessionId string) {
	delete(s.sessions, sessionId)
}

func (s *SessionStore) hasSession(sessionId string) bool {
	_, ok := s.sessions[sessionId]
	return ok
}

func (s *SessionStore) getSession(sessionId string) *ws.Session {
	return s.sessions[sessionId]
}

func (s *SessionStore) resetSession(sessionId string) {
	s.sessions[sessionId] = &ws.Session{}
}

package ws

import (
	"sync"

	"github.com/ether/etherpad-go/lib/models/ws"
)

type SessionStore struct {
	sessions map[string]*ws.Session
	sync     sync.RWMutex
}

type SessionStat struct {
	ActivePads  int
	ActiveUsers int
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

func (s *SessionStore) GetStats() (SessionStat, error) {
	var stats SessionStat
	s.sync.RLock()
	stats.ActiveUsers = len(s.sessions)
	padSet := make(map[string]struct{})
	for _, session := range s.sessions {
		if session.PadId != "" {
			padSet[session.PadId] = struct{}{}
		}
	}
	stats.ActivePads = len(padSet)
	s.sync.RUnlock()
	return stats, nil
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

// Test helper methods - these are exported for testing purposes only

// InitSessionForTest initializes a session for testing
func (s *SessionStore) InitSessionForTest(sessionId string) {
	s.initSession(sessionId)
}

// AddHandleClientInformationForTest adds client information for testing
func (s *SessionStore) AddHandleClientInformationForTest(sessionId string, padId string, token string) *ws.Session {
	return s.addHandleClientInformation(sessionId, padId, token)
}

// AddPadReadOnlyIdsForTest adds pad read-only IDs for testing
func (s *SessionStore) AddPadReadOnlyIdsForTest(sessionId, padId string, readOnlyPadId string, readOnly bool) {
	s.addPadReadOnlyIds(sessionId, padId, readOnlyPadId, readOnly)
}

// SetAuthorForTest sets the author for a session for testing
func (s *SessionStore) SetAuthorForTest(sessionId string, authorId string) {
	s.sync.Lock()
	if s.sessions[sessionId] != nil {
		s.sessions[sessionId].Author = authorId
	}
	s.sync.Unlock()
}

// SetPadIdForTest sets the pad ID for a session for testing
func (s *SessionStore) SetPadIdForTest(sessionId string, padId string) {
	s.sync.Lock()
	if s.sessions[sessionId] != nil {
		s.sessions[sessionId].PadId = padId
	}
	s.sync.Unlock()
}

// SetRevisionForTest sets the revision for a session for testing
func (s *SessionStore) SetRevisionForTest(sessionId string, revision int) {
	s.sync.Lock()
	if s.sessions[sessionId] != nil {
		s.sessions[sessionId].Revision = revision
	}
	s.sync.Unlock()
}

// SetReadOnlyForTest sets the read-only flag for a session for testing
func (s *SessionStore) SetReadOnlyForTest(sessionId string, readOnly bool) {
	s.sync.Lock()
	if s.sessions[sessionId] != nil {
		s.sessions[sessionId].ReadOnly = readOnly
	}
	s.sync.Unlock()
}

// GetSessionForTest returns a session for testing
func (s *SessionStore) GetSessionForTest(sessionId string) *ws.Session {
	return s.getSession(sessionId)
}

// RemoveSessionForTest removes a session for testing
func (s *SessionStore) RemoveSessionForTest(sessionId string) {
	s.removeSession(sessionId)
}

package ws

func init() {
	SessionStoreInstance = NewSessionStore()
}

type SessionAuth struct {
	PadId     string
	Token     string
	SessionId string
}

type Session struct {
	Author        string
	Auth          *SessionAuth
	revision      int
	PadId         string
	ReadOnlyPadId string
	ReadOnly      bool
	Time          int
}

type SessionStore struct {
	sessions map[string]*Session
}

func NewSessionStore() SessionStore {
	return SessionStore{
		sessions: make(map[string]*Session),
	}
}

func (s *SessionStore) initSession(sessionId string) {
	s.sessions[sessionId] = &Session{}
}

func (s *SessionStore) addHandleClientInformation(sessionId string, padId string, token string) *Session {
	s.sessions[sessionId] = &Session{
		Auth: &SessionAuth{
			Token:     token,
			PadId:     padId,
			SessionId: sessionId,
		},
	}
	return s.sessions[sessionId]
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

func (s *SessionStore) getSession(sessionId string) *Session {
	return s.sessions[sessionId]
}

func (s *SessionStore) resetSession(sessionId string) {
	s.sessions[sessionId] = &Session{}
}

var HubGlob *Hub

var SessionStoreInstance SessionStore

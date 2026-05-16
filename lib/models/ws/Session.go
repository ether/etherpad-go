package ws

type SessionAuth struct {
	PadId     string
	Token     string
	SessionId string
	// IntegratorSessionID is the integrator-set sessionID cookie value
	// (from createSession() HTTP API), read from the socket.io handshake
	// Cookie header so the cookie can be HttpOnly. Falls back to the
	// deprecated CLIENT_READY in-message `sessionID` field for legacy
	// clients. Upstream #7045 / #7755.
	IntegratorSessionID string
}

type Session struct {
	Author        string
	Auth          *SessionAuth
	Revision      int
	PadId         string
	ReadOnlyPadId string
	ReadOnly      bool
	Time          int64
}

package ws

type ClientReady struct {
	Event string          `json:"event"`
	Data  ClientReadyData `json:"data"`
}

type ClientReadyData struct {
	Component string              `json:"component"`
	Type      string              `json:"type"`
	PadID     string              `json:"padId"`
	Token     string              `json:"token"`
	// SessionID is the deprecated legacy field for the integrator-set
	// sessionID cookie value, kept for backward compatibility with old
	// clients. Current clients no longer forward it; the server now reads
	// the cookie directly from the socket.io handshake so the cookie can
	// be marked HttpOnly. Upstream #7045 / #7755.
	SessionID string              `json:"sessionID,omitempty"`
	UserInfo  ClientReadyUserInfo `json:"userInfo"`
	Reconnect *bool               `json:"reconnect"`
	ClientRev *int                `json:"client_rev"`
}

type ClientReadyUserInfo struct {
	ColorId *string `json:"colorId"`
	Name    *string `json:"name"`
}

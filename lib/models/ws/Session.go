package ws

type SessionAuth struct {
	PadId     string
	Token     string
	SessionId string
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

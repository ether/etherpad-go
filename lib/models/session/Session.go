package session

type Session struct {
	Id             string
	OriginalMaxAge int
	Expires        string
	Secure         bool
	HttpOnly       bool
	Path           string
	SameSite       string
	Connections    string
}

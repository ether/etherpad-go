package session

type Session struct {
	OriginalMaxAge int
	Expires        string
	Secure         bool
	HttpOnly       bool
	Path           string
	SameSite       string
}

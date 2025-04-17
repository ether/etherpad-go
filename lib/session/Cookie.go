package session

import "time"

type CookieOptions struct {
	Expires        *time.Time
	OriginalMaxAge *time.Time
}

type Cookie struct {
	Path     string
	MaxAge   *int
	HttpOnly bool
}

func NewCookie(options *CookieOptions) Cookie {
	var cookie = Cookie{
		Path:     "/",
		MaxAge:   nil,
		HttpOnly: true,
	}

	if options != nil {

	}

	return
}

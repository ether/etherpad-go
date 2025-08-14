package session

import (
	"time"
)

type CookieData struct {
	OriginalMaxAge *int64
	Expires        *int64
	Secure         bool
	HttpOnly       bool
	Domain         string
	Path           string
	SameSite       string
}

type Cookie struct {
	Path           string
	Domain         string
	SameSite       string
	MaxAge         *int
	HttpOnly       bool
	expires        int64
	originalMaxAge *int64
	Secure         bool
}

func NewCookie(options *Cookie) Cookie {
	var cookie = Cookie{
		Path:     "/",
		MaxAge:   nil,
		HttpOnly: true,
	}

	if options != nil {
		if options.Path != "" {
			cookie.Path = options.Path
		}
		if options.MaxAge != nil {
			cookie.MaxAge = options.MaxAge
		}
		if options.HttpOnly {
			cookie.HttpOnly = true
		}
		if options.expires != 0 {
			cookie.expires = options.expires
		} else {
			cookie.expires = time.Now().Add(24 * time.Hour).UnixMilli()
		}
	} else {
		cookie.expires = time.Now().Add(24 * time.Hour).UnixMilli()
	}

	if cookie.originalMaxAge == nil {
		var maxAge = cookie.GetMaxAge()
		cookie.originalMaxAge = &maxAge
	}

	return cookie
}

// SetExpires sets the max age of the cookie
func (c *Cookie) SetExpires(expires time.Time) {
	c.expires = expires.UnixMilli()
	var maxAge = c.GetMaxAge()
	c.originalMaxAge = &maxAge
}

// GetExpires returns the expiration time of the cookie
func (c *Cookie) GetExpires() int64 {
	return c.expires
}

func (c *Cookie) GetMaxAge() int64 {
	return c.expires
}

func (c *Cookie) SetMaxAge(ms *int64) {
	c.expires = *ms
}

func (c *Cookie) GetData() CookieData {
	return CookieData{
		OriginalMaxAge: c.originalMaxAge,
		Expires:        &c.expires,
		Secure:         c.Secure,
		HttpOnly:       c.HttpOnly,
		Domain:         c.Domain,
		Path:           c.Path,
		SameSite:       c.SameSite,
	}
}

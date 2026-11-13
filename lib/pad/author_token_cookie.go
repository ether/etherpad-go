package pad

import (
	"time"

	"github.com/ether/etherpad-go/lib/settings"
	"github.com/ether/etherpad-go/lib/utils"
	"github.com/gofiber/fiber/v3"
)

// authorTokenCookieLifetime is how long the browser keeps the token (the
// client used to set it for 60 days).
const authorTokenCookieLifetime = 60 * 24 * time.Hour

func authorTokenCookieName(cfg *settings.Settings) string {
	if cfg == nil {
		return "token"
	}
	return cfg.Cookie.Prefix + "token"
}

// AuthorTokenFromCookies is the author token of the request's cookies, or
// "" when there is none or it is malformed: the prefixed name first, then
// the plain "token" earlier versions wrote from JavaScript.
func AuthorTokenFromCookies(cookie func(key string, defaultValue ...string) string, cfg *settings.Settings) string {
	for _, name := range []string{authorTokenCookieName(cfg), "token"} {
		if v := cookie(name); v != "" && utils.IsValidAuthorToken(v) {
			return v
		}
	}
	return ""
}

// authorTokenCookieFlags: Secure over HTTPS; embedded in another site (an
// LMS, a wiki) the cookie has to be sent in that context, which needs
// SameSite=None, and that needs Secure.
func authorTokenCookieFlags(https bool, fetchSite string) (secure bool, sameSite string) {
	if https && fetchSite == "cross-site" {
		return true, fiber.CookieSameSiteNoneMode
	}
	return https, fiber.CookieSameSiteLaxMode
}

// EnsureAuthorTokenCookie makes sure the browser has an HttpOnly author
// token cookie and returns the token (etherpad-lite's
// ensureAuthorTokenCookie). The token is the author's identity; set from
// JavaScript, as before, any script running in the page could read it and
// act as the author (delete pads they created, for one). A valid token the
// browser already has is kept, so authors keep their identity; a cookie that
// is not HttpOnly yet is rewritten as HttpOnly with the same value.
func EnsureAuthorTokenCookie(c fiber.Ctx, cfg *settings.Settings) string {
	name := authorTokenCookieName(cfg)
	token := AuthorTokenFromCookies(c.Cookies, cfg)
	// The cookie is written on every page load, also when the browser has
	// it: the server cannot tell whether it is still the JavaScript cookie
	// of an older version, and the write moves the expiry forward.
	if token == "" {
		token = utils.GenerateAuthorToken()
	}
	secure, sameSite := authorTokenCookieFlags(c.Scheme() == "https", c.Get("Sec-Fetch-Site"))
	c.Cookie(&fiber.Cookie{
		Name:     name,
		Value:    token,
		Path:     "/",
		HTTPOnly: true,
		Secure:   secure,
		SameSite: sameSite,
		Expires:  time.Now().Add(authorTokenCookieLifetime),
	})
	if name != "token" && c.Cookies("token") != "" {
		// The older, JavaScript-readable cookie is not needed any more.
		c.Cookie(&fiber.Cookie{Name: "token", Value: "", Path: "/", Expires: time.Unix(0, 0), MaxAge: -1})
	}
	return token
}

package pad

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ether/etherpad-go/lib/settings"
	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Port of etherpad-lite's ensureAuthorTokenCookie and authorTokenCookie
// specs.
func TestEnsureAuthorTokenCookie(t *testing.T) {
	cfg := &settings.Settings{}
	cfg.Cookie.Prefix = "ep_"
	var got string
	app := fiber.New()
	app.Get("/p/:pad", func(c fiber.Ctx) error {
		got = EnsureAuthorTokenCookie(c, cfg)
		return c.SendString("ok")
	})
	do := func(t *testing.T, mod func(*http.Request)) *http.Response {
		t.Helper()
		req := httptest.NewRequest("GET", "/p/x", nil)
		if mod != nil {
			mod(req)
		}
		resp, err := app.Test(req)
		require.NoError(t, err)
		return resp
	}
	cookieOf := func(resp *http.Response, name string) *http.Cookie {
		for _, c := range resp.Cookies() {
			if c.Name == name {
				return c
			}
		}
		return nil
	}

	t.Run("mints an HttpOnly t.* token when there is none", func(t *testing.T) {
		resp := do(t, nil)
		c := cookieOf(resp, "ep_token")
		require.NotNil(t, c)
		assert.True(t, strings.HasPrefix(c.Value, "t."))
		assert.Equal(t, got, c.Value)
		assert.True(t, c.HttpOnly)
		assert.Equal(t, http.SameSiteLaxMode, c.SameSite)
		assert.Equal(t, "/", c.Path)
		assert.False(t, c.Secure)
	})
	t.Run("keeps the token the browser has", func(t *testing.T) {
		resp := do(t, func(r *http.Request) { r.AddCookie(&http.Cookie{Name: "ep_token", Value: "t.abcdefghij1234567890"}) })
		assert.Equal(t, "t.abcdefghij1234567890", got)
		c := cookieOf(resp, "ep_token")
		require.NotNil(t, c)
		assert.Equal(t, "t.abcdefghij1234567890", c.Value, "rewritten as HttpOnly with the same value")
	})
	t.Run("takes over the JavaScript cookie of older versions", func(t *testing.T) {
		resp := do(t, func(r *http.Request) { r.AddCookie(&http.Cookie{Name: "token", Value: "t.legacyToken1234567"}) })
		assert.Equal(t, "t.legacyToken1234567", got, "the author keeps their identity")
		require.NotNil(t, cookieOf(resp, "ep_token"))
		old := cookieOf(resp, "token")
		require.NotNil(t, old, "the readable cookie is removed")
		assert.Empty(t, old.Value)
	})
	t.Run("Secure over HTTPS, SameSite=None when embedded cross-site", func(t *testing.T) {
		secure, sameSite := authorTokenCookieFlags(true, "")
		assert.True(t, secure)
		assert.Equal(t, fiber.CookieSameSiteLaxMode, sameSite)
		secure, sameSite = authorTokenCookieFlags(true, "cross-site")
		assert.True(t, secure)
		assert.Equal(t, fiber.CookieSameSiteNoneMode, sameSite)
		secure, sameSite = authorTokenCookieFlags(false, "cross-site")
		assert.False(t, secure)
		assert.Equal(t, fiber.CookieSameSiteLaxMode, sameSite, "None needs Secure")
	})
	t.Run("ignores an invalid cookie and mints a fresh one", func(t *testing.T) {
		resp := do(t, func(r *http.Request) { r.AddCookie(&http.Cookie{Name: "ep_token", Value: "not-a-token"}) })
		c := cookieOf(resp, "ep_token")
		require.NotNil(t, c)
		assert.True(t, strings.HasPrefix(c.Value, "t."))
		assert.NotEqual(t, "not-a-token", c.Value)
	})
}

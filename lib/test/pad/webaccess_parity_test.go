package pad

import (
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/hooks"
	"github.com/ether/etherpad-go/lib/hooks/events"
	"github.com/ether/etherpad-go/lib/pad"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// Port of etherpad-lite's src/tests/backend/specs/webaccess.ts.

func strPtr(s string) *string { return &s }
func boolPtr(b bool) *bool    { return &b }

func parityUsers() map[string]settings.User {
	return map[string]settings.User{
		"admin": {Password: strPtr("admin-password"), IsAdmin: boolPtr(true)},
		"user":  {Password: strPtr("user-password")},
	}
}

func paritySettings(authn, authz bool) *settings.Settings {
	return &settings.Settings{RequireAuthentication: authn, RequireAuthorization: authz, Users: parityUsers()}
}

// parityApp has the routes the upstream spec requests: "/", a pad, a
// static file and robots.txt. /admin-auth/ is answered by the middleware.
func parityApp(hookSystem *hooks.Hook, cfg *settings.Settings) *fiber.App {
	app := fiber.New()
	readOnlyManager := pad.NewReadOnlyManager(db.NewMemoryDataStore())
	logger := zap.NewNop().Sugar()
	app.Use(func(c fiber.Ctx) error {
		return pad.CheckAccessWithHooks(c, logger, cfg, readOnlyManager, hookSystem)
	})
	ok := func(c fiber.Ctx) error { return c.SendString("ok") }
	app.Get("/", ok)
	app.Get("/p/*", ok)
	app.Get("/robots.txt", ok)
	app.Get("/static/*", ok)
	return app
}

func creds(userpass string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(userpass))
}

func do(t *testing.T, app *fiber.App, path, auth string) (*http.Response, string) {
	t.Helper()
	req := httptest.NewRequest("GET", path, nil)
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	resp, err := app.Test(req, adminTestConfig)
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	return resp, string(body)
}

func TestWebAccessWithoutPlugins(t *testing.T) {
	admin, user := creds("admin:admin-password"), creds("user:user-password")
	cases := []struct {
		name         string
		authn, authz bool
		path, auth   string
		want         int
	}{
		{"!authn !authz anonymous / -> 200", false, false, "/", "", 200},
		{"!authn !authz anonymous /admin-auth// -> 401", false, false, "/admin-auth//", "", 401},
		{"authn !authz anonymous / -> 401", true, false, "/", "", 401},
		{"authn !authz user / -> 200", true, false, "/", user, 200},
		{"authn !authz user //admin-auth// -> 403", true, false, "//admin-auth//", user, 403},
		{"authn !authz admin / -> 200", true, false, "/", admin, 200},
		{"authn !authz admin /admin-auth/ -> 200", true, false, "/admin-auth/", admin, 200},
		{"authn authz anonymous /robots.txt -> 200", true, true, "/robots.txt", "", 200},
		{"authn authz user / -> 403", true, true, "/", user, 403},
		{"authn authz user //admin-auth// -> 403", true, true, "//admin-auth//", user, 403},
		{"authn authz admin / -> 200", true, true, "/", admin, 200},
		{"authn authz admin /admin-auth/ -> 200", true, true, "/admin-auth/", admin, 200},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, _ := do(t, parityApp(nil, paritySettings(tc.authn, tc.authz)), tc.path, tc.auth)
			assert.Equal(t, tc.want, resp.StatusCode)
		})
	}

	// Reported alongside GHSA-62cj-9j72-mfrh.
	for _, c := range []string{"admin:", "admin", "admin:anything"} {
		t.Run("login fails if password is an empty string, credentials: "+c, func(t *testing.T) {
			cfg := paritySettings(false, false)
			cfg.Users["admin"] = settings.User{Password: strPtr(""), IsAdmin: boolPtr(true)}
			resp, _ := do(t, parityApp(nil, cfg), "/admin-auth/", creds(c))
			assert.Equal(t, 401, resp.StatusCode)
		})
		t.Run("login fails if password is nullish, credentials: "+c, func(t *testing.T) {
			cfg := paritySettings(false, false)
			cfg.Users["admin"] = settings.User{Password: nil, IsAdmin: boolPtr(true)}
			resp, _ := do(t, parityApp(nil, cfg), "/admin-auth/", creds(c))
			assert.Equal(t, 401, resp.StatusCode)
		})
	}
}

// Credentials the server must reject without crashing (a missing ':' used
// to panic and end the process) and passwords that contain ':'.
func TestWebAccessMalformedCredentials(t *testing.T) {
	cfg := paritySettings(true, false)
	for name, header := range map[string]string{
		"no colon":         creds("user"),
		"invalid base64":   "Basic %%%not-base64%%%",
		"empty":            "Basic ",
		"no scheme value":  "Basic",
		"other scheme":     "Bearer abc",
		"only a colon":     creds(":"),
		"unknown user":     creds("nobody:user-password"),
		"password prefix":  creds("user:user-pass"),
		"password + extra": creds("user:user-password:x"),
	} {
		t.Run(name, func(t *testing.T) {
			resp, _ := do(t, parityApp(nil, cfg), "/p/pad", header)
			assert.Equal(t, 401, resp.StatusCode)
			resp, _ = do(t, parityApp(nil, cfg), "/admin-auth/", header)
			assert.Equal(t, 401, resp.StatusCode)
		})
	}

	t.Run("a password may contain ':'", func(t *testing.T) {
		cfg := paritySettings(true, false)
		cfg.Users["colon"] = settings.User{Password: strPtr("pa:ss:word")}
		resp, _ := do(t, parityApp(nil, cfg), "/p/pad", creds("colon:pa:ss:word"))
		assert.Equal(t, 200, resp.StatusCode)
	})

	t.Run("the scheme is case-insensitive", func(t *testing.T) {
		resp, _ := do(t, parityApp(nil, cfg), "/p/pad", "basic "+base64.StdEncoding.EncodeToString([]byte("user:user-password")))
		assert.Equal(t, 200, resp.StatusCode)
	})
}

func TestWebAccessPreAuthorize(t *testing.T) {
	t.Run("defers if no callback answers", func(t *testing.T) {
		h := hooks.NewHook()
		called := 0
		h.EnqueuePreAuthorizeHook(func(*events.PreAuthorizeContext) { called++ })
		resp, _ := do(t, parityApp(&h, paritySettings(false, false)), "/", "")
		assert.Equal(t, 200, resp.StatusCode)
		assert.Equal(t, 1, called)
	})

	for _, permit := range []bool{true, false} {
		t.Run("bypasses authenticate and authorize when it answers", func(t *testing.T) {
			h := hooks.NewHook()
			var authn, authz int
			h.EnqueuePreAuthorizeHook(func(c *events.PreAuthorizeContext) {
				if permit {
					c.Permit()
				} else {
					c.Deny()
				}
			})
			h.EnqueueAuthenticateHook(func(*events.AuthenticateContext) { authn++ })
			h.EnqueueAuthorizeHook(func(*events.AuthorizeContext) { authz++ })
			resp, _ := do(t, parityApp(&h, paritySettings(true, true)), "/", "")
			if permit {
				assert.Equal(t, 200, resp.StatusCode)
			} else {
				assert.Equal(t, 403, resp.StatusCode)
			}
			assert.Zero(t, authn)
			assert.Zero(t, authz)
		})
	}

	t.Run("static content bypasses all auth checks", func(t *testing.T) {
		h := hooks.NewHook()
		h.EnqueuePreAuthorizeHook(func(c *events.PreAuthorizeContext) { c.Deny() })
		for _, path := range []string{"/static/robots.txt", "/robots.txt", "/static/js/x.js"} {
			resp, _ := do(t, parityApp(&h, paritySettings(true, true)), path, "")
			assert.Equal(t, 200, resp.StatusCode, path)
		}
	})

	t.Run("a path with .. below a public prefix is not public", func(t *testing.T) {
		resp, _ := do(t, parityApp(nil, paritySettings(true, false)), "/static/../p/pad", "")
		assert.NotEqual(t, 200, resp.StatusCode)
	})

	t.Run("cannot grant access to /admin-auth", func(t *testing.T) {
		h := hooks.NewHook()
		h.EnqueuePreAuthorizeHook(func(c *events.PreAuthorizeContext) { c.Permit() })
		resp, _ := do(t, parityApp(&h, paritySettings(false, false)), "/admin-auth/", "")
		assert.Equal(t, 401, resp.StatusCode)
		resp, _ = do(t, parityApp(&h, paritySettings(false, false)), "/admin-auth/", creds("admin:admin-password"))
		assert.Equal(t, 200, resp.StatusCode)
	})

	t.Run("can deny access to /admin-auth", func(t *testing.T) {
		h := hooks.NewHook()
		h.EnqueuePreAuthorizeHook(func(c *events.PreAuthorizeContext) { c.Deny() })
		resp, _ := do(t, parityApp(&h, paritySettings(false, false)), "/admin-auth/", creds("admin:admin-password"))
		assert.Equal(t, 403, resp.StatusCode)
	})

	t.Run("a panicking callback answers 500 instead of crashing", func(t *testing.T) {
		h := hooks.NewHook()
		h.EnqueuePreAuthorizeHook(func(*events.PreAuthorizeContext) { panic("plugin bug") })
		resp, _ := do(t, recoveringApp(&h, paritySettings(false, false)), "/", "")
		assert.Equal(t, 500, resp.StatusCode)
	})
}

func TestWebAccessAuthenticate(t *testing.T) {
	t.Run("is not called if !requireAuthentication and not /admin-auth", func(t *testing.T) {
		h := hooks.NewHook()
		called := 0
		h.EnqueueAuthenticateHook(func(*events.AuthenticateContext) { called++ })
		resp, _ := do(t, parityApp(&h, paritySettings(false, false)), "/", "")
		assert.Equal(t, 200, resp.StatusCode)
		assert.Zero(t, called)
	})

	t.Run("is called if !requireAuthentication and /admin-auth", func(t *testing.T) {
		h := hooks.NewHook()
		called := 0
		h.EnqueueAuthenticateHook(func(*events.AuthenticateContext) { called++ })
		resp, _ := do(t, parityApp(&h, paritySettings(false, false)), "/admin-auth/", "")
		assert.Equal(t, 401, resp.StatusCode)
		assert.Equal(t, 1, called)
	})

	t.Run("defers if no callback answers, then fails without credentials", func(t *testing.T) {
		h := hooks.NewHook()
		h.EnqueueAuthenticateHook(func(*events.AuthenticateContext) {})
		resp, _ := do(t, parityApp(&h, paritySettings(true, false)), "/", "")
		assert.Equal(t, 401, resp.StatusCode)
	})

	t.Run("does not defer when it authenticates: 200", func(t *testing.T) {
		h := hooks.NewHook()
		h.EnqueueAuthenticateHook(func(c *events.AuthenticateContext) { c.Authenticate("plugin") })
		resp, _ := do(t, parityApp(&h, paritySettings(true, false)), "/", "")
		assert.Equal(t, 200, resp.StatusCode)
	})

	t.Run("does not defer when it rejects: 401, even with valid credentials", func(t *testing.T) {
		h := hooks.NewHook()
		h.EnqueueAuthenticateHook(func(c *events.AuthenticateContext) { c.Reject() })
		resp, _ := do(t, parityApp(&h, paritySettings(true, false)), "/", creds("user:user-password"))
		assert.Equal(t, 401, resp.StatusCode)
	})

	t.Run("falls back to HTTP basic auth", func(t *testing.T) {
		h := hooks.NewHook()
		h.EnqueueAuthenticateHook(func(*events.AuthenticateContext) {})
		resp, _ := do(t, parityApp(&h, paritySettings(true, false)), "/", creds("user:user-password"))
		assert.Equal(t, 200, resp.StatusCode)
	})

	t.Run("passes settings.users in context", func(t *testing.T) {
		h := hooks.NewHook()
		var users map[string]settings.User
		h.EnqueueAuthenticateHook(func(c *events.AuthenticateContext) { users = c.Users })
		do(t, parityApp(&h, paritySettings(true, false)), "/", "")
		assert.Contains(t, users, "admin")
		assert.Contains(t, users, "user")
	})

	t.Run("passes user and password if provided", func(t *testing.T) {
		h := hooks.NewHook()
		var u, p string
		h.EnqueueAuthenticateHook(func(c *events.AuthenticateContext) { u, p = c.InputUsername, c.InputPassword })
		do(t, parityApp(&h, paritySettings(true, false)), "/", creds("user:user-password"))
		assert.Equal(t, "user", u)
		assert.Equal(t, "user-password", p)
	})

	t.Run("does not pass user and password if not provided", func(t *testing.T) {
		h := hooks.NewHook()
		u, p := "unset", "unset"
		h.EnqueueAuthenticateHook(func(c *events.AuthenticateContext) { u, p = c.InputUsername, c.InputPassword })
		do(t, parityApp(&h, paritySettings(true, false)), "/", "")
		assert.Empty(t, u)
		assert.Empty(t, p)
	})

	t.Run("a panicking callback answers 500 instead of crashing", func(t *testing.T) {
		h := hooks.NewHook()
		h.EnqueueAuthenticateHook(func(*events.AuthenticateContext) { panic("plugin bug") })
		resp, _ := do(t, recoveringApp(&h, paritySettings(true, false)), "/", "")
		assert.Equal(t, 500, resp.StatusCode)
	})
}

func TestWebAccessAuthorize(t *testing.T) {
	user := creds("user:user-password")
	t.Run("is not called if !requireAuthorization", func(t *testing.T) {
		for _, path := range []string{"/", "/admin-auth/"} {
			h := hooks.NewHook()
			called := 0
			h.EnqueueAuthorizeHook(func(*events.AuthorizeContext) { called++ })
			auth := user
			if path != "/" {
				auth = creds("admin:admin-password")
			}
			resp, _ := do(t, parityApp(&h, paritySettings(true, false)), path, auth)
			assert.Equal(t, 200, resp.StatusCode, path)
			assert.Zero(t, called, path)
		}
	})

	t.Run("defers if no callback answers: 403", func(t *testing.T) {
		h := hooks.NewHook()
		h.EnqueueAuthorizeHook(func(*events.AuthorizeContext) {})
		resp, _ := do(t, parityApp(&h, paritySettings(true, true)), "/", user)
		assert.Equal(t, 403, resp.StatusCode)
	})

	t.Run("grant: 200", func(t *testing.T) {
		h := hooks.NewHook()
		h.EnqueueAuthorizeHook(func(c *events.AuthorizeContext) { c.Grant("create") })
		resp, _ := do(t, parityApp(&h, paritySettings(true, true)), "/", user)
		assert.Equal(t, 200, resp.StatusCode)
	})

	t.Run("deny: 403", func(t *testing.T) {
		h := hooks.NewHook()
		h.EnqueueAuthorizeHook(func(c *events.AuthorizeContext) { c.Deny() })
		resp, _ := do(t, parityApp(&h, paritySettings(true, true)), "/", user)
		assert.Equal(t, 403, resp.StatusCode)
	})

	t.Run("passes the path", func(t *testing.T) {
		h := hooks.NewHook()
		var path string
		h.EnqueueAuthorizeHook(func(c *events.AuthorizeContext) { path = c.Path })
		do(t, parityApp(&h, paritySettings(true, true)), "/p/somePad", user)
		assert.Equal(t, "/p/somePad", path)
	})

	t.Run("a panicking callback answers 500 instead of crashing", func(t *testing.T) {
		h := hooks.NewHook()
		h.EnqueueAuthorizeHook(func(*events.AuthorizeContext) { panic("plugin bug") })
		resp, _ := do(t, recoveringApp(&h, paritySettings(true, true)), "/", user)
		assert.Equal(t, 500, resp.StatusCode)
	})
}

func TestWebAccessFailureHooks(t *testing.T) {
	user := creds("user:user-password")
	type hookSet struct{ authn, authz, auth bool }
	setup := func(set hookSet, calls *[]string) *hooks.Hook {
		h := hooks.NewHook()
		if set.authn {
			h.EnqueueAuthnFailureHook(func(c *events.AuthnFailureContext) {
				*calls = append(*calls, "authnFailure")
				c.Respond(200, "authnFailure")
			})
		}
		if set.authz {
			h.EnqueueAuthzFailureHook(func(c *events.AuthzFailureContext) {
				*calls = append(*calls, "authzFailure")
				c.Respond(200, "authzFailure")
			})
		}
		if set.auth {
			h.EnqueueAuthFailureHook(func(c *events.AuthFailureContext) {
				*calls = append(*calls, "authFailure")
				c.Respond(200, "authFailure")
			})
		}
		return &h
	}
	cases := []struct {
		name      string
		set       hookSet
		authz     bool
		auth      string
		wantCode  int
		wantBody  string
		wantCalls []string
	}{
		{"authn fail, no hooks handle -> 401", hookSet{}, false, "", 401, "", nil},
		{"authn fail, authnFailure handles", hookSet{authn: true}, false, "", 200, "authnFailure", []string{"authnFailure"}},
		{"authn fail, authFailure handles", hookSet{auth: true}, false, "", 200, "authFailure", []string{"authFailure"}},
		{"authnFailure trumps authFailure", hookSet{authn: true, auth: true}, false, "", 200, "authnFailure", []string{"authnFailure"}},
		{"authz fail, no hooks handle -> 403", hookSet{}, true, user, 403, "", nil},
		{"authz fail, authzFailure handles", hookSet{authz: true}, true, user, 200, "authzFailure", []string{"authzFailure"}},
		{"authz fail, authFailure handles", hookSet{auth: true}, true, user, 200, "authFailure", []string{"authFailure"}},
		{"authzFailure trumps authFailure", hookSet{authz: true, auth: true}, true, user, 200, "authzFailure", []string{"authzFailure"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var calls []string
			resp, body := do(t, parityApp(setup(tc.set, &calls), paritySettings(true, tc.authz)), "/", tc.auth)
			assert.Equal(t, tc.wantCode, resp.StatusCode)
			if tc.wantBody != "" {
				assert.Equal(t, tc.wantBody, body)
			}
			assert.Equal(t, tc.wantCalls, calls)
		})
	}
}

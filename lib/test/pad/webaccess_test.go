package pad

import (
	"io"
	"net/http/httptest"
	"testing"
	"time"

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

// newWebAccessApp builds a minimal fiber app with the CheckAccess middleware in
// front of a catch-all pad route, mirroring how lib/server/server.go installs it.
func newWebAccessApp(hookSystem *hooks.Hook, retrievedSettings *settings.Settings) *fiber.App {
	app := fiber.New()
	readOnlyManager := pad.NewReadOnlyManager(db.NewMemoryDataStore())
	logger := zap.NewNop().Sugar()
	app.Use(func(c fiber.Ctx) error {
		return pad.CheckAccessWithHooks(c, logger, retrievedSettings, readOnlyManager, hookSystem)
	})
	app.Get("/p/*", func(c fiber.Ctx) error {
		return c.SendString("pad content")
	})
	return app
}

// The admin-auth 401 branch sleeps one second to slow down brute force attacks,
// which is exactly fiber's default Test timeout — give those requests headroom.
var adminTestConfig = fiber.TestConfig{Timeout: 5 * time.Second, FailOnTimeout: true}

func TestCheckAccessWithoutPreAuthorizeHooksUnchanged(t *testing.T) {
	// No registered preAuthorize hook (and even a nil hook system) must leave the
	// existing behavior untouched.
	runUnchanged := func(hookSystem *hooks.Hook) func(t *testing.T) {
		return func(t *testing.T) {
			// Without authentication requirements a pad is freely accessible.
			app := newWebAccessApp(hookSystem, &settings.Settings{})
			resp, err := app.Test(httptest.NewRequest("GET", "/p/testpad", nil))
			require.NoError(t, err)
			assert.Equal(t, 200, resp.StatusCode)

			// With requireAuthentication an unauthenticated request gets a 401.
			app = newWebAccessApp(hookSystem, &settings.Settings{RequireAuthentication: true})
			resp, err = app.Test(httptest.NewRequest("GET", "/p/testpad", nil))
			require.NoError(t, err)
			assert.Equal(t, 401, resp.StatusCode)
			assert.Contains(t, resp.Header.Get("WWW-Authenticate"), "Basic")
		}
	}

	emptyHooks := hooks.NewHook()
	t.Run("nilHookSystem", runUnchanged(nil))
	t.Run("emptyHookSystem", runUnchanged(&emptyHooks))
}

func TestCheckAccessPreAuthorizeDeny(t *testing.T) {
	hookSystem := hooks.NewHook()
	var seenPath string
	hookSystem.EnqueuePreAuthorizeHook(func(ctx *events.PreAuthorizeContext) {
		seenPath = ctx.Path
		ctx.Deny()
	})

	// Even with no authentication requirement at all, an explicit deny rejects the
	// request before the regular steps run.
	app := newWebAccessApp(&hookSystem, &settings.Settings{})
	resp, err := app.Test(httptest.NewRequest("GET", "/p/testpad", nil))
	require.NoError(t, err)
	assert.Equal(t, 403, resp.StatusCode)
	assert.Equal(t, "/p/testpad", seenPath)
}

func TestCheckAccessPreAuthorizeDenyWinsOverPermit(t *testing.T) {
	hookSystem := hooks.NewHook()
	hookSystem.EnqueuePreAuthorizeHook(func(ctx *events.PreAuthorizeContext) { ctx.Permit() })
	hookSystem.EnqueuePreAuthorizeHook(func(ctx *events.PreAuthorizeContext) { ctx.Deny() })

	app := newWebAccessApp(&hookSystem, &settings.Settings{})
	resp, err := app.Test(httptest.NewRequest("GET", "/p/testpad", nil))
	require.NoError(t, err)
	assert.Equal(t, 403, resp.StatusCode)
}

func TestCheckAccessPreAuthorizePermitBypassesAuthentication(t *testing.T) {
	hookSystem := hooks.NewHook()
	hookSystem.EnqueuePreAuthorizeHook(func(ctx *events.PreAuthorizeContext) { ctx.Permit() })

	// requireAuthentication is on and the request carries no credentials, yet the
	// explicit permit skips the remaining steps for this non-admin page.
	app := newWebAccessApp(&hookSystem, &settings.Settings{RequireAuthentication: true})
	resp, err := app.Test(httptest.NewRequest("GET", "/p/testpad", nil))
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}

func TestCheckAccessPreAuthorizePermitDoesNotBypassAdmin(t *testing.T) {
	hookSystem := hooks.NewHook()
	hookSystem.EnqueuePreAuthorizeHook(func(ctx *events.PreAuthorizeContext) {
		assert.True(t, ctx.RequireAdmin)
		ctx.Permit()
	})

	// Permits are filtered out on /admin-auth pages (so plugins cannot
	// accidentally grant admin privileges); the request falls through to the
	// regular steps and fails authentication.
	app := newWebAccessApp(&hookSystem, &settings.Settings{})
	resp, err := app.Test(httptest.NewRequest("GET", "/admin-auth/", nil), adminTestConfig)
	require.NoError(t, err)
	assert.Equal(t, 401, resp.StatusCode)
}

func TestCheckAccessPreAuthorizeDenyAppliesToAdmin(t *testing.T) {
	hookSystem := hooks.NewHook()
	hookSystem.EnqueuePreAuthorizeHook(func(ctx *events.PreAuthorizeContext) { ctx.Deny() })

	app := newWebAccessApp(&hookSystem, &settings.Settings{})
	resp, err := app.Test(httptest.NewRequest("GET", "/admin-auth/", nil), adminTestConfig)
	require.NoError(t, err)
	assert.Equal(t, 403, resp.StatusCode)
}

func TestCheckAccessPreAuthzFailureOverridesResponse(t *testing.T) {
	hookSystem := hooks.NewHook()
	hookSystem.EnqueuePreAuthorizeHook(func(ctx *events.PreAuthorizeContext) { ctx.Deny() })
	hookSystem.EnqueuePreAuthzFailureHook(func(ctx *events.PreAuthzFailureContext) {
		ctx.SetHeader("Location", "/login")
		ctx.Respond(302, "redirecting to login")
	})

	app := newWebAccessApp(&hookSystem, &settings.Settings{})
	resp, err := app.Test(httptest.NewRequest("GET", "/p/testpad", nil))
	require.NoError(t, err)
	assert.Equal(t, 302, resp.StatusCode)
	assert.Equal(t, "/login", resp.Header.Get("Location"))
}

func TestPreAuthorizeDecisionSemantics(t *testing.T) {
	// No answer defers to the regular authenticate/authorize steps.
	ctx := &events.PreAuthorizeContext{Path: "/p/x"}
	assert.Equal(t, events.PreAuthorizeDefer, ctx.Decision())

	// All permits -> permit.
	ctx.Permit()
	assert.Equal(t, events.PreAuthorizePermit, ctx.Decision())

	// Any deny wins.
	ctx.Deny()
	assert.Equal(t, events.PreAuthorizeDeny, ctx.Decision())

	// On admin pages permits are filtered out: a lone permit defers...
	adminCtx := &events.PreAuthorizeContext{Path: "/admin-auth/", RequireAdmin: true}
	adminCtx.Permit()
	assert.Equal(t, events.PreAuthorizeDefer, adminCtx.Decision())

	// ...while a deny still counts.
	adminCtx.Deny()
	assert.Equal(t, events.PreAuthorizeDeny, adminCtx.Decision())
}

func TestAuthenticateHookSuccessGrantsAccess(t *testing.T) {
	// An authenticate hook that calls Authenticate("pluginuser") should allow
	// a request to succeed even when no basic-auth credentials are sent and
	// RequireAuthentication is on.
	hookSystem := hooks.NewHook()
	hookSystem.EnqueueAuthenticateHook(func(ctx *events.AuthenticateContext) {
		ctx.Authenticate("pluginuser")
	})

	app := newWebAccessApp(&hookSystem, &settings.Settings{RequireAuthentication: true})
	resp, err := app.Test(httptest.NewRequest("GET", "/p/testpad", nil))
	require.NoError(t, err)
	// The protected handler returns "pad content" with 200 on success.
	assert.Equal(t, 200, resp.StatusCode)
}

func TestAuthenticateHookRejectSends401(t *testing.T) {
	// An authenticate hook that calls Reject() must produce a 401.
	hookSystem := hooks.NewHook()
	hookSystem.EnqueueAuthenticateHook(func(ctx *events.AuthenticateContext) {
		ctx.Reject()
	})

	app := newWebAccessApp(&hookSystem, &settings.Settings{RequireAuthentication: true})
	resp, err := app.Test(httptest.NewRequest("GET", "/p/testpad", nil))
	require.NoError(t, err)
	assert.Equal(t, 401, resp.StatusCode)
}

func TestAuthnFailureHookOverridesResponse(t *testing.T) {
	// An authnFailure hook that calls Respond(302, "") and SetHeader("Location",
	// "/login") should override the default 401 with a redirect.
	hookSystem := hooks.NewHook()
	hookSystem.EnqueueAuthnFailureHook(func(ctx *events.AuthnFailureContext) {
		ctx.SetHeader("Location", "/login")
		ctx.Respond(302, "")
	})

	app := newWebAccessApp(&hookSystem, &settings.Settings{RequireAuthentication: true})
	resp, err := app.Test(httptest.NewRequest("GET", "/p/testpad", nil))
	require.NoError(t, err)
	assert.Equal(t, 302, resp.StatusCode)
	assert.Equal(t, "/login", resp.Header.Get("Location"))
}

// ---- authorize hook tests ----
// Setup: RequireAuthentication=true + RequireAuthorization=true; user is authenticated
// via an authenticate hook (username "u1", not in settings.Users → non-admin). This is
// exactly the code path where the authorize hook fires.

func TestAuthorizeHookGrantAllowsAccess(t *testing.T) {
	// authorize hook that calls Grant("readOnly") should let the request through (200).
	hookSystem := hooks.NewHook()
	hookSystem.EnqueueAuthenticateHook(func(ctx *events.AuthenticateContext) {
		ctx.Authenticate("u1")
	})
	hookSystem.EnqueueAuthorizeHook(func(ctx *events.AuthorizeContext) {
		ctx.Grant("readOnly")
	})

	app := newWebAccessApp(&hookSystem, &settings.Settings{
		RequireAuthentication: true,
		RequireAuthorization:  true,
	})
	resp, err := app.Test(httptest.NewRequest("GET", "/p/testpad", nil))
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "pad content", string(body))
}

func TestAuthorizeHookDenySends403(t *testing.T) {
	// authorize hook that calls Deny() must produce a 403.
	hookSystem := hooks.NewHook()
	hookSystem.EnqueueAuthenticateHook(func(ctx *events.AuthenticateContext) {
		ctx.Authenticate("u1")
	})
	hookSystem.EnqueueAuthorizeHook(func(ctx *events.AuthorizeContext) {
		ctx.Deny()
	})

	app := newWebAccessApp(&hookSystem, &settings.Settings{
		RequireAuthentication: true,
		RequireAuthorization:  true,
	})
	resp, err := app.Test(httptest.NewRequest("GET", "/p/testpad", nil))
	require.NoError(t, err)
	assert.Equal(t, 403, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "Forbidden", string(body))
}

func TestAuthzFailureHookOverridesResponse(t *testing.T) {
	// authzFailure hook that calls Respond(200, "upgrade required") overrides the
	// default 403 when the authorize hook denies.
	hookSystem := hooks.NewHook()
	hookSystem.EnqueueAuthenticateHook(func(ctx *events.AuthenticateContext) {
		ctx.Authenticate("u1")
	})
	hookSystem.EnqueueAuthorizeHook(func(ctx *events.AuthorizeContext) {
		ctx.Deny()
	})
	hookSystem.EnqueueAuthzFailureHook(func(ctx *events.AuthzFailureContext) {
		ctx.Respond(200, "upgrade required")
	})

	app := newWebAccessApp(&hookSystem, &settings.Settings{
		RequireAuthentication: true,
		RequireAuthorization:  true,
	})
	resp, err := app.Test(httptest.NewRequest("GET", "/p/testpad", nil))
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "upgrade required", string(body))
}

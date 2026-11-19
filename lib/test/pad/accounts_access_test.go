package pad

import (
	"testing"

	"github.com/ether/etherpad-go/lib/accounts"
	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/hooks"
	"github.com/ether/etherpad-go/lib/hooks/events"
	"github.com/ether/etherpad-go/lib/models/clientVars"
	"github.com/ether/etherpad-go/lib/models/webaccess"
	"github.com/ether/etherpad-go/lib/pad"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// accountsApp serves "/", pads and the admin check behind webaccess, on a
// store whose accounts the test sets up; it records the request user.
func accountsApp(t *testing.T, cfg *settings.Settings, hookSystem *hooks.Hook) (*fiber.App, *accounts.Manager, *pad.ReadOnlyManager, **webaccess.SocketClientRequest) {
	t.Helper()
	store := db.NewMemoryDataStore()
	readOnlyManager := pad.NewReadOnlyManager(store)
	m := pad.AccountsOf(store, cfg)
	logger := zap.NewNop().Sugar()
	var captured *webaccess.SocketClientRequest
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		return pad.CheckAccessWithHooks(c, logger, cfg, readOnlyManager, hookSystem)
	})
	handler := func(c fiber.Ctx) error {
		captured, _ = c.Locals(clientVars.WebAccessStore).(*webaccess.SocketClientRequest)
		return c.SendString("ok")
	}
	app.Get("/", handler)
	app.Get("/p/*", handler)
	app.Get("/s/*", handler)
	return app, m, readOnlyManager, &captured
}

func TestAccessByVisitorRolesAndPadPatterns(t *testing.T) {
	cfg := &settings.Settings{}
	app, m, rom, _ := accountsApp(t, cfg, nil)
	// Visitors read everything and write only under public/.
	require.NoError(t, m.ApplyTemplate(accounts.System, accounts.TemplateRead))
	require.NoError(t, m.Assign(accounts.System, accounts.AnonymousID, accounts.RoleEditor, "public/*"))

	for path, want := range map[string]int{
		"/":                  200,
		"/p/team%2Fnotes":    200,
		"/p/public%2Fflyer":  200,
		"/s/public%2Fbudget": 200,
		"/admin-auth/":       401,
	} {
		resp, _ := do(t, app, path, "")
		assert.Equal(t, want, resp.StatusCode, path)
	}
	assert.False(t, pad.UserCanModify(strPtr("team/notes"), nil, *rom, cfg), "visitors only read here")
	assert.True(t, pad.UserCanModify(strPtr("public/flyer"), nil, *rom, cfg))

	// Closed: visitors are asked to log in, even on "/".
	require.NoError(t, m.ApplyTemplate(accounts.System, accounts.TemplateClosed))
	resp, _ := do(t, app, "/", "")
	assert.Equal(t, 401, resp.StatusCode)
	resp, _ = do(t, app, "/p/x", "")
	assert.Equal(t, 401, resp.StatusCode)
}

func TestAccessOfAccounts(t *testing.T) {
	cfg := &settings.Settings{RequireAuthentication: true}
	app, m, rom, captured := accountsApp(t, cfg, nil)
	require.NoError(t, m.ApplyTemplate(accounts.System, accounts.TemplateClosed))
	// Logged-in people as a whole get nothing; anna edits team/ only.
	require.NoError(t, m.Unassign(accounts.System, accounts.AuthenticatedID, accounts.RoleEditor, ""))
	anna, err := m.CreateAccount(accounts.System, accounts.NewAccount{Username: "anna", Kind: accounts.KindHuman, Password: "anna-password"})
	require.NoError(t, err)
	require.NoError(t, m.Assign(accounts.System, anna.ID, accounts.RoleEditor, "team/*"))
	boss, err := m.CreateAccount(accounts.System, accounts.NewAccount{Username: "boss", Kind: accounts.KindHuman, Password: "boss-password"})
	require.NoError(t, err)
	require.NoError(t, m.Assign(accounts.System, boss.ID, accounts.RoleAdmin, ""))
	_, err = m.CreateAccount(accounts.System, accounts.NewAccount{Username: "claude", Kind: accounts.KindService})
	require.NoError(t, err)

	annaCreds := creds("anna:anna-password")
	resp, _ := do(t, app, "/p/team%2Fplan", annaCreds)
	assert.Equal(t, 200, resp.StatusCode)
	require.NotNil(t, *captured)
	assert.Equal(t, anna.ID, (*captured).AccountID)
	assert.Equal(t, accounts.LevelCreate, (*(*captured).PadAuthorizations)["team/plan"])
	assert.True(t, pad.UserCanModify(strPtr("team/plan"), *captured, *rom, cfg))

	resp, _ = do(t, app, "/p/other", annaCreds)
	assert.Equal(t, 403, resp.StatusCode, "logged in, but no rights on that pad")
	resp, _ = do(t, app, "/admin-auth/", annaCreds)
	assert.Equal(t, 403, resp.StatusCode)
	resp, _ = do(t, app, "/admin-auth/", creds("boss:boss-password"))
	assert.Equal(t, 200, resp.StatusCode)
	resp, _ = do(t, app, "/p/anything", creds("boss:boss-password"))
	assert.Equal(t, 200, resp.StatusCode)

	resp, _ = do(t, app, "/p/team%2Fplan", creds("anna:wrong"))
	assert.Equal(t, 401, resp.StatusCode)
	resp, _ = do(t, app, "/p/team%2Fplan", creds("claude:"))
	assert.Equal(t, 401, resp.StatusCode, "service accounts do not log in")

	require.NoError(t, m.SetDisabled(accounts.System, anna.ID, true))
	resp, _ = do(t, app, "/p/team%2Fplan", annaCreds)
	assert.Equal(t, 401, resp.StatusCode, "disabled at once, even right after a login")
}

func TestPluginAuthenticatedPeople(t *testing.T) {
	hookSystem := hooks.NewHook()
	hookSystem.EnqueueAuthenticateHook(func(ctx *events.AuthenticateContext) { ctx.Authenticate(ctx.InputUsername) })
	cfg := &settings.Settings{RequireAuthentication: true}
	app, m, _, captured := accountsApp(t, cfg, &hookSystem)
	require.NoError(t, m.ApplyTemplate(accounts.System, accounts.TemplateClosed))
	mod, err := m.CreateAccount(accounts.System, accounts.NewAccount{Username: "mod", Kind: accounts.KindHuman})
	require.NoError(t, err)
	require.NoError(t, m.Assign(accounts.System, mod.ID, accounts.RoleAdmin, ""))

	// Without an account: the rights of everyone logged in.
	resp, _ := do(t, app, "/p/x", creds("stranger:any"))
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "", (*captured).AccountID)
	assert.Equal(t, accounts.LevelCreate, (*(*captured).PadAuthorizations)["x"])
	resp, _ = do(t, app, "/admin-auth/", creds("stranger:any"))
	assert.Equal(t, 403, resp.StatusCode)

	// With an account by that name: its roles.
	resp, _ = do(t, app, "/admin-auth/", creds("mod:any"))
	assert.Equal(t, 200, resp.StatusCode)
}

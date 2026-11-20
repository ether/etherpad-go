package pad

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ether/etherpad-go/lib/accounts"
	"github.com/ether/etherpad-go/lib/api/padlogin"
	"github.com/ether/etherpad-go/lib/author"
	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/hooks"
	"github.com/ether/etherpad-go/lib/hooks/events"
	"github.com/ether/etherpad-go/lib/models/clientVars"
	"github.com/ether/etherpad-go/lib/pad"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/ether/etherpad-go/lib/utils"
	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// A person logged in with an account writes as the account's author in
// every browser.
func TestAccountWritesAsItsAuthor(t *testing.T) {
	store := db.NewMemoryDataStore()
	cfg := &settings.Settings{}
	m := pad.AccountsOf(store, cfg)
	require.NoError(t, m.ApplyTemplate(accounts.System, accounts.TemplateClosed))
	anna, err := m.CreateAccount(accounts.System, accounts.NewAccount{Username: "anna", DisplayName: "Anna B.", Kind: accounts.KindHuman, Password: "anna-password"})
	require.NoError(t, err)
	sm := pad.NewSecurityManager(store, nil, pad.NewManager(store, nil), cfg)

	padID := "notes"
	laptop, phone := utils.GenerateAuthorToken(), utils.GenerateAuthorToken()
	first, err := sm.CheckAccess(&padID, nil, &laptop, padlogin.UserOf(m, anna))
	require.NoError(t, err)
	second, err := sm.CheckAccess(&padID, nil, &phone, padlogin.UserOf(m, anna))
	require.NoError(t, err)
	assert.Equal(t, first.AuthorId, second.AuthorId, "one author in every browser")

	linked, err := m.Account(anna.ID)
	require.NoError(t, err)
	assert.Equal(t, first.AuthorId, linked.AuthorID)
	name, err := author.NewManager(store).GetAuthorName(first.AuthorId)
	require.NoError(t, err)
	require.NotNil(t, name)
	assert.Equal(t, "Anna B.", *name)

	// without the login, the browser's own author
	require.NoError(t, m.Assign(accounts.System, accounts.AnonymousID, accounts.RoleEditor, ""))
	visitor, err := sm.CheckAccess(&padID, nil, &laptop, nil)
	require.NoError(t, err)
	assert.NotEqual(t, first.AuthorId, visitor.AuthorId)
}

// A person logged in on the pads without rights for a pad gets 403, not
// the login again; a plugin that authenticates the request still decides
// first.
func TestLoggedInWithoutRights(t *testing.T) {
	cfg := &settings.Settings{}
	store := db.NewMemoryDataStore()
	m := pad.AccountsOf(store, cfg)
	require.NoError(t, m.ApplyTemplate(accounts.System, accounts.TemplateClosed))
	require.NoError(t, m.Unassign(accounts.System, accounts.AuthenticatedID, accounts.RoleEditor, ""))
	anna, err := m.CreateAccount(accounts.System, accounts.NewAccount{Username: "anna", Kind: accounts.KindHuman, Password: "anna-password"})
	require.NoError(t, err)
	mod, err := m.CreateAccount(accounts.System, accounts.NewAccount{Username: "mod", Kind: accounts.KindHuman})
	require.NoError(t, err)
	require.NoError(t, m.Assign(accounts.System, mod.ID, accounts.RoleEditor, ""))

	run := func(hookSystem *hooks.Hook) int {
		app := fiber.New()
		rom := pad.NewReadOnlyManager(store)
		app.Use(func(c fiber.Ctx) error {
			c.Locals(clientVars.WebAccessStore, padlogin.UserOf(m, anna)) // the login session's person
			return c.Next()
		})
		app.Use(func(c fiber.Ctx) error {
			return pad.CheckAccessWithHooks(c, zap.NewNop().Sugar(), cfg, rom, hookSystem)
		})
		app.Get("/p/:pad", func(c fiber.Ctx) error { return c.SendString("ok") })
		pad.SetLoginPage(padlogin.LoginURL)
		defer pad.SetLoginPage(nil)
		req := httptest.NewRequest("GET", "/p/notes", nil)
		req.Header.Set("Accept", "text/html")
		resp, err := app.Test(req)
		require.NoError(t, err)
		return resp.StatusCode
	}
	assert.Equal(t, http.StatusForbidden, run(nil), "no loop back to the login")
	h := hooks.NewHook()
	h.EnqueueAuthenticateHook(func(ctx *events.AuthenticateContext) { ctx.Authenticate("mod") })
	assert.Equal(t, http.StatusOK, run(&h), "the plugin's person may")
}

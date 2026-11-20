package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ether/etherpad-go/lib/accounts"
	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/tokens"
	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminAPIToken(t *testing.T) {
	store := db.NewMemoryDataStore()
	acc := accounts.NewManager(store)
	require.NoError(t, acc.EnsureDefaults(accounts.SeedConfig{Template: accounts.TemplateClosed}, nil))
	keys := tokens.NewManager(store, acc, nil)
	issue := func(role, pads string, scopes []string) string {
		a, err := acc.CreateAccount(accounts.System, accounts.NewAccount{Username: "svc-" + role + pads, Kind: accounts.KindService})
		require.NoError(t, err)
		require.NoError(t, acc.Assign(accounts.System, a.ID, role, pads))
		key, _, err := keys.Issue(accounts.System, tokens.Spec{AccountID: a.ID, Name: "t", Scopes: scopes})
		require.NoError(t, err)
		return key
	}
	padAdmin := issue(accounts.RoleAdmin, "", []string{accounts.ScopeAdminPads})
	editor := issue(accounts.RoleEditor, "", []string{accounts.ScopePadRead})

	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		if handled, err := AdminAPIToken(c, keys); handled {
			return err
		}
		return c.Status(http.StatusTeapot).SendString("the provider's token check")
	})
	app.Get("/pads", func(c fiber.Ctx) error { return c.SendString("pads") })
	call := func(auth string) int {
		req := httptest.NewRequest(http.MethodGet, "/pads", nil)
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		resp, err := app.Test(req)
		require.NoError(t, err)
		return resp.StatusCode
	}
	assert.Equal(t, http.StatusOK, call("Bearer "+padAdmin))
	assert.Equal(t, http.StatusForbidden, call("Bearer "+editor), "an editor's token is no pad admin")
	assert.Equal(t, http.StatusUnauthorized, call("Bearer ep_unknown"))
	assert.Equal(t, http.StatusTeapot, call("Bearer eyJhbGciOi.jwt"), "the provider's tokens go on")
	assert.Equal(t, http.StatusTeapot, call(""))
}

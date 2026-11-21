package padlogin_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ether/etherpad-go/lib/accounts"
	"github.com/ether/etherpad-go/lib/api/padlogin"
	"github.com/ether/etherpad-go/lib/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// call sends a JSON request like the pad's "My tokens" dialog does.
func (b *browser) call(t *testing.T, method, target string, body any, headers map[string]string) (*http.Response, padlogin.OwnTokens) {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		data, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(data)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, issuer+target, reader)
	if method != http.MethodGet {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", issuer)
	}
	for k, v := range headers {
		if v == "" {
			req.Header.Del(k)
		} else {
			req.Header.Set(k, v)
		}
	}
	for k, v := range b.cookies {
		req.AddCookie(&http.Cookie{Name: k, Value: v})
	}
	resp, err := b.e.app.Test(req, slowRunners)
	require.NoError(t, err)
	var out padlogin.OwnTokens
	if resp.StatusCode == http.StatusOK {
		require.NoError(t, jsonDecode(resp.Body, &out))
	}
	return resp, out
}

func scopeNames(s []padlogin.OwnScope) []string {
	out := make([]string, 0, len(s))
	for _, x := range s {
		out = append(out, x.Name)
	}
	return out
}

func TestOwnTokens(t *testing.T) {
	e := newEnv(t)
	e.cfg.Tokens.Personal.MaxLifetimeDays = 30
	role, err := e.accounts.SaveRole(accounts.System, db.Role{Name: "token-user",
		Scopes: []string{accounts.ScopeTokensOwn, accounts.ScopePadRead, accounts.ScopePadWrite}})
	require.NoError(t, err)
	anna, err := e.accounts.CreateAccount(accounts.System, accounts.NewAccount{Username: "anna", Kind: accounts.KindHuman, Password: "anna-password"})
	require.NoError(t, err)
	require.NoError(t, e.accounts.Assign(accounts.System, anna.ID, role.ID, ""))
	bob, err := e.accounts.CreateAccount(accounts.System, accounts.NewAccount{Username: "bob", Kind: accounts.KindHuman, Password: "bob-password"})
	require.NoError(t, err)
	require.NoError(t, e.accounts.Assign(accounts.System, bob.ID, accounts.RoleEditor, ""))

	// Without a login: 401.
	stranger := e.browser()
	resp, _ := stranger.call(t, http.MethodGet, padlogin.TokensPath, nil, nil)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	// Without tokens:own: 403, and the menu does not offer it.
	b := e.browser()
	b.logIn(t, "/p/x", "bob", "bob-password")
	assert.False(t, b.me(t).Tokens)
	resp, _ = b.call(t, http.MethodGet, padlogin.TokensPath, nil, nil)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)

	a := e.browser()
	a.logIn(t, "/p/x", "anna", "anna-password")
	assert.True(t, a.me(t).Tokens)
	resp, st := a.call(t, http.MethodGet, padlogin.TokensPath, nil, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "no-store", resp.Header.Get("Cache-Control"))
	assert.Empty(t, st.Tokens)
	assert.Subset(t, scopeNames(st.Scopes), []string{accounts.ScopePadRead, accounts.ScopePadWrite, accounts.ScopeTokensOwn})
	assert.NotContains(t, scopeNames(st.Scopes), accounts.ScopeAdminPads)
	assert.Equal(t, 30, st.MaxLifetimeDays)

	in := func(d time.Duration) *int64 { ms := time.Now().Add(d).UnixMilli(); return &ms }
	issue := map[string]any{"name": "laptop", "scopes": []string{accounts.ScopePadRead}, "pads": "notes/*", "days": 30}

	// Changes from another site, or not as JSON, are refused.
	resp, _ = a.call(t, http.MethodPost, padlogin.TokensPath, issue, map[string]string{"Origin": "http://evil.test"})
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	resp, _ = a.call(t, http.MethodPost, padlogin.TokensPath, issue, map[string]string{"Content-Type": "text/plain"})
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)

	// More than the person has, or longer than the policy: refused.
	resp, _ = a.call(t, http.MethodPost, padlogin.TokensPath, map[string]any{"name": "x", "scopes": []string{accounts.ScopeAdminPads}, "expiresAt": in(time.Hour)}, nil)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	resp, _ = a.call(t, http.MethodPost, padlogin.TokensPath, map[string]any{"name": "x", "scopes": []string{accounts.ScopePadRead}, "expiresAt": in(40 * 24 * time.Hour)}, nil)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	// Lifetimes in days: negative or absurd counts are refused, not read
	// as "never" or wrapped around.
	for _, days := range []int{-1, 40000, 1 << 40} {
		resp, _ = a.call(t, http.MethodPost, padlogin.TokensPath, map[string]any{"name": "x", "scopes": []string{accounts.ScopePadRead}, "days": days}, nil)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode, days)
	}

	resp, st = a.call(t, http.MethodPost, padlogin.TokensPath, issue, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.NotEmpty(t, st.Key)
	assert.Equal(t, "laptop", st.KeyName)
	assert.Equal(t, st.Tokens[0].ID, st.KeyID)
	require.Len(t, st.Tokens, 1)
	first := st.Tokens[0]
	assert.Equal(t, "notes/*", first.Pads)
	assert.Equal(t, []string{accounts.ScopePadRead}, first.Scopes)
	assert.Equal(t, "active", first.Status)
	assert.Equal(t, st.Key[:len(first.Prefix)], first.Prefix)

	// The key is not shown again.
	_, st = a.call(t, http.MethodGet, padlogin.TokensPath, nil, nil)
	assert.Empty(t, st.Key)

	// Bob cannot touch Anna's token; once he may have tokens, it is still
	// not his.
	require.NoError(t, e.accounts.Assign(accounts.System, bob.ID, role.ID, ""))
	resp, _ = b.call(t, http.MethodPost, padlogin.TokensPath+"/"+first.ID+"/revoke", nil, nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp, _ = b.call(t, http.MethodPost, padlogin.TokensPath+"/"+first.ID+"/rotate", map[string]any{}, nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	// Rotating keeps the old key for a while.
	resp, _ = a.call(t, http.MethodPost, padlogin.TokensPath+"/"+first.ID+"/rotate", map[string]any{"oldDays": -3}, nil)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp, st = a.call(t, http.MethodPost, padlogin.TokensPath+"/"+first.ID+"/rotate", map[string]any{"oldDays": 1}, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.NotEmpty(t, st.Key)
	require.Len(t, st.Tokens, 2)
	statuses := map[string]string{}
	for _, tk := range st.Tokens {
		statuses[tk.ID] = tk.Status
	}
	assert.Equal(t, "grace", statuses[first.ID])
	for _, tk := range st.Tokens {
		if tk.ID == first.ID {
			require.NotNil(t, tk.ExpiresAt)
			assert.InDelta(t, time.Now().Add(24*time.Hour).UnixMilli(), *tk.ExpiresAt, float64(time.Minute.Milliseconds()))
		}
	}

	// Revoking takes both off the list.
	for id := range statuses {
		resp, st = a.call(t, http.MethodPost, padlogin.TokensPath+"/"+id+"/revoke", nil, nil)
		require.Equal(t, http.StatusOK, resp.StatusCode)
	}
	assert.Empty(t, st.Tokens)
}

// Apps a person connected with OAuth (MCP) are listed and can be
// disconnected, by that person only.
func TestOwnApps(t *testing.T) {
	e := newEnv(t)
	role, err := e.accounts.SaveRole(accounts.System, db.Role{Name: "token-user",
		Scopes: []string{accounts.ScopeTokensOwn, accounts.ScopePadRead}})
	require.NoError(t, err)
	people := map[string]string{}
	for _, name := range []string{"anna", "bob"} {
		a, err := e.accounts.CreateAccount(accounts.System, accounts.NewAccount{Username: name, Kind: accounts.KindHuman, Password: name + "-password"})
		require.NoError(t, err)
		require.NoError(t, e.accounts.Assign(accounts.System, a.ID, role.ID, ""))
		people[name] = a.ID
	}
	grant := `[{"id":"g1","clientId":"mcp-1","clientName":"Claude","scopes":["pad:read"],"padPattern":"team-*","createdAt":"2026-11-21T10:00:00Z"}]`
	require.NoError(t, e.store.SetOIDCStorageValue("mcp-grants:"+people["anna"], grant))

	a := e.browser()
	a.logIn(t, "/p/x", "anna", "anna-password")
	resp, st := a.call(t, http.MethodGet, padlogin.TokensPath, nil, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Len(t, st.Apps, 1)
	assert.Equal(t, "Claude", st.Apps[0].Name)
	assert.Equal(t, "team-*", st.Apps[0].Pads)

	b := e.browser()
	b.logIn(t, "/p/x", "bob", "bob-password")
	resp, _ = b.call(t, http.MethodPost, padlogin.TokensPath+"/apps/g1/revoke", map[string]any{}, nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode, "not bob's")
	resp, _ = a.call(t, http.MethodPost, padlogin.TokensPath+"/apps/g1/revoke", map[string]any{}, map[string]string{"Origin": "http://evil.test"})
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)

	resp, st = a.call(t, http.MethodPost, padlogin.TokensPath+"/apps/g1/revoke", map[string]any{}, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Empty(t, st.Apps)
	resp, _ = a.call(t, http.MethodPost, padlogin.TokensPath+"/apps/g1/revoke", map[string]any{}, nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

package ws

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/ether/etherpad-go/lib/accounts"
	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/ether/etherpad-go/lib/tokens"
	libws "github.com/ether/etherpad-go/lib/ws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminTokens(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(testutils.TestRunConfig{Name: "issue, rotate, expire, revoke", Test: testAdminTokens})
	testDb.StartTestDBHandler()
}

func tokensCall(t *testing.T, ds testutils.TestDataStore, me, event string, data any) libws.TokensResult {
	t.Helper()
	name, raw := accountRequest(t, ds, me, event, data)
	require.Equal(t, "results:tokens", name)
	var res libws.TokensResult
	require.NoError(t, json.Unmarshal(raw, &res))
	return res
}

func findToken(res libws.TokensResult, id string) *libws.TokenView {
	for i := range res.Tokens {
		if res.Tokens[i].ID == id {
			return &res.Tokens[i]
		}
	}
	return nil
}

func testAdminTokens(t *testing.T, ds testutils.TestDataStore) {
	me := seedAdmin(t, ds, "root")
	m := accounts.For(ds.DS)
	svc, err := m.CreateAccount(accounts.System, accounts.NewAccount{Username: "claude", Kind: accounts.KindService})
	require.NoError(t, err)
	require.NoError(t, m.Assign(accounts.System, svc.ID, accounts.RoleEditor, "claude/*"))

	res := tokensCall(t, ds, me, "tokensLoad", nil)
	require.Empty(t, res.Error)
	var claude *libws.TokenAccount
	for i := range res.Accounts {
		if res.Accounts[i].ID == svc.ID {
			claude = &res.Accounts[i]
		}
	}
	require.NotNil(t, claude)
	assert.Contains(t, claude.Scopes, accounts.ScopePadWrite, "what the form preselects")
	assert.NotContains(t, claude.Scopes, accounts.ScopePadDelete)
	assert.NotEmpty(t, res.Scopes)

	expires := time.Now().Add(30 * 24 * time.Hour).UnixMilli()
	res = tokensCall(t, ds, me, "tokenIssue", map[string]any{"accountId": svc.ID, "name": "Claude Desktop",
		"scopes": []string{accounts.ScopePadRead, accounts.ScopePadWrite}, "pads": "claude/*", "expiresAt": expires})
	require.Empty(t, res.Error)
	require.NotNil(t, res.Created)
	key := res.Created.Key
	tok := findToken(res, res.Created.ID)
	require.NotNil(t, tok)
	assert.Equal(t, "active", tok.Status)
	assert.Equal(t, "claude/*", tok.Pads)
	assert.Equal(t, expires, *tok.ExpiresAt)
	assert.Equal(t, "root", res.Names[tok.CreatedBy])
	raw, _ := json.Marshal(res.Tokens)
	assert.NotContains(t, string(raw), key, "the key is shown once, in created only")
	assert.NotContains(t, string(raw), tokens.Hash(key))
	keys := tokens.For(ds.DS, nil)
	_, err = keys.Authenticate(key, "")
	require.NoError(t, err)
	assert.Nil(t, tokensCall(t, ds, me, "tokensLoad", nil).Created)

	res = tokensCall(t, ds, me, "tokenIssue", map[string]any{"accountId": svc.ID, "name": "x", "scopes": []string{"pad:fly"}})
	assert.Equal(t, tokens.ErrUnknownScope.Error(), res.Error)

	grace := time.Now().Add(10 * 24 * time.Hour).UnixMilli()
	res = tokensCall(t, ds, me, "tokenRotate", map[string]any{"id": tok.ID, "oldUntil": grace})
	require.Empty(t, res.Error)
	require.NotNil(t, res.Created)
	assert.Equal(t, "grace", findToken(res, tok.ID).Status)
	assert.Equal(t, grace, *findToken(res, tok.ID).ExpiresAt)
	assert.Equal(t, tok.ID, findToken(res, res.Created.ID).Predecessor)
	newKey := res.Created.Key

	shorter := time.Now().Add(time.Hour).UnixMilli()
	res = tokensCall(t, ds, me, "tokenSetExpiry", map[string]any{"id": tok.ID, "expiresAt": shorter})
	require.Empty(t, res.Error)
	assert.Equal(t, shorter, *findToken(res, tok.ID).ExpiresAt)

	res = tokensCall(t, ds, me, "tokenRevoke", map[string]any{"id": tok.ID})
	require.Empty(t, res.Error)
	assert.Equal(t, "revoked", findToken(res, tok.ID).Status)
	_, err = keys.Authenticate(key, "")
	assert.ErrorIs(t, err, tokens.ErrInvalid)
	_, err = keys.Authenticate(newKey, "")
	require.NoError(t, err, "the new key stays")

	// only with admin:apikeys, as of now
	editors, err := m.SaveRole(accounts.System, db.Role{Name: "users only", Scopes: []string{accounts.ScopeAdminUsers}})
	require.NoError(t, err)
	eve, err := m.CreateAccount(accounts.System, accounts.NewAccount{Username: "eve", Kind: accounts.KindHuman, Password: "eve-password"})
	require.NoError(t, err)
	require.NoError(t, m.Assign(accounts.System, eve.ID, editors.ID, ""))
	res = tokensCall(t, ds, eve.ID, "tokensLoad", nil)
	assert.Equal(t, accounts.ErrForbidden.Error(), res.Error)
	assert.Empty(t, res.Tokens, "nothing about the tokens")
	assert.Empty(t, res.Accounts)
}

package oidc

import (
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ether/etherpad-go/lib/accounts"
	"github.com/ether/etherpad-go/lib/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const mcpRedirect = "https://claude.example/api/mcp/auth_callback"

func newMCPServer(t *testing.T) *testServer {
	t.Helper()
	ts := newTestServer(t, db.NewMemoryDataStore())
	mux := ts.Config.Handler.(*http.ServeMux)
	mux.HandleFunc(registerPath, ts.auth.RegisterHandler)
	return ts
}

func (ts *testServer) postJSON(t *testing.T, path, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, ts.URL+path, strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := ts.client.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func (ts *testServer) register(t *testing.T) string {
	t.Helper()
	resp := ts.postJSON(t, registerPath, `{"client_name":"Claude","redirect_uris":["`+mcpRedirect+`"],
		"grant_types":["authorization_code","refresh_token"],"token_endpoint_auth_method":"none"}`)
	raw := body(t, resp)
	require.Equal(t, http.StatusCreated, resp.StatusCode, raw)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &out))
	assert.Equal(t, "none", out["token_endpoint_auth_method"])
	return out["client_id"].(string)
}

// mcpAuthorize runs an MCP client's authorization up to the consent page
// and returns its form.
func (ts *testServer) mcpAuthorize(t *testing.T, clientID, scope, username, password string) url.Values {
	t.Helper()
	q := url.Values{"response_type": {"code"}, "client_id": {clientID}, "redirect_uri": {mcpRedirect}, "state": {"st"},
		"code_challenge": {pkceS256(codeVerifier)}, "code_challenge_method": {"S256"}}
	if scope != "" {
		q.Set("scope", scope)
	}
	resp := ts.get(t, authPath+"?"+q.Encode())
	require.Equal(t, http.StatusFound, resp.StatusCode, body(t, resp))
	page := ts.get(t, resp.Header.Get("Location"))
	require.Equal(t, http.StatusOK, page.StatusCode)
	form := formInputs(t, page.Body)
	form.Set("username", username)
	form.Set("password", password)
	consent := ts.post(t, loginPath, form, "", "")
	if consent.StatusCode != http.StatusOK {
		return url.Values{"status": {consent.Status}, "location": {consent.Header.Get("Location")}}
	}
	assert.Equal(t, "DENY", consent.Header.Get("X-Frame-Options"))
	page2 := body(t, consent)
	form = formInputs(t, strings.NewReader(page2))
	// As a browser sends it: only the checked boxes.
	form.Del("scope")
	for _, m := range regexp.MustCompile(`name="scope" value="([^"]+)" checked`).FindAllStringSubmatch(page2, -1) {
		form.Add("scope", m[1])
	}
	return form
}

// decide sends the consent form and returns where the browser goes back to.
func (ts *testServer) decide(t *testing.T, form url.Values) *url.URL {
	t.Helper()
	resp := ts.post(t, loginPath, form, "", "")
	require.Equal(t, http.StatusFound, resp.StatusCode, body(t, resp))
	loc := resp.Header.Get("Location")
	if strings.HasPrefix(loc, callbackPath) {
		cb := ts.get(t, loc)
		require.Equal(t, http.StatusFound, cb.StatusCode, body(t, cb))
		loc = cb.Header.Get("Location")
	}
	u, err := url.Parse(loc)
	require.NoError(t, err)
	return u
}

func (ts *testServer) mcpTokens(t *testing.T, clientID, code string) (string, string) {
	t.Helper()
	resp := ts.post(t, tokenPath, url.Values{"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {mcpRedirect},
		"client_id": {clientID}, "code_verifier": {codeVerifier}}, "", "")
	raw := body(t, resp)
	require.Equal(t, http.StatusOK, resp.StatusCode, raw)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &out))
	refresh, _ := out["refresh_token"].(string)
	return out["access_token"].(string), refresh
}

func TestMCPMetadataAndRegistration(t *testing.T) {
	ts := newMCPServer(t)
	for _, p := range []string{asMetadataPath, discoveryPath} {
		resp := ts.get(t, p)
		require.Equal(t, http.StatusOK, resp.StatusCode, p)
		var doc map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&doc))
		assert.Equal(t, ts.URL, doc["issuer"])
		assert.Equal(t, ts.URL+registerPath, doc["registration_endpoint"], p)
		assert.Contains(t, doc["code_challenge_methods_supported"], "S256")
		assert.Contains(t, doc["token_endpoint_auth_methods_supported"], "none")
		assert.Contains(t, doc["scopes_supported"], accounts.ScopePadRead)
	}

	for name, req := range map[string]string{
		"no redirect":       `{"client_name":"x"}`,
		"plain http":        `{"redirect_uris":["http://evil.example/cb"]}`,
		"fragment":          `{"redirect_uris":["https://a.example/cb#x"]}`,
		"other grant":       `{"redirect_uris":["https://a.example/cb"],"grant_types":["client_credentials"]}`,
		"control character": `{"redirect_uris":["https://a.example/cb"],"client_name":"a\u0007b"}`,
		"not json":          `nope`,
	} {
		resp := ts.postJSON(t, registerPath, req)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode, name)
	}
	// http to this machine is fine (native clients, RFC 8252)
	resp := ts.postJSON(t, registerPath, `{"redirect_uris":["http://127.0.0.1:33418/callback"]}`)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	// PKCE with S256 is required
	id := ts.register(t)
	q := url.Values{"response_type": {"code"}, "client_id": {id}, "redirect_uri": {mcpRedirect}, "scope": {"pad:read"}}
	assert.Equal(t, http.StatusBadRequest, ts.get(t, authPath+"?"+q.Encode()).StatusCode)
	q.Set("code_challenge", "abc")
	q.Set("code_challenge_method", "plain")
	assert.Equal(t, http.StatusBadRequest, ts.get(t, authPath+"?"+q.Encode()).StatusCode)
}

func TestMCPConsentAndTokens(t *testing.T) {
	ts := newMCPServer(t)
	clientID := ts.register(t)
	admin, err := ts.auth.Accounts().AccountByUsername("admin")
	require.NoError(t, err)

	// No scope asked for: the MCP scopes are offered, all checked.
	form := ts.mcpAuthorize(t, clientID, "", "admin", "adminpass")
	require.NotEmpty(t, form.Get("consent_token"))
	assert.ElementsMatch(t, MCPScopes, form["scope"])

	// Allowing only reading, on team-* pads.
	form.Del("scope")
	form.Set("scope", accounts.ScopePadRead)
	form.Add("scope", "admin:pads") // not offered: dropped
	form.Set("pads", "team-*")
	form.Set("decision", "allow")
	back := ts.decide(t, form)
	assert.Equal(t, "claude.example", back.Host)
	assert.Equal(t, "st", back.Query().Get("state"))
	code := back.Query().Get("code")
	require.NotEmpty(t, code)

	access, refresh := ts.mcpTokens(t, clientID, code)
	require.NotEmpty(t, refresh, "offline access for the client")
	got, err := ts.auth.MCPAccessToken(access)
	require.NoError(t, err)
	assert.Equal(t, admin.ID, got.Account.ID)
	assert.Equal(t, []string{accounts.ScopePadRead}, got.Scopes)
	assert.Equal(t, "team-*", got.PadPattern)

	// The consent form cannot be sent twice.
	resp := ts.post(t, loginPath, form, "", "")
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	// A refresh keeps the pads and scopes.
	r := ts.post(t, tokenPath, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refresh}, "client_id": {clientID}}, "", "")
	rawR := body(t, r)
	require.Equal(t, http.StatusOK, r.StatusCode, rawR)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(rawR), &out))
	got, err = ts.auth.MCPAccessToken(out["access_token"].(string))
	require.NoError(t, err)
	assert.Equal(t, "team-*", got.PadPattern)
	refresh = out["refresh_token"].(string)

	// The grant is listed; ending it stops the tokens and the refresh.
	grants, err := ts.auth.MCPGrants(admin.ID)
	require.NoError(t, err)
	require.Len(t, grants, 1)
	assert.Equal(t, "Claude", grants[0].ClientName)
	require.NoError(t, ts.auth.RevokeMCPGrant(accounts.System, admin.ID, grants[0].ID))
	_, err = ts.auth.MCPAccessToken(out["access_token"].(string))
	assert.ErrorIs(t, err, ErrNotMCPToken)
	r = ts.post(t, tokenPath, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refresh}, "client_id": {clientID}}, "", "")
	assert.NotEqual(t, http.StatusOK, r.StatusCode)
	assert.ErrorIs(t, ts.auth.RevokeMCPGrant(accounts.System, admin.ID, grants[0].ID), ErrGrantNotFound)

	// A disabled account's tokens stop.
	form = ts.mcpAuthorize(t, clientID, "pad:read pad:write", "admin", "adminpass")
	assert.ElementsMatch(t, []string{accounts.ScopePadRead, accounts.ScopePadWrite}, form["scope"], "what the client asked for")
	form.Set("decision", "allow")
	access, _ = ts.mcpTokens(t, clientID, ts.decide(t, form).Query().Get("code"))
	_, err = ts.auth.MCPAccessToken(access)
	require.NoError(t, err)
	require.NoError(t, ts.auth.Accounts().SetDisabled(accounts.System, admin.ID, true))
	_, err = ts.auth.MCPAccessToken(access)
	assert.ErrorIs(t, err, ErrNotMCPToken)
	require.NoError(t, ts.auth.Accounts().SetDisabled(accounts.System, admin.ID, false))
}

func TestMCPConsentRefused(t *testing.T) {
	ts := newMCPServer(t)
	clientID := ts.register(t)

	// "Ablehnen": back to the client with access_denied, no code.
	form := ts.mcpAuthorize(t, clientID, "pad:read", "admin", "adminpass")
	form.Set("decision", "deny")
	back := ts.decide(t, form)
	assert.Equal(t, "access_denied", back.Query().Get("error"))
	assert.Empty(t, back.Query().Get("code"))

	// Nothing allowed, or a broken pattern: the page again.
	form = ts.mcpAuthorize(t, clientID, "pad:read", "admin", "adminpass")
	form.Del("scope")
	form.Set("decision", "allow")
	resp := ts.post(t, loginPath, form, "", "")
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, body(t, resp), "mindestens ein Recht")

	// A wrong consent token is refused.
	form = ts.mcpAuthorize(t, clientID, "pad:read", "admin", "adminpass")
	form.Set("consent_token", "forged")
	form.Set("decision", "allow")
	resp = ts.post(t, loginPath, form, "", "")
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	// Who may have no tokens connects no app.
	got := ts.mcpAuthorize(t, clientID, "pad:read", "editor", "editorpass")
	require.Contains(t, got.Get("status"), "302")
	loc, err := url.Parse(got.Get("location"))
	require.NoError(t, err)
	assert.Equal(t, "access_denied", loc.Query().Get("error"))
}

// Tokens of other clients are no MCP tokens: the admin UI's do not work at
// /mcp.
func TestMCPAccessOnlyForMCPClients(t *testing.T) {
	ts := newMCPServer(t)
	code := ts.login(t, ts.startAuth(t, "openid offline_access"), "admin", "adminpass")
	resp := ts.exchangeCode(t, code, codeVerifier)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var out map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	_, err := ts.auth.MCPAccessToken(out["access_token"].(string))
	assert.ErrorIs(t, err, ErrNotMCPToken)
	_, err = ts.auth.MCPAccessToken("ep_whatever")
	assert.ErrorIs(t, err, ErrNotMCPToken)
}

func TestMCPOAuthOff(t *testing.T) {
	ts := newMCPServer(t)
	off := false
	ts.auth.retrievedSettings.MCP.OAuth = &off
	resp := ts.postJSON(t, registerPath, `{"redirect_uris":["https://a.example/cb"]}`)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	resp = ts.get(t, asMetadataPath)
	var doc map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&doc))
	assert.Nil(t, doc["registration_endpoint"])
	_, ok := ts.auth.MCPOAuthIssuer()
	assert.False(t, ok)
}

// A client that asked for reading gets at most reading: rights added to
// the form are dropped.
func TestMCPConsentNeverWidens(t *testing.T) {
	ts := newMCPServer(t)
	clientID := ts.register(t)
	form := ts.mcpAuthorize(t, clientID, "pad:read", "admin", "adminpass")
	assert.Equal(t, []string{accounts.ScopePadRead}, form["scope"], "only what the client asked for is offered")
	form.Add("scope", accounts.ScopePadWrite)
	form.Add("scope", accounts.ScopeChatWrite)
	form.Set("decision", "allow")
	access, _ := ts.mcpTokens(t, clientID, ts.decide(t, form).Query().Get("code"))
	got, err := ts.auth.MCPAccessToken(access)
	require.NoError(t, err)
	assert.Equal(t, []string{accounts.ScopePadRead}, got.Scopes)
}

// The consent form sent twice at once makes one grant, and the token of
// the one that went through works.
func TestMCPDoubleConsent(t *testing.T) {
	ts := newMCPServer(t)
	clientID := ts.register(t)
	admin, err := ts.auth.Accounts().AccountByUsername("admin")
	require.NoError(t, err)
	form := ts.mcpAuthorize(t, clientID, "pad:read", "admin", "adminpass")
	form.Set("decision", "allow")
	statuses := make(chan int, 2)
	locations := make(chan string, 2)
	for i := 0; i < 2; i++ {
		go func() {
			req, _ := http.NewRequest(http.MethodPost, ts.URL+loginPath, strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			resp, err := ts.client.Do(req)
			if err != nil {
				statuses <- 0
				locations <- ""
				return
			}
			_ = resp.Body.Close()
			statuses <- resp.StatusCode
			locations <- resp.Header.Get("Location")
		}()
	}
	var ok []string
	for i := 0; i < 2; i++ {
		st, loc := <-statuses, <-locations
		if st == http.StatusFound {
			ok = append(ok, loc)
		}
	}
	require.Len(t, ok, 1, "one consent goes through")
	grants, err := ts.auth.MCPGrants(admin.ID)
	require.NoError(t, err)
	assert.Len(t, grants, 1)
	cb := ts.get(t, ok[0])
	back, err := url.Parse(cb.Header.Get("Location"))
	require.NoError(t, err)
	access, _ := ts.mcpTokens(t, clientID, back.Query().Get("code"))
	_, err = ts.auth.MCPAccessToken(access)
	assert.NoError(t, err)
}

func TestMCPRegistrationLimits(t *testing.T) {
	ts := newMCPServer(t)
	for i := 0; i < registerPerIP; i++ {
		resp := ts.postJSON(t, registerPath, `{"redirect_uris":["https://a.example/cb"]}`)
		require.Equal(t, http.StatusCreated, resp.StatusCode)
	}
	resp := ts.postJSON(t, registerPath, `{"redirect_uris":["https://a.example/cb"]}`)
	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)

	// Another address may still register; after the window, this one too.
	c := &mcpClients{}
	now := time.Now()
	for i := 0; i < registerPerIP; i++ {
		require.True(t, c.allowRegistration("a", now))
	}
	assert.False(t, c.allowRegistration("a", now))
	assert.True(t, c.allowRegistration("b", now))
	assert.True(t, c.allowRegistration("a", now.Add(registerWindow)))
}

// A registration nobody connected with goes away after a day; one a person
// connected stays.
func TestMCPUnusedClientsExpire(t *testing.T) {
	ts := newMCPServer(t)
	unused := ts.register(t)
	used := ts.register(t)
	form := ts.mcpAuthorize(t, used, "pad:read", "admin", "adminpass")
	form.Set("decision", "allow")
	ts.decide(t, form)

	later := time.Now().Add(unusedClientLifetime + time.Hour)
	ts.auth.storage.now = func() time.Time { return later }
	ts.auth.storage.mcp.m = sync.Map{}
	_, ok := ts.auth.storage.mcpClient(unused)
	assert.False(t, ok)
	raw, err := ts.store.GetOIDCStorageValue(mcpClientKeyPrefix + unused)
	require.NoError(t, err)
	assert.Nil(t, raw, "removed from the store")
	_, ok = ts.auth.storage.mcpClient(used)
	assert.True(t, ok)
}

// Taking tokens:own away stops the person's apps; connecting and ending
// are audited.
func TestMCPAppsNeedTokensOwnAndAreAudited(t *testing.T) {
	ts := newMCPServer(t)
	clientID := ts.register(t)
	acc := ts.auth.Accounts()
	role, err := acc.SaveRole(accounts.System, db.Role{Name: "connector", Scopes: []string{accounts.ScopeTokensOwn, accounts.ScopePadRead}})
	require.NoError(t, err)
	anna, err := acc.CreateAccount(accounts.System, accounts.NewAccount{Username: "anna", Kind: accounts.KindHuman, Password: "anna-password"})
	require.NoError(t, err)
	require.NoError(t, acc.Assign(accounts.System, anna.ID, role.ID, ""))

	form := ts.mcpAuthorize(t, clientID, "pad:read", "anna", "anna-password")
	form.Set("decision", "allow")
	access, refresh := ts.mcpTokens(t, clientID, ts.decide(t, form).Query().Get("code"))
	_, err = ts.auth.MCPAccessToken(access)
	require.NoError(t, err)

	require.NoError(t, acc.Unassign(accounts.System, anna.ID, role.ID, ""))
	_, err = ts.auth.MCPAccessToken(access)
	assert.ErrorIs(t, err, ErrNotMCPToken)
	r := ts.post(t, tokenPath, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refresh}, "client_id": {clientID}}, "", "")
	assert.NotEqual(t, http.StatusOK, r.StatusCode)

	grants, err := ts.auth.MCPGrants(anna.ID)
	require.NoError(t, err)
	require.Len(t, grants, 1)
	require.NoError(t, ts.auth.RevokeMCPGrant(accounts.Actor{ID: anna.ID}, anna.ID, grants[0].ID))
	entries, err := acc.Audit(50)
	require.NoError(t, err)
	actions := map[string]bool{}
	for _, e := range entries {
		actions[e.Action] = true
	}
	assert.True(t, actions["app.connect"])
	assert.True(t, actions["app.disconnect"])
}

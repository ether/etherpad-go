package oidc

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/ether/etherpad-go/lib/accounts"
	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// padLogin runs a pad login up to the code the provider sends back to the
// pads' callback.
func (ts *testServer) padLogin(t *testing.T, state, verifier, username, password string) string {
	t.Helper()
	target, err := ts.auth.PadAuthorizeURL(state, verifier)
	require.NoError(t, err)
	u, err := url.Parse(target)
	require.NoError(t, err)
	resp := ts.get(t, u.RequestURI())
	require.Equal(t, http.StatusFound, resp.StatusCode, body(t, resp))
	page := ts.get(t, resp.Header.Get("Location"))
	require.Equal(t, http.StatusOK, page.StatusCode)
	form := formInputs(t, page.Body)
	form.Set("username", username)
	form.Set("password", password)
	done := ts.post(t, loginPath, form, "", "")
	require.Equal(t, http.StatusFound, done.StatusCode, body(t, done))
	cb := ts.get(t, done.Header.Get("Location"))
	require.Equal(t, http.StatusFound, cb.StatusCode, body(t, cb))
	back, err := url.Parse(cb.Header.Get("Location"))
	require.NoError(t, err)
	assert.Equal(t, ts.auth.PadCallbackURL(), back.Scheme+"://"+back.Host+back.Path, "back to the pads' own callback")
	assert.Equal(t, state, back.Query().Get("state"))
	return back.Query().Get("code")
}

func TestPadLogin(t *testing.T) {
	ts := newTestServer(t, db.NewMemoryDataStore())
	require.True(t, ts.auth.PadLoginAvailable())
	assert.Equal(t, ts.URL+PadCallbackPath, ts.auth.PadCallbackURL())
	acc := ts.auth.Accounts()
	anna, err := acc.CreateAccount(accounts.System, accounts.NewAccount{Username: "anna", Kind: accounts.KindHuman, Password: "anna-password"})
	require.NoError(t, err)

	code := ts.padLogin(t, "s1", "verifier-one-verifier-one-verifier-one", "anna", "anna-password")
	_, err = ts.auth.RedeemPadCode(code, "another-verifier-another-verifier-xx")
	assert.ErrorIs(t, err, ErrLoginFailed, "the PKCE verifier must match")
	_, err = ts.auth.RedeemPadCode(code, "verifier-one-verifier-one-verifier-one")
	assert.ErrorIs(t, err, ErrLoginFailed, "a code is tried once, also after a wrong verifier")

	code = ts.padLogin(t, "s2", "verifier-two-verifier-two-verifier-two", "anna", "anna-password")
	a, err := ts.auth.RedeemPadCode(code, "verifier-two-verifier-two-verifier-two")
	require.NoError(t, err)
	assert.Equal(t, anna.ID, a.ID)
	_, err = ts.auth.RedeemPadCode(code, "verifier-two-verifier-two-verifier-two")
	assert.ErrorIs(t, err, ErrLoginFailed, "used once")
	// nor at the token endpoint
	tok := ts.post(t, tokenPath, url.Values{"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {ts.auth.PadCallbackURL()},
		"client_id": {"user_client"}, "code_verifier": {"verifier-two-verifier-two-verifier-two"}}, "", "")
	assert.NotEqual(t, http.StatusOK, tok.StatusCode)

	// a disabled account's code is worth nothing
	code = ts.padLogin(t, "s3", "verifier-3-verifier-3-verifier-3-verif", "anna", "anna-password")
	require.NoError(t, acc.SetDisabled(accounts.System, anna.ID, true))
	_, err = ts.auth.RedeemPadCode(code, "verifier-3-verifier-3-verifier-3-verif")
	assert.ErrorIs(t, err, ErrLoginFailed)
	require.NoError(t, acc.SetDisabled(accounts.System, anna.ID, false))

	// the admin UI's codes are not the pads'
	adminCode := ts.login(t, ts.startAuth(t, "openid"), "anna", "anna-password")
	_, err = ts.auth.RedeemPadCode(adminCode, codeVerifier)
	assert.ErrorIs(t, err, ErrLoginFailed)

	_, err = ts.auth.RedeemPadCode("", "x")
	assert.ErrorIs(t, err, ErrLoginFailed)
}

func TestPadCallbackOnlyForTheUserClient(t *testing.T) {
	ts := newTestServer(t, db.NewMemoryDataStore())
	// the admin client may not send codes to the pads' callback
	q := url.Values{"response_type": {"code"}, "client_id": {adminClientID}, "redirect_uri": {ts.auth.PadCallbackURL()},
		"scope": {"openid"}, "state": {"x"}, "code_challenge": {pkceS256(codeVerifier)}, "code_challenge_method": {"S256"}}
	resp := ts.get(t, authPath+"?"+q.Encode())
	loc := resp.Header.Get("Location")
	assert.False(t, strings.HasPrefix(loc, loginPath), "refused: %d %s", resp.StatusCode, loc)
}

func TestPadLoginNeedsAUserClient(t *testing.T) {
	cfg := testSettings("http://localhost:9999")
	cfg.SSO.Clients = cfg.SSO.Clients[:1]
	a, err := newAuthenticator(cfg, db.NewMemoryDataStore())
	require.NoError(t, err)
	assert.False(t, a.PadLoginAvailable())
	_, err = a.PadAuthorizeURL("s", "v")
	assert.ErrorIs(t, err, ErrLoginFailed)

	cfg = testSettings("http://localhost:9999")
	cfg.SSO.Clients = append(cfg.SSO.Clients[:1], settings.SSOClient{Type: "user", ClientId: "u", GrantTypes: []string{"client_credentials"}})
	a, err = newAuthenticator(cfg, db.NewMemoryDataStore())
	require.NoError(t, err)
	assert.False(t, a.PadLoginAvailable(), "the code grant is needed")
}

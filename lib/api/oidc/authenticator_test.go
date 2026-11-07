package oidc

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/ether/etherpad-go/lib/test/testdb"
	jose "github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"
)

const (
	adminClientID   = "admin_client"
	adminRedirect   = "http://localhost:9001/admin/"
	serviceClientID = "service-client"
	serviceSecret   = "service-secret"
	codeVerifier    = "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG"
)

func strPtr(s string) *string { return &s }
func boolPtr(b bool) *bool    { return &b }

func pkceS256(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func testSettings(issuer string) *settings.Settings {
	return &settings.Settings{
		SSO: &settings.SSO{
			Issuer: issuer,
			Clients: []settings.SSOClient{
				{
					Type:         "admin",
					ClientId:     adminClientID,
					ClientSecret: strPtr("admin"),
					RedirectUris: []string{adminRedirect},
					GrantTypes:   []string{"authorization_code"},
					DisplayName:  "Admin",
				},
				{
					ClientId:     serviceClientID,
					ClientSecret: strPtr(serviceSecret),
					RedirectUris: []string{"https://service.example/callback"},
					GrantTypes:   []string{"authorization_code", "client_credentials"},
				},
			},
		},
		Users: map[string]settings.User{
			"admin":  {Password: strPtr("adminpass"), IsAdmin: boolPtr(true)},
			"editor": {Password: strPtr("editorpass"), IsAdmin: boolPtr(false)},
			"empty":  {Password: strPtr(""), IsAdmin: boolPtr(true)},
		},
	}
}

// testServer runs the provider like Init mounts it.
type testServer struct {
	*httptest.Server
	auth   *Authenticator
	client *http.Client
	store  db.DataStore
}

func newTestServer(t *testing.T, store db.DataStore) *testServer {
	t.Helper()
	return newTestServerWithIssuerPath(t, store, "")
}

func newTestServerWithIssuerPath(t *testing.T, store db.DataStore, issuerPath string) *testServer {
	t.Helper()
	ts := &testServer{store: store}
	mux := http.NewServeMux()
	ts.Server = httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	auth, err := newAuthenticator(testSettings(ts.URL+issuerPath), store)
	require.NoError(t, err)
	ts.auth = auth
	for _, p := range auth.Paths() {
		mux.Handle(p, auth.Handler())
	}
	mux.HandleFunc(loginPath, auth.LoginHandler)

	ts.client = &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return ts
}

func (ts *testServer) get(t *testing.T, path string) *http.Response {
	t.Helper()
	resp, err := ts.client.Get(ts.URL + path)
	require.NoError(t, err)
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func (ts *testServer) post(t *testing.T, path string, form url.Values, basicUser, basicPass string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, ts.URL+path, strings.NewReader(form.Encode()))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if basicUser != "" {
		req.SetBasicAuth(basicUser, basicPass)
	}
	resp, err := ts.client.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func body(t *testing.T, resp *http.Response) string {
	t.Helper()
	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return string(b)
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	Error        string `json:"error"`
}

func decodeTokens(t *testing.T, resp *http.Response) tokenResponse {
	t.Helper()
	var tr tokenResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&tr))
	return tr
}

// startAuth starts the admin UI's login like admin/src/hooks/useAuth.ts and
// returns the login page URL the provider redirects to.
func (ts *testServer) startAuth(t *testing.T, scope string) string {
	t.Helper()
	q := url.Values{
		"response_type":         {"code"},
		"client_id":             {adminClientID},
		"redirect_uri":          {adminRedirect},
		"scope":                 {scope},
		"state":                 {"state-123"},
		"nonce":                 {"nonce-123"},
		"code_challenge":        {pkceS256(codeVerifier)},
		"code_challenge_method": {"S256"},
	}
	resp := ts.get(t, authPath+"?"+q.Encode())
	require.Equal(t, http.StatusFound, resp.StatusCode, body(t, resp))
	loc := resp.Header.Get("Location")
	require.True(t, strings.HasPrefix(loc, loginPath+"?authRequestID="), loc)
	return loc
}

func formInputs(t *testing.T, r io.Reader) url.Values {
	t.Helper()
	doc, err := html.Parse(r)
	require.NoError(t, err)
	values := url.Values{}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "input" {
			var name, value string
			for _, a := range n.Attr {
				switch a.Key {
				case "name":
					name = a.Val
				case "value":
					value = a.Val
				}
			}
			if name != "" {
				values.Add(name, value)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return values
}

// login submits the login form and follows the provider back to the
// client; it returns the authorization code.
func (ts *testServer) login(t *testing.T, loginURL, username, password string) string {
	t.Helper()
	page := ts.get(t, loginURL)
	require.Equal(t, http.StatusOK, page.StatusCode)
	form := formInputs(t, page.Body)
	form.Set("username", username)
	form.Set("password", password)

	resp := ts.post(t, loginPath, form, "", "")
	require.Equal(t, http.StatusFound, resp.StatusCode, body(t, resp))
	callback := ts.get(t, resp.Header.Get("Location"))
	require.Equal(t, http.StatusFound, callback.StatusCode, body(t, callback))

	redirect, err := url.Parse(callback.Header.Get("Location"))
	require.NoError(t, err)
	assert.Equal(t, adminRedirect, redirect.Scheme+"://"+redirect.Host+redirect.Path)
	assert.Equal(t, "state-123", redirect.Query().Get("state"))
	code := redirect.Query().Get("code")
	require.NotEmpty(t, code)
	return code
}

func (ts *testServer) exchangeCode(t *testing.T, code, verifier string) *http.Response {
	t.Helper()
	return ts.post(t, tokenPath, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {adminRedirect},
		"client_id":     {adminClientID},
		"code_verifier": {verifier},
	}, "", "")
}

func (ts *testServer) refresh(t *testing.T, refreshToken string) *http.Response {
	t.Helper()
	return ts.post(t, tokenPath, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {adminClientID},
	}, "", "")
}

func TestDiscoveryAndJWKS(t *testing.T) {
	ts := newTestServer(t, db.NewMemoryDataStore())

	resp := ts.get(t, discoveryPath)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var disc map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&disc))
	assert.Equal(t, ts.URL, disc["issuer"])
	assert.Equal(t, ts.URL+authPath, disc["authorization_endpoint"])
	assert.Equal(t, ts.URL+tokenPath, disc["token_endpoint"])
	assert.Equal(t, ts.URL+jwksPath, disc["jwks_uri"])
	assert.NotContains(t, disc["token_endpoint_auth_methods_supported"], "private_key_jwt")

	resp = ts.get(t, jwksPath)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var jwks jose.JSONWebKeySet
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&jwks))
	require.Len(t, jwks.Keys, 1)
	assert.Equal(t, "RS256", jwks.Keys[0].Algorithm)
	assert.True(t, jwks.Keys[0].IsPublic())
}

// settings.json may configure the issuer with a path, like
// "http://localhost:9001/oauth2/". The endpoints stay at the root.
func TestIssuerWithPath(t *testing.T) {
	ts := newTestServerWithIssuerPath(t, db.NewMemoryDataStore(), "/oauth2/")
	for _, p := range []string{discoveryPath, "/oauth2" + discoveryPath} {
		resp := ts.get(t, p)
		require.Equal(t, http.StatusOK, resp.StatusCode, p)
		var disc map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&disc))
		assert.Equal(t, ts.URL+"/oauth2", disc["issuer"])
		assert.Equal(t, ts.URL+authPath, disc["authorization_endpoint"])
		assert.Equal(t, ts.URL+tokenPath, disc["token_endpoint"])
		assert.Equal(t, ts.URL+jwksPath, disc["jwks_uri"])
	}

	code := ts.login(t, ts.startAuth(t, "openid"), "admin", "adminpass")
	resp := ts.exchangeCode(t, code, codeVerifier)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	ok, err := ts.auth.ValidateAdminToken(decodeTokens(t, resp).IDToken, &settings.SSOClient{ClientId: adminClientID})
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestAdminLoginFlow(t *testing.T) {
	ts := newTestServer(t, db.NewMemoryDataStore())
	adminClient := &settings.SSOClient{ClientId: adminClientID}

	// The admin UI used to request "offline"; it still gets a refresh token.
	for _, scope := range []string{"openid profile email offline_access", "openid profile email offline"} {
		t.Run(scope, func(t *testing.T) {
			code := ts.login(t, ts.startAuth(t, scope), "admin", "adminpass")
			resp := ts.exchangeCode(t, code, codeVerifier)
			require.Equal(t, http.StatusOK, resp.StatusCode)
			tokens := decodeTokens(t, resp)
			require.NotEmpty(t, tokens.IDToken)
			require.NotEmpty(t, tokens.AccessToken)
			require.NotEmpty(t, tokens.RefreshToken)

			ok, err := ts.auth.ValidateAdminToken(tokens.IDToken, adminClient)
			require.NoError(t, err)
			assert.True(t, ok)

			// The code is single-use.
			replay := ts.exchangeCode(t, code, codeVerifier)
			assert.Equal(t, http.StatusBadRequest, replay.StatusCode)

			// Refresh rotates the refresh token.
			resp = ts.refresh(t, tokens.RefreshToken)
			require.Equal(t, http.StatusOK, resp.StatusCode)
			renewed := decodeTokens(t, resp)
			require.NotEmpty(t, renewed.IDToken)
			require.NotEmpty(t, renewed.RefreshToken)
			assert.NotEqual(t, tokens.RefreshToken, renewed.RefreshToken)
			ok, err = ts.auth.ValidateAdminToken(renewed.IDToken, adminClient)
			require.NoError(t, err)
			assert.True(t, ok)

			reused := ts.refresh(t, tokens.RefreshToken)
			assert.Equal(t, http.StatusBadRequest, reused.StatusCode)
		})
	}
}

func TestNonAdminTokenIsRejected(t *testing.T) {
	ts := newTestServer(t, db.NewMemoryDataStore())
	code := ts.login(t, ts.startAuth(t, "openid"), "editor", "editorpass")
	resp := ts.exchangeCode(t, code, codeVerifier)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	tokens := decodeTokens(t, resp)
	assert.Empty(t, tokens.RefreshToken, "no offline_access requested")

	ok, err := ts.auth.ValidateAdminToken(tokens.IDToken, &settings.SSOClient{ClientId: adminClientID})
	assert.False(t, ok)
	assert.ErrorContains(t, err, "missing admin role")
}

func TestPKCE(t *testing.T) {
	ts := newTestServer(t, db.NewMemoryDataStore())

	code := ts.login(t, ts.startAuth(t, "openid"), "admin", "adminpass")
	assert.Equal(t, http.StatusBadRequest, ts.exchangeCode(t, code, "wrong-verifier-wrong-verifier-wrong-verifier").StatusCode)

	// The public admin client cannot get tokens without PKCE.
	q := url.Values{
		"response_type": {"code"},
		"client_id":     {adminClientID},
		"redirect_uri":  {adminRedirect},
		"scope":         {"openid"},
		"state":         {"state-123"},
	}
	resp := ts.get(t, authPath+"?"+q.Encode())
	require.Equal(t, http.StatusFound, resp.StatusCode)
	code = ts.login(t, resp.Header.Get("Location"), "admin", "adminpass")
	noPKCE := ts.post(t, tokenPath, url.Values{
		"grant_type":   {"authorization_code"},
		"code":         {code},
		"redirect_uri": {adminRedirect},
		"client_id":    {adminClientID},
	}, "", "")
	assert.Equal(t, http.StatusBadRequest, noPKCE.StatusCode)
	assert.Contains(t, body(t, noPKCE), "PKCE required")
}

func TestAuthorizeRejectsUnknownClientAndRedirect(t *testing.T) {
	ts := newTestServer(t, db.NewMemoryDataStore())
	for name, q := range map[string]url.Values{
		"unknown client": {"response_type": {"code"}, "client_id": {"nope"}, "redirect_uri": {adminRedirect}, "scope": {"openid"}},
		"foreign redirect": {"response_type": {"code"}, "client_id": {adminClientID}, "redirect_uri": {"https://evil.example/"}, "scope": {"openid"},
			"code_challenge": {pkceS256(codeVerifier)}, "code_challenge_method": {"S256"}},
	} {
		t.Run(name, func(t *testing.T) {
			resp := ts.get(t, authPath+"?"+q.Encode())
			assert.NotEqual(t, http.StatusFound, resp.StatusCode, "must not redirect")
		})
	}
}

func TestLoginPage(t *testing.T) {
	ts := newTestServer(t, db.NewMemoryDataStore())
	loginURL := ts.startAuth(t, "openid")

	page := ts.get(t, loginURL)
	require.Equal(t, http.StatusOK, page.StatusCode)
	assert.Equal(t, "text/html; charset=utf-8", page.Header.Get("Content-Type"))
	form := formInputs(t, page.Body)
	authRequestID := form.Get("authRequestID")
	require.NotEmpty(t, authRequestID)

	for name, creds := range map[string][2]string{
		"wrong password": {"admin", "nope"},
		"unknown user":   {"nobody", "adminpass"},
		"empty password": {"empty", ""},
	} {
		t.Run(name, func(t *testing.T) {
			f := url.Values{"authRequestID": {authRequestID}, "username": {creds[0]}, "password": {creds[1]}}
			resp := ts.post(t, loginPath, f, "", "")
			require.Equal(t, http.StatusOK, resp.StatusCode)
			b := body(t, resp)
			assert.Contains(t, b, "Username or password invalid")
			assert.Contains(t, b, `<body data-reset-pkce="true">`)
			assert.Contains(t, b, `name="authRequestID" type="hidden" value="`+authRequestID+`"`)
		})
	}

	// The login still works after failed attempts.
	code := ts.login(t, loginURL, "admin", "adminpass")
	assert.Equal(t, http.StatusOK, ts.exchangeCode(t, code, codeVerifier).StatusCode)

	// An unknown or used auth request cannot log in.
	resp := ts.get(t, loginPath+"?authRequestID=does-not-exist")
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp = ts.post(t, loginPath, url.Values{"authRequestID": {authRequestID}, "username": {"admin"}, "password": {"adminpass"}}, "", "")
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestAuthRequestExpires(t *testing.T) {
	ts := newTestServer(t, db.NewMemoryDataStore())
	loginURL := ts.startAuth(t, "openid")
	ts.auth.storage.now = func() time.Time { return time.Now().Add(authRequestLifetime + time.Minute) }
	resp := ts.get(t, loginURL)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestClientCredentialsIntrospectionAndRevocation(t *testing.T) {
	ts := newTestServer(t, db.NewMemoryDataStore())

	bad := ts.post(t, tokenPath, url.Values{"grant_type": {"client_credentials"}, "scope": {"openid"}}, serviceClientID, "wrong")
	assert.Equal(t, http.StatusUnauthorized, bad.StatusCode)
	public := ts.post(t, tokenPath, url.Values{"grant_type": {"client_credentials"}, "client_id": {adminClientID}, "client_secret": {"admin"}, "scope": {"openid"}}, "", "")
	assert.NotEqual(t, http.StatusOK, public.StatusCode, "the admin client may not use client_credentials")

	// Secret in the form (client_secret_post) like the old tests did.
	resp := ts.post(t, tokenPath, url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {serviceClientID},
		"client_secret": {serviceSecret},
		"scope":         {"openid email"},
	}, "", "")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	accessToken := decodeTokens(t, resp).AccessToken
	require.NotEmpty(t, accessToken)

	introspect := func() bool {
		r := ts.post(t, introspectPath, url.Values{"token": {accessToken}}, serviceClientID, serviceSecret)
		require.Equal(t, http.StatusOK, r.StatusCode)
		var ir struct {
			Active   bool   `json:"active"`
			ClientID string `json:"client_id"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&ir))
		if ir.Active {
			assert.Equal(t, serviceClientID, ir.ClientID)
		}
		return ir.Active
	}
	assert.True(t, introspect())

	r := ts.post(t, introspectPath, url.Values{"token": {accessToken}}, serviceClientID, "wrong")
	assert.Equal(t, http.StatusUnauthorized, r.StatusCode)

	r = ts.post(t, revokePath, url.Values{"token": {accessToken}}, serviceClientID, serviceSecret)
	require.Equal(t, http.StatusOK, r.StatusCode, body(t, r))
	assert.False(t, introspect(), "revoked")
}

// The provider must not parse client assertions: go-jose CVE-2026-116587
// (RSA JWK DoS) was reachable through fosite's private_key_jwt support.
func TestClientAssertionIsRejected(t *testing.T) {
	ts := newTestServer(t, db.NewMemoryDataStore())
	hdr, _ := json.Marshal(map[string]any{"alg": "RS256", "jwk": map[string]string{"kty": "RSA", "n": "AQAB", "e": "AQAB", "d": "Aw", "p": "Aw", "q": "Aw"}})
	assertion := base64.RawURLEncoding.EncodeToString(hdr) + ".e30.AAAA"
	start := time.Now()
	resp := ts.post(t, tokenPath, url.Values{
		"grant_type":            {"client_credentials"},
		"client_assertion_type": {"urn:ietf:params:oauth:client-assertion-type:jwt-bearer"},
		"client_assertion":      {assertion},
	}, "", "")
	assert.NotEqual(t, http.StatusOK, resp.StatusCode)
	assert.Less(t, time.Since(start), 2*time.Second)
}

// A second instance on the same database (or a restart) accepts the
// tokens and keys of the first.
func TestStateIsSharedThroughTheDatastore(t *testing.T) {
	store := db.NewMemoryDataStore()
	first := newTestServer(t, store)
	code := first.login(t, first.startAuth(t, "openid offline_access"), "admin", "adminpass")
	resp := first.exchangeCode(t, code, codeVerifier)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	tokens := decodeTokens(t, resp)

	second := newTestServer(t, store)
	assert.Equal(t, first.auth.privateKey.N, second.auth.privateKey.N)

	resp = second.post(t, tokenPath, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {tokens.RefreshToken},
		"client_id":     {adminClientID},
	}, "", "")
	require.Equal(t, http.StatusOK, resp.StatusCode, body(t, resp))

	// An auth request started on one instance can be finished on the other.
	loginURL := first.startAuth(t, "openid")
	code = second.login(t, loginURL, "admin", "adminpass")
	assert.Equal(t, http.StatusOK, first.exchangeCode(t, code, codeVerifier).StatusCode)
}

// The storage keeps everything in the datastore's oauth_* tables, which
// differ per database (e.g. Memory reports a missing row as an error,
// Postgres as nil).
func TestSQLDatastores(t *testing.T) {
	datastores := map[string]func(t *testing.T) db.DataStore{
		"SQLite": func(t *testing.T) db.DataStore {
			store, err := db.NewSQLiteDB(":memory:")
			require.NoError(t, err)
			return store
		},
		"Postgres": func(t *testing.T) db.DataStore {
			srv := testdb.Get(t, testdb.Postgres)
			store, err := db.NewPostgresDB(db.PostgresOptions{Username: srv.AdminUser(), Password: testdb.Password,
				Database: srv.NewDatabase(t), Host: srv.Host, Port: srv.Port, MaxConns: 4})
			require.NoError(t, err)
			return store
		},
		"MySQL": func(t *testing.T) db.DataStore {
			srv := testdb.Get(t, testdb.MySQL)
			store, err := db.NewMySQLDB(db.MySQLOptions{Username: srv.AdminUser(), Password: testdb.Password,
				Database: srv.NewDatabase(t), Host: srv.Host, Port: srv.Port, MaxConns: 4})
			require.NoError(t, err)
			return store
		},
	}
	for name, newStore := range datastores {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			store := newStore(t)
			t.Cleanup(func() { store.Close() })
			ts := newTestServer(t, store)

			code := ts.login(t, ts.startAuth(t, "openid offline_access"), "admin", "adminpass")
			resp := ts.exchangeCode(t, code, codeVerifier)
			require.Equal(t, http.StatusOK, resp.StatusCode)
			tokens := decodeTokens(t, resp)
			assert.Equal(t, http.StatusBadRequest, ts.exchangeCode(t, code, codeVerifier).StatusCode)
			ok, err := ts.auth.ValidateAdminToken(tokens.IDToken, &settings.SSOClient{ClientId: adminClientID})
			require.NoError(t, err)
			assert.True(t, ok)

			assert.Equal(t, http.StatusOK, ts.refresh(t, tokens.RefreshToken).StatusCode)
			assert.Equal(t, http.StatusBadRequest, ts.refresh(t, tokens.RefreshToken).StatusCode)

			// Restart on the same database: same key.
			again := newTestServer(t, store)
			assert.Equal(t, ts.auth.privateKey.N, again.auth.privateKey.N)
		})
	}
}

func TestValidateAdminToken(t *testing.T) {
	ts := newTestServer(t, db.NewMemoryDataStore())
	auth := ts.auth
	adminClient := &settings.SSOClient{ClientId: "admin-client"}
	now := time.Now()

	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: auth.privateKey}, nil)
	require.NoError(t, err)
	sign := func(claims map[string]any) string {
		s, err := jwt.Signed(signer).Claims(claims).Serialize()
		require.NoError(t, err)
		return s
	}
	otherKey, err := rsaKey()
	require.NoError(t, err)
	otherSigner, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: otherKey}, nil)
	require.NoError(t, err)
	hsSigner, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.HS256, Key: []byte("0123456789abcdef0123456789abcdef")}, nil)
	require.NoError(t, err)

	valid := map[string]any{"exp": now.Add(time.Hour).Unix(), "aud": []string{"admin-client"}, "iss": auth.issuer, "admin": true}
	with := func(k string, v any) map[string]any {
		c := map[string]any{}
		for key, val := range valid {
			c[key] = val
		}
		if v == nil {
			delete(c, k)
		} else {
			c[k] = v
		}
		return c
	}
	foreign, _ := jwt.Signed(otherSigner).Claims(valid).Serialize()
	hmac, _ := jwt.Signed(hsSigner).Claims(valid).Serialize()

	tests := []struct {
		name  string
		token string
		ok    bool
	}{
		{"invalid jwt", "not-a-jwt", false},
		{"expired", sign(with("exp", now.Add(-time.Hour).Unix())), false},
		{"missing exp", sign(with("exp", nil)), false},
		{"missing aud", sign(with("aud", nil)), false},
		{"other audience", sign(with("aud", []string{"other-client"})), false},
		{"other issuer", sign(with("iss", "https://wrong-issuer")), false},
		{"missing admin", sign(with("admin", nil)), false},
		{"admin false", sign(with("admin", false)), false},
		{"other key", foreign, false},
		{"HS256", hmac, false},
		{"valid", sign(valid), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, err := auth.ValidateAdminToken(tt.token, adminClient)
			assert.Equal(t, tt.ok, ok)
			if tt.ok {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func rsaKey() (*rsa.PrivateKey, error) { return rsa.GenerateKey(rand.Reader, 2048) }

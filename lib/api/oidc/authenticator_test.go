package oidc

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/ory/fosite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/net/html"
)

type JSONWebKey struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type JSONWebKeySet struct {
	Keys []JSONWebKey `json:"keys"`
}

func TestNewAuthenticator(t *testing.T) {
	settingsAuth := testSettings()
	auth := NewAuthenticator(settingsAuth, db.NewMemoryDataStore())
	require.NotNil(t, auth)
	require.NotNil(t, auth.provider)
	require.NotNil(t, auth.store)
	require.NotNil(t, auth.privateKey)
	assert.Equal(t, settingsAuth, auth.retrievedSettings)
}

func TestJwksEndpoint(t *testing.T) {
	settings := testSettings()
	auth := NewAuthenticator(settings, db.NewMemoryDataStore())

	req := httptest.NewRequest("GET", "/.well-known/jwks.json", nil)
	w := httptest.NewRecorder()

	auth.JwksEndpoint(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var jwks JSONWebKeySet
	err := json.NewDecoder(w.Body).Decode(&jwks)
	require.NoError(t, err)
	require.Len(t, jwks.Keys, 1)
	assert.Equal(t, "RSA", jwks.Keys[0].Kty)
	assert.Equal(t, "RS256", jwks.Keys[0].Alg)
}

func TestOicWellKnown(t *testing.T) {
	settingsAuth := testSettings()
	auth := &Authenticator{retrievedSettings: settingsAuth}

	req := httptest.NewRequest("GET", "/.well-known/openid_configuration", nil)
	w := httptest.NewRecorder()

	auth.OicWellKnown(w, req, settingsAuth)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var wellKnown WellKnownResponse
	err := json.NewDecoder(w.Body).Decode(&wellKnown)
	require.NoError(t, err)
	assert.Equal(t, settingsAuth.SSO.Issuer, wellKnown.Issuer)
	assert.Contains(t, wellKnown.ScopesSupported, "openid")
	assert.Contains(t, wellKnown.ScopesSupported, "email")
}

func TestAuthEndpoint_GET_MinimalOIDC(t *testing.T) {
	settingsAuth := testSettings()
	auth := NewAuthenticator(settingsAuth, db.NewMemoryDataStore())
	setupLogger := zap.NewNop().Sugar()

	// Minimale OpenID Connect Parameter
	q := url.Values{}
	q.Add("client_id", "test-client")
	q.Add("redirect_uri", "http://localhost/callback")
	q.Add("scope", "openid")
	q.Add("response_type", "code")
	q.Add("state", "xyz12323123123")

	req := httptest.NewRequest("GET", "/oauth2/auth?"+q.Encode(), nil)
	w := httptest.NewRecorder()

	auth.AuthEndpoint(w, req, setupLogger, settingsAuth)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/html; charset=utf-8", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Body.String(), `<form`)
}

func TestAuthEndpoint_GET_FullOIDC(t *testing.T) {
	settingsAuth := testSettings()
	auth := NewAuthenticator(settingsAuth, db.NewMemoryDataStore())
	setupLogger := zap.NewNop().Sugar()

	q := url.Values{}
	q.Add("client_id", "test-client")
	q.Add("redirect_uri", "http://localhost/callback")
	q.Add("scope", "openid email profile")
	q.Add("response_type", "code")
	q.Add("state", "xyz12323123123")
	q.Add("nonce", "random-nonce-123")
	q.Add("max_age", "3600")
	q.Add("ui_locales", "de")

	req := httptest.NewRequest("GET", "/oauth2/auth?"+q.Encode(), nil)
	w := httptest.NewRecorder()

	auth.AuthEndpoint(w, req, setupLogger, settingsAuth)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/html; charset=utf-8", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Body.String(), `<form`)
}

func TestAuthEndpoint_POST_Success(t *testing.T) {
	settingsAuth := testSettings()
	auth := NewAuthenticator(settingsAuth, db.NewMemoryDataStore())
	setupLogger := zap.NewNop().Sugar()

	form := url.Values{}
	form.Add("username", "testuser")
	form.Add("password", "testpass")
	form.Add("scopes", "openid")
	form.Add("scopes", "email")
	form.Add("client_id", "test-client")
	form.Add("redirect_uri", "http://localhost/callback")
	form.Add("response_type", "code")
	form.Add("state", "xyz12323123123")

	req := httptest.NewRequest("POST", "/oauth2/auth", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.PostForm = form
	w := httptest.NewRecorder()

	auth.AuthEndpoint(w, req, setupLogger, settingsAuth)

	assert.Equal(t, http.StatusSeeOther, w.Code)
	assert.NotEmpty(t, w.Header().Get("Location"))
}

func TestAuthEndpoint_POST_InvalidClient(t *testing.T) {
	settingsAuth := testSettings()
	auth := NewAuthenticator(settingsAuth, db.NewMemoryDataStore())
	setupLogger := zap.NewNop().Sugar()

	form := url.Values{}
	form.Add("username", "wronguser")
	form.Add("password", "wrongpass")
	form.Add("scopes", "openid")

	req := httptest.NewRequest("POST", "/oauth2/auth", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.PostForm = form
	w := httptest.NewRecorder()

	auth.AuthEndpoint(w, req, setupLogger, settingsAuth)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "The requested OAuth 2.0 Client does not exist")
}

func TestAuthEndpoint_POST_InvalidCredentials(t *testing.T) {
	settingsAuth := testSettings()
	auth := NewAuthenticator(settingsAuth, db.NewMemoryDataStore())
	setupLogger := zap.NewNop().Sugar()

	form := url.Values{}
	form.Add("username", "wronguser")
	form.Add("password", "wrongpass")
	form.Add("scopes", "openid")
	form.Add("client_id", "test-client")
	form.Add("redirect_uri", "http://localhost/callback")
	form.Add("response_type", "code")
	form.Add("state", "xyz12323123123")
	form.Add("code_challenge", "pkce-challenge")
	form.Add("code_challenge_method", "S256")

	req := httptest.NewRequest("POST", "/oauth2/auth", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.PostForm = form
	w := httptest.NewRecorder()

	auth.AuthEndpoint(w, req, setupLogger, settingsAuth)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Username or password invalid")
	assert.Contains(t, w.Body.String(), `<body data-reset-pkce="true">`)
	assert.Contains(t, w.Body.String(), `name="client_id" type="hidden" value="test-client"`)
	assert.Contains(t, w.Body.String(), `name="redirect_uri" type="hidden" value="http://localhost/callback"`)
	assert.Contains(t, w.Body.String(), `name="state" type="hidden" value="xyz12323123123"`)
	assert.Contains(t, w.Body.String(), `name="code_challenge" type="hidden" value="pkce-challenge"`)
	assert.Contains(t, w.Body.String(), `name="code_challenge_method" type="hidden" value="S256"`)
}

func TestAuthEndpoint_PKCESurvivesFailedLoginRetry(t *testing.T) {
	settingsAuth := testSettings()
	auth := NewAuthenticator(settingsAuth, db.NewMemoryDataStore())
	setupLogger := zap.NewNop().Sugar()
	codeVerifier := "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG"
	codeChallenge := pkceS256(codeVerifier)

	initialQuery := url.Values{}
	initialQuery.Add("client_id", "test-client")
	initialQuery.Add("redirect_uri", "http://localhost/callback")
	initialQuery.Add("response_type", "code")
	initialQuery.Add("scope", "openid email")
	initialQuery.Add("state", "xyz12323123123")
	initialQuery.Add("code_challenge", codeChallenge)
	initialQuery.Add("code_challenge_method", "S256")

	getReq := httptest.NewRequest("GET", "/oauth2/auth?"+initialQuery.Encode(), nil)
	getW := httptest.NewRecorder()
	auth.AuthEndpoint(getW, getReq, setupLogger, settingsAuth)
	require.Equal(t, http.StatusOK, getW.Code)

	formAfterGet := extractFormInputs(t, getW.Body)
	formAfterGet.Set("username", "wronguser")
	formAfterGet.Set("password", "wrongpass")

	invalidReq := httptest.NewRequest("POST", "/oauth2/auth", strings.NewReader(formAfterGet.Encode()))
	invalidReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	invalidReq.PostForm = formAfterGet
	invalidW := httptest.NewRecorder()
	auth.AuthEndpoint(invalidW, invalidReq, setupLogger, settingsAuth)
	require.Equal(t, http.StatusOK, invalidW.Code)

	formAfterInvalid := extractFormInputs(t, invalidW.Body)
	assert.Equal(t, "test-client", formAfterInvalid.Get("client_id"))
	assert.Equal(t, "http://localhost/callback", formAfterInvalid.Get("redirect_uri"))
	assert.Equal(t, "xyz12323123123", formAfterInvalid.Get("state"))
	assert.Equal(t, codeChallenge, formAfterInvalid.Get("code_challenge"))
	assert.Equal(t, "S256", formAfterInvalid.Get("code_challenge_method"))

	formAfterInvalid.Set("username", "testuser")
	formAfterInvalid.Set("password", "testpass")

	validReq := httptest.NewRequest("POST", "/oauth2/auth", strings.NewReader(formAfterInvalid.Encode()))
	validReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	validReq.PostForm = formAfterInvalid
	validW := httptest.NewRecorder()
	auth.AuthEndpoint(validW, validReq, setupLogger, settingsAuth)
	require.Equal(t, http.StatusSeeOther, validW.Code)
	require.Len(t, auth.store.PKCES, 1)

	redirectLocation := validW.Header().Get("Location")
	require.NotEmpty(t, redirectLocation)

	redirectURL, err := url.Parse(redirectLocation)
	require.NoError(t, err)
	code := redirectURL.Query().Get("code")
	require.NotEmpty(t, code)

	tokenForm := url.Values{}
	tokenForm.Add("grant_type", "authorization_code")
	tokenForm.Add("code", code)
	tokenForm.Add("redirect_uri", "http://localhost/callback")
	tokenForm.Add("client_id", "test-client")
	tokenForm.Add("client_secret", "test-secret")
	tokenForm.Add("code_verifier", codeVerifier)

	tokenReq := httptest.NewRequest("POST", "/oauth2/token", strings.NewReader(tokenForm.Encode()))
	tokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	tokenReq.PostForm = tokenForm
	tokenW := httptest.NewRecorder()
	auth.TokenEndpoint(tokenW, tokenReq)

	assert.Equal(t, http.StatusOK, tokenW.Code)
}

func extractFormInputs(t *testing.T, body io.Reader) url.Values {
	t.Helper()

	doc, err := html.Parse(body)
	require.NoError(t, err)

	values := url.Values{}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "input" {
			var (
				name  string
				value string
			)
			for _, attr := range n.Attr {
				if attr.Key == "name" {
					name = attr.Val
				}
				if attr.Key == "value" {
					value = attr.Val
				}
			}
			if name != "" {
				values.Add(name, value)
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)

	return values
}

func TestTokenEndpoint_ClientCredentials(t *testing.T) {
	settingsAuth := testSettings()

	// Plaintext client secret - fosite expects plaintext, not bcrypt
	form := url.Values{}
	form.Add("grant_type", "client_credentials")
	form.Add("client_id", "test-client")
	form.Add("client_secret", "test-secret")
	form.Add("scope", "openid email")

	req := httptest.NewRequest("POST", "/oauth2/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.PostForm = form
	w := httptest.NewRecorder()

	auth := NewAuthenticator(settingsAuth, db.NewMemoryDataStore())
	auth.TokenEndpoint(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestIntrospectionEndpoint(t *testing.T) {
	retrievedSettings := testSettings()
	auth := NewAuthenticator(retrievedSettings, db.NewMemoryDataStore())

	tokenForm := url.Values{}
	tokenForm.Add("grant_type", "client_credentials")
	tokenForm.Add("client_id", "test-client")
	tokenForm.Add("client_secret", "test-secret")
	tokenForm.Add("scope", "openid email")

	tokenReq := httptest.NewRequest("POST", "/oauth2/token", strings.NewReader(tokenForm.Encode()))
	tokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	tokenReq.PostForm = tokenForm
	tokenW := httptest.NewRecorder()

	auth.TokenEndpoint(tokenW, tokenReq)

	assert.Equal(t, http.StatusOK, tokenW.Code)

	var tokenResp struct {
		AccessToken string `json:"access_token"`
	}
	err := json.NewDecoder(tokenW.Body).Decode(&tokenResp)
	require.NoError(t, err)
	assert.NotEmpty(t, tokenResp.AccessToken)

	introspectForm := url.Values{}
	introspectForm.Add("token", tokenResp.AccessToken)

	req := httptest.NewRequest("POST", "/oauth2/introspect", strings.NewReader(introspectForm.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.PostForm = introspectForm

	authHeader := base64.StdEncoding.EncodeToString([]byte("test-client:test-secret"))
	req.Header.Set("Authorization", "Basic "+authHeader)

	w := httptest.NewRecorder()

	auth.IntrospectionEndpoint(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var introspectResp struct {
		Active bool `json:"active"`
	}
	err = json.NewDecoder(w.Body).Decode(&introspectResp)
	require.NoError(t, err)
	assert.True(t, introspectResp.Active)
}

func TestRevokeEndpoint(t *testing.T) {
	retrievedSettings := testSettings()
	auth := NewAuthenticator(retrievedSettings, db.NewMemoryDataStore())

	tokenForm := url.Values{}
	tokenForm.Add("grant_type", "client_credentials")
	tokenForm.Add("client_id", "test-client")
	tokenForm.Add("client_secret", "test-secret")
	tokenForm.Add("scope", "openid email")

	tokenReq := httptest.NewRequest("POST", "/oauth2/token", strings.NewReader(tokenForm.Encode()))
	tokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	tokenReq.PostForm = tokenForm
	tokenW := httptest.NewRecorder()

	auth.TokenEndpoint(tokenW, tokenReq)

	assert.Equal(t, http.StatusOK, tokenW.Code)

	var tokenResp struct {
		AccessToken string `json:"access_token"`
	}
	err := json.NewDecoder(tokenW.Body).Decode(&tokenResp)
	require.NoError(t, err)
	assert.NotEmpty(t, tokenResp.AccessToken)

	// Nun den Token revoken
	revokeForm := url.Values{}
	revokeForm.Add("token", tokenResp.AccessToken)

	req := httptest.NewRequest("POST", "/oauth2/revoke", strings.NewReader(revokeForm.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.PostForm = revokeForm

	authHeader := base64.StdEncoding.EncodeToString([]byte("test-client:test-secret"))
	req.Header.Set("Authorization", "Basic "+authHeader)

	w := httptest.NewRecorder()

	auth.RevokeEndpoint(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	introspectForm := url.Values{}
	introspectForm.Add("token", tokenResp.AccessToken)

	introspectReq := httptest.NewRequest("POST", "/oauth2/introspect", strings.NewReader(introspectForm.Encode()))
	introspectReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	introspectReq.PostForm = introspectForm
	introspectReq.Header.Set("Authorization", "Basic "+authHeader)
	introspectW := httptest.NewRecorder()

	auth.IntrospectionEndpoint(introspectW, introspectReq)

	assert.Equal(t, http.StatusOK, introspectW.Code)

	var introspectResp struct {
		Active bool `json:"active"`
	}
	err = json.NewDecoder(introspectW.Body).Decode(&introspectResp)
	require.NoError(t, err)
	assert.False(t, introspectResp.Active, "Token should be inactive after revocation")
}

func TestNewSession(t *testing.T) {
	settingsAuth := testSettings()
	auth := NewAuthenticator(settingsAuth, db.NewMemoryDataStore())

	user := &MemoryUserRelation{
		Username: "testuser",
		Admin:    true,
	}

	session := auth.newSession(user, "test-client")

	assert.Equal(t, "testuser", session.Claims.Subject)
	assert.Equal(t, []string{"test-client"}, session.Claims.Audience)
	assert.True(t, session.Claims.Extra["admin"].(bool))
}

func testSettings() *settings.Settings {
	testSecret := "test-secret"
	return &settings.Settings{
		SSO: &settings.SSO{
			Issuer: "http://localhost:9000",
			Clients: []settings.SSOClient{
				{
					ClientId:     "test-client",
					ClientSecret: &testSecret,
					RedirectUris: []string{"http://localhost/callback"},
					GrantTypes:   []string{"authorization_code", "client_credentials"},
				},
			},
		},
		Users: map[string]settings.User{
			"testuser": {
				Password: strPtr("testpass"),
				IsAdmin:  boolPtr(true),
			},
		},
	}
}

func strPtr(s string) *string {
	return &s
}

func boolPtr(b bool) *bool {
	return &b
}

func TestAuthenticatorPersistsSigningKeyAndRefreshSessions(t *testing.T) {
	persistence := db.NewMemoryDataStore()
	settingsAuth := testSettings()

	auth1 := NewAuthenticator(settingsAuth, persistence)
	require.NotNil(t, auth1.privateKey)

	req := fosite.NewRequest()
	req.Client = auth1.store.Clients["test-client"]
	req.ID = "request-1"
	req.RequestedAt = req.RequestedAt.UTC()
	req.Form = url.Values{
		"grant_type": {"refresh_token"},
		"client_id":  {"test-client"},
	}
	req.SetRequestedScopes(fosite.Arguments{"openid", "offline"})
	req.GrantScope("openid")
	req.GrantScope("offline")
	req.SetSession(auth1.newSession(&MemoryUserRelation{
		Username: "testuser",
		Admin:    true,
	}, "test-client"))

	err := auth1.store.CreateRefreshTokenSession(t.Context(), "refresh-signature", "access-signature", req)
	require.NoError(t, err)

	auth2 := NewAuthenticator(settingsAuth, persistence)

	assert.Equal(t, auth1.privateKey.PublicKey.N.String(), auth2.privateKey.PublicKey.N.String())
	assert.Equal(t, auth1.privateKey.PublicKey.E, auth2.privateKey.PublicKey.E)

	restored, err := auth2.store.GetRefreshTokenSession(t.Context(), "refresh-signature", nil)
	require.NoError(t, err)
	require.NotNil(t, restored)
	assert.Equal(t, "request-1", restored.GetID())
	assert.Equal(t, "test-client", restored.GetClient().GetID())
	assert.Equal(t, []string{"openid", "offline"}, []string(restored.GetGrantedScopes()))
}

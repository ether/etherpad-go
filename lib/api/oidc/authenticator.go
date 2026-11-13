package oidc

import (
	"crypto/rsa"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/ether/etherpad-go/assets/login"
	"github.com/ether/etherpad-go/lib/api/constants"
	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/settings"
	jose "github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/zitadel/oidc/v3/pkg/op"
)

// Endpoint paths, relative to the issuer. They are the ones the fosite based
// provider used, so clients keep working.
const (
	authPath       = "/oauth2/auth"
	tokenPath      = "/oauth2/token"
	introspectPath = "/oauth2/introspect"
	revokePath     = "/oauth2/revoke"
	userinfoPath   = "/oauth2/userinfo"
	logoutPath     = "/oauth2/logout"
	jwksPath       = "/.well-known/jwks.json"
	discoveryPath  = "/.well-known/openid-configuration"
	callbackPath   = authPath + "/callback"
)

// Authenticator is Etherpad's OpenID Connect provider, built on
// github.com/zitadel/oidc. Users log in on Etherpad's own login page with
// the users from the settings.
type Authenticator struct {
	provider          *op.Provider
	storage           *Storage
	privateKey        *rsa.PrivateKey
	issuer            string
	retrievedSettings *settings.Settings
}

func issuerOf(s *settings.Settings) string {
	if s.SSO == nil || s.SSO.Issuer == "" {
		return "http://localhost:" + s.Port
	}
	return strings.TrimRight(s.SSO.Issuer, "/")
}

func NewAuthenticator(retrievedSettings *settings.Settings, persistence db.DataStore) *Authenticator {
	a, err := newAuthenticator(retrievedSettings, persistence)
	if err != nil {
		log.Fatalf("Error setting up the OpenID Connect provider: %v", err)
	}
	return a
}

func newAuthenticator(retrievedSettings *settings.Settings, persistence db.DataStore) (*Authenticator, error) {
	clients := make(map[string]*Client)
	if retrievedSettings.SSO != nil {
		for _, sso := range retrievedSettings.SSO.Clients {
			c, err := newClient(sso)
			if err != nil {
				return nil, fmt.Errorf("SSO client %s: %w", sso.ClientId, err)
			}
			clients[sso.ClientId] = c
		}
	}

	users := make(map[string]user)
	for username, u := range retrievedSettings.Users {
		if u.Password == nil || *u.Password == "" {
			continue
		}
		users[username] = user{
			Username: username,
			Password: *u.Password,
			Admin:    u.IsAdmin != nil && *u.IsAdmin,
		}
	}

	privateKey, err := loadOrCreateSigningKey(persistence)
	if err != nil {
		return nil, fmt.Errorf("loading the signing key: %w", err)
	}
	cryptoKey, err := loadOrCreateCryptoKey(persistence)
	if err != nil {
		return nil, fmt.Errorf("loading the token key: %w", err)
	}
	// State of the former fosite provider; its tokens are no longer valid.
	_ = persistence.DeleteOIDCStorageValue(legacyStateStorageKey)

	storage := newStorage(persistence, clients, users, privateKey)
	issuer := issuerOf(retrievedSettings)

	config := &op.Config{
		CryptoKey:                cryptoKey,
		CodeMethodS256:           true,
		AuthMethodPost:           true,
		AuthMethodPrivateKeyJWT:  false,
		GrantTypeRefreshToken:    true,
		RequestObjectSupported:   false,
		DefaultLogoutRedirectURI: "/",
	}
	// The endpoints are served at the root of the server, also when the
	// issuer has a path (e.g. "http://localhost:9001/oauth2"), so their
	// absolute URLs are built from the issuer's origin.
	origin := originOf(issuer)
	endpoint := func(path string) *op.Endpoint {
		return op.NewEndpointWithURL(strings.TrimPrefix(path, "/"), origin+path)
	}
	opts := []op.Option{
		op.WithCustomAuthEndpoint(endpoint(authPath)),
		op.WithCustomTokenEndpoint(endpoint(tokenPath)),
		op.WithCustomIntrospectionEndpoint(endpoint(introspectPath)),
		op.WithCustomRevocationEndpoint(endpoint(revokePath)),
		op.WithCustomUserinfoEndpoint(endpoint(userinfoPath)),
		op.WithCustomEndSessionEndpoint(endpoint(logoutPath)),
		op.WithCustomKeysEndpoint(endpoint(jwksPath)),
		// CORS is configured on the Fiber app (see Init).
		op.WithCORSOptions(nil),
		op.WithLogger(slog.Default().With("component", "oidc")),
	}
	if strings.HasPrefix(issuer, "http://") {
		opts = append(opts, op.WithAllowInsecure())
	}
	provider, err := op.NewOpenIDProvider(issuer, config, storage, opts...)
	if err != nil {
		return nil, err
	}

	return &Authenticator{
		provider:          provider,
		storage:           storage,
		privateKey:        privateKey,
		issuer:            issuer,
		retrievedSettings: retrievedSettings,
	}, nil
}

// originOf returns scheme://host of the issuer.
func originOf(issuer string) string {
	u, err := url.Parse(issuer)
	if err != nil || u.Host == "" {
		return issuer
	}
	return u.Scheme + "://" + u.Host
}

// discoveryPaths are where the discovery document is served: at the root
// and, as OpenID Connect Discovery requires, below the issuer's path.
func (a *Authenticator) discoveryPaths() []string {
	paths := []string{discoveryPath}
	if u, err := url.Parse(a.issuer); err == nil && strings.Trim(u.Path, "/") != "" {
		paths = append(paths, strings.TrimRight(u.Path, "/")+discoveryPath)
	}
	return paths
}

// Handler serves the OpenID Connect endpoints.
func (a *Authenticator) Handler() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if req.URL.Path != discoveryPath && strings.HasSuffix(req.URL.Path, discoveryPath) {
			req.URL.Path = discoveryPath
		}
		a.provider.ServeHTTP(rw, req)
	})
}

// Paths lists every path Handler serves.
func (a *Authenticator) Paths() []string {
	return append(a.discoveryPaths(), jwksPath, authPath, callbackPath, tokenPath,
		introspectPath, revokePath, userinfoPath, logoutPath)
}

// Stop is kept for the server's shutdown sequence; nothing runs in the
// background anymore.
func (a *Authenticator) Stop() {}

// ValidateAdminToken checks an ID token issued by this provider for the
// admin client and reports whether it belongs to an admin.
func (a *Authenticator) ValidateAdminToken(tokenString string, adminClient *settings.SSOClient) (bool, error) {
	token, err := jwt.ParseSigned(tokenString, []jose.SignatureAlgorithm{jose.RS256})
	if err != nil {
		return false, fmt.Errorf("token validation failed: %w", err)
	}
	var claims jwt.Claims
	var extra struct {
		Admin *bool `json:"admin"`
	}
	if err := token.Claims(&a.privateKey.PublicKey, &claims, &extra); err != nil {
		return false, fmt.Errorf("token validation failed: %w", err)
	}
	if claims.Expiry == nil {
		return false, errors.New("missing exp claim")
	}
	if claims.Expiry.Time().Before(time.Now()) {
		return false, errors.New("token expired")
	}
	if len(claims.Audience) == 0 {
		return false, errors.New("missing aud claim")
	}
	if !slices.Contains([]string(claims.Audience), adminClient.ClientId) {
		return false, errors.New("invalid audience")
	}
	if claims.Issuer != a.issuer {
		return false, errors.New("invalid issuer")
	}
	if extra.Admin == nil || !*extra.Admin {
		return false, errors.New("missing admin role")
	}
	return true, nil
}

// LoginHandler is the login page the provider redirects auth requests to.
// GET shows the form, POST checks the credentials and hands the auth request
// back to the provider, which redirects to the client with a code.
func (a *Authenticator) LoginHandler(rw http.ResponseWriter, req *http.Request) {
	if err := req.ParseForm(); err != nil {
		http.Error(rw, "invalid request", http.StatusBadRequest)
		return
	}
	id := req.Form.Get("authRequestID")
	ar, err := a.storage.getAuthRequest(id)
	if err != nil {
		http.Error(rw, "The login request is invalid or has expired. Please start the login again.", http.StatusBadRequest)
		return
	}

	if req.Method != http.MethodPost {
		a.renderLogin(rw, req, ar, false, nil)
		return
	}

	username := req.PostForm.Get("username")
	if !a.storage.checkPassword(username, req.PostForm.Get("password")) {
		time.Sleep(500 * time.Millisecond)
		msg := "Username or password invalid"
		a.renderLogin(rw, req, ar, true, &msg)
		return
	}
	if err := a.storage.completeAuthRequest(ar.ID, username); err != nil {
		http.Error(rw, "The login request is invalid or has expired. Please start the login again.", http.StatusBadRequest)
		return
	}
	http.Redirect(rw, req, callbackPath+"?id="+url.QueryEscape(ar.ID), http.StatusFound)
}

func (a *Authenticator) renderLogin(rw http.ResponseWriter, req *http.Request, ar *authRequest, resetPKCE bool, errorMessage *string) {
	var client settings.SSOClient
	if c, ok := a.storage.clients[ar.ClientID]; ok {
		client = c.settings
	}
	params := url.Values{"authRequestID": {ar.ID}}
	rw.Header().Set("Content-Type", constants.ContentTypeHTML)
	if err := login.Login(client, ar.Scopes, params, resetPKCE, errorMessage).Render(req.Context(), rw); err != nil {
		http.Error(rw, "rendering the login page failed", http.StatusInternalServerError)
	}
}

// ValidateServiceToken reports whether token is an access token a
// configured client got with the client_credentials grant. etherpad-lite
// accepts such tokens for its HTTP API (/api); /admin/api takes admin
// tokens only. A token issued to a user is never one, even when the user
// is named like a client.
func (a *Authenticator) ValidateServiceToken(token string) bool {
	if token == "" || a.provider == nil {
		return false
	}
	// Opaque access tokens are "<token id>:<subject>", encrypted by the
	// provider (op.CreateBearerToken).
	plain, err := a.provider.Crypto().Decrypt(token)
	if err != nil {
		return false
	}
	id, subject, found := strings.Cut(plain, ":")
	if !found {
		return false
	}
	t, err := a.storage.getAccessToken(id)
	if err != nil || t.Subject != subject {
		return false
	}
	c, ok := a.storage.clients[t.ClientID]
	if !ok || !c.confidential {
		return false
	}
	if _, isUser := a.storage.users[t.Subject]; isUser {
		return false
	}
	return t.Subject == t.ClientID && t.RefreshTokenID == ""
}

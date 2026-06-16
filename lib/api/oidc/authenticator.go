package oidc

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"slices"
	"sync"
	"time"

	"github.com/ether/etherpad-go/assets/login"
	"github.com/ether/etherpad-go/lib/api/constants"
	db "github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/models/oidc"
	"github.com/ether/etherpad-go/lib/security"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/ory/fosite"
	"github.com/ory/fosite/compose"
	"github.com/ory/fosite/handler/openid"
	"github.com/ory/fosite/token/jwt"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type Authenticator struct {
	// provider is rebuilt whenever the SecretRotator rotates the global HMAC
	// secret, so it is guarded by mu. Read it via currentProvider().
	mu                sync.RWMutex
	provider          fosite.OAuth2Provider
	store             *MemoryStore
	privateKey        *rsa.PrivateKey
	retrievedSettings *settings.Settings
	rotator           *security.SecretRotator
}

func NewAuthenticator(retrievedSettings *settings.Settings, persistence db.DataStore) *Authenticator {
	store := NewMemoryStore(persistence)
	for _, sso := range retrievedSettings.SSO.Clients {
		isPublic := false

		if !slices.Contains(sso.GrantTypes, "client_credentials") {
			isPublic = true
		}

		clientToSso := &fosite.DefaultClient{
			ID:            sso.ClientId,
			RedirectURIs:  sso.RedirectUris,
			GrantTypes:    sso.GrantTypes,
			Audience:      []string{"etherpad-go"},
			Public:        isPublic,
			ResponseTypes: []string{"code"},
			Scopes:        []string{"openid", "email", "profile", "offline"},
		}

		if sso.ClientSecret != nil && *sso.ClientSecret != "" {
			hashedSecret, err := bcrypt.GenerateFromPassword([]byte(*sso.ClientSecret), bcrypt.DefaultCost)
			if err != nil {
				log.Fatalf("Error hashing client secret: %v", err)
			}

			clientToSso.Secret = hashedSecret
		}
		store.Clients[sso.ClientId] = clientToSso

	}

	for username, user := range retrievedSettings.Users {
		if user.Password == nil || *user.Password == "" {
			continue
		}
		isAdmin := false
		if user.IsAdmin != nil {
			isAdmin = *user.IsAdmin
		}
		store.Users[username] = MemoryUserRelation{
			Username: username,
			Password: *user.Password,
			Admin:    isAdmin,
		}
	}

	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	privateKey, err := loadOrCreatePrivateKey(persistence, privateKey)
	if err != nil {
		log.Fatalf("Error loading oidc signing key: %v", err)
	}
	if err := store.loadSnapshot(); err != nil {
		log.Fatalf("Error loading oidc store snapshot: %v", err)
	}

	a := &Authenticator{
		store:             store,
		privateKey:        privateKey,
		retrievedSettings: retrievedSettings,
	}

	// The fosite GlobalSecret (used to HMAC short-lived artifacts such as
	// authorize codes) was previously hard-coded. It is now a randomly
	// generated, database-persisted secret that rotates on the configured
	// cookie key-rotation interval. Old secrets remain valid for verification
	// for the session lifetime so in-flight artifacts keep working across a
	// rotation. See lib/security/secretrotator.go.
	interval := time.Duration(retrievedSettings.Cookie.KeyRotationInterval) * time.Millisecond
	if interval <= 0 {
		interval = 24 * time.Hour
	}
	lifetime := time.Duration(retrievedSettings.Cookie.SessionLifetime) * time.Millisecond
	if lifetime <= 0 {
		lifetime = interval
	}
	a.rotator = security.NewSecretRotator(persistence, "oidc_global_secret", interval, lifetime, nil, nil)
	a.rotator.OnRotate(a.rebuildProvider)
	if err := a.rotator.Start(); err != nil {
		log.Fatalf("Error starting oidc secret rotator: %v", err)
	}
	// Start triggers the first update which fires OnRotate -> rebuildProvider,
	// but guard against an empty provider just in case.
	if a.currentProvider() == nil {
		a.rebuildProvider()
	}
	return a
}

// rebuildProvider composes a fresh OAuth2 provider using the rotator's current
// secrets. A brand-new fosite.Config is built each time (never mutated in
// place) so that requests holding an older provider keep reading a consistent
// secret. Invoked on startup and on every rotation.
func (a *Authenticator) rebuildProvider() {
	secrets := a.rotator.Secrets()
	var global []byte
	var rotated [][]byte
	if len(secrets) > 0 {
		global = secrets[0]
		rotated = secrets[1:]
	}
	cfg := &fosite.Config{
		AccessTokenLifespan:  time.Minute * 30,
		GlobalSecret:         global,
		RotatedGlobalSecrets: rotated,
	}
	prov := compose.ComposeAllEnabled(cfg, a.store, a.privateKey)
	a.mu.Lock()
	a.provider = prov
	a.mu.Unlock()
}

// currentProvider returns the active OAuth2 provider. Capture it once per
// request so a concurrent rotation cannot swap the provider mid-handler.
func (a *Authenticator) currentProvider() fosite.OAuth2Provider {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.provider
}

// Stop halts the background secret rotation. Call during server shutdown.
func (a *Authenticator) Stop() {
	if a.rotator != nil {
		a.rotator.Stop()
	}
}

func (a *Authenticator) ValidateAdminToken(tokenString string, adminClient *settings.SSOClient) (bool, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return &a.privateKey.PublicKey, nil
	})
	if err != nil {
		return false, fmt.Errorf("token validation failed: %w", err)
	}

	claims := token.Claims

	if exp, ok := claims["exp"].(float64); ok {
		if time.Unix(int64(exp), 0).Before(time.Now()) {
			return false, fmt.Errorf("token expired")
		}
	}

	if aud, ok := claims["aud"].([]interface{}); ok {
		validAudience := false
		for _, a := range aud {
			if audStr, ok := a.(string); ok && audStr == adminClient.ClientId {
				validAudience = true
				break
			}
		}
		if !validAudience {
			return false, fmt.Errorf("invalid audience")
		}
	} else {
		return false, fmt.Errorf("missing aud claim")
	}

	if iss, ok := claims["iss"].(string); !ok || iss != a.retrievedSettings.SSO.Issuer {
		return false, fmt.Errorf("invalid issuer")
	}

	if ext, ok := claims["admin"]; ok {
		if isAdmin, ok := ext.(bool); ok && isAdmin {
			return true, nil
		}
	}

	return false, fmt.Errorf("missing admin role")
}

func (a *Authenticator) JwksEndpoint(rw http.ResponseWriter, req *http.Request) {
	rw.Header().Set("Content-Type", "application/json")

	nBytes := a.privateKey.PublicKey.N.Bytes()
	n := base64.RawURLEncoding.EncodeToString(nBytes)
	eBytes := big.NewInt(int64(a.privateKey.PublicKey.E)).Bytes()
	e := base64.RawURLEncoding.EncodeToString(eBytes)

	jwks := &oidc.JSONWebKeySet{
		Keys: []oidc.JSONWebKey{
			{
				E:   e,
				N:   n,
				Kty: "RSA",
				Kid: "my-key-id",
				Alg: "RS256",
				Use: "sig",
			},
		},
	}
	byteResponse, err := json.Marshal(jwks)
	if err != nil {
		log.Printf("Error occurred in marshalling JWKS response: %+v", err)
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	rw.Write(byteResponse)
}

func (a *Authenticator) IntrospectionEndpoint(rw http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	provider := a.currentProvider()
	mySessionData := a.newSession(nil, "")
	ir, err := provider.NewIntrospectionRequest(ctx, req, mySessionData)
	if err != nil {
		log.Printf("Error occurred in NewIntrospectionRequest: %+v", err)
		provider.WriteIntrospectionError(ctx, rw, err)
		return
	}
	provider.WriteIntrospectionResponse(ctx, rw, ir)
}

func (a *Authenticator) TokenEndpoint(rw http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	provider := a.currentProvider()
	clientId := req.Form.Get("client_id")

	mySessionData := a.newSession(nil, clientId)

	accessRequest, err := provider.NewAccessRequest(ctx, req, mySessionData)
	if err != nil {
		log.Printf("Error occurred in NewAccessRequest: %+v", err)
		provider.WriteAccessError(ctx, rw, accessRequest, err)
		return
	}

	if accessRequest.GetGrantTypes().ExactOne("client_credentials") {
		for _, scope := range accessRequest.GetRequestedScopes() {
			accessRequest.GrantScope(scope)
		}
	}

	response, err := provider.NewAccessResponse(ctx, accessRequest)
	if err != nil {
		log.Printf("Error occurred in NewAccessResponse: %+v", err)
		provider.WriteAccessError(ctx, rw, accessRequest, err)
		return
	}

	provider.WriteAccessResponse(ctx, rw, accessRequest, response)
}

func (a *Authenticator) RevokeEndpoint(rw http.ResponseWriter, req *http.Request) {
	// This context will be passed to all methods.
	ctx := req.Context()
	provider := a.currentProvider()

	// This will accept the token revocation request and validate various parameters.
	err := provider.NewRevocationRequest(ctx, req)

	// All done, send the response.
	provider.WriteRevocationResponse(ctx, rw, err)
}

func (a *Authenticator) OicWellKnown(rw http.ResponseWriter, req *http.Request, retrievedSettings *settings.Settings) {
	rw.Header().Set("Content-Type", "application/json")

	wellKnown := WellKnownResponse{
		Issuer:                            retrievedSettings.SSO.Issuer,
		AuthorizationEndpoint:             retrievedSettings.SSO.Issuer + "/oauth2/auth",
		TokenEndpoint:                     retrievedSettings.SSO.Issuer + "/oauth2/token",
		JwksURI:                           retrievedSettings.SSO.Issuer + "/.well-known/jwks.json",
		ResponseTypesSupported:            []string{"code", "token", "id_token", "code token", "code id_token", "token id_token", "code token id_token"},
		SubjectTypesSupported:             []string{"public"},
		IdTokenSigningAlgValuesSupported:  []string{"RS256"},
		ScopesSupported:                   []string{"openid", "offline", "photos", "email", "profile"},
		TokenEndpointAuthMethodsSupported: []string{"client_secret_basic", "client_secret_post"},
	}

	byteResponse, err := json.Marshal(wellKnown)
	if err != nil {
		log.Printf("Error occurred in marshalling well-known response: %+v", err)
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	rw.Write(byteResponse)
}

func collectAuthorizeParams(req *http.Request) url.Values {
	params := url.Values{}
	for key, values := range req.Form {
		for _, value := range values {
			params.Add(key, value)
		}
	}
	for key, values := range req.URL.Query() {
		if _, exists := params[key]; exists {
			continue
		}
		for _, value := range values {
			params.Add(key, value)
		}
	}
	return params
}

func hydrateAuthorizeRequestForm(req *http.Request, ar fosite.AuthorizeRequester) {
	if ar == nil {
		return
	}
	form := ar.GetRequestForm()
	if form == nil {
		return
	}
	for _, key := range []string{
		"client_id",
		"redirect_uri",
		"response_type",
		"scope",
		"state",
		"nonce",
		"code_challenge",
		"code_challenge_method",
	} {
		if value := req.FormValue(key); value != "" {
			form.Set(key, value)
		}
	}
}

func renderLoginPage(rw http.ResponseWriter, req *http.Request, clients []settings.SSOClient, ar fosite.AuthorizeRequester, errorMessage *string) {
	clientId := req.URL.Query().Get("client_id")
	if clientId == "" {
		clientId = req.FormValue("client_id")
	}
	var clientFound settings.SSOClient

	for _, sso := range clients {
		if sso.ClientId == clientId {
			clientFound = sso
		}
	}

	scopes := make([]string, 0)
	for _, scope := range ar.GetRequestedScopes() {
		scopes = append(scopes, scope)
	}
	loginComp := login.Login(clientFound, scopes, collectAuthorizeParams(req), req.Method == http.MethodPost, errorMessage)
	req.Header.Set("Content-Type", constants.ContentTypeHTML)
	loginComp.Render(req.Context(), rw)
}

func (a *Authenticator) AuthEndpoint(rw http.ResponseWriter, req *http.Request, setupLogger *zap.SugaredLogger, retrievedSettings *settings.Settings) {
	ctx := req.Context()
	provider := a.currentProvider()
	req.ParseForm()

	ar, err := provider.NewAuthorizeRequest(ctx, req)
	if err != nil {
		setupLogger.Error("Error occurred in NewAuthorizeRequest: ", err)
		provider.WriteAuthorizeError(ctx, rw, ar, err)
		return
	}
	hydrateAuthorizeRequestForm(req, ar)
	if req.Method == "GET" {
		renderLoginPage(rw, req, retrievedSettings.SSO.Clients, ar, nil)
		return
	}

	for _, scope := range req.PostForm["scopes"] {
		ar.GrantScope(scope)
	}

	username := req.PostFormValue("username")
	password := req.PostFormValue("password")
	clientId := ar.GetClient().GetID()

	user, ok := a.store.Users[username]
	if !ok || user.Password != password {
		time.Sleep(500 * time.Millisecond)
		rw.WriteHeader(http.StatusOK)
		usernameOrPasswordInvalid := "Username or password invalid"
		renderLoginPage(rw, req, retrievedSettings.SSO.Clients, ar, &usernameOrPasswordInvalid)
		return
	}

	mySessionData := a.newSession(&user, clientId)
	response, err := provider.NewAuthorizeResponse(ctx, ar, mySessionData)
	if err != nil {
		log.Printf("Error occurred in NewAuthorizeResponse: %+v", err)
		provider.WriteAuthorizeError(ctx, rw, ar, err)
		return
	}
	provider.WriteAuthorizeResponse(ctx, rw, ar, response)
}

func (a *Authenticator) newSession(user *MemoryUserRelation, clientId string) *openid.DefaultSession {
	if user == nil {
		user = &MemoryUserRelation{
			Username: "",
		}

	}
	extraClaims := make(map[string]interface{})
	extraClaims["admin"] = user.Admin

	return &openid.DefaultSession{
		Claims: &jwt.IDTokenClaims{
			Issuer:      a.retrievedSettings.SSO.Issuer,
			Subject:     user.Username,
			Audience:    []string{clientId},
			ExpiresAt:   time.Now().Add(time.Hour * 6),
			IssuedAt:    time.Now(),
			RequestedAt: time.Now(),
			AuthTime:    time.Now(),
			Extra:       extraClaims,
		},
		Headers: &jwt.Headers{
			Extra: make(map[string]interface{}),
		},
	}
}

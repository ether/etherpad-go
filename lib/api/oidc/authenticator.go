package oidc

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"log"
	"math/big"
	"net/http"
	"slices"
	"time"

	"github.com/ether/etherpad-go/assets/login"
	"github.com/ether/etherpad-go/lib/models/oidc"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/ory/fosite"
	"github.com/ory/fosite/compose"
	"github.com/ory/fosite/handler/openid"
	"github.com/ory/fosite/token/jwt"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type Authenticator struct {
	provider          fosite.OAuth2Provider
	store             *MemoryStore
	privateKey        *rsa.PrivateKey
	retrievedSettings *settings.Settings
}

func NewAuthenticator(retrievedSettings *settings.Settings) *Authenticator {
	store := NewMemoryStore()
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

	secret := []byte("some-cool-secret-that-is-32bytes")
	config := &fosite.Config{
		AccessTokenLifespan: time.Minute * 30,
		GlobalSecret:        secret,
	}
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)

	var oauth2 = compose.ComposeAllEnabled(config, store, privateKey)

	return &Authenticator{
		provider:          oauth2,
		store:             store,
		privateKey:        privateKey,
		retrievedSettings: retrievedSettings,
	}
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
	mySessionData := a.newSession(nil, "")
	ir, err := a.provider.NewIntrospectionRequest(ctx, req, mySessionData)
	if err != nil {
		log.Printf("Error occurred in NewIntrospectionRequest: %+v", err)
		a.provider.WriteIntrospectionError(ctx, rw, err)
		return
	}
	a.provider.WriteIntrospectionResponse(ctx, rw, ir)
}

func (a *Authenticator) TokenEndpoint(rw http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	clientId := req.Form.Get("client_id")

	mySessionData := a.newSession(nil, clientId)

	accessRequest, err := a.provider.NewAccessRequest(ctx, req, mySessionData)
	if err != nil {
		log.Printf("Error occurred in NewAccessRequest: %+v", err)
		a.provider.WriteAccessError(ctx, rw, accessRequest, err)
		return
	}

	if accessRequest.GetGrantTypes().ExactOne("client_credentials") {
		for _, scope := range accessRequest.GetRequestedScopes() {
			accessRequest.GrantScope(scope)
		}
	}

	response, err := a.provider.NewAccessResponse(ctx, accessRequest)
	if err != nil {
		log.Printf("Error occurred in NewAccessResponse: %+v", err)
		a.provider.WriteAccessError(ctx, rw, accessRequest, err)
		return
	}

	a.provider.WriteAccessResponse(ctx, rw, accessRequest, response)
}

func (a *Authenticator) RevokeEndpoint(rw http.ResponseWriter, req *http.Request) {
	// This context will be passed to all methods.
	ctx := req.Context()

	// This will accept the token revocation request and validate various parameters.
	err := a.provider.NewRevocationRequest(ctx, req)

	// All done, send the response.
	a.provider.WriteRevocationResponse(ctx, rw, err)
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

func (a *Authenticator) AuthEndpoint(rw http.ResponseWriter, req *http.Request, setupLogger *zap.SugaredLogger, retrievedSettings *settings.Settings) {
	ctx := req.Context()

	ar, err := a.provider.NewAuthorizeRequest(ctx, req)
	if err != nil {
		setupLogger.Error("Error occurred in NewAuthorizeRequest: ", err)
		a.provider.WriteAuthorizeError(ctx, rw, ar, err)
		return
	}

	req.ParseForm()
	if req.Method == "GET" {
		clientId := req.URL.Query().Get("client_id")
		var clientFound settings.SSOClient

		for _, sso := range retrievedSettings.SSO.Clients {
			if sso.ClientId == clientId {
				clientFound = sso
			}
		}

		rw.Header().Set("Content-Type", "text/html; charset=utf-8")
		scopes := make([]string, 0)
		for _, scope := range ar.GetRequestedScopes() {
			scopes = append(scopes, scope)
		}
		loginComp := login.Login(clientFound, scopes, nil)
		req.Header.Set("Content-Type", "text/html; charset=utf-8")
		loginComp.Render(req.Context(), rw)
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
		var clientFound settings.SSOClient

		for _, sso := range retrievedSettings.SSO.Clients {
			if sso.ClientId == clientId {
				clientFound = sso
			}
		}

		rw.Header().Set("Content-Type", "text/html; charset=utf-8")
		usernameOrPasswordInvalid := "Username or password invalid"
		scopes := make([]string, 0)
		for _, scope := range ar.GetRequestedScopes() {
			scopes = append(scopes, scope)
		}
		loginComp := login.Login(clientFound, scopes, &usernameOrPasswordInvalid)
		loginComp.Render(req.Context(), rw)
		return
	}

	mySessionData := a.newSession(&user, clientId)
	response, err := a.provider.NewAuthorizeResponse(ctx, ar, mySessionData)
	if err != nil {
		log.Printf("Error occurred in NewAuthorizeResponse: %+v", err)
		a.provider.WriteAuthorizeError(ctx, rw, ar, err)
		return
	}
	a.provider.WriteAuthorizeResponse(ctx, rw, ar, response)
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

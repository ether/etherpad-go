package oidc

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/ether/etherpad-go/lib/settings"
	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/ory/fosite"
	"github.com/ory/fosite/compose"
	"github.com/ory/fosite/handler/openid"
	"github.com/ory/fosite/storage"
	"github.com/ory/fosite/token/jwt"
)

func Init(app *fiber.App, settings settings.Settings) {

	store := storage.NewMemoryStore()
	secret := []byte("some-cool-secret-that-is-32bytes")
	config := &fosite.Config{
		AccessTokenLifespan: time.Minute * 30,
		GlobalSecret:        secret,
	}
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)

	var oauth2 = compose.ComposeAllEnabled(config, store, privateKey)

	app.Post("/oauth2/introspect", adaptor.HTTPHandler(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		introspectionEndpoint(writer, request, oauth2)
	})))
	app.Post("/oauth2/revoke", adaptor.HTTPHandler(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		revokeEndpoint(writer, request, oauth2)
	})))
	app.Post("/oauth2/token", adaptor.HTTPHandler(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		tokenEndpoint(writer, request, oauth2)
	})))
	app.Get("/oauth2/auth", adaptor.HTTPHandler(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		authEndpoint(writer, request, oauth2)
	})))
	app.Post("/oauth2/auth", adaptor.HTTPHandler(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		authEndpoint(writer, request, oauth2)
	})))

	app.Get("/.well-known/openid-configuration", adaptor.HTTPHandler(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		oicWellKnown(writer, request, oauth2, &settings)
	})))
}

func introspectionEndpoint(rw http.ResponseWriter, req *http.Request, oauth2 fosite.OAuth2Provider) {
	ctx := req.Context()
	mySessionData := newSession("")
	ir, err := oauth2.NewIntrospectionRequest(ctx, req, mySessionData)
	if err != nil {
		log.Printf("Error occurred in NewIntrospectionRequest: %+v", err)
		oauth2.WriteIntrospectionError(ctx, rw, err)
		return
	}
	oauth2.WriteIntrospectionResponse(ctx, rw, ir)
}

type WellKnownResponse struct {
	Issuer                            string   `json:"issuer"`
	AuthorizationEndpoint             string   `json:"authorization_endpoint"`
	TokenEndpoint                     string   `json:"token_endpoint"`
	JwksURI                           string   `json:"jwks_uri"`
	ResponseTypesSupported            []string `json:"response_types_supported"`
	SubjectTypesSupported             []string `json:"subject_types_supported"`
	IdTokenSigningAlgValuesSupported  []string `json:"id_token_signing_alg_values_supported"`
	ScopesSupported                   []string `json:"scopes_supported"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported"`
}

func oicWellKnown(rw http.ResponseWriter, req *http.Request, oauth2 fosite.OAuth2Provider, retrievedSettings *settings.Settings) {
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

func revokeEndpoint(rw http.ResponseWriter, req *http.Request, oauth2 fosite.OAuth2Provider) {
	// This context will be passed to all methods.
	ctx := req.Context()

	// This will accept the token revocation request and validate various parameters.
	err := oauth2.NewRevocationRequest(ctx, req)

	// All done, send the response.
	oauth2.WriteRevocationResponse(ctx, rw, err)
}

func tokenEndpoint(rw http.ResponseWriter, req *http.Request, oauth2 fosite.OAuth2Provider) {
	// This context will be passed to all methods.
	ctx := req.Context()

	// Create an empty session object which will be passed to the request handlers
	mySessionData := newSession("")

	// This will create an access request object and iterate through the registered TokenEndpointHandlers to validate the request.
	accessRequest, err := oauth2.NewAccessRequest(ctx, req, mySessionData)

	// Catch any errors, e.g.:
	// * unknown client
	// * invalid redirect
	// * ...
	if err != nil {
		log.Printf("Error occurred in NewAccessRequest: %+v", err)
		oauth2.WriteAccessError(ctx, rw, accessRequest, err)
		return
	}

	// If this is a client_credentials grant, grant all requested scopes
	// NewAccessRequest validated that all requested scopes the client is allowed to perform
	// based on configured scope matching strategy.
	if accessRequest.GetGrantTypes().ExactOne("client_credentials") {
		for _, scope := range accessRequest.GetRequestedScopes() {
			accessRequest.GrantScope(scope)
		}
	}

	// Next we create a response for the access request. Again, we iterate through the TokenEndpointHandlers
	// and aggregate the result in response.
	response, err := oauth2.NewAccessResponse(ctx, accessRequest)
	if err != nil {
		log.Printf("Error occurred in NewAccessResponse: %+v", err)
		oauth2.WriteAccessError(ctx, rw, accessRequest, err)
		return
	}

	// All done, send the response.
	oauth2.WriteAccessResponse(ctx, rw, accessRequest, response)

	// The client now has a valid access token
}

func newSession(user string) *openid.DefaultSession {
	return &openid.DefaultSession{
		Claims: &jwt.IDTokenClaims{
			Issuer:      "https://fosite.my-application.com",
			Subject:     user,
			Audience:    []string{"https://my-client.my-application.com"},
			ExpiresAt:   time.Now().Add(time.Hour * 6),
			IssuedAt:    time.Now(),
			RequestedAt: time.Now(),
			AuthTime:    time.Now(),
		},
		Headers: &jwt.Headers{
			Extra: make(map[string]interface{}),
		},
	}
}

func authEndpoint(rw http.ResponseWriter, req *http.Request, oauth2 fosite.OAuth2Provider) {
	ctx := req.Context()

	ar, err := oauth2.NewAuthorizeRequest(ctx, req)
	if err != nil {
		log.Printf("Error occurred in NewAuthorizeRequest: %+v", err)
		oauth2.WriteAuthorizeError(ctx, rw, ar, err)
		return
	}

	var requestedScopes string
	for _, this := range ar.GetRequestedScopes() {
		requestedScopes += fmt.Sprintf(`<li><input type="checkbox" name="scopes" value="%s">%s</li>`, this, this)
	}

	req.ParseForm()
	if req.PostForm.Get("username") != "peter" {
		rw.Header().Set("Content-Type", "text/html; charset=utf-8")
		rw.Write([]byte(`<h1>Login page</h1>`))
		rw.Write([]byte(fmt.Sprintf(`
			<p>Howdy! This is the log in page. For this example, it is enough to supply the username.</p>
			<form method="post">
				<p>
					By logging in, you consent to grant these scopes:
					<ul>%s</ul>
				</p>
				<input type="text" name="username" /> <small>try peter</small><br>
				<input type="submit">
			</form>
		`, requestedScopes)))
		return
	}

	for _, scope := range req.PostForm["scopes"] {
		ar.GrantScope(scope)
	}

	mySessionData := newSession("peter")
	response, err := oauth2.NewAuthorizeResponse(ctx, ar, mySessionData)
	if err != nil {
		log.Printf("Error occurred in NewAuthorizeResponse: %+v", err)
		oauth2.WriteAuthorizeError(ctx, rw, ar, err)
		return
	}

	// Last but not least, send the response!
	oauth2.WriteAuthorizeResponse(ctx, rw, ar, response)
}

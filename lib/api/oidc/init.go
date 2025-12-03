package oidc

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"

	"github.com/ether/etherpad-go/lib/settings"
	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"go.uber.org/zap"
)

func GenerateAuthCodeURL(issuer, clientID, redirectURI string, scopes []string) (string, string, string, error) {
	state, err := randBase64URL(32)
	if err != nil {
		return "", "", "", err
	}
	nonce, err := randBase64URL(32)
	if err != nil {
		return "", "", "", err
	}
	codeVerifier, err := randBase64URL(64) // 43-128 bytes empfohlen
	if err != nil {
		return "", "", "", err
	}
	codeChallenge := pkceS256(codeVerifier)

	u, err := url.Parse(strings.TrimRight(issuer, "/") + "/oauth2/auth")
	if err != nil {
		return "", "", "", err
	}
	q := u.Query()
	q.Set("response_type", "code")
	q.Set("client_id", clientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("scope", strings.Join(scopes, " "))
	q.Set("state", state)
	q.Set("nonce", nonce)
	q.Set("code_challenge", codeChallenge)
	q.Set("code_challenge_method", "S256")
	u.RawQuery = q.Encode()

	return u.String(), codeVerifier, state, nil
}

func randBase64URL(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func pkceS256(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func Init(app *fiber.App, retrievedSettings *settings.Settings, setupLogger *zap.SugaredLogger) {
	authenticator := NewAuthenticator(retrievedSettings)
	allowedUrls := make([]string, 0)
	for _, sso := range retrievedSettings.SSO.Clients {
		for _, redirectUri := range sso.RedirectUris {
			u, err := url.Parse(redirectUri)
			if err != nil {
				setupLogger.Errorf("Invalid redirect URI in SSO client %s: %s", sso.ClientId, redirectUri)
				continue
			}
			allowedUrls = append(allowedUrls, u.Scheme+"://"+u.Host)
		}
	}
	app.Use(cors.New(cors.Config{
		AllowOriginsFunc: func(origin string) bool {
			for _, allowed := range allowedUrls {
				if origin == allowed {
					return true
				}
			}
			return false
		},
	}))

	app.Post("/oauth2/introspect", adaptor.HTTPHandler(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		authenticator.IntrospectionEndpoint(writer, request)
	})))
	app.Post("/oauth2/revoke", adaptor.HTTPHandler(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		authenticator.RevokeEndpoint(writer, request)
	})))
	app.Post("/oauth2/token", adaptor.HTTPHandler(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		authenticator.TokenEndpoint(writer, request)
	})))

	app.Get("/oauth2/auth", adaptor.HTTPHandler(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		authenticator.AuthEndpoint(writer, request, setupLogger, retrievedSettings)
	})))
	app.Post("/oauth2/auth", adaptor.HTTPHandler(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		authenticator.AuthEndpoint(writer, request, setupLogger, retrievedSettings)
	})))

	app.Get("/.well-known/openid-configuration", adaptor.HTTPHandler(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		authenticator.OicWellKnown(writer, request, retrievedSettings)
	})))
	app.Get("/.well-known/jwks.json", adaptor.HTTPHandler(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		authenticator.JwksEndpoint(writer, request)
	})))
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

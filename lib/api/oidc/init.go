package oidc

import (
	"net/http"
	"net/url"

	"github.com/ether/etherpad-go/lib"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/gofiber/fiber/v3/middleware/adaptor"
	"github.com/gofiber/fiber/v3/middleware/cors"
)

// Init sets up the OpenID Connect provider and mounts its endpoints.
func Init(store *lib.InitStore) *Authenticator {
	authenticator := NewAuthenticator(store.RetrievedSettings, store.Store)

	allowedUrls := allowedOrigins(store.RetrievedSettings.SSO, func(clientID, uri string) {
		store.Logger.Errorf("Invalid redirect URI in SSO client %s: %s", clientID, uri)
	})
	store.C.Use(cors.New(cors.Config{
		AllowOriginsFunc: func(origin string) bool { return originAllowed(origin, allowedUrls) },
	}))

	provider := adaptor.HTTPHandler(authenticator.Handler())
	for _, path := range authenticator.Paths() {
		store.C.All(path, provider)
	}
	store.C.All(loginPath, adaptor.HTTPHandler(http.HandlerFunc(authenticator.LoginHandler)))
	return authenticator
}

// originAllowed reports whether a CORS request from origin may read the
// provider's answers: only the exact scheme://host[:port] of a registered
// redirect URI qualifies (no suffix or substring matches, no scheme change).
func originAllowed(origin string, allowed []string) bool {
	if origin == "" {
		return false
	}
	for _, a := range allowed {
		if origin == a {
			return true
		}
	}
	return false
}

// allowedOrigins are the origins (scheme://host[:port]) of the registered
// redirect URIs of all SSO clients.
func allowedOrigins(sso *settings.SSO, invalid func(clientID, uri string)) []string {
	out := make([]string, 0)
	if sso == nil {
		return out
	}
	for _, client := range sso.Clients {
		for _, redirectURI := range client.RedirectUris {
			u, err := url.Parse(redirectURI)
			if err != nil || u.Scheme == "" || u.Host == "" {
				invalid(client.ClientId, redirectURI)
				continue
			}
			out = append(out, u.Scheme+"://"+u.Host)
		}
	}
	return out
}

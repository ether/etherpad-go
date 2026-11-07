package oidc

import (
	"net/http"
	"net/url"

	"github.com/ether/etherpad-go/lib"
	"github.com/gofiber/fiber/v3/middleware/adaptor"
	"github.com/gofiber/fiber/v3/middleware/cors"
)

// Init sets up the OpenID Connect provider and mounts its endpoints.
func Init(store *lib.InitStore) *Authenticator {
	authenticator := NewAuthenticator(store.RetrievedSettings, store.Store)

	allowedUrls := make([]string, 0)
	if store.RetrievedSettings.SSO != nil {
		for _, sso := range store.RetrievedSettings.SSO.Clients {
			for _, redirectUri := range sso.RedirectUris {
				u, err := url.Parse(redirectUri)
				if err != nil {
					store.Logger.Errorf("Invalid redirect URI in SSO client %s: %s", sso.ClientId, redirectUri)
					continue
				}
				allowedUrls = append(allowedUrls, u.Scheme+"://"+u.Host)
			}
		}
	}
	store.C.Use(cors.New(cors.Config{
		AllowOriginsFunc: func(origin string) bool {
			for _, allowed := range allowedUrls {
				if origin == allowed {
					return true
				}
			}
			return false
		},
	}))

	provider := adaptor.HTTPHandler(authenticator.Handler())
	for _, path := range authenticator.Paths() {
		store.C.All(path, provider)
	}
	store.C.All(loginPath, adaptor.HTTPHandler(http.HandlerFunc(authenticator.LoginHandler)))
	return authenticator
}

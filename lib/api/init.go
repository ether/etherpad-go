package api

import (
	"net/http"
	"strings"

	"github.com/ether/etherpad-go/lib"
	"github.com/ether/etherpad-go/lib/api/author"
	"github.com/ether/etherpad-go/lib/api/groups"
	"github.com/ether/etherpad-go/lib/api/io"
	"github.com/ether/etherpad-go/lib/api/legacy"
	"github.com/ether/etherpad-go/lib/api/oidc"
	"github.com/ether/etherpad-go/lib/api/pad"
	"github.com/ether/etherpad-go/lib/api/session"
	"github.com/ether/etherpad-go/lib/api/sheetio"
	"github.com/ether/etherpad-go/lib/api/static"
	"github.com/ether/etherpad-go/lib/api/stats"
	swagger2 "github.com/ether/etherpad-go/lib/api/swagger"
	"github.com/ether/etherpad-go/lib/locales"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/gofiber/fiber/v3"
)

func InitAPI(store *lib.InitStore) *oidc.Authenticator {
	ssoAdminClient := store.RetrievedSettings.SSO.GetAdminClient()
	if ssoAdminClient == nil {
		store.Logger.Warnf("SSO admin client is not configured, cannot start admin API")
	}
	authenticator := oidc.Init(store)
	store.PrivateAPI.Use(func(c fiber.Ctx) error {
		if ssoAdminClient == nil {
			store.Logger.Warnf("SSO admin client is not configured, cannot validate admin token")
			return c.Status(http.StatusUnauthorized).Send([]byte("No Authorization header provided"))
		}
		authorizationValue := c.Get("Authorization", "")
		if authorizationValue == "" {
			store.Logger.Warn("No Authorization header provided for admin API")
			return c.Status(http.StatusUnauthorized).Send([]byte("No Authorization header provided"))
		}
		bearerToken := strings.Split(authorizationValue, " ")
		if len(bearerToken) != 2 || bearerToken[0] != "Bearer" {
			store.Logger.Warn("Invalid Authorization header format for admin API")
			return c.Status(http.StatusUnauthorized).Send([]byte("No Authorization header provided"))
		}

		ok, err := authenticator.ValidateAdminToken(bearerToken[1], ssoAdminClient)
		if err != nil || !ok {
			reason := "not an admin token"
			if err != nil {
				reason = err.Error()
			}
			store.Logger.Warn("Invalid token provided for admin API: " + reason)
			return c.Status(http.StatusUnauthorized).Send([]byte("No Authorization header provided"))
		}
		return c.Next()
	})

	swagger2.Init(store.C)
	locales.Init(store)
	author.Init(store)
	pad.Init(store)
	groups.Init(store)
	session.Init(store)
	static.Init(store)
	io.Init(store)
	sheetio.Init(store)
	stats.Init(store)
	initLegacyAPI(store, authenticator, ssoAdminClient)
	return authenticator
}

// initLegacyAPI serves etherpad-lite's HTTP API under /api for existing
// clients (Moodle and others). With authenticationMethod "apikey" it takes
// the key from APIKEY.txt; otherwise the admin bearer token.
func initLegacyAPI(store *lib.InitStore, authenticator *oidc.Authenticator, adminClient *settings.SSOClient) {
	var apiKey string
	if store.RetrievedSettings.AuthenticationMethod == "apikey" {
		path := legacy.APIKeyFile(store.RetrievedSettings.Root)
		key, err := legacy.LoadAPIKey(path)
		if err != nil {
			store.Logger.Errorf("legacy API disabled: cannot read or create the API key file %s: %v", path, err)
			return
		}
		if key == "" {
			store.Logger.Errorf("legacy API disabled: the API key file %s is empty", path)
			return
		}
		apiKey = key
		store.Logger.Infof("HTTP API (/api) uses the API key from %s", path)
	}
	// An admin's token, as for /admin/api, or a client_credentials token
	// of a configured client (etherpad-lite accepts both).
	bearer := func(token string) bool {
		if adminClient != nil {
			if ok, err := authenticator.ValidateAdminToken(token, adminClient); err == nil && ok {
				return true
			}
		}
		return authenticator.ValidateServiceToken(token)
	}
	legacy.New(store, apiKey, bearer).Register(store.C)
}

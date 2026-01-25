package api

import (
	"net/http"
	"strings"

	"github.com/ether/etherpad-go/lib"
	"github.com/ether/etherpad-go/lib/api/author"
	"github.com/ether/etherpad-go/lib/api/groups"
	"github.com/ether/etherpad-go/lib/api/io"
	"github.com/ether/etherpad-go/lib/api/oidc"
	"github.com/ether/etherpad-go/lib/api/pad"
	"github.com/ether/etherpad-go/lib/api/static"
	"github.com/ether/etherpad-go/lib/locales"
	"github.com/gofiber/fiber/v2"
)

func InitAPI(store *lib.InitStore) *oidc.Authenticator {
	ssoAdminClient := store.RetrievedSettings.SSO.GetAdminClient()
	if ssoAdminClient == nil {
		store.Logger.Fatal("SSO admin client is not configured, cannot start admin API")
	}
	authenticator := oidc.Init(store)
	store.PrivateAPI.Use(func(c *fiber.Ctx) error {
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

		if ssoAdminClient == nil {
			store.Logger.Fatal("SSO admin client is not configured, cannot validate admin token")
			return c.Status(http.StatusUnauthorized).Send([]byte("No Authorization header provided"))
		}

		ok, err := authenticator.ValidateAdminToken(bearerToken[1], ssoAdminClient)
		if err != nil || !ok {
			store.Logger.Warn("Invalid token provided for admin API: " + err.Error())
			return c.Status(http.StatusUnauthorized).Send([]byte("No Authorization header provided"))
		}
		return c.Next()
	})

	locales.Init(store)
	author.Init(store)
	pad.Init(store)
	groups.Init(store)
	static.Init(store)
	io.Init(store)
	return authenticator
}

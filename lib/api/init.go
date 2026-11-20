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
	"github.com/ether/etherpad-go/lib/accounts"
	"github.com/ether/etherpad-go/lib/tokens"
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
	keys := tokens.For(store.Store, store.RetrievedSettings)
	store.PrivateAPI.Use(func(c fiber.Ctx) error {
		if handled, err := AdminAPIToken(c, keys); handled {
			return err
		}
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

// AdminAPIToken lets a token use the admin API (/admin/api) with
// admin:pads for all pads; the API with a check per function and pad is
// /api. It reports whether the request carried a token (then it has
// answered or passed it on); other bearer tokens are the provider's.
func AdminAPIToken(c fiber.Ctx, keys *tokens.Manager) (bool, error) {
	bearer, ok := strings.CutPrefix(c.Get(fiber.HeaderAuthorization), "Bearer ")
	bearer = strings.TrimSpace(bearer)
	if !ok || !strings.HasPrefix(bearer, tokens.KeyPrefix) {
		return false, nil
	}
	use, err := keys.Authenticate(bearer, c.IP())
	if err != nil {
		return true, c.Status(http.StatusUnauthorized).SendString("Invalid token")
	}
	if !use.Grants.HasGlobal(accounts.ScopeAdminPads) {
		return true, c.Status(http.StatusForbidden).SendString("This token needs admin:pads for all pads")
	}
	return true, c.Next()
}

// initLegacyAPI serves etherpad-lite's HTTP API under /api for existing
// clients (Moodle and others). Calls need a token (admin UI: Tokens) that
// may do what they do, or the admin bearer token. The API keys of before
// the tokens and a key in APIKEY.txt (etherpad-lite and earlier releases)
// are taken over as tokens once.
func initLegacyAPI(store *lib.InitStore, authenticator *oidc.Authenticator, adminClient *settings.SSOClient) {
	keys := tokens.For(store.Store, store.RetrievedSettings)
	// The keys of before the tokens become tokens of the service account
	// legacy-api, under the same hash: they keep working.
	if n, err := keys.TakeOverAPIKeys(store.Store); err != nil {
		store.Logger.Errorf("cannot take over the API keys as tokens: %v", err)
	} else if n > 0 {
		store.Logger.Infof("%d API keys are now tokens of the service account %q (admin UI: Tokens); they keep working", n, tokens.LegacyAccount)
	}
	path := legacy.APIKeyFile(store.RetrievedSettings.Root)
	if stored, err := legacy.ImportAPIKeyFile(keys, path); err != nil {
		store.Logger.Errorf("cannot take over the API key of %s: %v", path, err)
	} else if stored {
		store.Logger.Infof("The API key of %s is now a token named \"APIKEY.txt\" (admin UI: Tokens); the file is no longer needed", path)
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
	legacy.New(store, keys, bearer).Register(store.C)
}

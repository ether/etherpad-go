package pad

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ether/etherpad-go/lib/models/clientVars"
	"github.com/ether/etherpad-go/lib/models/webaccess"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

func UserCanModify(padId *string, req *webaccess.SocketClientRequest, readOnlyManager ReadOnlyManager) bool {
	if readOnlyManager.IsReadOnlyID(padId) {
		return false
	}

	if !settings.Displayed.RequireAuthentication {
		return true
	}

	if req == nil || *req.ReadOnly {
		return false
	}

	if req.PadAuthorizations == nil {
		panic("This should not happen")
	}

	var padMap = *req.PadAuthorizations
	var padLevel = padMap[*padId]
	var level, _ = NormalizeAuthzLevel(padLevel)

	return level != nil && *level != "readOnly"
}

func CheckAccess(ctx *fiber.Ctx, logger *zap.SugaredLogger, retrievedSettings *settings.Settings, readOnlyManager *ReadOnlyManager) error {
	var requireAdmin = strings.HasPrefix(strings.ToLower(ctx.Path()), "/admin-auth")
	//FIXME this needs to be set
	// ///////////////////////////////////////////////////////////////////////////////////////////////
	// Step 1: Check the preAuthorize hook for early permit/deny (permit is only allowed for non-admin
	// pages). If any plugin explicitly grants or denies access, skip the remaining steps. Plugins can
	// use the preAuthzFailure hook to override the default 403 error.
	// ///////////////////////////////////////////////////////////////////////////////////////////////

	// This helper is used in steps 2 and 4 below, so it may be called twice per access: once before
	// authentication is checked and once after (if settings.requireAuthorization is true).

	authorize := func() bool {
		grant := func(level string) bool {
			var user, ok = ctx.Locals(clientVars.WebAccessStore).(*webaccess.SocketClientRequest)

			if !ok {
				user = nil
			}

			var detectedLevel, err = NormalizeAuthzLevel(level)

			if err != nil {
				return false
			}

			if user == nil {
				return true // This will happen if authentication is not required.
			}

			var encodedPadRegex = regexp.MustCompile("^/p/([^/]*)")

			var encodedPadIds = encodedPadRegex.FindAllString(ctx.Path(), -1)

			if len(encodedPadIds) == 0 {
				return true
			}

			encodedPadId := encodedPadIds[0]

			if utf8.RuneCountInString(encodedPadId) == 0 {
				return true
			}

			var padId, queryErr = url.QueryUnescape(encodedPadId)

			if queryErr != nil {
				return false
			}

			if readOnlyManager.IsReadOnlyID(&padId) {
				// pad is read-only, first get the real pad ID
				var realPadId, err = readOnlyManager.GetPadId(padId)
				if err != nil {
					println("Error getting real pad ID:", err.Error())
					return false
				}
				if realPadId == nil {
					return false
				}
			}

			if user.PadAuthorizations == nil {
				var newMap = make(map[string]string)
				user.PadAuthorizations = &newMap
			}
			var padAuthorizations = *user.PadAuthorizations
			padAuthorizations[padId] = *detectedLevel
			return true
		}

		var sessionReq, okay = ctx.Locals(clientVars.WebAccessStore).(*webaccess.SocketClientRequest)

		if !okay {
			sessionReq = nil
		}

		var isAuthenticated = sessionReq != nil

		if isAuthenticated && sessionReq.IsAdmin {
			return grant("create")
		}
		var requireAuthn = requireAdmin || retrievedSettings.RequireAuthentication
		if !requireAuthn {
			return grant("create")
		}

		if !isAuthenticated {
			return false
		}

		if requireAdmin && !sessionReq.IsAdmin {
			return false
		}

		if !retrievedSettings.RequireAuthorization {
			return grant("create")
		}
		return false
	}

	// ///////////////////////////////////////////////////////////////////////////////////////////////
	// Step 2: Try to just access the thing. If access fails (perhaps authentication has not yet
	// completed, or maybe different credentials are required), go to the next step.
	// ///////////////////////////////////////////////////////////////////////////////////////////////

	if authorize() {
		if requireAdmin {
			return ctx.Status(200).SendString("Authorized")
		}
		return ctx.Next()
	}

	if retrievedSettings.Users == nil {
		var newUsers = make(map[string]settings.User)
		retrievedSettings.Users = newUsers
	}

	var user, ok = ctx.Locals(clientVars.WebAccessStore).(*webaccess.SocketClientRequest)

	var webAccessCtx = webaccess.WebAccessType{
		Users: settings.Displayed.Users,
		Next:  ctx.Next,
	}

	if ok {
		webAccessCtx.Username = user.Username
	}

	var authheader = ctx.Get("authorization")
	var httpBasicAuth = authheader != "" && strings.HasPrefix(authheader, "Basic ")

	if httpBasicAuth {
		var basicAuthColonPassword, err = base64.StdEncoding.DecodeString(strings.Split(authheader, " ")[1])

		if err != nil {
			return errors.New("invalid base 64")
		}

		var userNamePassword = strings.Split(string(basicAuthColonPassword), ":")
		webAccessCtx.Username = &userNamePassword[0]
		webAccessCtx.Password = &userNamePassword[1]
	}

	var foundUsers = retrievedSettings.Users

	var password *string

	if webAccessCtx.Username != nil {
		retrievedUser, ok := foundUsers[*webAccessCtx.Username]
		logger.Infof("Retrieved user: %s", *webAccessCtx.Username)
		if ok {
			password = retrievedUser.Password
		}
	}

	if !httpBasicAuth || webAccessCtx.Username == nil || password == nil || *password != *webAccessCtx.Password {
		logger.Infof("failed authentication from IP %s", ctx.IP())
		// No plugin handled the authentication failure. Fall back to basic authentication.
		if !requireAdmin {
			ctx.Set("WWW-Authenticate", `Basic realm="Restricted area"`)
			return ctx.SendStatus(fiber.StatusUnauthorized)
		}
		time.Sleep(1 * time.Second)
		return ctx.Status(401).SendString("Authentication Required")
	}
	var retrievedUserFromMap = retrievedSettings.Users[*webAccessCtx.Username]
	// Make a shallow copy so that the password property can be deleted (to prevent it from
	// appearing in logs or in the database) without breaking future authentication attempts.
	ctx.Locals(clientVars.WebAccessStore, &webaccess.SocketClientRequest{
		Username: retrievedUserFromMap.Username,
		IsAdmin:  retrievedUserFromMap.IsAdmin != nil && *retrievedUserFromMap.IsAdmin,
	})

	retrievedUserFromMap.Username = webAccessCtx.Username
	// Remove password
	if webAccessCtx.Username == nil {
		logger.Warn("authenticate hook failed to add user settings to session")
		return ctx.Status(500).SendString("Internal Server Status")
	}
	if webAccessCtx.Username == nil {
		var newUsername = "<no username>"
		webAccessCtx.Username = &newUsername
	}

	logger.Infof(fmt.Sprintf(`Successful authentication from IP %s for user %s`, ctx.IP(), *webAccessCtx.Username))
	// ///////////////////////////////////////////////////////////////////////////////////////////////
	// Step 4: Try to access the thing again. If this fails, give the user a 403 error. Plugins can
	// use the authzFailure hook to override the default error handling behavior (e.g., to redirect to
	// a login page).
	// ///////////////////////////////////////////////////////////////////////////////////////////////

	var auth = authorize()
	if auth && !requireAdmin {
		return ctx.Next()
	}

	if auth && requireAdmin {
		return ctx.Status(200).SendString("Authorized")
	}

	// No plugin handled the authorization failure.
	return ctx.Status(403).SendString("Forbidden")
}

func NormalizeAuthzLevel(level interface{}) (*string, error) {
	switch castedExpr := level.(type) {
	case string:
		{
			switch castedExpr {
			case "readOnly":
			case "modify":
			case "create":
				return &castedExpr, nil
			default:
				println("Invalid level")
				return nil, errors.New("unknown authorization level " + castedExpr)
			}
		}
	case bool:
		{
			if castedExpr {
				return nil, nil
			}
		}
	}
	return nil, errors.New("access denied")
}

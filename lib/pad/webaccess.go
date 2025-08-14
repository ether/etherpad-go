package pad

import (
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/ether/etherpad-go/lib/models/clientVars"
	"github.com/ether/etherpad-go/lib/models/webaccess"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/gofiber/fiber/v2"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var readOnlyManager = NewReadOnlyManager()

func UserCanModify(padId *string, req *webaccess.SocketClientRequest) bool {
	if readOnlyManager.isReadOnlyID(padId) {
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

func CheckAccess(ctx *fiber.Ctx) error {
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

			var encodedPadId = encodedPadRegex.FindAllString(ctx.Path(), -1)[1]

			if len(encodedPadId) == 0 {
				return true
			}

			var padId, queryErr = url.QueryUnescape(encodedPadId)

			if queryErr != nil {
				return false
			}

			if readOnlyManager.isReadOnlyID(&padId) {
				// pad is read-only, first get the real pad ID
				var realPadId = readOnlyManager.getPadId(padId)
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
		var requireAuthn = requireAdmin || settings.Displayed.RequireAuthentication
		if !requireAuthn {
			return grant("create")
		}

		if !isAuthenticated {
			return false
		}

		if requireAdmin && !sessionReq.IsAdmin {
			return false
		}

		if !settings.Displayed.RequireAuthorization {
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

	if settings.Displayed.Users == nil {
		var newUsers = make(map[string]settings.User)
		settings.Displayed.Users = newUsers
	}
	var user = ctx.Locals(clientVars.WebAccessStore).(*webaccess.SocketClientRequest)

	var webAccessCtx = webaccess.WebAccessType{
		User:  user,
		Users: settings.Displayed.Users,
		Next:  ctx.Next,
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

	var foundUsers = settings.Displayed.Users

	webUsername := *webAccessCtx.Username
	retrievedUser, ok := foundUsers[webUsername]
	var password *string

	if ok {
		password = retrievedUser.Password
	}

	if !httpBasicAuth || webAccessCtx.Username == nil || password == nil || *password != *webAccessCtx.Password {
		println("failed authentication")
		// No plugin handled the authentication failure. Fall back to basic authentication.
		if !requireAdmin {
			ctx.Set("WWW-Authenticate", `Basic realm="Restricted area"`)
			return ctx.SendStatus(fiber.StatusUnauthorized)
		}
		time.Sleep(1 * time.Second)
		return ctx.Status(401).SendString("Authentication Required")
	}
	var retrievedUserFromMap = settings.Displayed.Users[*webAccessCtx.Username]
	// Make a shallow copy so that the password property can be deleted (to prevent it from
	// appearing in logs or in the database) without breaking future authentication attempts.
	ctx.Locals(clientVars.WebAccessStore, retrievedUserFromMap)

	retrievedUserFromMap.Username = webAccessCtx.Username
	// Remove password
	if user == nil {
		println("authenticate hook failed to add user settings to session")
		return ctx.Status(50).SendString("Internal Server Status")
	}
	if user.Username == nil {
		var newUsername = "<no username>"
		user.Username = &newUsername
	}

	println(fmt.Sprintf(`Successful authentication from IP %s for user %s`, ctx.IP(), *user.Username))
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

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

	"github.com/ether/etherpad-go/lib/hooks"
	"github.com/ether/etherpad-go/lib/hooks/events"
	"github.com/ether/etherpad-go/lib/models/clientVars"
	"github.com/ether/etherpad-go/lib/models/webaccess"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/gofiber/fiber/v3"
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

	// The authentication layer normally populates PadAuthorizations even when
	// requireAuthorization is off (mirrors the original's assert) — but a
	// missing map must deny, not crash the request.
	if req.PadAuthorizations == nil {
		return false
	}

	var padMap = *req.PadAuthorizations
	var padLevel = padMap[*padId]
	var level, _ = NormalizeAuthzLevel(padLevel)

	return level != nil && *level != "readOnly"
}

// CheckAccess keeps the historical signature (no hook system) and runs without
// any plugin preAuthorize/preAuthzFailure hooks. New callers should prefer
// CheckAccessWithHooks so that plugins get a chance to permit or deny early.
func CheckAccess(ctx fiber.Ctx, logger *zap.SugaredLogger, retrievedSettings *settings.Settings, readOnlyManager *ReadOnlyManager) error {
	return CheckAccessWithHooks(ctx, logger, retrievedSettings, readOnlyManager, nil)
}

func CheckAccessWithHooks(ctx fiber.Ctx, logger *zap.SugaredLogger, retrievedSettings *settings.Settings, readOnlyManager *ReadOnlyManager, hookSystem *hooks.Hook) error {
	var requireAdmin = strings.HasPrefix(strings.ToLower(ctx.Path()), "/admin-auth")
	// ///////////////////////////////////////////////////////////////////////////////////////////////
	// Step 1: Check the preAuthorize hook for early permit/deny (permit is only allowed for
	// non-admin pages). If any plugin explicitly grants or denies access, skip the remaining steps.
	// Plugins can use the preAuthzFailure hook to override the default 403 error.
	// ///////////////////////////////////////////////////////////////////////////////////////////////

	if hookSystem != nil {
		preAuthorizeCtx := &events.PreAuthorizeContext{Path: ctx.Path(), RequireAdmin: requireAdmin}
		hookSystem.ExecutePreAuthorizeHooks(preAuthorizeCtx)
		switch preAuthorizeCtx.Decision() {
		case events.PreAuthorizePermit:
			return ctx.Next()
		case events.PreAuthorizeDeny:
			preAuthzFailureCtx := &events.PreAuthzFailureContext{Path: ctx.Path(), RequireAdmin: requireAdmin}
			hookSystem.ExecutePreAuthzFailureHooks(preAuthzFailureCtx)
			if preAuthzFailureCtx.Handled() {
				for key, value := range preAuthzFailureCtx.Headers() {
					ctx.Set(key, value)
				}
				return ctx.Status(preAuthzFailureCtx.Status()).SendString(preAuthzFailureCtx.Body())
			}
			// No plugin handled the pre-authentication authorization failure.
			return ctx.Status(403).SendString("Forbidden")
		case events.PreAuthorizeDefer:
			// No plugin answered; fall through to the regular authorize/authenticate steps below.
		}
	}

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

	// sendAuthnFailure fires the authnFailure hook then falls back to the
	// default 401 response. It REPLACES all inline 401 logic below.
	sendAuthnFailure := func() error {
		logger.Infof("failed authentication from IP %s", ctx.IP())
		if hookSystem != nil {
			failCtx := &events.AuthnFailureContext{Path: ctx.Path(), RequireAdmin: requireAdmin}
			hookSystem.ExecuteAuthnFailureHooks(failCtx)
			if failCtx.Handled() {
				for k, v := range failCtx.Headers() {
					ctx.Set(k, v)
				}
				return ctx.Status(failCtx.Status()).SendString(failCtx.Body())
			}
		}
		if !requireAdmin {
			ctx.Set("WWW-Authenticate", `Basic realm="Restricted area"`)
			return ctx.SendStatus(fiber.StatusUnauthorized)
		}
		time.Sleep(1 * time.Second)
		return ctx.Status(401).SendString("Authentication Required")
	}

	pluginAuthenticated := false
	if hookSystem != nil {
		var inUser, inPass string
		if webAccessCtx.Username != nil {
			inUser = *webAccessCtx.Username
		}
		if webAccessCtx.Password != nil {
			inPass = *webAccessCtx.Password
		}
		authCtx := &events.AuthenticateContext{
			InputUsername: inUser,
			InputPassword: inPass,
			Path:          ctx.Path(),
			RequireAdmin:  requireAdmin,
			GetHeader:     func(k string) string { return ctx.Get(k) },
		}
		hookSystem.ExecuteAuthenticateHooks(authCtx)
		if authCtx.Answered() {
			if authCtx.Rejected() {
				return sendAuthnFailure()
			}
			// Plugin authenticated the user. Populate the session user, looking up
			// settings.Users for admin status when the username is known there.
			username := authCtx.Username()
			var isAdmin bool
			if u, ok := retrievedSettings.Users[username]; ok {
				isAdmin = u.IsAdmin != nil && *u.IsAdmin
			}
			unameCopy := username
			ctx.Locals(clientVars.WebAccessStore, &webaccess.SocketClientRequest{
				Username: &unameCopy,
				IsAdmin:  isAdmin,
			})
			pluginAuthenticated = true
		}
	}

	if !pluginAuthenticated {
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
			return sendAuthnFailure()
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
	}
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

// NormalizeAuthzLevel mirrors the original webaccess.normalizeAuthzLevel:
// `true` normalizes to "create", the three known levels pass through, and
// everything else (false, empty, unknown strings) is denied.
func NormalizeAuthzLevel(level interface{}) (*string, error) {
	switch castedExpr := level.(type) {
	case string:
		switch castedExpr {
		case "readOnly", "modify", "create":
			return &castedExpr, nil
		}
	case bool:
		if castedExpr {
			create := "create"
			return &create, nil
		}
	}
	return nil, errors.New("access denied")
}

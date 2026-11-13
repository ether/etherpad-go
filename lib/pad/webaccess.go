package pad

import (
	"crypto/subtle"
	"encoding/base64"
	"errors"
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

// encodedPadRegex matches the encoded pad id in a "/p/<id>" or "/s/<id>"
// request path. Spreadsheet ("/s/") pages share the pad authorization model, so
// their pad id must be extracted here too — otherwise PadAuthorizations is never
// populated for sheets and the authorize hook receives an empty PadId. Compiled
// once (used on the per-request access hot path).
var encodedPadRegex = regexp.MustCompile("^/[ps]/([^/]*)")

// extractEncodedPadId returns the still-encoded pad id from a "/p/<id>" or
// "/s/<id>" request path, or "" when the path is neither (or has no id). The
// caller is responsible for URL-unescaping the result.
func extractEncodedPadId(path string) string {
	m := encodedPadRegex.FindStringSubmatch(path)
	if len(m) < 2 || m[1] == "" {
		return ""
	}
	return m[1]
}

func UserCanModify(padId *string, req *webaccess.SocketClientRequest, readOnlyManager ReadOnlyManager, cfg *settings.Settings) bool {
	if readOnlyManager.IsReadOnlyID(padId) {
		return false
	}

	if !cfg.RequireAuthentication {
		return true
	}

	if req == nil || (req.ReadOnly != nil && *req.ReadOnly) {
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
	// Liveness and readiness probes carry no data and must work for the
	// orchestrator even when the pads require a login.
	if p := ctx.Path(); p == "/health/live" || p == "/health/ready" || isPublicAsset(p) {
		return ctx.Next()
	}
	var requireAdmin = isAdminPath(ctx.Path())
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

			// Use the bare pad id (e.g. "testpad"), not the full path ("/p/testpad"
			// or "/s/testpad"): SecurityManager.CheckAccess looks up
			// PadAuthorizations by the bare socket pad id, so the keys must agree.
			encodedPadId := extractEncodedPadId(ctx.Path())

			if utf8.RuneCountInString(encodedPadId) == 0 {
				return true
			}

			var padId, queryErr = url.QueryUnescape(encodedPadId)

			if queryErr != nil {
				return false
			}

			if readOnlyManager.IsReadOnlyID(&padId) {
				// pad is read-only; store the authorization under the real pad id,
				// because SecurityManager resolves to and looks up the real pad id.
				var realPadId, err = readOnlyManager.GetPadId(padId)
				if err != nil {
					logger.Warnf("Error getting real pad ID for %s: %v", padId, err)
					return false
				}
				if realPadId == nil {
					return false
				}
				padId = *realPadId
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

		if hookSystem != nil {
			var padIdForHook string
			if enc := extractEncodedPadId(ctx.Path()); enc != "" {
				if decoded, err := url.QueryUnescape(enc); err == nil {
					padIdForHook = decoded
				}
			}
			authorizeCtx := &events.AuthorizeContext{
				Path:         ctx.Path(),
				PadId:        padIdForHook,
				RequireAdmin: requireAdmin,
				User:         sessionReq,
			}
			hookSystem.ExecuteAuthorizeHooks(authorizeCtx)
			switch authorizeCtx.Decision() {
			case events.AuthorizeGrant:
				return grant(authorizeCtx.Level())
			case events.AuthorizeDeny:
				return false
			}
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

	// Lookups in a nil Users map are fine; don't write it back into the
	// shared settings, which every request reads concurrently.
	var user, ok = ctx.Locals(clientVars.WebAccessStore).(*webaccess.SocketClientRequest)

	var webAccessCtx = webaccess.WebAccessType{
		Users: retrievedSettings.Users,
		Next:  ctx.Next,
	}

	if ok {
		webAccessCtx.Username = user.Username
	}

	username, password, httpBasicAuth := parseBasicAuth(ctx.Get("authorization"))
	if httpBasicAuth {
		webAccessCtx.Username = &username
		webAccessCtx.Password = &password
	}

	// sendAuthnFailure fires the authnFailure hook then falls back to the
	// default 401 response. It REPLACES all inline 401 logic below.
	sendAuthnFailure := func() error {
		logger.Infof("failed authentication from IP %s", retrievedSettings.LogIP(ctx.IP()))
		if hookSystem != nil {
			failCtx := &events.AuthnFailureContext{Path: ctx.Path(), RequireAdmin: requireAdmin}
			hookSystem.ExecuteAuthnFailureHooks(failCtx)
			if failCtx.Handled() {
				for k, v := range failCtx.Headers() {
					ctx.Set(k, v)
				}
				return ctx.Status(failCtx.Status()).SendString(failCtx.Body())
			}
			if handled, err := sendAuthFailure(ctx, hookSystem, requireAdmin, true); handled {
				return err
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
			Users:         retrievedSettings.Users,
		}
		hookSystem.ExecuteAuthenticateHooks(authCtx)
		if authCtx.Answered() {
			if authCtx.Rejected() {
				return sendAuthnFailure()
			}
			// Plugin authenticated the user. Populate the session user, looking up
			// settings.Users for admin status when the username is known there.
			username := authCtx.Username()
			var known *settings.User
			if u, ok := retrievedSettings.Users[username]; ok {
				known = &u
			}
			ctx.Locals(clientVars.WebAccessStore, sessionUser(username, known))
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

		if !httpBasicAuth || webAccessCtx.Username == nil || password == nil || *password == "" || subtle.ConstantTimeCompare([]byte(*password), []byte(*webAccessCtx.Password)) != 1 {
			return sendAuthnFailure()
		}
		var retrievedUserFromMap = retrievedSettings.Users[*webAccessCtx.Username]
		// The session user carries no password, so it cannot leak into logs.
		ctx.Locals(clientVars.WebAccessStore, sessionUser(*webAccessCtx.Username, &retrievedUserFromMap))
		// Remove password
		if webAccessCtx.Username == nil {
			logger.Warn("authenticate hook failed to add user settings to session")
			return ctx.Status(500).SendString("Internal Server Status")
		}
		if webAccessCtx.Username == nil {
			var newUsername = "<no username>"
			webAccessCtx.Username = &newUsername
		}

		logger.Infof("Successful authentication from IP %s for user %s", retrievedSettings.LogIP(ctx.IP()), *webAccessCtx.Username)
	}
	// ///////////////////////////////////////////////////////////////////////////////////////////////
	// Step 4: Try to access the thing again. If this fails, give the user a 403 error. Plugins can
	// use the authzFailure hook to override the default error handling behavior (e.g., to redirect to
	// a login page).
	// ///////////////////////////////////////////////////////////////////////////////////////////////

	sendAuthzFailure := func() error {
		if hookSystem != nil {
			failCtx := &events.AuthzFailureContext{Path: ctx.Path(), RequireAdmin: requireAdmin}
			hookSystem.ExecuteAuthzFailureHooks(failCtx)
			if failCtx.Handled() {
				for k, v := range failCtx.Headers() {
					ctx.Set(k, v)
				}
				return ctx.Status(failCtx.Status()).SendString(failCtx.Body())
			}
			if handled, err := sendAuthFailure(ctx, hookSystem, requireAdmin, false); handled {
				return err
			}
		}
		return ctx.Status(403).SendString("Forbidden")
	}

	var auth = authorize()
	if auth && !requireAdmin {
		return ctx.Next()
	}

	if auth && requireAdmin {
		return ctx.Status(200).SendString("Authorized")
	}

	return sendAuthzFailure()
}

// ResolvePadAuthorization decides an authenticated user's level for a pad
// like webaccess does for "/p/<padId>": admins and, without
// requireAuthorization, every user get "create"; otherwise the authorize
// hooks decide, and no answer means no access (""). padId is the real pad id.
func ResolvePadAuthorization(user *webaccess.SocketClientRequest, padId string, cfg *settings.Settings, hookSystem *hooks.Hook) string {
	if user == nil {
		return ""
	}
	if user.IsAdmin || !cfg.RequireAuthorization {
		return "create"
	}
	if hookSystem == nil {
		return ""
	}
	authorizeCtx := &events.AuthorizeContext{Path: "/p/" + url.PathEscape(padId), PadId: padId, User: user}
	hookSystem.ExecuteAuthorizeHooks(authorizeCtx)
	if authorizeCtx.Decision() != events.AuthorizeGrant {
		return ""
	}
	level, err := NormalizeAuthzLevel(authorizeCtx.Level())
	if err != nil {
		return ""
	}
	return *level
}

// sessionUser is the authenticated user as the rest of the request sees it
// (etherpad-lite's req.session.user): name, admin flag and the readOnly and
// canCreate restrictions from settings.users, without the password.
func sessionUser(username string, u *settings.User) *webaccess.SocketClientRequest {
	name := username
	user := &webaccess.SocketClientRequest{Username: &name}
	if u == nil {
		return user
	}
	user.IsAdmin = u.IsAdmin != nil && *u.IsAdmin
	if u.ReadOnly != nil {
		readOnly := *u.ReadOnly
		user.ReadOnly = &readOnly
	}
	if u.CanCreate != nil {
		canCreate := *u.CanCreate
		user.CanCreate = &canCreate
	}
	return user
}

// sendAuthFailure runs the deprecated authFailure hook and sends its response
// when a callback handled the failure.
func sendAuthFailure(ctx fiber.Ctx, hookSystem *hooks.Hook, requireAdmin, authentication bool) (bool, error) {
	failCtx := &events.AuthFailureContext{Path: ctx.Path(), RequireAdmin: requireAdmin, Authentication: authentication}
	hookSystem.ExecuteAuthFailureHooks(failCtx)
	if !failCtx.Handled() {
		return false, nil
	}
	for k, v := range failCtx.Headers() {
		ctx.Set(k, v)
	}
	return true, ctx.Status(failCtx.Status()).SendString(failCtx.Body())
}

// parseBasicAuth reads "Basic base64(user:password)". The password is
// everything after the first ':', so it may contain ':' itself. Anything
// malformed (no ':', invalid base64) counts as no credentials, which leads to
// the normal authentication failure.
func parseBasicAuth(header string) (username, password string, ok bool) {
	const prefix = "Basic "
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return "", "", false
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(header[len(prefix):]))
	if err != nil {
		return "", "", false
	}
	username, password, ok = strings.Cut(string(decoded), ":")
	return username, password, ok
}

// isAdminPath reports whether path is (below) /admin-auth. Repeated slashes
// are collapsed first, so "//admin-auth//" is treated as the admin path too.
func isAdminPath(path string) bool {
	for strings.Contains(path, "//") {
		path = strings.ReplaceAll(path, "//", "/")
	}
	return strings.HasPrefix(strings.ToLower(path), "/admin-auth")
}

// publicAssetPrefixes are served without authentication, like
// etherpad-lite's static content (expressPreSession): they hold no pad data,
// and a login page could not be styled without them.
var publicAssetPrefixes = []string{"/static/", "/css/", "/images/", "/js/"}

func isPublicAsset(path string) bool {
	if path == "/favicon.ico" || path == "/robots.txt" {
		return true
	}
	if strings.Contains(path, "..") {
		return false
	}
	for _, prefix := range publicAssetPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
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

package pad

import (
	"encoding/base64"
	"errors"
	"net/url"
	"regexp"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/ether/etherpad-go/lib/accounts"
	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/hooks"
	"github.com/ether/etherpad-go/lib/hooks/events"
	"github.com/ether/etherpad-go/lib/models/clientVars"
	"github.com/ether/etherpad-go/lib/models/webaccess"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/ether/etherpad-go/lib/utils"
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

// UserCanModify tells whether the request user (nil: a visitor) may change
// the pad: never through a read-only id; otherwise by the level webaccess
// remembered for the pad, or the level the account management gives.
func UserCanModify(padId *string, req *webaccess.SocketClientRequest, readOnlyManager ReadOnlyManager, cfg *settings.Settings) bool {
	if readOnlyManager.IsReadOnlyID(padId) {
		return false
	}
	var level string
	if req != nil && req.PadAuthorizations != nil {
		if known, ok := (*req.PadAuthorizations)[*padId]; ok {
			level = capByFlags(known, req)
			return level == accounts.LevelModify || level == accounts.LevelCreate
		}
	}
	if req != nil && req.Grants == nil {
		// A user webaccess did not make, without a level for the pad:
		// deny rather than guess.
		return false
	}
	// Without the level webaccess decided with the authorize hooks, a
	// logged-in person under requireAuthorization gets none here.
	level = padLevel(AccountsOf(readOnlyManager.Store, cfg), req, *padId, "", cfg, nil)
	return level == accounts.LevelModify || level == accounts.LevelCreate
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
	if p := ctx.Path(); p == "/health" || p == "/health/live" || p == "/health/ready" || isPublicAsset(p) || isLoginFlow(p) {
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

	acc := AccountsOf(readOnlyManager.Store, retrievedSettings)

	// authorize decides with the account management (lib/accounts): a
	// visitor who is not logged in has the anonymous account's rights, a
	// logged-in person those of its roles (and the visitors'). Admin pages
	// need an admin. The level for the pad is remembered in
	// PadAuthorizations for the websocket, as etherpad-lite keeps it in
	// the session.
	authorize := func() bool {
		var user, _ = ctx.Locals(clientVars.WebAccessStore).(*webaccess.SocketClientRequest)
		if requireAdmin {
			return user != nil && (user.IsAdmin || requestGrants(acc, user).IsAdmin())
		}

		// Use the bare pad id (e.g. "testpad"), not the full path ("/p/testpad"
		// or "/s/testpad"): SecurityManager.CheckAccess looks up
		// PadAuthorizations by the bare socket pad id, so the keys must agree.
		encodedPadId := extractEncodedPadId(ctx.Path())
		if utf8.RuneCountInString(encodedPadId) == 0 {
			// Not a pad: visitors may see it unless the instance lets
			// them do nothing at all; logged-in people may, but under
			// requireAuthorization only if the authorize hooks agree.
			if user == nil {
				return !requestGrants(acc, nil).Empty()
			}
			if user.IsAdmin || !retrievedSettings.RequireAuthorization {
				return true
			}
			if hookSystem == nil {
				return false
			}
			authorizeCtx := &events.AuthorizeContext{Path: ctx.Path(), RequireAdmin: requireAdmin, User: user}
			hookSystem.ExecuteAuthorizeHooks(authorizeCtx)
			return authorizeCtx.Decision() == events.AuthorizeGrant
		}
		padId, queryErr := url.QueryUnescape(encodedPadId)
		if queryErr != nil {
			return false
		}
		readOnlyId := readOnlyManager.IsReadOnlyID(&padId)
		if readOnlyId {
			// Store the authorization under the real pad id, because
			// SecurityManager resolves to and looks up the real pad id.
			realPadId, err := readOnlyManager.GetPadId(padId)
			if err != nil {
				logger.Warnf("Error getting real pad ID for %s: %v", padId, err)
				return false
			}
			if realPadId == nil {
				return false
			}
			padId = *realPadId
		}
		level := padLevel(acc, user, padId, ctx.Path(), retrievedSettings, hookSystem)
		if readOnlyId {
			level = minLevel(level, accounts.LevelReadOnly)
		}
		if level == "" {
			return false
		}
		if user != nil {
			if user.PadAuthorizations == nil {
				newMap := make(map[string]string)
				user.PadAuthorizations = &newMap
			}
			(*user.PadAuthorizations)[padId] = level
		}
		return true
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
	// loggedIn: the request came with a user, e.g. of a pad login session.
	loggedIn := ok && user != nil

	username, password, httpBasicAuth := parseBasicAuth(ctx.Get("authorization"))
	if httpBasicAuth {
		webAccessCtx.Username = &username
		webAccessCtx.Password = &password
	}

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
			// A browser that opens a pad goes to the login page instead
			// of the browser's password dialog; tools keep basic auth.
			if !httpBasicAuth && !loggedIn {
				if target, ok := loginRedirectFor(ctx, retrievedSettings.TrustProxy); ok {
					return ctx.Redirect().Status(fiber.StatusFound).To(target)
				}
			}
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
			// Plugin authenticated the user. Its rights are those of its
			// account when there is one by that name, else those of
			// everyone logged in.
			username := authCtx.Username()
			a, err := acc.AccountByUsername(username)
			if err != nil || (a != nil && (a.DisabledAt != nil || a.Kind != accounts.KindHuman)) {
				// A disabled account (or a name taken by a service or
				// built-in account) gets in neither way.
				return sendAuthnFailure()
			}
			ctx.Locals(clientVars.WebAccessStore, sessionUserOf(acc, username, a))
			pluginAuthenticated = true
		}
	}

	if !pluginAuthenticated {
		// Already logged in (the pads' login session), no plugin knew
		// better and still not allowed: logging in again would not help,
		// and would loop.
		if loggedIn && !httpBasicAuth {
			return sendAuthzFailure()
		}
		if !httpBasicAuth || webAccessCtx.Username == nil || webAccessCtx.Password == nil {
			return sendAuthnFailure()
		}
		a, err := acc.Authenticate(*webAccessCtx.Username, *webAccessCtx.Password)
		if err != nil {
			if errors.Is(err, accounts.ErrLocked) {
				logger.Warnf("Login of %q locked after repeated failures (IP %s)", *webAccessCtx.Username, retrievedSettings.LogIP(ctx.IP()))
			}
			return sendAuthnFailure()
		}
		if a.MustChangePassword {
			// The password from the bootstrap or a reset only opens the
			// login page, where it must be replaced.
			logger.Infof("Login of %q refused: the password must be changed at the login page first", a.Username)
			return sendAuthnFailure()
		}
		// The session user carries no password, so it cannot leak into logs.
		ctx.Locals(clientVars.WebAccessStore, sessionUserOf(acc, a.Username, a))
		logger.Infof("Successful authentication from IP %s for user %s", retrievedSettings.LogIP(ctx.IP()), a.Username)
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

	return sendAuthzFailure()
}

// ResolvePadAuthorization decides the level of user (nil: a visitor who is
// not logged in) for the real pad padId like webaccess does for
// "/p/<padId>": by the grants of the account management, admins always
// "create", and with requireAuthorization the authorize hooks limit a
// logged-in person's level ("" = no access).
func ResolvePadAuthorization(store db.DataStore, user *webaccess.SocketClientRequest, padId string, cfg *settings.Settings, hookSystem *hooks.Hook) string {
	return padLevel(AccountsOf(store, cfg), user, padId, "", cfg, hookSystem)
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

// loginFlowPrefixes are the login's own pages (the OpenID Connect
// provider and the pads' login): they hold no pad data and must be
// reachable to log in at all, also when visitors may do nothing. The
// provider's endpoints check their tokens themselves.
var loginFlowPrefixes = []string{"/oauth2/", "/.well-known/"}

// loginFlowPages are the pads' login (lib/api/padlogin).
var loginFlowPages = map[string]bool{"/auth/login": true, "/auth/callback": true, "/auth/logout": true, "/auth/me": true}

// NeedsUser reports whether the access check of path looks at who is
// logged in; health probes, public assets and the login's own pages do
// not, so a login session need not be read for them.
func NeedsUser(path string) bool {
	return !(path == "/health" || path == "/health/live" || path == "/health/ready" || isPublicAsset(path) || isLoginFlow(path))
}

func isLoginFlow(path string) bool {
	if strings.Contains(path, "..") || strings.Contains(path, "//") {
		return false
	}
	if loginFlowPages[path] {
		return true
	}
	for _, prefix := range loginFlowPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// loginPage builds the URL of the pads' login page that comes back to
// redirect; nil while nobody can log in on the pads.
var loginPage atomic.Pointer[func(redirect string) string]

// SetLoginPage sets the pads' login page (lib/api/padlogin); nil turns
// the redirect to it off.
func SetLoginPage(fn func(redirect string) string) {
	if fn == nil {
		loginPage.Store(nil)
		return
	}
	loginPage.Store(&fn)
}

// loginRedirectFor returns where a browser that may not open a page goes
// to log in: only for pages people open (a pad, a sheet, its history),
// read with GET and asking for HTML.
func loginRedirectFor(ctx fiber.Ctx, trustProxy bool) (string, bool) {
	fn := loginPage.Load()
	if fn == nil || ctx.Method() != fiber.MethodGet || extractEncodedPadId(ctx.Path()) == "" {
		return "", false
	}
	if !strings.Contains(ctx.Get(fiber.HeaderAccept), fiber.MIMETextHTML) {
		return "", false
	}
	// The path as sent (fasthttp's request URI is normalized and would turn
	// "team%2Fnotes" into another pad), with its query.
	// Behind a proxy that serves Etherpad under a prefix, both the login
	// page and the way back are under it.
	prefix := utils.RequestProxyPath(ctx, trustProxy)
	back := prefix + ctx.Path()
	if q := ctx.Request().URI().QueryString(); len(q) > 0 {
		back += "?" + string(q)
	}
	return prefix + (*fn)(back), true
}

// publicAssetPrefixes are served without authentication, like
// etherpad-lite's static content (expressPreSession): they hold no pad data,
// and a login page could not be styled without them.
var publicAssetPrefixes = []string{"/static/", "/css/", "/images/", "/js/"}

func isPublicAsset(path string) bool {
	if path == "/favicon.ico" || path == "/robots.txt" || path == "/manifest.json" {
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

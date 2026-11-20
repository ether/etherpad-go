// Package padlogin is the optional login on pads: a person logs in on
// Etherpad's own OpenID Connect provider and keeps a session cookie; who
// does not log in has the visitors' rights (lib/accounts).
package padlogin

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/gob"
	"net/url"
	"strings"
	"time"

	"github.com/ether/etherpad-go/lib/accounts"
	"github.com/ether/etherpad-go/lib/api/oidc"
	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/models/clientVars"
	"github.com/ether/etherpad-go/lib/models/webaccess"
	"github.com/ether/etherpad-go/lib/pad"
	epsession "github.com/ether/etherpad-go/lib/session"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/ether/etherpad-go/lib/tokens"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	"github.com/gofiber/fiber/v3/middleware/session"
	"go.uber.org/zap"
)

const (
	LoginPath  = "/auth/login"
	LogoutPath = "/auth/logout"
	MePath     = "/auth/me"

	// keyAccount holds the account id of a logged-in session.
	keyAccount = "account"
	// keyPending holds a login in progress: state, PKCE verifier and where
	// to go back to.
	keyPending = "login"
	// keySeen is when the session was last pushed forward (Unix seconds).
	keySeen = "seen"

	// refreshEvery is how often a used session is saved again, which
	// pushes its idle timeout forward.
	refreshEvery = time.Hour

	// pendingTTL is how long a started login may take.
	pendingTTL = 15 * time.Minute
	// defaultLifetime is how long a session lasts without use when
	// cookie.sessionLifetime is not set.
	defaultLifetime = 10 * 24 * time.Hour
	// absoluteLifetime is how long a session lasts at most.
	absoluteLifetime = 30 * 24 * time.Hour
)

// Login is the pads' login.
type Login struct {
	store    *session.Store
	authn    *oidc.Authenticator
	accounts *accounts.Manager
	settings *settings.Settings
	logger   *zap.SugaredLogger
	onLogout func(sessionID string)
	tokens   *tokens.Manager
}

// pending is a login in progress, kept in the session.
type pending struct {
	State    string
	Verifier string
	Redirect string
	Expires  int64
}

func init() {
	// The session keeps its values with gob.
	gob.Register(pending{})
}

// SetAuthenticator sets the provider people log in on; it is made after
// the routes are mounted.
func (l *Login) SetAuthenticator(a *oidc.Authenticator) { l.authn = a }

// OnLogout sets what runs when a session logs out, e.g. closing the pad
// connections opened with it.
func (l *Login) OnLogout(fn func(sessionID string)) { l.onLogout = fn }

// CookieName is the name of the login session cookie.
func CookieName(s *settings.Settings) string { return s.Cookie.Prefix + "ep_login" }

// New sets up the login on store; authn may be nil when no OpenID Connect
// provider runs, then nobody logs in on the pads.
func New(store db.DataStore, authn *oidc.Authenticator, cfg *settings.Settings, logger *zap.SugaredLogger) *Login {
	lifetime := time.Duration(cfg.Cookie.SessionLifetime) * time.Millisecond
	if lifetime <= 0 {
		lifetime = defaultLifetime
	}
	lifetime = min(lifetime, absoluteLifetime)
	sameSite := cfg.Cookie.SameSite
	if sameSite == "" {
		sameSite = fiber.CookieSameSiteLaxMode
	}
	secure := strings.EqualFold(sameSite, fiber.CookieSameSiteNoneMode)
	if cfg.SSO != nil && strings.HasPrefix(cfg.SSO.Issuer, "https://") {
		secure = true
	}
	var dataStore = store
	return &Login{
		store: session.NewStore(session.Config{
			Storage:         epsession.NewSessionDatabase(&dataStore),
			Extractor:       extractors.FromCookie(CookieName(cfg)),
			CookieHTTPOnly:  true,
			CookieSecure:    secure,
			CookieSameSite:  sameSite,
			CookiePath:      "/",
			IdleTimeout:     lifetime,
			AbsoluteTimeout: absoluteLifetime,
		}),
		authn:    authn,
		accounts: accounts.For(store),
		tokens:   tokens.For(store, cfg),
		settings: cfg,
		logger:   logger,
	}
}

// Available reports whether people can log in on the pads.
func (l *Login) Available() bool { return l != nil && l.authn != nil && l.authn.PadLoginAvailable() }

// Middleware makes the person of a login session the request's user, so
// the access checks (lib/pad) give it its roles. A session of an account
// that is gone or disabled counts for nothing. A request that already has
// a user (e.g. set by a plugin) keeps it.
func (l *Login) Middleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		if c.Cookies(CookieName(l.settings)) == "" || !pad.NeedsUser(c.Path()) {
			return c.Next()
		}
		if _, set := c.Locals(clientVars.WebAccessStore).(*webaccess.SocketClientRequest); set {
			return c.Next()
		}
		if user := l.userOf(c); user != nil {
			c.Locals(clientVars.WebAccessStore, user)
		}
		return c.Next()
	}
}

// userOf returns the request user of the session's account, or nil.
func (l *Login) userOf(c fiber.Ctx) *webaccess.SocketClientRequest {
	a := l.personOf(c)
	if a == nil {
		return nil
	}
	return UserOf(l.accounts, a)
}

// UserOf is the request user of a logged-in person.
func UserOf(m *accounts.Manager, a *db.Account) *webaccess.SocketClientRequest {
	name := a.Username
	g, _ := m.GrantsFor(a.ID)
	return &webaccess.SocketClientRequest{Username: &name, AccountID: a.ID, Grants: &g, IsAdmin: g.IsAdmin()}
}

// personOf returns the active person of the request's session, or nil.
func (l *Login) personOf(c fiber.Ctx) *db.Account {
	sess, err := l.store.Get(c)
	if err != nil {
		return nil
	}
	defer sess.Release()
	id, _ := sess.Get(keyAccount).(string)
	if id == "" {
		return nil
	}
	a, err := l.accounts.Person(id)
	if err != nil {
		l.logger.Warnf("pad login: loading account %s: %v", id, err)
		return nil
	}
	if a == nil {
		return nil
	}
	// In use: the idle timeout starts again (saved at most hourly).
	seen, _ := sess.Get(keySeen).(int64)
	if now := time.Now().Unix(); now-seen >= int64(refreshEvery/time.Second) {
		sess.Set(keySeen, now)
		if err := sess.Save(); err != nil {
			l.logger.Warnf("pad login: refreshing a session: %v", err)
		}
	}
	return a
}

// Register mounts the login's pages.
func (l *Login) Register(app fiber.Router) {
	app.Get(LoginPath, l.login)
	app.Get(oidc.PadCallbackPath, l.callback)
	app.Post(LogoutPath, l.logout)
	app.Get(MePath, l.me)
	l.registerTokens(app)
}

// LoginURL is the page that logs in and comes back to redirect.
func LoginURL(redirect string) string {
	return LoginPath + "?redirect=" + url.QueryEscape(redirect)
}

// SafeRedirect keeps a redirect target on this server: a path, never a
// URL to another site ("//evil", "/\evil", "https://…"). Anything else is
// "/".
func SafeRedirect(target string) string {
	if target == "" || !strings.HasPrefix(target, "/") || strings.HasPrefix(target, "//") ||
		strings.ContainsAny(target, "\\\r\n\t") {
		return "/"
	}
	u, err := url.Parse(target)
	if err != nil || u.Scheme != "" || u.Host != "" {
		return "/"
	}
	return target
}

func randomString() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (l *Login) login(c fiber.Ctx) error {
	c.Set(fiber.HeaderCacheControl, "no-store")
	if !l.Available() {
		return c.Status(fiber.StatusNotFound).SendString("Logging in is not set up on this server.")
	}
	redirect := SafeRedirect(c.Query("redirect"))
	state, err := randomString()
	if err != nil {
		return err
	}
	verifier, err := randomString()
	if err != nil {
		return err
	}
	target, err := l.authn.PadAuthorizeURL(state, verifier)
	if err != nil {
		return c.Status(fiber.StatusNotFound).SendString("Logging in is not set up on this server.")
	}
	sess, err := l.store.Get(c)
	if err != nil {
		return err
	}
	defer sess.Release()
	sess.Set(keyPending, pending{State: state, Verifier: verifier, Redirect: redirect, Expires: time.Now().Add(pendingTTL).Unix()})
	if err := sess.Save(); err != nil {
		return err
	}
	return c.Redirect().Status(fiber.StatusFound).To(target)
}

func (l *Login) callback(c fiber.Ctx) error {
	c.Set(fiber.HeaderCacheControl, "no-store")
	sess, err := l.store.Get(c)
	if err != nil {
		return err
	}
	defer sess.Release()
	p, _ := sess.Get(keyPending).(pending)
	// One answer per started login, right or wrong.
	sess.Delete(keyPending)
	fail := func(status int, msg string) error {
		_ = sess.Save()
		return c.Status(status).SendString(msg)
	}
	if p.State == "" || time.Now().Unix() > p.Expires {
		return fail(fiber.StatusBadRequest, "This login has expired. Please log in again.")
	}
	if subtle.ConstantTimeCompare([]byte(c.Query("state")), []byte(p.State)) != 1 {
		return fail(fiber.StatusBadRequest, "This login does not belong to this browser. Please log in again.")
	}
	if c.Query("error") != "" {
		// Cancelled or refused at the provider: back where the person was.
		_ = sess.Save()
		return c.Redirect().Status(fiber.StatusFound).To(p.Redirect)
	}
	a, err := l.authn.RedeemPadCode(c.Query("code"), p.Verifier)
	if err != nil {
		l.logger.Infof("pad login failed from IP %s: %v", l.settings.LogIP(c.IP()), err)
		return fail(fiber.StatusBadRequest, "The login could not be completed. Please log in again.")
	}
	// A new id for the logged-in session, so one planted before the login
	// is not logged in with it (session fixation).
	if err := sess.Regenerate(); err != nil {
		return err
	}
	sess.Set(keyAccount, a.ID)
	sess.Set(keySeen, time.Now().Unix())
	if err := sess.Save(); err != nil {
		return err
	}
	l.logger.Infof("pad login of %s from IP %s", a.Username, l.settings.LogIP(c.IP()))
	return c.Redirect().Status(fiber.StatusFound).To(p.Redirect)
}

// sameOrigin reports whether a state-changing request comes from this
// site. Browsers say so in Sec-Fetch-Site, which no page can set; pads
// send no referrer, so their Origin is "null" there. Without the header
// (old browsers, tools) Origin and then Referer must name this host.
func sameOrigin(c fiber.Ctx) bool {
	if site := c.Get("Sec-Fetch-Site"); site != "" {
		return site == "same-origin"
	}
	from := c.Get(fiber.HeaderOrigin)
	if from == "null" {
		return false
	}
	if from == "" {
		from = c.Get(fiber.HeaderReferer)
	}
	if from == "" {
		return false
	}
	u, err := url.Parse(from)
	return err == nil && u.Host != "" && strings.EqualFold(u.Host, c.Host())
}

func (l *Login) logout(c fiber.Ctx) error {
	c.Set(fiber.HeaderCacheControl, "no-store")
	if !sameOrigin(c) {
		return c.Status(fiber.StatusForbidden).SendString("Forbidden")
	}
	if c.Cookies(CookieName(l.settings)) != "" {
		sess, err := l.store.Get(c)
		if err != nil {
			return err
		}
		id := sess.ID()
		err = sess.Destroy()
		sess.Release()
		if err != nil {
			return err
		}
		if l.onLogout != nil {
			l.onLogout(id)
		}
	}
	if strings.Contains(c.Get(fiber.HeaderAccept), fiber.MIMEApplicationJSON) {
		return c.SendStatus(fiber.StatusNoContent)
	}
	return c.Redirect().Status(fiber.StatusSeeOther).To(SafeRedirect(c.FormValue("redirect")))
}

// Me is what the pad's profile menu shows.
type Me struct {
	LoginAvailable bool   `json:"loginAvailable"`
	LoggedIn       bool   `json:"loggedIn"`
	Username       string `json:"username,omitempty"`
	DisplayName    string `json:"displayName,omitempty"`
	Admin          bool   `json:"admin,omitempty"`
	// Tokens: may manage own tokens (tokens:own), "My tokens" in the menu.
	Tokens bool `json:"tokens,omitempty"`
}

func (l *Login) me(c fiber.Ctx) error {
	c.Set(fiber.HeaderCacheControl, "no-store")
	me := Me{LoginAvailable: l.Available()}
	if c.Cookies(CookieName(l.settings)) != "" {
		if a := l.personOf(c); a != nil {
			me.LoggedIn, me.Username, me.DisplayName = true, a.Username, a.DisplayName
			if me.DisplayName == "" {
				me.DisplayName = a.Username
			}
			if g, err := l.accounts.GrantsFor(a.ID); err == nil {
				me.Admin = g.IsAdmin()
				me.Tokens = g.HasGlobal(accounts.ScopeTokensOwn)
			}
		}
	}
	return c.JSON(me)
}

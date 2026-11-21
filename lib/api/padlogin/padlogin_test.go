package padlogin_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/ether/etherpad-go/lib/accounts"
	"github.com/ether/etherpad-go/lib/api/oidc"
	"github.com/ether/etherpad-go/lib/api/padlogin"
	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/models/clientVars"
	"github.com/ether/etherpad-go/lib/models/webaccess"
	"github.com/ether/etherpad-go/lib/pad"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/adaptor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/net/html"
)

const issuer = "http://pads.test"

type env struct {
	app      *fiber.App
	store    db.DataStore
	accounts *accounts.Manager
	cfg      *settings.Settings
	// seen is the request user the last pad page saw.
	seen *webaccess.SocketClientRequest
}

func newEnv(t *testing.T) *env {
	t.Helper()
	cfg := &settings.Settings{
		Cookie: settings.Cookie{SameSite: "Lax"},
		SSO: &settings.SSO{Issuer: issuer, Clients: []settings.SSOClient{
			{Type: "admin", ClientId: "admin_client", RedirectUris: []string{issuer + "/admin/"}, GrantTypes: []string{"authorization_code"}},
			{Type: "user", ClientId: "user_client", RedirectUris: []string{issuer + "/"}, GrantTypes: []string{"authorization_code"}},
		}},
	}
	store := db.NewMemoryDataStore()
	logger := zap.NewNop().Sugar()
	e := &env{cfg: cfg, accounts: pad.AccountsOf(store, cfg), store: store}
	// Closed: visitors may do nothing.
	require.NoError(t, e.accounts.ApplyTemplate(accounts.System, accounts.TemplateClosed))

	authn := oidc.NewAuthenticator(cfg, store)
	login := padlogin.New(store, nil, cfg, logger)
	app := fiber.New()
	rom := pad.NewReadOnlyManager(store)
	app.Use(login.Middleware())
	app.Use(func(c fiber.Ctx) error { return pad.CheckAccessWithHooks(c, logger, cfg, rom, nil) })
	login.Register(app)
	login.SetAuthenticator(authn)
	pad.SetLoginPage(padlogin.LoginURL)
	t.Cleanup(func() { pad.SetLoginPage(nil) })
	provider := adaptor.HTTPHandler(authn.Handler())
	for _, p := range authn.Paths() {
		app.All(p, provider)
	}
	app.All("/oauth2/login", adaptor.HTTPHandler(http.HandlerFunc(authn.LoginHandler)))
	app.Get("/p/:pad", func(c fiber.Ctx) error {
		e.seen, _ = c.Locals(clientVars.WebAccessStore).(*webaccess.SocketClientRequest)
		return c.SendString("pad")
	})
	e.app = app
	return e
}

// browser keeps cookies like a browser on one site.
type browser struct {
	e       *env
	cookies map[string]string
}

func (e *env) browser() *browser { return &browser{e: e, cookies: map[string]string{}} }

func (b *browser) do(t *testing.T, method, target string, form url.Values, headers map[string]string) *http.Response {
	t.Helper()
	if strings.HasPrefix(target, issuer) {
		target = strings.TrimPrefix(target, issuer)
	}
	var bodyReader io.Reader
	if form != nil {
		bodyReader = strings.NewReader(form.Encode())
	}
	req := httptest.NewRequest(method, issuer+target, bodyReader)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	for k, v := range b.cookies {
		req.AddCookie(&http.Cookie{Name: k, Value: v})
	}
	resp, err := b.e.app.Test(req, slowRunners)
	require.NoError(t, err)
	for _, c := range resp.Cookies() {
		if c.MaxAge < 0 || c.Value == "" {
			delete(b.cookies, c.Name)
		} else {
			b.cookies[c.Name] = c.Value
		}
	}
	return resp
}

func (b *browser) get(t *testing.T, target string) *http.Response {
	return b.do(t, http.MethodGet, target, nil, map[string]string{"Accept": "text/html"})
}

func readBody(t *testing.T, r *http.Response) string {
	t.Helper()
	data, err := io.ReadAll(r.Body)
	require.NoError(t, err)
	return string(data)
}

func formOf(t *testing.T, r *http.Response) url.Values {
	t.Helper()
	doc, err := html.Parse(r.Body)
	require.NoError(t, err)
	values := url.Values{}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "input" {
			var name, value string
			for _, a := range n.Attr {
				if a.Key == "name" {
					name = a.Val
				} else if a.Key == "value" {
					value = a.Val
				}
			}
			if name != "" {
				values.Set(name, value)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return values
}

// logIn goes the whole way from the pad's login link back to the pad; it
// returns the last answer.
func (b *browser) logIn(t *testing.T, redirect, username, password string) *http.Response {
	t.Helper()
	resp := b.get(t, padlogin.LoginURL(redirect))
	require.Equal(t, http.StatusFound, resp.StatusCode, readBody(t, resp))
	resp = b.get(t, resp.Header.Get("Location")) // authorize
	require.Equal(t, http.StatusFound, resp.StatusCode, readBody(t, resp))
	page := b.get(t, resp.Header.Get("Location"))
	require.Equal(t, http.StatusOK, page.StatusCode)
	form := formOf(t, page)
	form.Set("username", username)
	form.Set("password", password)
	resp = b.do(t, http.MethodPost, "/oauth2/login", form, nil)
	require.Equal(t, http.StatusFound, resp.StatusCode, readBody(t, resp))
	resp = b.get(t, resp.Header.Get("Location")) // provider callback
	require.Equal(t, http.StatusFound, resp.StatusCode, readBody(t, resp))
	cb := resp.Header.Get("Location")
	require.True(t, strings.HasPrefix(cb, issuer+oidc.PadCallbackPath), cb)
	return b.get(t, cb)
}

func (b *browser) me(t *testing.T) padlogin.Me {
	t.Helper()
	resp := b.do(t, http.MethodGet, padlogin.MePath, nil, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "no-store", resp.Header.Get("Cache-Control"))
	var me padlogin.Me
	require.NoError(t, jsonDecode(resp.Body, &me))
	return me
}

func TestPadLogin(t *testing.T) {
	e := newEnv(t)
	anna, err := e.accounts.CreateAccount(accounts.System, accounts.NewAccount{Username: "anna", DisplayName: "Anna B.", Kind: accounts.KindHuman, Password: "anna-password"})
	require.NoError(t, err)
	require.NoError(t, e.accounts.Assign(accounts.System, anna.ID, accounts.RoleEditor, "team/*"))
	// Only the own roles count here, not those of everyone logged in.
	require.NoError(t, e.accounts.Unassign(accounts.System, accounts.AuthenticatedID, accounts.RoleEditor, ""))
	b := e.browser()

	// a pad visitors may not open sends a browser to the login…
	resp := b.get(t, "/p/team%2Fnotes?x=1")
	require.Equal(t, http.StatusFound, resp.StatusCode)
	assert.Equal(t, padlogin.LoginURL("/p/team%2Fnotes?x=1"), resp.Header.Get("Location"))
	// behind a proxy that serves Etherpad under a prefix, under it
	resp = b.do(t, http.MethodGet, "/p/team%2Fnotes", nil, map[string]string{"Accept": "text/html", "X-Proxy-Path": "/sub"})
	require.Equal(t, http.StatusFound, resp.StatusCode)
	assert.Equal(t, "/sub"+padlogin.LoginURL("/sub/p/team%2Fnotes"), resp.Header.Get("Location"))
	// …and tools to basic auth
	resp = b.do(t, http.MethodGet, "/p/team%2Fnotes", nil, map[string]string{"Accept": "application/json"})
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.NotEmpty(t, resp.Header.Get("WWW-Authenticate"))

	// the login works also when visitors may do nothing
	assert.Equal(t, padlogin.Me{LoginAvailable: true}, b.me(t))

	b.get(t, padlogin.LoginURL("/p/x")) // a session from before the login
	before := b.cookies[padlogin.CookieName(e.cfg)]
	require.NotEmpty(t, before)
	resp = b.logIn(t, "/p/team%2Fnotes?x=1", "anna", "anna-password")
	require.Equal(t, http.StatusFound, resp.StatusCode, readBody(t, resp))
	assert.Equal(t, "/p/team%2Fnotes?x=1", resp.Header.Get("Location"))
	cookie := resp.Header.Get("Set-Cookie")
	assert.Contains(t, strings.ToLower(cookie), "httponly")
	after := b.cookies[padlogin.CookieName(e.cfg)]
	assert.NotEqual(t, before, after, "a new session id after the login")

	resp = b.get(t, "/p/team%2Fnotes")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.NotNil(t, e.seen)
	assert.Equal(t, anna.ID, e.seen.AccountID)
	assert.Equal(t, padlogin.Me{LoginAvailable: true, LoggedIn: true, Username: "anna", DisplayName: "Anna B."}, b.me(t))
	// only where the roles say
	resp = b.get(t, "/p/elsewhere")
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)

	// the session from before the login is not logged in
	old := e.browser()
	old.cookies[padlogin.CookieName(e.cfg)] = before
	assert.False(t, old.me(t).LoggedIn)

	// logging out needs a request from this site
	resp = b.do(t, http.MethodPost, padlogin.LogoutPath, url.Values{"redirect": {"/p/team%2Fnotes"}}, nil)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	resp = b.do(t, http.MethodPost, padlogin.LogoutPath, url.Values{"redirect": {"/p/team%2Fnotes"}}, map[string]string{"Origin": "http://evil.test"})
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	resp = b.do(t, http.MethodPost, padlogin.LogoutPath, nil, map[string]string{"Sec-Fetch-Site": "cross-site", "Origin": issuer})
	assert.Equal(t, http.StatusForbidden, resp.StatusCode, "the browser's word counts")
	resp = b.do(t, http.MethodPost, padlogin.LogoutPath, nil, map[string]string{"Origin": "null"})
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	assert.True(t, b.me(t).LoggedIn)
	// a pad sends no referrer: Origin "null", but the browser says same-origin
	resp = b.do(t, http.MethodPost, padlogin.LogoutPath, url.Values{"redirect": {"/p/team%2Fnotes"}}, map[string]string{"Origin": "null", "Sec-Fetch-Site": "same-origin"})
	require.Equal(t, http.StatusSeeOther, resp.StatusCode)
	assert.Equal(t, "/p/team%2Fnotes", resp.Header.Get("Location"))
	assert.False(t, b.me(t).LoggedIn)
	stolen := e.browser()
	stolen.cookies[padlogin.CookieName(e.cfg)] = after
	assert.False(t, stolen.me(t).LoggedIn, "the logged-out session is gone on the server")
	assert.Equal(t, http.StatusFound, b.get(t, "/p/team%2Fnotes").StatusCode)

	// a disabled account is logged out at once
	resp = b.logIn(t, "/", "anna", "anna-password")
	require.Equal(t, http.StatusFound, resp.StatusCode)
	require.True(t, b.me(t).LoggedIn)
	require.NoError(t, e.accounts.SetDisabled(accounts.System, anna.ID, true))
	assert.False(t, b.me(t).LoggedIn)
	assert.Equal(t, http.StatusFound, b.get(t, "/p/team%2Fnotes").StatusCode)
}

func TestPadLoginCallbackChecks(t *testing.T) {
	e := newEnv(t)
	_, err := e.accounts.CreateAccount(accounts.System, accounts.NewAccount{Username: "anna", Kind: accounts.KindHuman, Password: "anna-password"})
	require.NoError(t, err)
	b := e.browser()

	// a callback without a started login
	resp := b.get(t, oidc.PadCallbackPath+"?code=x&state=y")
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	// a code meant for another browser's login
	resp = b.get(t, padlogin.LoginURL("/"))
	require.Equal(t, http.StatusFound, resp.StatusCode)
	resp = b.get(t, oidc.PadCallbackPath+"?code=x&state=wrong")
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	// a callback is answered once
	victim := e.browser()
	resp = victim.logIn(t, "/", "anna", "anna-password")
	require.Equal(t, http.StatusFound, resp.StatusCode)
	assert.True(t, victim.me(t).LoggedIn)

	// cancelled at the provider: back without a login
	resp = b.get(t, padlogin.LoginURL("/p/x"))
	target, err := url.Parse(resp.Header.Get("Location"))
	require.NoError(t, err)
	resp = b.get(t, oidc.PadCallbackPath+"?error=access_denied&state="+url.QueryEscape(target.Query().Get("state")))
	assert.Equal(t, http.StatusFound, resp.StatusCode)
	assert.Equal(t, "/p/x", resp.Header.Get("Location"))
	assert.False(t, b.me(t).LoggedIn)
}

func TestSafeRedirect(t *testing.T) {
	for in, want := range map[string]string{
		"/p/a?b=1":            "/p/a?b=1",
		"":                    "/",
		"//evil.test/x":       "/",
		"/\\evil.test":        "/",
		"https://evil.test/":  "/",
		"p/relative":          "/",
		"/p/a\r\nSet-Cookie:": "/",
		"javascript:alert(1)": "/",
	} {
		assert.Equal(t, want, padlogin.SafeRedirect(in), in)
	}
}

func TestLoginOffWithoutUserClient(t *testing.T) {
	cfg := &settings.Settings{SSO: &settings.SSO{Issuer: issuer, Clients: []settings.SSOClient{
		{Type: "admin", ClientId: "admin_client", RedirectUris: []string{issuer + "/admin/"}, GrantTypes: []string{"authorization_code"}},
	}}}
	store := db.NewMemoryDataStore()
	login := padlogin.New(store, oidc.NewAuthenticator(cfg, store), cfg, zap.NewNop().Sugar())
	assert.False(t, login.Available())
	app := fiber.New()
	login.Register(app)
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, padlogin.LoginURL("/"), nil))
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func jsonDecode(r io.Reader, v any) error { return json.NewDecoder(r).Decode(v) }

// slowRunners: logins hash passwords (bcrypt), which takes longer than
// fiber's default second on slow CI machines (arm64).
var slowRunners = fiber.TestConfig{Timeout: 15 * time.Second}

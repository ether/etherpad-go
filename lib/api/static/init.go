package static

import (
	"embed"
	"encoding/json"
	"fmt"
	"github.com/ether/etherpad-go/assets/head"
	padManager "github.com/ether/etherpad-go/lib/pad"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/a-h/templ"
	"github.com/ether/etherpad-go/assets/welcome"
	"github.com/ether/etherpad-go/lib"
	pad2 "github.com/ether/etherpad-go/lib/api/pad"
	"github.com/ether/etherpad-go/lib/hooks"
	"github.com/ether/etherpad-go/lib/plugins"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/ether/etherpad-go/lib/socialmeta"
	"github.com/ether/etherpad-go/lib/timeslider"
	"github.com/ether/etherpad-go/lib/utils"
	"github.com/evanw/esbuild/pkg/api"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/adaptor"
)

// buildHomeSocialMeta renders the Open Graph + Twitter Card meta block for
// the homepage. Upstream #7635.
func buildHomeSocialMeta(c fiber.Ctx, s *settings.Settings, proxyPath string) string {
	scheme := "http"
	if c.Scheme() == "https" {
		scheme = "https"
	}
	favicon := ""
	if s.Favicon != nil {
		favicon = *s.Favicon
	}
	availableLangsSet := make(map[string]struct{}, len(hooks.AvailableLangs))
	for k := range hooks.AvailableLangs {
		availableLangsSet[k] = struct{}{}
	}
	return socialmeta.Render(socialmeta.Opts{
		Req: socialmeta.RequestInfo{
			Scheme:         scheme,
			Host:           c.Host(),
			Path:           c.Path(),
			AcceptLanguage: c.Get("Accept-Language"),
			ProxyPath:      proxyPath,
		},
		Settings: socialmeta.Settings{
			Title:               s.Title,
			Favicon:             favicon,
			PublicURL:           s.PublicURL,
			DescriptionOverride: s.SocialMeta.Description,
		},
		AvailableLangs: availableLangsSet,
		Locales:        hooks.AllLocales,
		Kind:           socialmeta.KindHome,
	})
}

// servePluginStatic answers /static/plugins/<plugin>/static/<file>; anything
// else, and a missing file, is a plain 404 that names no path.
func servePluginStatic(c fiber.Ctx) error {
	filePath, ok := pluginStaticFile(c.Params("*"))
	if !ok {
		return c.SendStatus(fiber.StatusNotFound)
	}
	if err := c.SendFile(filePath); err != nil {
		return c.SendStatus(fiber.StatusNotFound)
	}
	return nil
}

var pluginNamePattern = regexp.MustCompile(`^ep_[A-Za-z0-9_-]+$`)

// adminLocaleName is a locale file of the admin ("de.json", "zh-hant.json").
// The name ends up in a file path in dev mode, where a backslash is a
// separator on Windows ("..\\..\\settings.json"), so only these characters
// pass (etherpad-lite's sanitizePathname guards the same).
var adminLocaleName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*\.json$`)

// pluginStaticFile maps "<plugin>/static/<file>" to its path below
// ./plugins, or reports false for anything else (other folders of the
// plugin, "..", absolute or empty paths).
func pluginStaticFile(rel string) (string, bool) {
	plugin, rest, ok := strings.Cut(rel, "/")
	if !ok || !pluginNamePattern.MatchString(plugin) {
		return "", false
	}
	rest, ok = strings.CutPrefix(rest, "static/")
	if !ok || rest == "" || strings.Contains(rest, "\\") {
		return "", false
	}
	for _, segment := range strings.Split(rest, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return "", false
		}
	}
	return "./plugins/" + plugin + "/static/" + rest, true
}

func registerEmbeddedStatic(app *fiber.App, route string, subPath string, uiAssets embed.FS) {
	prefix := strings.TrimSuffix(route, "/")
	sub, err := fs.Sub(uiAssets, subPath)
	if err != nil {
		panic(err)
	}
	handler := http.StripPrefix(prefix+"/", http.FileServer(http.FS(sub)))
	// Matcht /css/* etc.
	app.Get(prefix+"/*", adaptor.HTTPHandler(handler))
}

func shouldServeAdminSPA(requestPath string) bool {
	switch {
	case requestPath == "/admin/ws":
		return false
	case requestPath == "/admin/validate":
		return false
	case requestPath == "/admin/config":
		return false
	case strings.HasPrefix(requestPath, "/admin/api"):
		return false
	case strings.HasPrefix(requestPath, "/admin/assets"):
		return false
	case strings.HasPrefix(requestPath, "/admin/locales/"):
		return false
	case strings.HasPrefix(requestPath, "/admin/ep_admin_pads"):
		return false
	default:
		return true
	}
}

var loaderConfig = map[string]api.Loader{".css": api.LoaderCSS, ".svg": api.LoaderDataURL}

// externalFonts keeps font files out of bundled CSS. The query strings on the
// icon font URLs (for example, "fontawesome-etherpad.woff?2") are part of the
// path seen by esbuild, so this cannot be expressed reliably with a simple
// external glob.
var externalFonts = api.Plugin{
	Name: "external-fonts",
	Setup: func(build api.PluginBuild) {
		build.OnResolve(api.OnResolveOptions{
			Filter: `\.(ttf|otf|woff2?|eot)(\?.*)?$`,
		}, func(args api.OnResolveArgs) (api.OnResolveResult, error) {
			return api.OnResolveResult{Path: args.Path, External: true}, nil
		})
	},
}

func isDevEnabled(retrievedSettings *settings.Settings) bool {
	if retrievedSettings != nil {
		return retrievedSettings.DevMode
	}
	return utils.IsDevModeEnabled()
}

// devCSSBundle is one stylesheet the dev mode bundles: entry (relative to
// assets/), the output file and the URL directory it is served from.
type devCSSBundle struct {
	entry, out, servedAt string
	extra                api.BuildOptions
}

var devCSSBundles = []devCSSBundle{
	{entry: "./css/skin/colibris/pad.css", out: "css/build/skin/colibris/pad.css", servedAt: "/css/skin/colibris"},
	{entry: "./css/static/pad.css", out: "css/build/static/pad.css", servedAt: "/css/static",
		extra: api.BuildOptions{AssetNames: "fonts/[name]-[hash]", PublicPath: "/font"}},
}

// fontURL is an external font URL as externalFonts leaves it: relative to
// the stylesheet that names it, not to the bundle.
var fontURL = regexp.MustCompile(`url\((["']?)(?:\.\.?/)*(font/[^"')]*)(["']?)\)`)

// rebaseFontURLs points the font URLs of a bundle served from servedAt at
// assets/font (served at /font), relative, so a reverse proxy's path
// prefix stays (as ui/css-build.js's rebaseUrls does for the build).
func rebaseFontURLs(css []byte, servedAt string) []byte {
	up := strings.Repeat("../", strings.Count(strings.Trim(servedAt, "/"), "/")+1)
	return fontURL.ReplaceAll(css, []byte("url(${1}"+up+"${2}${3})"))
}

// buildDevCSS bundles b from the assets under root and returns the CSS.
func buildDevCSS(root string, b devCSSBundle) ([]byte, error) {
	pathToBuild, err := filepath.Abs(path.Join(root, "assets"))
	if err != nil {
		return nil, err
	}
	opts := b.extra
	opts.EntryPoints = []string{b.entry}
	opts.AbsWorkingDir = pathToBuild
	opts.Bundle = true
	opts.Write = false
	opts.Outdir = path.Dir(path.Join(pathToBuild, b.out))
	opts.LogLevel = api.LogLevelWarning
	opts.Loader = loaderConfig
	opts.Plugins = []api.Plugin{externalFonts}
	result := api.Build(opts)
	if len(result.Errors) > 0 {
		return nil, fmt.Errorf("building %s: %s", b.entry, result.Errors[0].Text)
	}
	for _, f := range result.OutputFiles {
		if strings.HasSuffix(f.Path, ".css") {
			return rebaseFontURLs(f.Contents, b.servedAt), nil
		}
	}
	return nil, fmt.Errorf("building %s: no CSS output", b.entry)
}

func buildCssInDev(retrievedSettings *settings.Settings) {
	if !isDevEnabled(retrievedSettings) {
		return
	}

	for _, b := range devCSSBundles {
		css, err := buildDevCSS(retrievedSettings.Root, b)
		if err != nil {
			panic(err.Error())
		}
		out := path.Join(retrievedSettings.Root, "assets", b.out)
		if err := os.MkdirAll(path.Dir(out), 0o755); err != nil {
			panic(err.Error())
		}
		if err := os.WriteFile(out, css, 0o644); err != nil {
			panic(err.Error())
		}
	}
}

func Init(store *lib.InitStore) {
	buildCssInDev(store.RetrievedSettings)
	var devHMR *esbuildDevHMR
	if isDevEnabled(store.RetrievedSettings) {
		var err error
		devHMR, err = startEsbuildDevHMR(store)
		if err != nil {
			store.Logger.Warnf("Could not initialize esbuild dev HMR: %v", err)
		}
	}

	store.C.Use("/p/", func(c fiber.Ctx) error {
		c.Path()

		if store.CookieStore != nil {
			var _, err = store.CookieStore.Get(c)
			if err != nil {
				store.Logger.Warnf("Error loading session cookie: %v", err)
			}
		}

		return c.Next()
	})

	store.C.Get("/pluginfw/plugin-definitions.json", func(ctx fiber.Ctx) error {
		return plugins.ReturnPluginResponse(ctx)
	})
	store.C.Post("/jserror", func(ctx fiber.Ctx) error {
		store.Logger.Warnf("Frontend error report: %s", clientReport(ctx.Body()))
		return ctx.SendStatus(fiber.StatusNoContent)
	})
	// The pad posts what it knows about a lost connection (etherpad-lite's
	// apicalls): logged, nothing else.
	store.C.Post("/ep/pad/connection-diagnostic-info", func(ctx fiber.Ctx) error {
		var body struct {
			DiagnosticInfo json.RawMessage `json:"diagnosticInfo"`
		}
		if json.Unmarshal(ctx.Body(), &body) != nil || len(body.DiagnosticInfo) == 0 || body.DiagnosticInfo[0] != '{' {
			store.Logger.Warn("DIAGNOSTIC-INFO: No diagnostic info provided")
			return ctx.Status(fiber.StatusBadRequest).SendString("No diagnostic info provided")
		}
		store.Logger.Infof("DIAGNOSTIC-INFO: %s", clientReport(body.DiagnosticInfo))
		return ctx.SendString("OK")
	})

	// Plugins' own static files, as in etherpad-lite: only what a plugin
	// keeps in its static/ folder, never its sources or configuration. The
	// route needs no login (public assets), so nothing outside is reachable.
	store.C.Get("/static/plugins/*", servePluginStatic)

	// Serve the React admin SPA index.html from the embedded build output
	adminIndexHTML, err := store.UiAssets.ReadFile("assets/js/admin/index.html")
	if err != nil {
		store.Logger.Errorf("Error reading admin index.html: %v", err)
	} else {
		adminHTML := string(adminIndexHTML)
		store.C.Get("/admin/index.html", func(c fiber.Ctx) error {
			return c.Type("html").SendString(adminPage(c, adminHTML, store.RetrievedSettings.TrustProxy))
		})
		store.C.Get("/admin/", func(c fiber.Ctx) error {
			return c.Type("html").SendString(adminPage(c, adminHTML, store.RetrievedSettings.TrustProxy))
		})
	}

	// Serve OIDC config for the admin SPA (public, no auth needed)
	store.C.Get("/admin/config", func(c fiber.Ctx) error {
		sso := store.RetrievedSettings.SSO
		if sso == nil {
			return c.JSON(map[string]any{"oidc": nil})
		}
		for _, client := range sso.Clients {
			if client.Type == "admin" {
				if sso.Issuer == "" || len(client.RedirectUris) == 0 {
					break
				}
				return c.JSON(map[string]any{
					"oidc": map[string]any{
						"authority":   sso.Issuer,
						"clientId":    client.ClientId,
						"redirectUri": client.RedirectUris[0],
						"scope":       "openid profile email offline_access",
					},
				})
			}
		}
		return c.JSON(map[string]any{"oidc": nil})
	})

	store.C.Get("/admin/locales/:file", func(c fiber.Ctx) error {
		if !adminLocaleName.MatchString(c.Params("file")) {
			return c.SendStatus(fiber.StatusNotFound)
		}
		err := serveAdminAsset(c, store.UiAssets, store.RetrievedSettings, path.Join("locales", c.Params("file")), "application/json; charset=utf-8")
		if err != nil {
			store.Logger.Errorf("Error serving admin locale: %v", err)
			return c.SendStatus(fiber.StatusNotFound)
		}
		return nil
	})

	store.C.Get("/css/static/pad.css", func(ctx fiber.Ctx) error {
		if isDevEnabled(store.RetrievedSettings) {
			fileContent, err := os.ReadFile("assets/css/build/static/pad.css")
			if err != nil {
				store.Logger.Errorf("Error setting up build page: %v. Did you forget to run the build script in ui directory?", err)
			}
			return ctx.Type("css").Send(fileContent)
		} else {
			fileContent, err := store.UiAssets.ReadFile("assets/css/build/static/pad.css")
			if err != nil {
				store.Logger.Errorf("Error setting up build page: %v. Did you forget to run the build script in ui directory?", err)
			}
			return ctx.Type("css").Send(fileContent)
		}
	})

	store.C.Get("/css/skin/colibris/pad.css", func(ctx fiber.Ctx) error {
		if isDevEnabled(store.RetrievedSettings) {
			fileContent, err := os.ReadFile("assets/css/build/skin/colibris/pad.css")
			if err != nil {
				store.Logger.Errorf("Error setting up build page: %v. Did you forget to run the build script in ui directory?", err)
			}
			return ctx.Type("css").Send(fileContent)
		} else {
			fileContent, err := store.UiAssets.ReadFile("assets/css/build/skin/colibris/pad.css")
			if err != nil {
				store.Logger.Errorf("Error setting up build page: %v. Did you forget to run the build script in ui directory?", err)
			}
			return ctx.Type("css").Send(fileContent)
		}
	})

	registerEmbeddedStatic(store.C, "/images/", "assets/images", store.UiAssets)
	registerEmbeddedStatic(store.C, "/admin/assets/", "assets/js/admin/assets", store.UiAssets)
	registerEmbeddedStatic(store.C, "/admin/static/", "assets/js/admin/static", store.UiAssets)
	registerEmbeddedStatic(store.C, "/images/favicon.ico", "assets/images/favicon.ico", store.UiAssets)
	registerEmbeddedStatic(store.C, "/css/", "assets/css", store.UiAssets)
	registerEmbeddedStatic(store.C, "/static/css/", "assets/css/static", store.UiAssets)
	registerEmbeddedStatic(store.C, "/static/skins/colibris/", "assets/css/skin", store.UiAssets)
	registerEmbeddedStatic(store.C, "/html/", "assets/html", store.UiAssets)
	registerEmbeddedStatic(store.C, "/font/", "assets/font", store.UiAssets)
	registerEmbeddedStatic(store.C, "/admin/ep_admin_pads/", "assets/locales/ep_admin_pads", store.UiAssets)

	store.C.Get("/p/:pad", func(ctx fiber.Ctx) error {
		if handled, err := pad2.GuardPadURL(ctx, store.PadManager); handled {
			return err
		}
		padManager.EnsureAuthorTokenCookie(ctx, store.RetrievedSettings)
		return pad2.HandlePadOpen(ctx, store.UiAssets, store.RetrievedSettings, store.Hooks)
	})

	store.C.Get("/s/:pad", func(ctx fiber.Ctx) error {
		if handled, err := pad2.RedirectTrailingSlash(ctx); handled {
			return err
		}
		padManager.EnsureAuthorTokenCookie(ctx, store.RetrievedSettings)
		return pad2.HandleSheetOpen(ctx)
	})

	store.C.Get("/p/:pad/qr", func(ctx fiber.Ctx) error {
		return pad2.HandlePadQr(ctx, store)
	})

	store.C.Get("/p/:pad/timeslider", func(c fiber.Ctx) error {
		if handled, err := pad2.GuardPadURL(c, store.PadManager); handled {
			return err
		}
		padManager.EnsureAuthorTokenCookie(c, store.RetrievedSettings)
		return timeslider.HandleTimesliderOpen(c, store.UiAssets, store.RetrievedSettings, store.Hooks)
	})

	// Like etherpad-lite's default robots.txt: crawlers stay out of pads.
	store.C.Get("/robots.txt", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderContentType, fiber.MIMETextPlainCharsetUTF8)
		return c.SendString("User-agent: *\nDisallow: /p/\n")
	})

	store.C.Get("/favicon.ico", serveFavicon(store))
	store.C.Get("/manifest.json", serveManifest(store.RetrievedSettings))
	store.C.Get("/javascript", serveJavascriptPage(store.UiAssets))

	store.C.Get("/", func(c fiber.Ctx) error {
		var language = c.Cookies("language", "en")
		var keyValues, err = utils.LoadTranslations(language, store.UiAssets, store.Hooks)
		if err != nil {
			return err
		}
		// The home page is "/" or "<prefix>/", so its links are absolute
		// from the prefix, as in etherpad-lite: relative ones would miss it
		// when the proxy forwards "<prefix>" without the trailing slash.
		proxyPath := utils.RequestProxyPath(c, store.RetrievedSettings.TrustProxy)
		socialMetaHTML := buildHomeSocialMeta(c, store.RetrievedSettings, proxyPath)
		component := welcome.Page(store.RetrievedSettings, keyValues, socialMetaHTML, head.For(store.RetrievedSettings, language, proxyPath+"/", false))
		return adaptor.HTTPHandler(templ.Handler(component))(c)
	})

	if !isDevEnabled(store.RetrievedSettings) {
		registerEmbeddedStatic(store.C, "/js/pad/assets/", "assets/js/pad/assets", store.UiAssets)
		registerEmbeddedStatic(store.C, "/js/welcome/assets/", "assets/js/welcome/assets", store.UiAssets)
		registerEmbeddedStatic(store.C, "/admin/assets", "assets/js/admin/assets", store.UiAssets)
		registerEmbeddedStatic(store.C, "/js/timeslider/assets/", "assets/js/timeslider/assets", store.UiAssets)
		registerEmbeddedStatic(store.C, "/js/sheet/assets/", "assets/js/sheet/assets", store.UiAssets)
	} else {
		store.C.Get("/js/*", func(c fiber.Ctx) error {
			if devHMR == nil {
				return c.Status(fiber.StatusServiceUnavailable).SendString("Dev bundler unavailable")
			}
			return devHMR.serveBundle(c)
		})
	}

	registerEmbeddedStatic(store.C, "/images", "assets/images", store.UiAssets)
	registerEmbeddedStatic(store.C, "/static/", "assets/html", store.UiAssets)
	registerEmbeddedStatic(store.C, "/pluginfw", "assets/plugin", store.UiAssets)

	if adminIndexHTML != nil {
		adminHTML := string(adminIndexHTML)
		store.C.Get("/admin/*", func(c fiber.Ctx) error {
			if !shouldServeAdminSPA(c.Path()) {
				return c.Next()
			}
			return c.Type("html").SendString(adminPage(c, adminHTML, store.RetrievedSettings.TrustProxy))
		})
	}
}

// adminPage is the admin SPA's index.html for this request. Vite builds it
// with absolute /admin/ and /favicon.ico URLs; behind a reverse proxy
// prefix they get it in front (the prefix is cleaned, it has no quote).
// The SPA itself takes the prefix from its location.
func adminPage(c fiber.Ctx, html string, trustProxy bool) string {
	prefix := utils.RequestProxyPath(c, trustProxy)
	if prefix == "" {
		return html
	}
	return strings.NewReplacer(
		`"/admin/`, `"`+prefix+`/admin/`,
		`"/favicon.ico"`, `"`+prefix+`/favicon.ico"`,
	).Replace(html)
}

// clientReport makes text a browser sent fit for one log line: at most
// 4 KiB, control characters (line breaks that would fake log lines)
// replaced.
func clientReport(b []byte) string {
	const limit = 4096
	truncated := len(b) > limit
	if truncated {
		b = b[:limit]
	}
	s := strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return ' '
		}
		return r
	}, strings.ToValidUTF8(string(b), "?"))
	if truncated {
		s += " …(truncated)"
	}
	return s
}

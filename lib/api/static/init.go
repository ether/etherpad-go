package static

import (
	"embed"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/a-h/templ"
	"github.com/ether/etherpad-go/assets/welcome"
	"github.com/ether/etherpad-go/lib"
	pad2 "github.com/ether/etherpad-go/lib/api/pad"
	"github.com/ether/etherpad-go/lib/plugins"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/ether/etherpad-go/lib/timeslider"
	"github.com/ether/etherpad-go/lib/utils"
	"github.com/evanw/esbuild/pkg/api"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/adaptor"
)

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

var loaderConfig = map[string]api.Loader{".css": api.LoaderCSS, ".svg": api.LoaderDataURL, ".woff2": api.LoaderDataURL, ".woff": api.LoaderDataURL, ".ttf": api.LoaderDataURL, ".eot": api.LoaderDataURL, ".otf": api.LoaderDataURL}

func isDevEnabled(retrievedSettings *settings.Settings) bool {
	if retrievedSettings != nil {
		return retrievedSettings.DevMode
	}
	return utils.IsDevModeEnabled()
}

func buildColibrisCssInDev(retrievedSettings *settings.Settings) {
	pathToBuild := path.Join(retrievedSettings.Root, "assets")
	entryPoints := []string{"./css/skin/colibris/pad.css"}
	outDir := path.Join(pathToBuild, "css", "build", "skin", "colibris")
	for _, ep := range entryPoints {
		result := api.Build(api.BuildOptions{
			EntryPoints:   []string{ep},
			AbsWorkingDir: pathToBuild,
			Bundle:        true,
			Write:         true,
			Outdir:        outDir,
			LogLevel:      api.LogLevelWarning,
			Loader:        loaderConfig,
		})
		if len(result.Errors) > 0 {
			panic("Error building css/static/pad.css")
		}
	}
}

func buildStaticPadCSSInDev(retrievedSettings *settings.Settings) {
	pathToBuild := path.Join(retrievedSettings.Root, "assets")
	entryPoints := []string{"./css/static/pad.css"}
	outDir := path.Join(pathToBuild, "css", "build", "static")

	for _, ep := range entryPoints {
		result := api.Build(api.BuildOptions{
			EntryPoints:   []string{ep},
			AbsWorkingDir: pathToBuild,
			Bundle:        true,
			Write:         true,
			Outdir:        outDir,
			LogLevel:      api.LogLevelWarning,
			AssetNames:    "fonts/[name]-[hash]",
			PublicPath:    "/font",
			Loader:        loaderConfig,
		})
		if len(result.Errors) > 0 {
			panic("Error building css/static/pad.css")
		}
	}
}

func buildCssInDev(retrievedSettings *settings.Settings) {
	if !isDevEnabled(retrievedSettings) {
		return
	}

	buildColibrisCssInDev(retrievedSettings)
	buildStaticPadCSSInDev(retrievedSettings)
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
				println("Error with session")
			}
		}

		return c.Next()
	})

	store.C.Get("/pluginfw/plugin-definitions.json", func(ctx fiber.Ctx) error {
		return plugins.ReturnPluginResponse(ctx)
	})
	store.C.Post("/jserror", func(ctx fiber.Ctx) error {
		store.Logger.Warnf("Frontend error report: %s", string(ctx.Body()))
		return ctx.SendStatus(fiber.StatusNoContent)
	})

	store.C.Get("/static/plugins/*", func(c fiber.Ctx) error {
		filePath := c.Params("*")
		return c.SendFile("./plugins/" + filePath)
	})

	// Serve the React admin SPA index.html from the embedded build output
	adminIndexHTML, err := store.UiAssets.ReadFile("assets/js/admin/index.html")
	if err != nil {
		store.Logger.Errorf("Error reading admin index.html: %v", err)
	} else {
		adminHTML := string(adminIndexHTML)
		store.C.Get("/admin/index.html", func(c fiber.Ctx) error {
			return c.Type("html").SendString(adminHTML)
		})
		store.C.Get("/admin/", func(c fiber.Ctx) error {
			return c.Type("html").SendString(adminHTML)
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
						"scope":       "openid profile email offline",
					},
				})
			}
		}
		return c.JSON(map[string]any{"oidc": nil})
	})

	store.C.Get("/admin/locales/:file", func(c fiber.Ctx) error {
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
		return pad2.HandlePadOpen(ctx, store.UiAssets, store.RetrievedSettings, store.Hooks)
	})

	store.C.Get("/p/:pad/qr", func(ctx fiber.Ctx) error {
		return pad2.HandlePadQr(ctx, store)
	})

	store.C.Get("/p/:pad/timeslider", func(c fiber.Ctx) error {
		return timeslider.HandleTimesliderOpen(c, store.UiAssets, store.RetrievedSettings, store.Hooks)
	})

	store.C.Get("/favicon.ico", func(c fiber.Ctx) error {
		return c.Redirect().Status(fiber.StatusMovedPermanently).To("/images/favicon.ico")
	})

	store.C.Get("/", func(c fiber.Ctx) error {
		var language = c.Cookies("language", "en")
		var keyValues, err = utils.LoadTranslations(language, store.UiAssets, store.Hooks)
		if err != nil {
			return err
		}
		component := welcome.Page(store.RetrievedSettings, keyValues)
		return adaptor.HTTPHandler(templ.Handler(component))(c)
	})

	if !isDevEnabled(store.RetrievedSettings) {
		registerEmbeddedStatic(store.C, "/js/pad/assets/", "assets/js/pad/assets", store.UiAssets)
		registerEmbeddedStatic(store.C, "/js/welcome/assets/", "assets/js/welcome/assets", store.UiAssets)
		registerEmbeddedStatic(store.C, "/admin/assets", "assets/js/admin/assets", store.UiAssets)
		registerEmbeddedStatic(store.C, "/js/timeslider/assets/", "assets/js/timeslider/assets", store.UiAssets)
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
			return c.Type("html").SendString(adminHTML)
		})
	}
}

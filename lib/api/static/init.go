package static

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/a-h/templ"
	"github.com/ether/etherpad-go/assets/welcome"
	"github.com/ether/etherpad-go/lib/pad"
	"github.com/ether/etherpad-go/lib/plugins"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/ether/etherpad-go/lib/utils"
	"github.com/evanw/esbuild/pkg/api"
	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
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

func Init(app *fiber.App, uiAssets embed.FS, settings settings.Settings, cookieStore *session.Store) {
	app.Use("/p/", func(c *fiber.Ctx) error {
		c.Path()

		var _, err = cookieStore.Get(c)
		if err != nil {
			println("Error with session")
		}

		return c.Next()
	})

	app.Get("/pluginfw/plugin-definitions.json", plugins.ReturnPluginResponse)
	registerEmbeddedStatic(app, "/images/favicon.ico", "assets/images/favicon.ico", uiAssets)
	registerEmbeddedStatic(app, "/css/", "assets/css", uiAssets)
	registerEmbeddedStatic(app, "/static/css/", "assets/css/static", uiAssets)
	registerEmbeddedStatic(app, "/static/skins/colibris/", "assets/css/skin", uiAssets)
	registerEmbeddedStatic(app, "/html/", "assets/html", uiAssets)
	registerEmbeddedStatic(app, "/font/", "assets/font", uiAssets)

	app.Get("/p/*", func(ctx *fiber.Ctx) error {
		return pad.HandlePadOpen(ctx, uiAssets)
	})

	app.Get("/favicon.ico", func(c *fiber.Ctx) error {
		return c.Redirect("/images/favicon.ico", fiber.StatusMovedPermanently)
	})

	app.Get("/", func(c *fiber.Ctx) error {
		var language = c.Cookies("language", "en")
		var keyValues, err = utils.LoadTranslations(language, uiAssets)
		if err != nil {
			return err
		}
		component := welcome.Page(settings, keyValues)
		return adaptor.HTTPHandler(templ.Handler(component))(c)
	})

	var nodeEnv = os.Getenv("NODE_ENV")

	if nodeEnv == "production" {
		registerEmbeddedStatic(app, "/js/pad/assets/", "assets/js/pad/assets", uiAssets)
		registerEmbeddedStatic(app, "/js/welcome/assets/", "assets/js/welcome/assets", uiAssets)
	} else {
		app.Get("/js/*", func(c *fiber.Ctx) error {
			var entrypoint string

			if strings.Contains(c.Path(), "welcome") {
				entrypoint = "./src/welcome.js"
			} else {
				entrypoint = "./src/pad.js"
			}

			relativePath := "./src/js"
			var alias = make(map[string]string)
			alias["ep_etherpad-lite/static/js/ace2_inner"] = relativePath + "/ace2_inner"
			alias["ep_etherpad-lite/static/js/ace2_common"] = relativePath + "/ace2_common"
			alias["ep_etherpad-lite/static/js/pluginfw/client_plugins"] = relativePath + "/pluginfw/client_plugins"
			alias["ep_etherpad-lite/static/js/rjquery"] = relativePath + "/rjquery"
			alias["ep_etherpad-lite/static/js/nice-select"] = "ep_etherpad-lite/static/js/vendors/nice-select"

			var pathToBuild = path.Join(*settings.Root, "ui")

			result := api.Build(api.BuildOptions{
				EntryPoints:   []string{entrypoint},
				AbsWorkingDir: pathToBuild,
				Bundle:        true,
				Write:         false,
				LogLevel:      api.LogLevelInfo,
				Metafile:      true,
				Target:        api.ES2020,
				Alias:         alias,
				Sourcemap:     api.SourceMapInline,
			})

			if len(result.Errors) > 0 {
				fmt.Println("Build failed with errors:", result.Errors)
				return c.SendString("Build failed")
			}

			c.Set("Content-Type", "application/javascript")

			return c.Send(result.OutputFiles[0].Contents)
		})
	}

	registerEmbeddedStatic(app, "/images", "assets/images", uiAssets)
	registerEmbeddedStatic(app, "/static/", "assets/html", uiAssets)
	registerEmbeddedStatic(app, "/pluginfw", "assets/plugin", uiAssets)
}

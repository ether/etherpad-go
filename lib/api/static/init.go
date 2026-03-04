package static

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
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
	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
	"golang.org/x/net/html"
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

func getAdminBody(uiAssets embed.FS, retrievedSettings *settings.Settings) (*string, error) {

	calcDataConfig := func() string {
		for _, client := range retrievedSettings.SSO.Clients {
			if client.Type == "admin" {
				selectedClient := client
				oidcConfig := settings.OidcConfig{
					ClientId:    selectedClient.ClientId,
					Authority:   retrievedSettings.SSO.Issuer,
					JwksUri:     retrievedSettings.SSO.Issuer + ".well-known/jwks.json",
					RedirectUri: selectedClient.RedirectUris[0],
					Scope:       []string{"openid", "profile", "email", "offline"},
				}
				conf, _ := json.Marshal(oidcConfig)
				return string(conf)
			}
		}
		return ""
	}

	fileContent, err := uiAssets.ReadFile("assets/js/admin/index.html")
	if err != nil {
		return nil, errors.New("error reading admin page HTML: %v" + err.Error())
	}

	stringContent := string(fileContent)
	node, err := html.Parse(strings.NewReader(stringContent))

	if err != nil {
		return nil, errors.New("Error parsing admin page HTML: " + err.Error())
	}

	spanNode := &html.Node{
		Type: html.ElementNode,
		Data: "span",
		Attr: []html.Attribute{
			{Key: "id", Val: "config"},
			{Key: "data-config", Val: calcDataConfig()},
		},
	}

	var body *html.Node
	var findBody func(*html.Node)
	findBody = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "body" {
			body = n
			return
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			findBody(child)
		}
	}
	findBody(node)

	if body != nil {
		body.AppendChild(spanNode)
	}

	var buf bytes.Buffer
	html.Render(&buf, node)
	result := buf.String()
	return &result, nil
}

var loaderConfig = map[string]api.Loader{".css": api.LoaderCSS, ".svg": api.LoaderDataURL, ".woff2": api.LoaderDataURL, ".woff": api.LoaderDataURL, ".ttf": api.LoaderDataURL, ".eot": api.LoaderDataURL, ".otf": api.LoaderDataURL}

func isDevEnabled(retrievedSettings *settings.Settings) bool {
	return utils.IsDevModeEnabled() || (retrievedSettings != nil && retrievedSettings.DevMode)
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

	store.C.Use("/p/", func(c *fiber.Ctx) error {
		c.Path()

		var _, err = store.CookieStore.Get(c)
		if err != nil {
			println("Error with session")
		}

		return c.Next()
	})

	store.C.Get("/pluginfw/plugin-definitions.json", func(ctx *fiber.Ctx) error {
		return plugins.ReturnPluginResponse(ctx)
	})
	store.C.Post("/jserror", func(ctx *fiber.Ctx) error {
		store.Logger.Warnf("Frontend error report: %s", string(ctx.Body()))
		return ctx.SendStatus(fiber.StatusNoContent)
	})

	store.C.Static("/static/plugins/", "./plugins")

	adminHtml, err := getAdminBody(store.UiAssets, store.RetrievedSettings)

	if err != nil {
		store.Logger.Errorf("Error setting up admin page: %v", err)
	} else {
		store.C.Get("/admin/index.html", func(c *fiber.Ctx) error {
			return c.Type("html").SendString(*adminHtml)
		})
		store.C.Get("/admin/", func(c *fiber.Ctx) error {
			return c.Type("html").SendString(*adminHtml)
		})
	}

	store.C.Get("/css/static/pad.css", func(ctx *fiber.Ctx) error {
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

	store.C.Get("/css/skin/colibris/pad.css", func(ctx *fiber.Ctx) error {
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

	store.C.Get("/p/:pad", func(ctx *fiber.Ctx) error {
		return pad2.HandlePadOpen(ctx, store.UiAssets, store.RetrievedSettings, store.Hooks)
	})

	store.C.Get("/p/:pad/timeslider", func(c *fiber.Ctx) error {
		return timeslider.HandleTimesliderOpen(c, store.UiAssets, store.RetrievedSettings, store.Hooks)
	})

	store.C.Get("/favicon.ico", func(c *fiber.Ctx) error {
		return c.Redirect("/images/favicon.ico", fiber.StatusMovedPermanently)
	})

	store.C.Get("/", func(c *fiber.Ctx) error {
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
		store.C.Get("/js/*", func(c *fiber.Ctx) error {
			if devHMR == nil {
				return c.Status(fiber.StatusServiceUnavailable).SendString("Dev bundler unavailable")
			}
			return devHMR.serveBundle(c)
		})
	}

	registerEmbeddedStatic(store.C, "/images", "assets/images", store.UiAssets)
	registerEmbeddedStatic(store.C, "/static/", "assets/html", store.UiAssets)
	registerEmbeddedStatic(store.C, "/pluginfw", "assets/plugin", store.UiAssets)
}

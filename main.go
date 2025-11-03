package main

import (
	"context"
	"embed"
	"fmt"
	_ "fmt"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/a-h/templ"
	"github.com/ether/etherpad-go/assets/welcome"
	_ "github.com/ether/etherpad-go/docs"
	api2 "github.com/ether/etherpad-go/lib/api"
	"github.com/ether/etherpad-go/lib/hooks"
	"github.com/ether/etherpad-go/lib/pad"
	"github.com/ether/etherpad-go/lib/plugins"
	session2 "github.com/ether/etherpad-go/lib/session"
	settings2 "github.com/ether/etherpad-go/lib/settings"
	"github.com/ether/etherpad-go/lib/ws"
	"github.com/evanw/esbuild/pkg/api"
	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/fiber/v2/middleware/session"
	"github.com/gofiber/swagger"
	"github.com/gorilla/sessions"
	sio "github.com/njones/socketio"
	ser "github.com/njones/socketio/serialize"
)

var store *sessions.CookieStore

func sessionMiddleware(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionFromCookie, err := store.Get(r, "express_sid")
		if err != nil {
			println("Error getting sessionFromCookie", err)
			http.SetCookie(w, &http.Cookie{Name: "express_sid", MaxAge: -1, Path: "/"})
			return
		}

		if sessionFromCookie.IsNew {
			http.SetCookie(w, &http.Cookie{Name: "express_sid", MaxAge: -1, Path: "/"})
			err := sessionFromCookie.Save(r, w)
			if err != nil {
				println("Error saving sessionFromCookie", err)
				return
			}
		}

		r = r.WithContext(context.WithValue(r.Context(), "sessionFromCookie", sessionFromCookie))
		h(w, r)
	}
}

//go:embed assets
var uiAssets embed.FS

// Hilfsfunktion: registriert eine Fiber-Route, die Dateien aus dem eingebetteten
// Unterverzeichnis `subPath` (z.B. `assets/css`) unter `route` (z.B. /css/) ausliefert.
func registerEmbeddedStatic(app *fiber.App, route string, subPath string) {
	prefix := strings.TrimSuffix(route, "/")
	println(prefix)
	sub, err := fs.Sub(uiAssets, subPath)
	if err != nil {
		panic(err)
	}
	handler := http.StripPrefix(prefix+"/", http.FileServer(http.FS(sub)))
	// Matcht /css/* etc.
	app.Get(prefix+"/*", adaptor.HTTPHandler(handler))
}

// @title Fiber Example API
// @version 1.0
// @description This is a sample swagger for Fiber
// @termsOfService http://swagger.io/terms/
// @contact.name API Support
// @contact.email fiber@swagger.io
// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html
// @host localhost:3000
// @BasePath /
func main() {

	var settings = settings2.Displayed

	var db = session2.NewSessionDatabase(nil)
	app := fiber.New()
	var cookieStore = session.New(session.Config{
		KeyLookup: "cookie:express_sid",
		Storage:   db,
	})
	server := sio.NewServer()
	component := welcome.Page(settings)

	app.Get("/swagger/*", swagger.HandlerDefault) // default

	app.Get("/swagger/*", swagger.New(swagger.Config{ // custom
		URL:         "http://example.com/doc.json",
		DeepLinking: false,
		// Expand ("list") or Collapse ("none") tag groups by default
		DocExpansion: "none",
		// Prefill OAuth ClientId on Authorize popup
		OAuth: &swagger.OAuthConfig{
			AppName:  "OAuth Provider",
			ClientId: "21bb4edc-05a7-4afc-86f1-2e151e4ba6e2",
		},
		// Ability to change OAuth2 redirect uri location
		OAuth2RedirectUrl: "http://localhost:8080/swagger/oauth2-redirect.html",
	}))

	app.Use(func(c *fiber.Ctx) error {
		return pad.CheckAccess(c)
	})
	app.Get("/swagger/*", swagger.HandlerDefault)

	registerEmbeddedStatic(app, "/css/", "assets/css")
	registerEmbeddedStatic(app, "/static/css/", "assets/css/static")
	registerEmbeddedStatic(app, "/static/skins/colibris/", "assets/css/skin")
	registerEmbeddedStatic(app, "/html/", "assets/html")
	registerEmbeddedStatic(app, "/font/", "assets/font")

	relativePath := "./src/js"

	var alias = make(map[string]string)
	alias["ep_etherpad-lite/static/js/ace2_inner"] = relativePath + "/ace2_inner"
	alias["ep_etherpad-lite/static/js/ace2_common"] = relativePath + "/ace2_common"
	alias["ep_etherpad-lite/static/js/pluginfw/client_plugins"] = relativePath + "/pluginfw/client_plugins"
	alias["ep_etherpad-lite/static/js/rjquery"] = relativePath + "/rjquery"
	alias["ep_etherpad-lite/static/js/nice-select"] = "ep_etherpad-lite/static/js/vendors/nice-select"

	app.Get("/js/*", func(c *fiber.Ctx) error {
		println("Calling js", c.Path())
		var entrypoint string

		var nodeEnv = os.Getenv("NODE_ENV")

		if nodeEnv == "production" {
			if strings.Contains(c.Path(), "welcome") {
				entriesForWelcomePage, err := uiAssets.ReadDir("js/welcome/assets")
				if err != nil {
					panic("Could not find static js files in production mode" + err.Error())
				}
				return filesystem.SendFile(c, http.FS(uiAssets), "js/welcome/assets"+entriesForWelcomePage[0].Name())
			} else {
				entriesForPadPage, err := uiAssets.ReadDir("js/pad/assets")
				if err != nil {
					panic("Could not find static js files in production mode" + err.Error())
				}
				return filesystem.SendFile(c, http.FS(uiAssets), "js/welcome/assets"+entriesForPadPage[0].Name())
			}
		}

		if strings.Contains(c.Path(), "welcome") {
			entrypoint = "./src/welcome.js"
		} else {
			entrypoint = "./src/pad.js"
		}

		var pathToBuild = path.Join(*settings2.Displayed.Root, "ui")

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
	registerEmbeddedStatic(app, "/images", "assets/images")
	registerEmbeddedStatic(app, "/static/empty.html", "assets/html/empty.html")
	registerEmbeddedStatic(app, "/pluginfw/plugin-definitions.json", "assets/plugin/plugin-definitions.json")

	app.Use("/p/", func(c *fiber.Ctx) error {
		c.Path()

		var _, err = cookieStore.Get(c)
		if err != nil {
			println("Error with session")
		}

		return c.Next()
	})

	app.Get("/pluginfw/plugin-definitions.json", plugins.ReturnPluginResponse)
	registerEmbeddedStatic(app, "/images/favicon.ico", "assets/images/favicon.ico")
	app.Get("/p/*", pad.HandlePadOpen)

	app.Get("/favicon.ico", func(c *fiber.Ctx) error {
		return c.Redirect("/images/favicon.ico", fiber.StatusMovedPermanently)
	})

	app.Get("/", func(c *fiber.Ctx) error {
		return adaptor.HTTPHandler(templ.Handler(component))(c)
	})

	hooks.ExpressPreSession(app, uiAssets)

	ws.HubGlob = ws.NewHub()
	go ws.HubGlob.Run()
	app.Get("/socket.io/*", func(c *fiber.Ctx) error {
		return adaptor.HTTPHandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			ws.ServeWs(ws.HubGlob, writer, request, cookieStore, c)
		})(c)
	})

	// use a OnConnect handler for incoming "connection" messages
	server.OnConnect(func(socket *sio.SocketV4) error {
		println("connected")
		canYouHear := ser.String("can you hear me?")
		extra := ser.String("abc")

		var questions = ser.Integer(1)
		var responses = ser.Map(map[string]interface{}{"one": "no"})

		// send out a message to the hello
		err := socket.Emit("hello", canYouHear, questions, responses, extra)
		if err != nil {
			return err
		}

		return nil
	})

	api2.InitAPI(app)

	err := app.Listen(":3000")
	if err != nil {
		return
	}

}

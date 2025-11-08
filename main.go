package main

import (
	"context"
	"embed"
	"fmt"
	_ "fmt"
	"net/http"

	_ "github.com/ether/etherpad-go/docs"
	api2 "github.com/ether/etherpad-go/lib/api"
	"github.com/ether/etherpad-go/lib/hooks"
	"github.com/ether/etherpad-go/lib/pad"
	session2 "github.com/ether/etherpad-go/lib/session"
	settings2 "github.com/ether/etherpad-go/lib/settings"
	"github.com/ether/etherpad-go/lib/utils"
	"github.com/ether/etherpad-go/lib/ws"
	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
	"github.com/gorilla/sessions"
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
	setupLogger := utils.SetupLogger()
	defer setupLogger.Sync()
	var settings = settings2.Displayed

	setupLogger.Info("Starting Etherpad Go...")

	var db = session2.NewSessionDatabase(nil)
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	app.Use(func(c *fiber.Ctx) error {
		return pad.CheckAccess(c)
	})

	var cookieStore = session.New(session.Config{
		KeyLookup: "cookie:express_sid",
		Storage:   db,
	})

	hooks.ExpressPreSession(app, uiAssets)
	ws.HubGlob = ws.NewHub()
	go ws.HubGlob.Run()
	app.Get("/socket.io/*", func(c *fiber.Ctx) error {
		return adaptor.HTTPHandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			ws.ServeWs(ws.HubGlob, writer, request, cookieStore, c, &settings)
		})(c)
	})
	api2.InitAPI(app, uiAssets, settings, cookieStore)

	fiberString := fmt.Sprintf("%s:%s", settings.IP, settings.Port)
	setupLogger.Info("Starting Web UI on " + fiberString)
	err := app.Listen(fiberString)
	if err != nil {
		return
	}

}

package main

import (
	"context"
	"encoding/json"
	"fmt"
	_ "fmt"
	"github.com/a-h/templ"
	"github.com/ether/etherpad-go/assets/welcome"
	"github.com/ether/etherpad-go/lib/locales"
	"github.com/ether/etherpad-go/lib/pad"
	"github.com/ether/etherpad-go/lib/plugins"
	session2 "github.com/ether/etherpad-go/lib/session"
	"github.com/ether/etherpad-go/lib/ws"
	"github.com/evanw/esbuild/pkg/api"
	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
	"github.com/gorilla/sessions"
	sio "github.com/njones/socketio"
	ser "github.com/njones/socketio/serialize"
	"net/http"
)

var store *sessions.CookieStore

func sessionMiddleware(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, err := store.Get(r, "express_sid")
		if err != nil {
			println("Error getting session", err)
			http.SetCookie(w, &http.Cookie{Name: "express_sid", MaxAge: -1, Path: "/"})
			return
		}

		if session.IsNew {
			http.SetCookie(w, &http.Cookie{Name: "express_sid", MaxAge: -1, Path: "/"})
			err := session.Save(r, w)
			if err != nil {
				println("Error saving session", err)
				return
			}
		}

		r = r.WithContext(context.WithValue(r.Context(), "session", session))
		h(w, r)
	}
}

func main() {
	var db = session2.NewSessionDatabase(nil)
	app := fiber.New()
	var cookieStore = session.New(session.Config{
		KeyLookup: "cookie:express_sid",
		Storage:   db,
	})
	server := sio.NewServer()
	component := welcome.Page()

	app.Use(func(c *fiber.Ctx) error {
		return pad.CheckAccess(c)
	})

	app.Static("/css/", "./assets/css")
	app.Static("/html/", "./assets/html")
	app.Static("/font/", "./assets/font")

	relativePath := "./ui/src/js"

	var alias = make(map[string]string)
	alias["ep_etherpad-lite/static/js/ace2_inner"] = relativePath + "/ace2_inner"
	alias["ep_etherpad-lite/static/js/ace2_common"] = relativePath + "/ace2_common"
	alias["ep_etherpad-lite/static/js/pluginfw/client_plugins"] = relativePath + "/pluginfw/client_plugins"
	alias["ep_etherpad-lite/static/js/rjquery"] = relativePath + "/rjquery"
	alias["ep_etherpad-lite/static/js/nice-select"] = "ep_etherpad-lite/static/js/vendors/nice-select"

	app.Get("/js/*", func(c *fiber.Ctx) error {
		result := api.Build(api.BuildOptions{
			EntryPoints: []string{"./ui/src/main.js"},
			Bundle:      true,
			Write:       false,
			LogLevel:    api.LogLevelInfo,
			Metafile:    true,
			Target:      api.ES2020,
			Alias:       alias,
		})

		if len(result.Errors) > 0 {
			fmt.Println("Build failed with errors:", result.Errors)
			return c.SendString("Build failed")
		}

		c.Set("Content-Type", "application/javascript")

		return c.Send(result.OutputFiles[0].Contents)
	})
	app.Static("/locales", "./assets/locales")
	app.Static("/images", "./assets/images")
	app.Static("/pluginfw/plugin-definitions.json", "./assets/plugin/plugin-definitions.json")

	app.Get("/locales.json", func(c *fiber.Ctx) error {
		var respHeaders = c.GetRespHeaders()
		respHeaders["Content-Type"] = []string{"application/json"}
		var marshalledLocales, _ = json.Marshal(locales.Locales)
		return c.Send(marshalledLocales)
	})

	app.Use("/p/", func(c *fiber.Ctx) error {
		c.Path()

		var _, err = cookieStore.Get(c)
		if err != nil {
			println("Error with session")
		}

		return c.Next()
	})

	app.Get("/pluginfw/plugin-definitions.json", plugins.ReturnPluginResponse)
	app.Static("/favicon.ico", "./assets/images/favicon.ico")
	app.Get("/p/*", pad.HandlePadOpen)

	app.Get("/", func(c *fiber.Ctx) error {
		return adaptor.HTTPHandler(templ.Handler(component))(c)
	})

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

	err := app.Listen(":3000")
	if err != nil {
		return
	}

}

package main

import (
	"context"
	"encoding/json"
	_ "fmt"
	"github.com/a-h/templ"
	"github.com/ether/etherpad-go/assets/welcome"
	"github.com/ether/etherpad-go/lib/locales"
	"github.com/ether/etherpad-go/lib/pad"
	"github.com/ether/etherpad-go/lib/plugins"
	"github.com/ether/etherpad-go/lib/ws"
	"github.com/gorilla/securecookie"
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

	server := sio.NewServer()
	store = sessions.NewCookieStore(securecookie.GenerateRandomKey(32))

	component := welcome.Page()
	cssDir := http.FileServer(http.Dir("./assets/css"))
	htmlDir := http.FileServer(http.Dir("./assets/html"))
	fontDir := http.FileServer(http.Dir("./assets/font"))
	jsDir := http.FileServer(http.Dir("./assets/js"))
	imagesDir := http.FileServer(http.Dir("./assets/images"))
	pluginDir := http.FileServer(http.Dir("./plugins"))

	http.Handle("/css/", http.StripPrefix("/css/", cssDir))
	http.HandleFunc("GET /locales.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var marshalledLocales, _ = json.Marshal(locales.Locales)
		w.Write(marshalledLocales)
	})
	http.Handle("/js/", http.StripPrefix("/js/", jsDir))
	http.Handle("/html/", http.StripPrefix("/html/", htmlDir))
	http.Handle("/font/", http.StripPrefix("/font/", fontDir))
	http.Handle("/locales/", http.StripPrefix("/locales/", http.FileServer(http.Dir("./assets/locales"))))
	http.Handle("/images/", http.StripPrefix("/images/", imagesDir))
	http.HandleFunc("GET /pluginfw/plugin-definitions.json", plugins.ReturnPluginResponse)
	http.HandleFunc("/pluginfw/plugin-definitions.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

	})
	http.Handle("/pluginfw/", http.StripPrefix("/pluginfw", pluginDir))
	http.Handle("/p/*", sessionMiddleware(pad.HandlePadOpen))

	http.Handle("/", templ.Handler(component))

	ws.HubGlob = ws.NewHub()
	go ws.HubGlob.Run()
	http.HandleFunc("/socket.io/*", func(w http.ResponseWriter, r *http.Request) {
		ws.ServeWs(ws.HubGlob, w, r)
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

	err := http.ListenAndServe(":3000", nil)
	if err != nil {
		return
	}

}

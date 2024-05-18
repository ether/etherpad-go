package main

import (
	_ "fmt"
	"github.com/a-h/templ"
	"github.com/ether/etherpad-go/assets/welcome"
	"github.com/ether/etherpad-go/lib/pad"
	"github.com/ether/etherpad-go/lib/ws"
	"net/http"
)

func main() {
	component := welcome.Page()
	cssDir := http.FileServer(http.Dir("./assets/css"))
	jsDir := http.FileServer(http.Dir("./assets/js"))
	imagesDir := http.FileServer(http.Dir("./assets/images"))
	pluginDir := http.FileServer(http.Dir("./plugins"))

	http.Handle("/css/", http.StripPrefix("/css/", cssDir))
	http.Handle("/js/", http.StripPrefix("/js/", jsDir))
	http.Handle("/images/", http.StripPrefix("/images/", imagesDir))

	http.Handle("/pluginfw/", http.StripPrefix("/pluginfw", pluginDir))
	http.HandleFunc("/p/*", pad.HandlePadOpen)

	http.Handle("/", templ.Handler(component))

	hub := ws.NewHub()
	go hub.Run()
	http.HandleFunc("/ws/*", func(w http.ResponseWriter, r *http.Request) {
		ws.ServeWs(hub, w, r)
	})

	err := http.ListenAndServe(":3000", nil)
	if err != nil {
		return
	}

}

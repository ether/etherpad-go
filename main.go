package main

import (
	_ "fmt"
	"github.com/a-h/templ"
	"github.com/ether/etherpad-go/assets/welcome"
	"github.com/ether/etherpad-go/lib/pad"
	"net/http"
)

func main() {
	component := welcome.Page()

	cssDir := http.FileServer(http.Dir("./assets/css"))
	jsDir := http.FileServer(http.Dir("./assets/js"))
	imagesDir := http.FileServer(http.Dir("./assets/images"))

	http.Handle("/css/", http.StripPrefix("/css/", cssDir))
	http.Handle("/js/", http.StripPrefix("/js/", jsDir))
	http.Handle("/images/", http.StripPrefix("/images/", imagesDir))

	http.HandleFunc("/p/*", pad.HandlePadOpen)

	http.Handle("/", templ.Handler(component))

	err := http.ListenAndServe(":3000", nil)
	if err != nil {
		return
	}

}

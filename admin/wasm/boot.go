//go:build js && wasm

package main

import goapp "github.com/maxence-charriere/go-app/v10/pkg/app"

func main() {
	goapp.RouteWithRegexp(`^/admin(?:/.*)?$`, func() goapp.Composer {
		return newAdminPage()
	})
	goapp.RunWhenOnBrowser()
}

//go:build js && wasm

package main

func main() {
	a := newApp()
	a.connectSocket()
	a.render()
	select {}
}

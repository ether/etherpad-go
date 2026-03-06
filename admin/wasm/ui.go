//go:build js && wasm

package main

import (
	"strings"
	"syscall/js"
)

func (a *app) bindUI() {
	a.bindClicks("[data-nav]", func(el js.Value) {
		a.state.CurrentPage = el.Get("dataset").Get("nav").String()
		a.render()
	})
	a.bindClicks("#dismiss-toast", func(el js.Value) {
		a.state.Toast = nil
		a.render()
	})
	a.bindClicks("#refresh-overview", func(el js.Value) {
		a.emit("checkUpdates", nil)
		a.emit("getInstalled", nil)
		a.emit("getStats", nil)
		a.requestPads()
	})
	a.bindClicks("#pad-search-btn", func(el js.Value) {
		a.state.PadSearch = a.document.Call("getElementById", "pad-search").Get("value").String()
		a.state.PadOffset = 0
		a.requestPads()
		a.render()
	})
	a.bindClicks("#create-pad", func(el js.Value) {
		name := strings.TrimSpace(a.window.Call("prompt", a.t("pads.promptName", "Pad name?")).String())
		if name == "" {
			return
		}
		a.emit("createPad", map[string]any{"padName": name})
	})
	a.bindClicks("[data-open-pad]", func(el js.Value) {
		name := el.Get("dataset").Get("openPad").String()
		a.window.Call("open", "/p/"+name, "_blank")
	})
	a.bindClicks("[data-delete-pad]", func(el js.Value) {
		name := el.Get("dataset").Get("deletePad").String()
		if a.window.Call("confirm", a.tf("pads.confirmDelete", "Delete pad %s?", name)).Bool() {
			a.emit("deletePad", name)
		}
	})
	a.bindClicks("[data-clean-pad]", func(el js.Value) {
		name := el.Get("dataset").Get("cleanPad").String()
		if a.window.Call("confirm", a.tf("pads.confirmClean", "Clean revisions for %s?", name)).Bool() {
			a.emit("cleanupPadRevisions", name)
		}
	})
	a.bindClicks("#pads-prev", func(el js.Value) {
		if a.state.PadOffset >= a.state.PadLimit {
			a.state.PadOffset -= a.state.PadLimit
			a.requestPads()
			a.render()
		}
	})
	a.bindClicks("#pads-next", func(el js.Value) {
		if a.state.PadOffset+a.state.PadLimit < a.state.PadsTotal {
			a.state.PadOffset += a.state.PadLimit
			a.requestPads()
			a.render()
		}
	})
	a.bindClicks("[data-sort]", func(el js.Value) {
		key := el.Get("dataset").Get("sort").String()
		if a.state.PadSort == key {
			a.state.PadAscending = !a.state.PadAscending
		} else {
			a.state.PadSort = key
			a.state.PadAscending = true
		}
		a.requestPads()
		a.render()
	})
	a.bindClicks("#send-shout", func(el js.Value) {
		msg := strings.TrimSpace(a.document.Call("getElementById", "shout-message").Get("value").String())
		sticky := a.document.Call("getElementById", "shout-sticky").Get("checked").Bool()
		if msg == "" {
			return
		}
		a.state.ShoutMessage = msg
		a.state.ShoutSticky = sticky
		a.emit("shout", map[string]any{"message": msg, "sticky": sticky})
	})
	a.bindClicks("#reload-settings", func(el js.Value) {
		a.emit("load", map[string]any{})
	})
	a.bindClicks("#save-settings", func(el js.Value) {
		value := a.document.Call("getElementById", "settings-editor").Get("value").String()
		a.state.Settings = value
		a.emit("saveSettings", value)
		a.state.Toast = &toast{Kind: "success", Message: a.t("toast.saveSent", "Save event sent.")}
		a.render()
	})
	a.bindClicks("#restart-server", func(el js.Value) {
		a.emit("restartServer", nil)
		a.state.Toast = &toast{Kind: "success", Message: a.t("toast.restartSent", "Restart event sent.")}
		a.render()
	})
}

func (a *app) bindClicks(selector string, handler func(js.Value)) {
	nodes := a.document.Call("querySelectorAll", selector)
	length := nodes.Get("length").Int()
	for i := 0; i < length; i++ {
		el := nodes.Index(i)
		cb := js.FuncOf(func(this js.Value, args []js.Value) any {
			if len(args) > 0 {
				args[0].Call("preventDefault")
			}
			handler(this)
			return nil
		})
		a.funcs = append(a.funcs, cb)
		el.Call("addEventListener", "click", cb)
	}
}

//go:build js && wasm

package main

import (
	"encoding/json"
	"fmt"
	"syscall/js"
	"time"
)

func (a *app) connectSocket() {
	if a.state.Token == "" {
		a.failSocket(a.t("socket.noToken", "No admin token available."), false)
		return
	}

	valid, err := a.validateToken(a.state.Token)
	if err != nil {
		a.failSocket(a.t("socket.validateFailed", "Token validation failed."), true)
		return
	}
	if !valid {
		a.state.Loading = true
		a.state.LoadingMessage = a.t("socket.reauth", "Refreshing session...")
		a.render()
		newToken, err := a.reauth()
		if err != nil || newToken == "" {
			a.failSocket(a.t("socket.reauthFailed", "Could not refresh session."), true)
			return
		}
		a.state.Token = newToken
	}

	protocol := "ws:"
	if a.window.Get("location").Get("protocol").String() == "https:" {
		protocol = "wss:"
	}
	wsURL := fmt.Sprintf("%s//%s/admin/ws?token=%s", protocol, a.window.Get("location").Get("host").String(), a.state.Token)
	socket := js.Global().Get("WebSocket").New(wsURL)
	a.socket = socket

	onOpen := js.FuncOf(func(this js.Value, args []js.Value) any {
		a.state.Connected = true
		a.state.Loading = false
		a.state.Error = ""
		a.reconnecting = false
		a.emit("load", map[string]any{})
		a.emit("checkUpdates", nil)
		a.emit("getInstalled", nil)
		a.emit("getStats", nil)
		a.requestPads()
		a.render()
		return nil
	})
	a.funcs = append(a.funcs, onOpen)
	socket.Set("onopen", onOpen)

	onMessage := js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) == 0 {
			return nil
		}
		a.handleMessage(args[0].Get("data").String())
		return nil
	})
	a.funcs = append(a.funcs, onMessage)
	socket.Set("onmessage", onMessage)

	onClose := js.FuncOf(func(this js.Value, args []js.Value) any {
		a.scheduleReconnect(1000)
		return nil
	})
	a.funcs = append(a.funcs, onClose)
	socket.Set("onclose", onClose)

	onError := js.FuncOf(func(this js.Value, args []js.Value) any {
		a.state.Error = a.t("socket.error", "Admin connection error.")
		a.state.Connected = false
		a.render()
		a.scheduleReconnect(1000)
		return nil
	})
	a.funcs = append(a.funcs, onError)
	socket.Set("onerror", onError)
}

func (a *app) failSocket(message string, retry bool) {
	a.state.Connected = false
	a.state.Error = message
	a.state.Loading = retry
	if retry {
		a.state.LoadingMessage = a.t("socket.reconnecting", "Reconnecting...")
		a.scheduleReconnect(1500)
	} else {
		a.reconnecting = false
	}
	a.render()
}

func (a *app) scheduleReconnect(delay int) {
	if !a.mounted || a.reconnecting {
		return
	}
	a.reconnecting = true
	a.state.Connected = false
	a.state.Loading = true
	a.state.LoadingMessage = a.t("socket.reconnecting", "Reconnecting...")
	a.render()
	cb := js.FuncOf(func(this js.Value, args []js.Value) any {
		a.reconnecting = false
		go a.connectSocket()
		return nil
	})
	a.funcs = append(a.funcs, cb)
	a.window.Call("setTimeout", cb, delay)
}

func (a *app) emit(event string, data any) {
	if a.socket.IsUndefined() || a.socket.IsNull() {
		return
	}
	if a.socket.Get("readyState").Int() != 1 {
		return
	}
	payload := map[string]any{"event": event}
	if data != nil {
		payload["data"] = data
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return
	}
	a.socket.Call("send", string(raw))
}

func (a *app) handleMessage(raw string) {
	var envelope []json.RawMessage
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil || len(envelope) < 2 {
		return
	}
	var event string
	if err := json.Unmarshal(envelope[0], &event); err != nil {
		return
	}

	switch event {
	case "settings":
		var msg settingsMessage
		if json.Unmarshal(envelope[1], &msg) == nil {
			pretty, _ := json.MarshalIndent(msg.Results, "", "  ")
			a.state.Settings = string(pretty)
		}
	case "results:checkUpdates":
		var result updateCheckResult
		if json.Unmarshal(envelope[1], &result) == nil {
			a.state.Update = &result
		}
	case "results:padLoad":
		var result padsResponse
		if json.Unmarshal(envelope[1], &result) == nil {
			a.state.Pads = result.Results
			a.state.PadsTotal = result.Total
		}
	case "results:installed":
		var result installedPlugins
		if json.Unmarshal(envelope[1], &result) == nil {
			sortPluginsByName(result.Installed)
			a.state.Plugins = result.Installed
		}
	case "results:stats":
		var result statsResponse
		if json.Unmarshal(envelope[1], &result) == nil {
			a.state.TotalUsers = result.TotalUsers
		}
	case "result:shout":
		var result shoutEnvelope
		if json.Unmarshal(envelope[1], &result) == nil {
			a.state.Shouts = append([]shoutEnvelope{result}, a.state.Shouts...)
			if len(a.state.Shouts) > 20 {
				a.state.Shouts = a.state.Shouts[:20]
			}
			a.state.ShoutMessage = ""
			a.state.Toast = &toast{Kind: "success", Message: a.t("toast.messageSent", "Message sent.")}
		}
	case "results:deletePad":
		a.state.Toast = &toast{Kind: "success", Message: a.t("toast.padDeleted", "Pad deleted.")}
		a.requestPads()
	case "results:createPad":
		var resp map[string]string
		if json.Unmarshal(envelope[1], &resp) == nil {
			if msg := resp["error"]; msg != "" {
				a.state.Toast = &toast{Kind: "error", Message: msg}
			} else {
				a.state.Toast = &toast{Kind: "success", Message: resp["success"]}
			}
		}
		a.requestPads()
	case "results:cleanupPadRevisions":
		a.state.Toast = &toast{Kind: "success", Message: a.t("toast.padCleaned", "Revisions cleaned.")}
		a.requestPads()
	}

	a.state.LastUpdated = time.Now()
	a.render()
}

func (a *app) requestPads() {
	a.emit("padLoad", map[string]any{
		"offset":    a.state.PadOffset,
		"limit":     a.state.PadLimit,
		"pattern":   a.state.PadSearch,
		"sortBy":    a.state.PadSort,
		"ascending": a.state.PadAscending,
	})
}

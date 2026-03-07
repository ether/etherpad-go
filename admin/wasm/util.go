//go:build js && wasm

package main

import "time"

func pageTitle(a *app, page string) string {
	switch page {
	case "pads":
		return a.t("pads.title", "Pads")
	case "broadcast":
		return a.t("broadcast.title", "Broadcast")
	case "settings":
		return a.t("settings.title", "Settings")
	default:
		return a.t("overview.title", "Overview")
	}
}

func pageDescription(a *app, page string) string {
	switch page {
	case "pads":
		return a.t("pads.description", "Search, create and maintain pads.")
	case "broadcast":
		return a.t("broadcast.description", "Send operational messages to connected users.")
	case "settings":
		return a.t("settings.description", "Edit configuration and trigger admin actions.")
	default:
		return a.t("overview.description", "Monitor activity, plugins and server status.")
	}
}

func formatTimestamp(value int64) string {
	if value == 0 {
		return "-"
	}
	return time.UnixMilli(value).Format("02.01.2006 15:04")
}

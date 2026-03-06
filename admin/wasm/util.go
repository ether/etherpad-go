//go:build js && wasm

package main

import (
	"strings"
	"time"
)

func renderShout(shout shoutEnvelope) string {
	return renderShoutWithLabel(shout, "sticky")
}

func renderShoutWithLabel(shout shoutEnvelope, stickyLabel string) string {
	stamp := time.UnixMilli(shout.Data.Payload.Timestamp).Format("02.01.2006 15:04")
	sticky := ""
	if shout.Data.Payload.Message.Sticky {
		sticky = `<span class="badge on">` + escapeHTML(stickyLabel) + `</span>`
	}
	return `<article class="message-card"><div><p>` + escapeHTML(shout.Data.Payload.Message.Message) + `</p><span class="muted">` + escapeHTML(stamp) + `</span></div>` + sticky + `</article>`
}

func metricCard(label, value, meta string) string {
	return `<article class="metric-card"><span class="metric-label">` + label + `</span><strong>` + value + `</strong><p class="muted">` + meta + `</p></article>`
}

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

func escapeHTML(input string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		`'`, "&#39;",
	)
	return replacer.Replace(input)
}

func (a *app) sortHeader(key, label string) string {
	className := "sortable"
	indicator := ""
	if a.state.PadSort == key {
		className += " active"
		if a.state.PadAscending {
			indicator = " ^"
		} else {
			indicator = " v"
		}
	}
	return `<th><button class="` + className + `" data-sort="` + key + `">` + label + indicator + `</button></th>`
}

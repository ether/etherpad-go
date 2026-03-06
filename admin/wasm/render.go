//go:build js && wasm

package main

import (
	"fmt"
	"strings"
)

func (a *app) render() {
	a.root.Set("innerHTML", a.renderHTML())
	a.bindUI()
}

func (a *app) renderHTML() string {
	var b strings.Builder
	b.WriteString(`<div class="admin-shell">`)
	b.WriteString(`<aside class="sidebar">`)
	b.WriteString(`<div><p class="eyebrow">` + escapeHTML(a.t("app.brand", "Etherpad")) + `</p><h1>` + escapeHTML(a.t("app.title", "Admin")) + `</h1><p class="muted">` + escapeHTML(a.t("app.subtitle", "Administration")) + `</p></div>`)
	b.WriteString(`<nav class="nav">`)
	b.WriteString(a.navItem("overview", a.t("overview.title", "Overview")))
	b.WriteString(a.navItem("pads", a.t("pads.title", "Pads")))
	b.WriteString(a.navItem("broadcast", a.t("broadcast.title", "Broadcast")))
	b.WriteString(a.navItem("settings", a.t("settings.title", "Settings")))
	b.WriteString(`</nav>`)
	statusClass := "status offline"
	statusText := a.t("status.offline", "offline")
	if a.state.Connected {
		statusClass = "status online"
		statusText = a.t("status.connected", "connected")
	}
	b.WriteString(`<div class="sidebar-footer"><span class="` + statusClass + `"></span><span>` + statusText + `</span></div>`)
	b.WriteString(`</aside>`)

	b.WriteString(`<main class="content">`)
	b.WriteString(`<header class="hero"><div><p class="eyebrow">` + escapeHTML(a.t("app.console", "Admin Console")) + `</p><h2>` + pageTitle(a, a.state.CurrentPage) + `</h2><p class="muted">` + pageDescription(a, a.state.CurrentPage) + `</p></div>`)
	b.WriteString(`<div class="hero-actions">`)
	if a.state.Update != nil && a.state.Update.UpdateAvailable {
		b.WriteString(`<div class="pill warn">` + escapeHTML(a.tf("overview.updateAvailable", "Update available: %s -> %s", a.state.Update.CurrentVersion, a.state.Update.LatestVersion)) + `</div>`)
	}
	if !a.state.LastUpdated.IsZero() {
		b.WriteString(`<div class="pill">` + escapeHTML(a.t("status.sync", "Updated")) + ` ` + escapeHTML(a.state.LastUpdated.Format("15:04:05")) + `</div>`)
	}
	b.WriteString(`</div></header>`)

	if a.state.Toast != nil {
		b.WriteString(`<div class="toast ` + escapeHTML(a.state.Toast.Kind) + `">` + escapeHTML(a.state.Toast.Message) + `<button id="dismiss-toast" class="link-button">` + escapeHTML(a.t("toast.dismiss", "Dismiss")) + `</button></div>`)
	}
	if a.state.Error != "" {
		b.WriteString(`<div class="toast error">` + escapeHTML(a.state.Error) + `</div>`)
	}
	if a.state.Loading {
		b.WriteString(`<div class="loading-card">` + escapeHTML(a.state.LoadingMessage) + `</div>`)
	}

	switch a.state.CurrentPage {
	case "pads":
		b.WriteString(a.renderPads())
	case "broadcast":
		b.WriteString(a.renderBroadcast())
	case "settings":
		b.WriteString(a.renderSettings())
	default:
		b.WriteString(a.renderOverview())
	}

	b.WriteString(`</main></div>`)
	return b.String()
}

func (a *app) navItem(id, label string) string {
	className := "nav-item"
	if a.state.CurrentPage == id {
		className += " active"
	}
	return `<button class="` + className + `" data-nav="` + id + `">` + label + `</button>`
}

func (a *app) renderOverview() string {
	var b strings.Builder
	b.WriteString(`<section class="metrics">`)
	b.WriteString(metricCard(a.t("overview.liveUsers", "Live users"), fmt.Sprintf("%d", a.state.TotalUsers), a.t("overview.liveUsers.meta", "currently connected")))
	b.WriteString(metricCard(a.t("overview.padsIndexed", "Pads indexed"), fmt.Sprintf("%d", a.state.PadsTotal), a.t("overview.padsIndexed.meta", "current search scope")))
	versionValue := a.t("common.notAvailable", "n/a")
	if a.state.Update != nil {
		versionValue = a.state.Update.CurrentVersion
	}
	b.WriteString(metricCard(a.t("overview.serverVersion", "Server version"), escapeHTML(versionValue), a.t("overview.serverVersion.meta", "running release")))
	pluginCount := 0
	for _, p := range a.state.Plugins {
		if p.Enabled {
			pluginCount++
		}
	}
	b.WriteString(metricCard(a.t("overview.pluginsActive", "Active plugins"), fmt.Sprintf("%d", pluginCount), a.t("overview.pluginsActive.meta", "enabled integrations")))
	b.WriteString(`</section>`)

	b.WriteString(`<section class="panel-grid">`)
	b.WriteString(`<article class="panel"><div class="panel-head"><h3>` + escapeHTML(a.t("overview.pluginsInstalled", "Installed plugins")) + `</h3><button class="link-button" id="refresh-overview">` + escapeHTML(a.t("common.refresh", "Refresh")) + `</button></div><div class="plugin-list">`)
	if len(a.state.Plugins) == 0 {
		b.WriteString(`<p class="muted">` + escapeHTML(a.t("overview.noPlugins", "No plugin data loaded yet.")) + `</p>`)
	} else {
		max := len(a.state.Plugins)
		if max > 8 {
			max = 8
		}
		for i := 0; i < max; i++ {
			p := a.state.Plugins[i]
			status := "off"
			if p.Enabled {
				status = "on"
			}
			b.WriteString(`<div class="plugin-row"><div><strong>` + escapeHTML(p.Name) + `</strong><p class="muted">` + escapeHTML(p.Description) + `</p></div><span class="badge ` + status + `">` + escapeHTML(p.Version) + `</span></div>`)
		}
	}
	b.WriteString(`</div></article>`)

	b.WriteString(`<article class="panel"><div class="panel-head"><h3>` + escapeHTML(a.t("overview.broadcastActivity", "Recent broadcast activity")) + `</h3><button class="link-button" data-nav="broadcast">` + escapeHTML(a.t("common.open", "Open")) + `</button></div>`)
	if len(a.state.Shouts) == 0 {
		b.WriteString(`<p class="muted">` + escapeHTML(a.t("overview.noBroadcasts", "No broadcast messages yet.")) + `</p>`)
	} else {
		b.WriteString(`<div class="message-list">`)
		max := len(a.state.Shouts)
		if max > 5 {
			max = 5
		}
		for i := 0; i < max; i++ {
			b.WriteString(renderShoutWithLabel(a.state.Shouts[i], a.t("broadcast.stickyBadge", "sticky")))
		}
		b.WriteString(`</div>`)
	}
	b.WriteString(`</article></section>`)
	return b.String()
}

func (a *app) renderPads() string {
	var b strings.Builder
	b.WriteString(`<section class="panel">`)
	b.WriteString(`<div class="panel-head"><h3>` + escapeHTML(a.t("pads.management", "Pad management")) + `</h3><div class="toolbar"><input id="pad-search" class="search" placeholder="` + escapeHTML(a.t("pads.searchPlaceholder", "Search pads")) + `" value="` + escapeHTML(a.state.PadSearch) + `"><button id="pad-search-btn" class="primary-button">` + escapeHTML(a.t("pads.apply", "Apply")) + `</button><button id="create-pad" class="secondary-button">` + escapeHTML(a.t("pads.create", "Create pad")) + `</button></div></div>`)
	b.WriteString(`<div class="table-wrap"><table><thead><tr>`)
	b.WriteString(a.sortHeader("padName", a.t("pads.name", "Pad")))
	b.WriteString(a.sortHeader("userCount", a.t("pads.users", "Users")))
	b.WriteString(a.sortHeader("lastEdited", a.t("pads.lastEdited", "Last edited")))
	b.WriteString(a.sortHeader("revisionNumber", a.t("pads.revisions", "Revisions")))
	b.WriteString(`<th>` + escapeHTML(a.t("pads.actions", "Actions")) + `</th></tr></thead><tbody>`)
	if len(a.state.Pads) == 0 {
		b.WriteString(`<tr><td colspan="5" class="empty">` + escapeHTML(a.t("pads.none", "No pads found.")) + `</td></tr>`)
	} else {
		for _, pad := range a.state.Pads {
			b.WriteString(`<tr><td>` + escapeHTML(pad.PadName) + `</td><td>` + fmt.Sprintf("%d", pad.UserCount) + `</td><td>` + escapeHTML(formatTimestamp(pad.LastEdited)) + `</td><td>` + fmt.Sprintf("%d", pad.RevisionNumber) + `</td><td><div class="action-row"><button class="chip" data-open-pad="` + escapeHTML(pad.PadName) + `">` + escapeHTML(a.t("common.open", "Open")) + `</button><button class="chip" data-clean-pad="` + escapeHTML(pad.PadName) + `">` + escapeHTML(a.t("pads.clean", "Clean")) + `</button><button class="chip danger" data-delete-pad="` + escapeHTML(pad.PadName) + `">` + escapeHTML(a.t("pads.delete", "Delete")) + `</button></div></td></tr>`)
		}
	}
	b.WriteString(`</tbody></table></div>`)

	totalPages := 1
	if a.state.PadLimit > 0 && a.state.PadsTotal > 0 {
		totalPages = (a.state.PadsTotal + a.state.PadLimit - 1) / a.state.PadLimit
	}
	currentPage := 1 + a.state.PadOffset/a.state.PadLimit
	b.WriteString(`<div class="pagination"><button id="pads-prev" class="secondary-button">` + escapeHTML(a.t("pads.prev", "Previous")) + `</button><span>` + escapeHTML(a.t("pads.page", "Page")) + ` ` + fmt.Sprintf("%d", currentPage) + ` / ` + fmt.Sprintf("%d", totalPages) + `</span><button id="pads-next" class="secondary-button">` + escapeHTML(a.t("pads.next", "Next")) + `</button></div>`)
	b.WriteString(`</section>`)
	return b.String()
}

func (a *app) renderBroadcast() string {
	var b strings.Builder
	b.WriteString(`<section class="panel">`)
	b.WriteString(`<div class="panel-head"><h3>` + escapeHTML(a.t("broadcast.message", "Broadcast message")) + `</h3><span class="pill">` + escapeHTML(a.tf("broadcast.liveUsers", "%d users online", a.state.TotalUsers)) + `</span></div>`)
	b.WriteString(`<div class="broadcast-compose"><textarea id="shout-message" class="composer" placeholder="` + escapeHTML(a.t("broadcast.placeholder", "Message for all connected users...")) + `">` + escapeHTML(a.state.ShoutMessage) + `</textarea><label class="toggle"><input id="shout-sticky" type="checkbox"`)
	if a.state.ShoutSticky {
		b.WriteString(` checked`)
	}
	b.WriteString(`> ` + escapeHTML(a.t("broadcast.sticky", "Sticky message")) + `</label><button id="send-shout" class="primary-button">` + escapeHTML(a.t("broadcast.send", "Send")) + `</button></div>`)
	b.WriteString(`<div class="message-list">`)
	if len(a.state.Shouts) == 0 {
		b.WriteString(`<p class="muted">` + escapeHTML(a.t("broadcast.none", "No messages sent yet.")) + `</p>`)
	} else {
		for _, shout := range a.state.Shouts {
			b.WriteString(renderShoutWithLabel(shout, a.t("broadcast.stickyBadge", "sticky")))
		}
	}
	b.WriteString(`</div></section>`)
	return b.String()
}

func (a *app) renderSettings() string {
	var b strings.Builder
	b.WriteString(`<section class="panel-grid">`)
	b.WriteString(`<article class="panel" style=""><div class="panel-head"><h3>` + escapeHTML(a.t("settings.file", "settings.json")) + `</h3><div class="toolbar"><button id="reload-settings" class="secondary-button">` + escapeHTML(a.t("settings.reload", "Reload")) + `</button><button id="save-settings" class="primary-button">` + escapeHTML(a.t("settings.save", "Save")) + `</button><button id="restart-server" class="secondary-button">` + escapeHTML(a.t("settings.restart", "Restart")) + `</button></div></div><textarea id="settings-editor" class="settings-editor">` + escapeHTML(a.state.Settings) + `</textarea><p class="muted">` + escapeHTML(a.t("settings.note", "Changes are sent directly to the server.")) + `</p></article>`)
	b.WriteString(`</section>`)
	return b.String()
}

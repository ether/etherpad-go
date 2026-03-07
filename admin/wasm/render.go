//go:build js && wasm

package main

import (
	"fmt"
	"strings"
	"time"

	goapp "github.com/maxence-charriere/go-app/v10/pkg/app"
)

type adminPage struct {
	goapp.Compo
	model *app
}

func newAdminPage() *adminPage {
	model := newApp()
	page := &adminPage{model: model}
	model.page = page
	return page
}

func (p *adminPage) OnMount(ctx goapp.Context) {
	p.model.pageCtx = ctx
	p.model.mounted = true
	p.model.syncPageFromLocation()
	if p.model.started {
		p.model.render()
		return
	}
	p.model.started = true
	go p.model.connectSocket()
}

func (p *adminPage) OnDismount() {
	p.model.mounted = false
	p.model.release()
}

func (a *app) render() {
	if !a.mounted {
		return
	}
	a.pageCtx.Dispatch(func(ctx goapp.Context) {
		ctx.Update()
	})
}

func (a *app) release() {
	if a.socket.Truthy() {
		a.socket.Call("close")
		jsNullSafe(a.socket)
	}
	for _, fn := range a.funcs {
		fn.Release()
	}
	a.funcs = nil
}

func jsNullSafe(value interface{ Set(string, any) }) {
	value.Set("onopen", nil)
	value.Set("onmessage", nil)
	value.Set("onclose", nil)
	value.Set("onerror", nil)
}

func (a *app) syncPageFromLocation() {
	path := strings.TrimSpace(a.window.Get("location").Get("pathname").String())
	path = strings.TrimPrefix(path, "/admin")
	path = strings.Trim(path, "/")
	switch path {
	case "pads", "broadcast", "settings":
		a.state.CurrentPage = path
	default:
		a.state.CurrentPage = "overview"
	}
}

func (a *app) syncLocation() {
	path := "/admin/"
	if a.state.CurrentPage != "overview" {
		path += a.state.CurrentPage
	}
	a.window.Get("history").Call("replaceState", nil, "", path)
}

func (p *adminPage) Render() goapp.UI {
	return goapp.Div().Class("admin-shell").Body(
		p.renderSidebar(),
		goapp.Main().Class("content").Body(
			p.renderHero(),
			goapp.If(p.model.state.Toast != nil, func() goapp.UI {
				return goapp.Div().Class("toast", p.model.state.Toast.Kind).Body(
					goapp.Text(p.model.state.Toast.Message),
					goapp.Button().Type("button").Class("link-button").Text(p.model.t("toast.dismiss", "Dismiss")).
						OnClick(func(ctx goapp.Context, e goapp.Event) {
							p.model.state.Toast = nil
							p.model.render()
						}),
				)
			}),
			goapp.If(p.model.state.Error != "", func() goapp.UI {
				return goapp.Div().Class("toast", "error").Text(p.model.state.Error)
			}),
			goapp.If(p.model.state.Loading, func() goapp.UI {
				return goapp.Div().Class("loading-card").Text(p.model.state.LoadingMessage)
			}),
			p.renderCurrentPage(),
		),
	)
}

func (p *adminPage) renderSidebar() goapp.UI {
	statusClass := "status offline"
	statusText := p.model.t("status.offline", "offline")
	if p.model.state.Connected {
		statusClass = "status online"
		statusText = p.model.t("status.connected", "connected")
	}

	return goapp.Aside().Class("sidebar").Body(
		goapp.Div().Body(
			goapp.P().Class("eyebrow").Text(p.model.t("app.brand", "Etherpad")),
			goapp.H1().Text(p.model.t("app.title", "Admin")),
			goapp.P().Class("muted").Text(p.model.t("app.subtitle", "Administration")),
		),
		goapp.Nav().Class("nav").Body(
			p.navItem("overview", p.model.t("overview.title", "Overview")),
			p.navItem("pads", p.model.t("pads.title", "Pads")),
			p.navItem("broadcast", p.model.t("broadcast.title", "Broadcast")),
			p.navItem("settings", p.model.t("settings.title", "Settings")),
		),
		goapp.Div().Class("sidebar-footer").Body(
			goapp.Span().Class(statusClass),
			goapp.Span().Text(statusText),
		),
	)
}

func (p *adminPage) navItem(id, label string) goapp.UI {
	className := "nav-item"
	if p.model.state.CurrentPage == id {
		className += " active"
	}
	return goapp.Button().Type("button").Class(className).Text(label).OnClick(func(ctx goapp.Context, e goapp.Event) {
		p.model.state.CurrentPage = id
		p.model.syncLocation()
		p.model.render()
	})
}

func (p *adminPage) renderHero() goapp.UI {
	items := []goapp.UI{}
	if p.model.state.Update != nil && p.model.state.Update.UpdateAvailable {
		items = append(items, goapp.Div().Class("pill", "warn").Text(
			p.model.tf(
				"overview.updateAvailable",
				"Update available: %s -> %s",
				p.model.state.Update.CurrentVersion,
				p.model.state.Update.LatestVersion,
			),
		))
	}
	if !p.model.state.LastUpdated.IsZero() {
		items = append(items, goapp.Div().Class("pill").Text(
			fmt.Sprintf("%s %s", p.model.t("status.sync", "Updated"), p.model.state.LastUpdated.Format("15:04:05")),
		))
	}

	return goapp.Header().Class("hero").Body(
		goapp.Div().Body(
			goapp.P().Class("eyebrow").Text(p.model.t("app.console", "Admin Console")),
			goapp.H2().Text(pageTitle(p.model, p.model.state.CurrentPage)),
			goapp.P().Class("muted").Text(pageDescription(p.model, p.model.state.CurrentPage)),
		),
		goapp.Div().Class("hero-actions").Body(items...),
	)
}

func (p *adminPage) renderCurrentPage() goapp.UI {
	switch p.model.state.CurrentPage {
	case "pads":
		return p.renderPads()
	case "broadcast":
		return p.renderBroadcast()
	case "settings":
		return p.renderSettings()
	default:
		return p.renderOverview()
	}
}

func (p *adminPage) renderOverview() goapp.UI {
	pluginCount := 0
	for _, plugin := range p.model.state.Plugins {
		if plugin.Enabled {
			pluginCount++
		}
	}
	versionValue := p.model.t("common.notAvailable", "n/a")
	if p.model.state.Update != nil {
		versionValue = p.model.state.Update.CurrentVersion
	}

	metrics := goapp.Section().Class("metrics").Body(
		metricCard(p.model.t("overview.liveUsers", "Live users"), fmt.Sprintf("%d", p.model.state.TotalUsers), p.model.t("overview.liveUsers.meta", "currently connected")),
		metricCard(p.model.t("overview.padsIndexed", "Pads indexed"), fmt.Sprintf("%d", p.model.state.PadsTotal), p.model.t("overview.padsIndexed.meta", "current search scope")),
		metricCard(p.model.t("overview.serverVersion", "Server version"), versionValue, p.model.t("overview.serverVersion.meta", "running release")),
		metricCard(p.model.t("overview.pluginsActive", "Active plugins"), fmt.Sprintf("%d", pluginCount), p.model.t("overview.pluginsActive.meta", "enabled integrations")),
	)

	pluginRows := []goapp.UI{}
	if len(p.model.state.Plugins) == 0 {
		pluginRows = append(pluginRows, goapp.P().Class("muted").Text(p.model.t("overview.noPlugins", "No plugin data loaded yet.")))
	} else {
		max := len(p.model.state.Plugins)
		if max > 8 {
			max = 8
		}
		for i := 0; i < max; i++ {
			plugin := p.model.state.Plugins[i]
			status := "off"
			if plugin.Enabled {
				status = "on"
			}
			pluginRows = append(pluginRows,
				goapp.Div().Class("plugin-row").Body(
					goapp.Div().Body(
						goapp.Strong().Text(plugin.Name),
						goapp.P().Class("muted").Text(plugin.Description),
					),
					goapp.Span().Class("badge", status).Text(plugin.Version),
				),
			)
		}
	}

	broadcastRows := []goapp.UI{}
	if len(p.model.state.Shouts) == 0 {
		broadcastRows = append(broadcastRows, goapp.P().Class("muted").Text(p.model.t("overview.noBroadcasts", "No broadcast messages yet.")))
	} else {
		max := len(p.model.state.Shouts)
		if max > 5 {
			max = 5
		}
		for i := 0; i < max; i++ {
			broadcastRows = append(broadcastRows, renderShoutCard(p.model.state.Shouts[i], p.model.t("broadcast.stickyBadge", "sticky")))
		}
	}

	panels := goapp.Section().Class("panel-grid").Body(
		goapp.Article().Class("panel").Body(
			goapp.Div().Class("panel-head").Body(
				goapp.H3().Text(p.model.t("overview.pluginsInstalled", "Installed plugins")),
				goapp.Button().Type("button").Class("link-button").Text(p.model.t("common.refresh", "Refresh")).
					OnClick(func(ctx goapp.Context, e goapp.Event) {
						p.model.emit("checkUpdates", nil)
						p.model.emit("getInstalled", nil)
						p.model.emit("getStats", nil)
						p.model.requestPads()
					}),
			),
			goapp.Div().Class("plugin-list").Body(pluginRows...),
		),
		goapp.Article().Class("panel").Body(
			goapp.Div().Class("panel-head").Body(
				goapp.H3().Text(p.model.t("overview.broadcastActivity", "Recent broadcast activity")),
				goapp.Button().Type("button").Class("link-button").Text(p.model.t("common.open", "Open")).
					OnClick(func(ctx goapp.Context, e goapp.Event) {
						p.model.state.CurrentPage = "broadcast"
						p.model.syncLocation()
						p.model.render()
					}),
			),
			goapp.Div().Class("message-list").Body(broadcastRows...),
		),
	)

	return goapp.Div().Body(metrics, panels)
}

func (p *adminPage) renderPads() goapp.UI {
	headers := goapp.THead().Body(goapp.Tr().Body(
		p.sortHeader("padName", p.model.t("pads.name", "Pad")),
		p.sortHeader("userCount", p.model.t("pads.users", "Users")),
		p.sortHeader("lastEdited", p.model.t("pads.lastEdited", "Last edited")),
		p.sortHeader("revisionNumber", p.model.t("pads.revisions", "Revisions")),
		goapp.Th().Text(p.model.t("pads.actions", "Actions")),
	))

	rows := []goapp.UI{}
	if len(p.model.state.Pads) == 0 {
		rows = append(rows, goapp.Tr().Body(
			goapp.Td().Attr("colspan", 5).Class("empty").Text(p.model.t("pads.none", "No pads found.")),
		))
	} else {
		for _, pad := range p.model.state.Pads {
			padName := pad.PadName
			rows = append(rows, goapp.Tr().Body(
				goapp.Td().DataSet("label", p.model.t("pads.name", "Pad")).Text(padName),
				goapp.Td().DataSet("label", p.model.t("pads.users", "Users")).Text(fmt.Sprintf("%d", pad.UserCount)),
				goapp.Td().DataSet("label", p.model.t("pads.lastEdited", "Last edited")).Text(formatTimestamp(pad.LastEdited)),
				goapp.Td().DataSet("label", p.model.t("pads.revisions", "Revisions")).Text(fmt.Sprintf("%d", pad.RevisionNumber)),
				goapp.Td().DataSet("label", p.model.t("pads.actions", "Actions")).Body(
					goapp.Div().Class("action-row").Body(
						goapp.Button().Type("button").Class("chip").Text(p.model.t("common.open", "Open")).OnClick(func(ctx goapp.Context, e goapp.Event) {
							p.model.window.Call("open", "/p/"+padName, "_blank")
						}),
						goapp.Button().Type("button").Class("chip").Text(p.model.t("pads.clean", "Clean")).OnClick(func(ctx goapp.Context, e goapp.Event) {
							if p.model.window.Call("confirm", p.model.tf("pads.confirmClean", "Clean revisions for %s?", padName)).Bool() {
								p.model.emit("cleanupPadRevisions", padName)
							}
						}),
						goapp.Button().Type("button").Class("chip", "danger").Text(p.model.t("pads.delete", "Delete")).OnClick(func(ctx goapp.Context, e goapp.Event) {
							if p.model.window.Call("confirm", p.model.tf("pads.confirmDelete", "Delete pad %s?", padName)).Bool() {
								p.model.emit("deletePad", padName)
							}
						}),
					),
				),
			))
		}
	}

	totalPages := 1
	if p.model.state.PadLimit > 0 && p.model.state.PadsTotal > 0 {
		totalPages = (p.model.state.PadsTotal + p.model.state.PadLimit - 1) / p.model.state.PadLimit
	}
	currentPage := 1
	if p.model.state.PadLimit > 0 {
		currentPage += p.model.state.PadOffset / p.model.state.PadLimit
	}

	return goapp.Section().Class("panel").Body(
		goapp.Div().Class("panel-head").Body(
			goapp.H3().Text(p.model.t("pads.management", "Pad management")),
			goapp.Div().Class("toolbar").Body(
				goapp.Input().ID("pad-search").Class("search").Placeholder(p.model.t("pads.searchPlaceholder", "Search pads")).Value(p.model.state.PadSearch).
					OnInput(func(ctx goapp.Context, e goapp.Event) {
						p.model.state.PadSearch = e.Get("target").Get("value").String()
					}),
				goapp.Button().Type("button").ID("pad-search-btn").Class("primary-button").Text(p.model.t("pads.apply", "Apply")).
					OnClick(func(ctx goapp.Context, e goapp.Event) {
						p.model.state.PadOffset = 0
						p.model.requestPads()
					}),
				goapp.Button().Type("button").ID("create-pad").Class("secondary-button").Text(p.model.t("pads.create", "Create pad")).
					OnClick(func(ctx goapp.Context, e goapp.Event) {
						name := strings.TrimSpace(p.model.window.Call("prompt", p.model.t("pads.promptName", "Pad name?")).String())
						if name == "" {
							return
						}
						p.model.emit("createPad", map[string]any{"padName": name})
					}),
			),
		),
		goapp.Div().Class("table-wrap").Body(
			goapp.Table().Body(headers, goapp.TBody().Body(rows...)),
		),
		goapp.Div().Class("pagination").Body(
			goapp.Button().Type("button").ID("pads-prev").Class("secondary-button").Text(p.model.t("pads.prev", "Previous")).
				Disabled(p.model.state.PadOffset < p.model.state.PadLimit).
				OnClick(func(ctx goapp.Context, e goapp.Event) {
					if p.model.state.PadOffset < p.model.state.PadLimit {
						return
					}
					p.model.state.PadOffset -= p.model.state.PadLimit
					p.model.requestPads()
				}),
			goapp.Span().Text(fmt.Sprintf("%s %d / %d", p.model.t("pads.page", "Page"), currentPage, totalPages)),
			goapp.Button().Type("button").ID("pads-next").Class("secondary-button").Text(p.model.t("pads.next", "Next")).
				Disabled(p.model.state.PadOffset+p.model.state.PadLimit >= p.model.state.PadsTotal).
				OnClick(func(ctx goapp.Context, e goapp.Event) {
					if p.model.state.PadOffset+p.model.state.PadLimit >= p.model.state.PadsTotal {
						return
					}
					p.model.state.PadOffset += p.model.state.PadLimit
					p.model.requestPads()
				}),
		),
	)
}

func (p *adminPage) sortHeader(key, label string) goapp.UI {
	className := "sortable"
	indicator := ""
	if p.model.state.PadSort == key {
		className += " active"
		if p.model.state.PadAscending {
			indicator = " ^"
		} else {
			indicator = " v"
		}
	}
	return goapp.Th().Body(
		goapp.Button().Type("button").Class(className).Text(label + indicator).OnClick(func(ctx goapp.Context, e goapp.Event) {
			if p.model.state.PadSort == key {
				p.model.state.PadAscending = !p.model.state.PadAscending
			} else {
				p.model.state.PadSort = key
				p.model.state.PadAscending = true
			}
			p.model.requestPads()
		}),
	)
}

func (p *adminPage) renderBroadcast() goapp.UI {
	messages := []goapp.UI{}
	if len(p.model.state.Shouts) == 0 {
		messages = append(messages, goapp.P().Class("muted").Text(p.model.t("broadcast.none", "No messages sent yet.")))
	} else {
		for _, shout := range p.model.state.Shouts {
			messages = append(messages, renderShoutCard(shout, p.model.t("broadcast.stickyBadge", "sticky")))
		}
	}

	return goapp.Section().Class("panel").Body(
		goapp.Div().Class("panel-head").Body(
			goapp.H3().Text(p.model.t("broadcast.message", "Broadcast message")),
			goapp.Span().Class("pill").Text(p.model.tf("broadcast.liveUsers", "%d users online", p.model.state.TotalUsers)),
		),
		goapp.Div().Class("broadcast-compose").Body(
			goapp.Textarea().ID("shout-message").Class("composer").Placeholder(p.model.t("broadcast.placeholder", "Message for all connected users...")).
				Text(p.model.state.ShoutMessage).
				OnInput(func(ctx goapp.Context, e goapp.Event) {
					p.model.state.ShoutMessage = e.Get("target").Get("value").String()
				}),
			goapp.Label().Class("toggle").Body(
				goapp.Input().ID("shout-sticky").Type("checkbox").Checked(p.model.state.ShoutSticky).
					OnChange(func(ctx goapp.Context, e goapp.Event) {
						p.model.state.ShoutSticky = e.Get("target").Get("checked").Bool()
					}),
				goapp.Text(" "+p.model.t("broadcast.sticky", "Sticky message")),
			),
			goapp.Button().Type("button").ID("send-shout").Class("primary-button").Text(p.model.t("broadcast.send", "Send")).
				OnClick(func(ctx goapp.Context, e goapp.Event) {
					msg := strings.TrimSpace(p.model.state.ShoutMessage)
					if msg == "" {
						return
					}
					p.model.emit("shout", map[string]any{"message": msg, "sticky": p.model.state.ShoutSticky})
				}),
		),
		goapp.Div().Class("message-list").Body(messages...),
	)
}

func (p *adminPage) renderSettings() goapp.UI {
	return goapp.Section().Class("panel-grid", "settings-grid").Body(
		goapp.Article().Class("panel", "settings-panel").Body(
			goapp.Div().Class("panel-head").Body(
				goapp.H3().Text(p.model.t("settings.file", "settings.json")),
				goapp.Div().Class("toolbar").Body(
					goapp.Button().Type("button").ID("reload-settings").Class("secondary-button").Text(p.model.t("settings.reload", "Reload")).
						OnClick(func(ctx goapp.Context, e goapp.Event) {
							p.model.emit("load", map[string]any{})
						}),
					goapp.Button().Type("button").ID("save-settings").Class("primary-button").Text(p.model.t("settings.save", "Save")).
						OnClick(func(ctx goapp.Context, e goapp.Event) {
							p.model.emit("saveSettings", p.model.state.Settings)
							p.model.state.Toast = &toast{Kind: "success", Message: p.model.t("toast.saveSent", "Save event sent.")}
							p.model.render()
						}),
					goapp.Button().Type("button").ID("restart-server").Class("secondary-button").Text(p.model.t("settings.restart", "Restart")).
						OnClick(func(ctx goapp.Context, e goapp.Event) {
							p.model.emit("restartServer", nil)
							p.model.state.Toast = &toast{Kind: "success", Message: p.model.t("toast.restartSent", "Restart event sent.")}
							p.model.render()
						}),
				),
			),
			goapp.Textarea().ID("settings-editor").Class("settings-editor").Text(p.model.state.Settings).
				OnInput(func(ctx goapp.Context, e goapp.Event) {
					p.model.state.Settings = e.Get("target").Get("value").String()
				}),
			goapp.P().Class("muted").Text(p.model.t("settings.note", "Changes are sent directly to the server.")),
		),
	)
}

func renderShoutCard(shout shoutEnvelope, stickyLabel string) goapp.UI {
	stamp := time.UnixMilli(shout.Data.Payload.Timestamp).Format("02.01.2006 15:04")
	children := []goapp.UI{
		goapp.Div().Body(
			goapp.P().Text(shout.Data.Payload.Message.Message),
			goapp.Span().Class("muted").Text(stamp),
		),
	}
	if shout.Data.Payload.Message.Sticky {
		children = append(children, goapp.Span().Class("badge", "on").Text(stickyLabel))
	}
	return goapp.Article().Class("message-card").Body(children...)
}

func metricCard(label, value, meta string) goapp.UI {
	return goapp.Article().Class("metric-card").Body(
		goapp.Span().Class("metric-label").Text(label),
		goapp.Strong().Text(value),
		goapp.P().Class("muted").Text(meta),
	)
}

package hooks

import (
	"github.com/ether/etherpad-go/lib/hooks/events"
	"github.com/gofiber/fiber/v2/utils"
)

type Hook struct {
	hooks map[string]map[string]func(ctx any)
}

func NewHook() Hook {
	return Hook{
		hooks: make(map[string]map[string]func(ctx any)),
	}
}

func (h *Hook) EnqueueGetLineHtmlForExportHook(ctx func(ctx any)) {
	h.EnqueueHook("getLineHTMLForExport", ctx)
}

func (h *Hook) EnqueueGetPluginTranslationHooks(cb func(ctx *events.LocaleLoadContext)) {
	h.EnqueueHook("loadTranslations", func(ctx any) {
		if localeCtx, ok := ctx.(*events.LocaleLoadContext); ok {
			cb(localeCtx)
		}
	})
}

func (h *Hook) ExecuteGetPluginTranslationHooks(ctx *events.LocaleLoadContext) {
	h.ExecuteHooks("loadTranslations", ctx)
}

func (h *Hook) ExecuteGetLineHtmlForExportHooks(ctx any) {
	h.ExecuteHooks("getLineHTMLForExport", ctx)
}

func (h *Hook) EnqueueHook(key string, ctx func(ctx any)) string {
	var uuid = utils.UUID()
	var _, ok = h.hooks[key]

	if !ok {
		h.hooks[key] = make(map[string]func(ctx any))
	}

	h.hooks[key][uuid] = ctx

	return uuid
}

func (h *Hook) DequeueHook(key, id string) {
	delete(h.hooks[key], id)
}

func (h *Hook) ExecuteHooks(key string, ctx any) {

	var _, ok = h.hooks[key]

	if !ok {
		return
	}

	for _, v := range h.hooks[key] {
		v(ctx)
	}
}

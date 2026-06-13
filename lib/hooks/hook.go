package hooks

import (
	"github.com/ether/etherpad-go/lib/hooks/events"
	"github.com/gofiber/utils/v2"
)

type hookEntry struct {
	id string
	fn func(ctx any)
}

type Hook struct {
	hooks map[string][]hookEntry
}

func NewHook() Hook {
	return Hook{
		hooks: make(map[string][]hookEntry),
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

// EnqueuePreAuthorizeHook registers a callback for the preAuthorize hook,
// which lets plugins permit or deny a request before authentication runs (see
// events.PreAuthorizeContext).
func (h *Hook) EnqueuePreAuthorizeHook(cb func(ctx *events.PreAuthorizeContext)) string {
	return h.EnqueueHook(PreAuthorizeString, func(ctx any) {
		if preAuthorizeCtx, ok := ctx.(*events.PreAuthorizeContext); ok {
			cb(preAuthorizeCtx)
		}
	})
}

func (h *Hook) ExecutePreAuthorizeHooks(ctx *events.PreAuthorizeContext) {
	h.ExecuteHooks(PreAuthorizeString, ctx)
}

// EnqueuePreAuthzFailureHook registers a callback for the preAuthzFailure
// hook, which lets plugins override the default 403 response after a
// preAuthorize deny (see events.PreAuthzFailureContext).
func (h *Hook) EnqueuePreAuthzFailureHook(cb func(ctx *events.PreAuthzFailureContext)) string {
	return h.EnqueueHook(PreAuthzFailureString, func(ctx any) {
		if preAuthzFailureCtx, ok := ctx.(*events.PreAuthzFailureContext); ok {
			cb(preAuthzFailureCtx)
		}
	})
}

func (h *Hook) ExecutePreAuthzFailureHooks(ctx *events.PreAuthzFailureContext) {
	h.ExecuteHooks(PreAuthzFailureString, ctx)
}

func (h *Hook) EnqueueHook(key string, ctx func(ctx any)) string {
	var uuid = utils.UUID()
	h.hooks[key] = append(h.hooks[key], hookEntry{id: uuid, fn: ctx})
	return uuid
}

func (h *Hook) DequeueHook(key, id string) {
	entries := h.hooks[key]
	for i, e := range entries {
		if e.id == id {
			h.hooks[key] = append(entries[:i], entries[i+1:]...)
			return
		}
	}
}

func (h *Hook) ExecuteHooks(key string, ctx any) {
	for _, e := range h.hooks[key] {
		e.fn(ctx)
	}
}

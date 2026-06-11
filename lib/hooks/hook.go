package hooks

import (
	"github.com/ether/etherpad-go/lib/hooks/events"
	"github.com/gofiber/utils/v2"
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

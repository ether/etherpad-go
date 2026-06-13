package hooks

import (
	"slices"

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

func (h *Hook) EnqueuePadDefaultContentHook(cb func(ctx *events.PadDefaultContentContext)) string {
	return h.EnqueueHook(PadDefaultContentString, func(ctx any) {
		if c, ok := ctx.(*events.PadDefaultContentContext); ok {
			cb(c)
		}
	})
}

func (h *Hook) ExecutePadDefaultContentHooks(ctx *events.PadDefaultContentContext) {
	h.ExecuteHooks(PadDefaultContentString, ctx)
}

func (h *Hook) EnqueuePadLoadHook(cb func(ctx *events.PadLoadContext)) string {
	return h.EnqueueHook(PadLoadString, func(ctx any) {
		if c, ok := ctx.(*events.PadLoadContext); ok {
			cb(c)
		}
	})
}

func (h *Hook) ExecutePadLoadHooks(ctx *events.PadLoadContext) {
	h.ExecuteHooks(PadLoadString, ctx)
}

func (h *Hook) EnqueuePadCreateHook(cb func(ctx *events.PadCreateContext)) string {
	return h.EnqueueHook(PadCreateString, func(ctx any) {
		if c, ok := ctx.(*events.PadCreateContext); ok {
			cb(c)
		}
	})
}

func (h *Hook) ExecutePadCreateHooks(ctx *events.PadCreateContext) {
	h.ExecuteHooks(PadCreateString, ctx)
}

func (h *Hook) EnqueuePadUpdateHook(cb func(ctx *events.PadUpdateContext)) string {
	return h.EnqueueHook(PadUpdateString, func(ctx any) {
		if c, ok := ctx.(*events.PadUpdateContext); ok {
			cb(c)
		}
	})
}

func (h *Hook) ExecutePadUpdateHooks(ctx *events.PadUpdateContext) {
	h.ExecuteHooks(PadUpdateString, ctx)
}

func (h *Hook) EnqueuePadCopyHook(cb func(ctx *events.PadCopyContext)) string {
	return h.EnqueueHook(PadCopyString, func(ctx any) {
		if c, ok := ctx.(*events.PadCopyContext); ok {
			cb(c)
		}
	})
}

func (h *Hook) ExecutePadCopyHooks(ctx *events.PadCopyContext) {
	h.ExecuteHooks(PadCopyString, ctx)
}

func (h *Hook) EnqueuePadRemoveHook(cb func(ctx *events.PadRemoveContext)) string {
	return h.EnqueueHook(PadRemoveString, func(ctx any) {
		if c, ok := ctx.(*events.PadRemoveContext); ok {
			cb(c)
		}
	})
}

func (h *Hook) ExecutePadRemoveHooks(ctx *events.PadRemoveContext) {
	h.ExecuteHooks(PadRemoveString, ctx)
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
			h.hooks[key] = slices.Delete(entries, i, i+1)
			return
		}
	}
}

func (h *Hook) ExecuteHooks(key string, ctx any) {
	for _, e := range h.hooks[key] {
		e.fn(ctx)
	}
}

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

// EnqueuePadDefaultContentHook registers a callback for the padDefaultContent hook, which runs before a new pad's initial revision is written and may replace the default content (see events.PadDefaultContentContext).
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

// EnqueuePadLoadHook registers a callback for the padLoad hook, fired whenever a pad is materialized (see events.PadLoadContext).
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

// EnqueuePadCreateHook registers a callback for the padCreate hook, fired right after a pad's first revision is persisted (see events.PadCreateContext).
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

// EnqueuePadUpdateHook registers a callback for the padUpdate hook, fired after a revision is appended (see events.PadUpdateContext).
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

// EnqueuePadCopyHook registers a callback for the padCopy hook, fired after a pad is copied to a new destination (see events.PadCopyContext).
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

// EnqueuePadRemoveHook registers a callback for the padRemove hook, fired when a pad is deleted (see events.PadRemoveContext).
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

// EnqueueHandleMessageHook registers a callback for the handleMessage hook,
// fired before an incoming socket message is dispatched; a callback may drop it
// (see events.HandleMessageContext).
func (h *Hook) EnqueueHandleMessageHook(cb func(ctx *events.HandleMessageContext)) string {
	return h.EnqueueHook(HandleMessageString, func(ctx any) {
		if c, ok := ctx.(*events.HandleMessageContext); ok {
			cb(c)
		}
	})
}

func (h *Hook) ExecuteHandleMessageHooks(ctx *events.HandleMessageContext) {
	h.ExecuteHooks(HandleMessageString, ctx)
}

// EnqueueHandleMessageSecurityHook registers a callback for the
// handleMessageSecurity hook, which may grant write access to a read-only
// connection for a single message (see events.HandleMessageSecurityContext).
func (h *Hook) EnqueueHandleMessageSecurityHook(cb func(ctx *events.HandleMessageSecurityContext)) string {
	return h.EnqueueHook(HandleMessageSecurityString, func(ctx any) {
		if c, ok := ctx.(*events.HandleMessageSecurityContext); ok {
			cb(c)
		}
	})
}

func (h *Hook) ExecuteHandleMessageSecurityHooks(ctx *events.HandleMessageSecurityContext) {
	h.ExecuteHooks(HandleMessageSecurityString, ctx)
}

// EnqueueClientReadyHook registers a callback for the clientReady hook, fired
// once a client has finished joining a pad (see events.ClientReadyContext).
func (h *Hook) EnqueueClientReadyHook(cb func(ctx *events.ClientReadyContext)) string {
	return h.EnqueueHook(ClientReadyString, func(ctx any) {
		if c, ok := ctx.(*events.ClientReadyContext); ok {
			cb(c)
		}
	})
}

func (h *Hook) ExecuteClientReadyHooks(ctx *events.ClientReadyContext) {
	h.ExecuteHooks(ClientReadyString, ctx)
}

// EnqueueClientVarsHook registers a callback for the clientVars hook, fired just
// before the CLIENT_VARS payload is sent; a callback may mutate typed fields or
// add keys via Extra (see events.ClientVarsContext).
func (h *Hook) EnqueueClientVarsHook(cb func(ctx *events.ClientVarsContext)) string {
	return h.EnqueueHook(ClientVarsString, func(ctx any) {
		if c, ok := ctx.(*events.ClientVarsContext); ok {
			cb(c)
		}
	})
}

func (h *Hook) ExecuteClientVarsHooks(ctx *events.ClientVarsContext) {
	h.ExecuteHooks(ClientVarsString, ctx)
}

// EnqueueChatNewMessageHook registers a callback for the chatNewMessage hook,
// fired before a chat message is stored and broadcast; a callback may edit the
// text or drop it (see events.ChatNewMessageContext).
func (h *Hook) EnqueueChatNewMessageHook(cb func(ctx *events.ChatNewMessageContext)) string {
	return h.EnqueueHook(ChatNewMessageString, func(ctx any) {
		if c, ok := ctx.(*events.ChatNewMessageContext); ok {
			cb(c)
		}
	})
}

func (h *Hook) ExecuteChatNewMessageHooks(ctx *events.ChatNewMessageContext) {
	h.ExecuteHooks(ChatNewMessageString, ctx)
}

// EnqueueUserJoinHook registers a callback for the userJoin hook, fired when a
// user finishes joining a pad (see events.UserJoinLeaveContext).
func (h *Hook) EnqueueUserJoinHook(cb func(ctx *events.UserJoinLeaveContext)) string {
	return h.EnqueueHook(UserJoinString, func(ctx any) {
		if c, ok := ctx.(*events.UserJoinLeaveContext); ok {
			cb(c)
		}
	})
}

func (h *Hook) ExecuteUserJoinHooks(ctx *events.UserJoinLeaveContext) {
	h.ExecuteHooks(UserJoinString, ctx)
}

// EnqueueUserLeaveHook registers a callback for the userLeave hook, fired when a
// user disconnects from a pad (see events.UserJoinLeaveContext).
func (h *Hook) EnqueueUserLeaveHook(cb func(ctx *events.UserJoinLeaveContext)) string {
	return h.EnqueueHook(UserLeaveString, func(ctx any) {
		if c, ok := ctx.(*events.UserJoinLeaveContext); ok {
			cb(c)
		}
	})
}

func (h *Hook) ExecuteUserLeaveHooks(ctx *events.UserJoinLeaveContext) {
	h.ExecuteHooks(UserLeaveString, ctx)
}

// EnqueueOnAccessCheckHook registers a callback for the onAccessCheck hook, fired
// when access to a concrete pad is being checked via the socket; a callback may
// call Deny() to block access (see events.OnAccessCheckContext).
func (h *Hook) EnqueueOnAccessCheckHook(cb func(ctx *events.OnAccessCheckContext)) string {
	return h.EnqueueHook(OnAccessCheckString, func(ctx any) {
		if c, ok := ctx.(*events.OnAccessCheckContext); ok {
			cb(c)
		}
	})
}

func (h *Hook) ExecuteOnAccessCheckHooks(ctx *events.OnAccessCheckContext) {
	h.ExecuteHooks(OnAccessCheckString, ctx)
}

// EnqueueGetAuthorIdHook registers a callback for the getAuthorId hook, which lets
// plugins resolve or override the author id from a token (first non-empty answer
// wins; see events.GetAuthorIdContext).
func (h *Hook) EnqueueGetAuthorIdHook(cb func(ctx *events.GetAuthorIdContext)) string {
	return h.EnqueueHook(GetAuthorIdString, func(ctx any) {
		if c, ok := ctx.(*events.GetAuthorIdContext); ok {
			cb(c)
		}
	})
}

func (h *Hook) ExecuteGetAuthorIdHooks(ctx *events.GetAuthorIdContext) {
	h.ExecuteHooks(GetAuthorIdString, ctx)
}

// EnqueueAuthenticateHook registers a callback for the authenticate hook, fired
// during HTTP authentication before the built-in basic-auth check; the first
// callback to answer wins (see events.AuthenticateContext).
func (h *Hook) EnqueueAuthenticateHook(cb func(ctx *events.AuthenticateContext)) string {
	return h.EnqueueHook(AuthenticateString, func(ctx any) {
		if c, ok := ctx.(*events.AuthenticateContext); ok {
			cb(c)
		}
	})
}

func (h *Hook) ExecuteAuthenticateHooks(ctx *events.AuthenticateContext) {
	h.ExecuteHooks(AuthenticateString, ctx)
}

// EnqueueAuthorizeHook registers a callback for the authorize hook, fired during
// post-authentication authorization; Deny wins over any Grant, and the first
// Grant level is used (see events.AuthorizeContext).
func (h *Hook) EnqueueAuthorizeHook(cb func(ctx *events.AuthorizeContext)) string {
	return h.EnqueueHook(AuthorizeString, func(ctx any) {
		if c, ok := ctx.(*events.AuthorizeContext); ok {
			cb(c)
		}
	})
}

func (h *Hook) ExecuteAuthorizeHooks(ctx *events.AuthorizeContext) {
	h.ExecuteHooks(AuthorizeString, ctx)
}

// EnqueueAuthnFailureHook registers a callback for the authnFailure hook, fired
// when authentication fails; a callback may override the default 401 response by
// calling Respond (see events.AuthnFailureContext).
func (h *Hook) EnqueueAuthnFailureHook(cb func(ctx *events.AuthnFailureContext)) string {
	return h.EnqueueHook(AuthnFailureString, func(ctx any) {
		if c, ok := ctx.(*events.AuthnFailureContext); ok {
			cb(c)
		}
	})
}

func (h *Hook) ExecuteAuthnFailureHooks(ctx *events.AuthnFailureContext) {
	h.ExecuteHooks(AuthnFailureString, ctx)
}

// EnqueueAuthzFailureHook registers a callback for the authzFailure hook, fired
// when authorization fails; a callback may override the default 403 response by
// calling Respond (see events.AuthzFailureContext).
func (h *Hook) EnqueueAuthzFailureHook(cb func(ctx *events.AuthzFailureContext)) string {
	return h.EnqueueHook(AuthzFailureString, func(ctx any) {
		if c, ok := ctx.(*events.AuthzFailureContext); ok {
			cb(c)
		}
	})
}

func (h *Hook) ExecuteAuthzFailureHooks(ctx *events.AuthzFailureContext) {
	h.ExecuteHooks(AuthzFailureString, ctx)
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

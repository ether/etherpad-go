# Server-side hooks — Go-native plugin API

This document describes the hook system available to Go-native plugins in
etherpad-go. It covers conventions, the full hook reference, and code examples.

---

## Overview

### Plugin registration

Go-native plugins implement the `interfaces.EpPlugin` interface
(`lib/plugins/interfaces/epPlugin.go`):

```go
type EpPlugin interface {
    Name() string
    Description() string
    Init(store *EpPluginStore)
    SetEnabled(enabled bool)
    IsEnabled() bool
}
```

Hooks are registered inside `Init` via the `store.HookSystem` field, which is a
`*hooks.Hook`. Each hook has a typed `Enqueue<X>Hook` method that accepts a
callback:

```go
func (p *MyPlugin) Init(store *interfaces.EpPluginStore) {
    store.HookSystem.EnqueueClientVarsHook(func(ctx *events.ClientVarsContext) {
        ctx.Extra["myPluginVersion"] = "1.2.3"
    })
}
```

The `EpPluginStore` also exposes:

| Field               | Type                        | Purpose                          |
|---------------------|-----------------------------|----------------------------------|
| `Logger`            | `*zap.SugaredLogger`        | structured logging               |
| `HookSystem`        | `*hooks.Hook`               | hook registration                |
| `UIAssets`          | `embed.FS`                  | embedded front-end assets        |
| `PadManager`        | `*pad.Manager`              | pad CRUD operations              |
| `App`               | `*fiber.App`                | Fiber HTTP application           |
| `RetrievedSettings` | `*settings.Settings`        | parsed server settings           |

### Execution model

- Callbacks registered with `Enqueue<X>Hook` run **in registration order**
  (deterministic).
- Every registered callback is called; there is no early-exit after the first
  match.
- **Deny/drop wins**: for hooks that support `DropMessage()` or a deny
  decision, any single callback calling that method will suppress the action
  even if other callbacks do not.

### Import-cycle safety

The engine's core types (`*pad.Pad`, `*ws.Client`, and concrete WebSocket
message structs) would create an import cycle if referenced directly from
`lib/hooks/events`. They are therefore exposed as `any` on the context struct.
Plugins are leaf packages and may import those types, so they type-assert them:

```go
store.HookSystem.EnqueuePadLoadHook(func(ctx *events.PadLoadContext) {
    p := ctx.Pad.(*pad.Pad)   // safe: plugins can import lib/models/pad
    _ = p.Id
})
```

Cycle-safe types (e.g. `*clientVars.ClientVars`) are exposed concretely.

---

## Hook reference

### Pad lifecycle (Phase A)

#### `padDefaultContent`

| | |
|---|---|
| Enqueue | `EnqueuePadDefaultContentHook(cb func(*events.PadDefaultContentContext))` |
| Context type | `events.PadDefaultContentContext` |
| Fires | Before a new pad's initial revision is written |

**Context fields:**

| Field      | Type      | Notes                                                  |
|------------|-----------|--------------------------------------------------------|
| `PadId`    | `string`  | Pad identifier                                         |
| `Pad`      | `any`     | Type-assert to `*pad.Pad`                              |
| `AuthorId` | `*string` | Creating author; `nil` when no author is known         |
| `Type`     | `*string` | Content type; mutable                                  |
| `Content`  | `*string` | Initial text; mutable — set `*ctx.Content` to replace |

A callback can replace the pad's default text by writing to `*ctx.Content`.
The caller reads `Content` back after the hook runs.

---

#### `padCreate`

| | |
|---|---|
| Enqueue | `EnqueuePadCreateHook(cb func(*events.PadCreateContext))` |
| Context type | `events.PadCreateContext` |
| Fires | Right after a pad's first revision is persisted |

**Context fields:**

| Field      | Type     | Notes                                                             |
|------------|----------|-------------------------------------------------------------------|
| `PadId`    | `string` | Pad identifier                                                    |
| `Pad`      | `any`    | Type-assert to `*pad.Pad`                                         |
| `AuthorId` | `string` | Creating author; empty string when created without a known author |

Informational; no accumulator method.

---

#### `padLoad`

| | |
|---|---|
| Enqueue | `EnqueuePadLoadHook(cb func(*events.PadLoadContext))` |
| Context type | `events.PadLoadContext` |
| Fires | Whenever a pad is materialized from storage (create or load) |

**Context fields:**

| Field   | Type     | Notes                     |
|---------|----------|---------------------------|
| `PadId` | `string` | Pad identifier            |
| `Pad`   | `any`    | Type-assert to `*pad.Pad` |

Informational; no accumulator method.

---

#### `padUpdate`

| | |
|---|---|
| Enqueue | `EnqueuePadUpdateHook(cb func(*events.PadUpdateContext))` |
| Context type | `events.PadUpdateContext` |
| Fires | After a revision is appended to a pad |

**Context fields:**

| Field       | Type     | Notes                                         |
|-------------|----------|-----------------------------------------------|
| `PadId`     | `string` | Pad identifier                                |
| `Pad`       | `any`    | Type-assert to `*pad.Pad`                     |
| `AuthorId`  | `string` | Author who submitted the changeset            |
| `Revs`      | `int`    | New head revision number after this update    |
| `Changeset` | `string` | The changeset string that was applied         |

Informational; no accumulator method.

---

#### `padCopy`

| | |
|---|---|
| Enqueue | `EnqueuePadCopyHook(cb func(*events.PadCopyContext))` |
| Context type | `events.PadCopyContext` |
| Fires | After a pad is copied (`copyPad`, `copyPadWithoutHistory`, `movePad`) |

**Context fields:**

| Field    | Type     | Notes                       |
|----------|----------|-----------------------------|
| `SrcId`  | `string` | Source pad identifier       |
| `DstId`  | `string` | Destination pad identifier  |
| `SrcPad` | `any`    | Type-assert to `*pad.Pad`   |
| `DstPad` | `any`    | Type-assert to `*pad.Pad`   |

Informational; no accumulator method.

---

#### `padRemove`

| | |
|---|---|
| Enqueue | `EnqueuePadRemoveHook(cb func(*events.PadRemoveContext))` |
| Context type | `events.PadRemoveContext` |
| Fires | When a pad is deleted |

**Context fields:**

| Field   | Type     | Notes                     |
|---------|----------|---------------------------|
| `PadId` | `string` | Pad identifier            |
| `Pad`   | `any`    | Type-assert to `*pad.Pad` |

Informational; no accumulator method.

---

### Collab / client hooks (Phase B)

#### `handleMessage`

| | |
|---|---|
| Enqueue | `EnqueueHandleMessageHook(cb func(*events.HandleMessageContext))` |
| Context type | `events.HandleMessageContext` |
| Fires | Before every incoming socket message is dispatched, including `CLIENT_READY` |

**Context fields:**

| Field      | Type     | Notes                                     |
|------------|----------|-------------------------------------------|
| `PadId`    | `string` | Pad identifier                            |
| `AuthorId` | `string` | Session author                            |
| `Message`  | `any`    | Concrete ws message type; type-assert it  |
| `Client`   | `any`    | Type-assert to `*ws.Client`               |

**Accumulator methods:**

| Method          | Effect                                          |
|-----------------|-------------------------------------------------|
| `DropMessage()` | Prevents the message from being dispatched      |
| `Dropped() bool`| Reports whether any callback dropped the message|

Note: dropping a `CLIENT_READY` message leaves the session half-initialised
(auth/padId set, no `CLIENT_VARS` sent), which matches the general
message-interceptor semantics of the original etherpad-lite hook.

---

#### `handleMessageSecurity`

| | |
|---|---|
| Enqueue | `EnqueueHandleMessageSecurityHook(cb func(*events.HandleMessageSecurityContext))` |
| Context type | `events.HandleMessageSecurityContext` |
| Fires | When a write message (`UserChange`) arrives on a read-only connection |

**Context fields:**

| Field      | Type     | Notes                                    |
|------------|----------|------------------------------------------|
| `PadId`    | `string` | Pad identifier                           |
| `AuthorId` | `string` | Session author                           |
| `Message`  | `any`    | The `UserChange` message; type-assert it |

**Accumulator methods:**

| Method                    | Effect                                                |
|---------------------------|-------------------------------------------------------|
| `GrantWriteAccess()`      | Allows this single write message despite read-only    |
| `WriteAccessGranted() bool` | Reports whether access was granted                  |

If no callback calls `GrantWriteAccess()`, the message is silently dropped.

---

#### `clientReady`

| | |
|---|---|
| Enqueue | `EnqueueClientReadyHook(cb func(*events.ClientReadyContext))` |
| Context type | `events.ClientReadyContext` |
| Fires | After a client has fully joined the pad (after `userJoin`) |

**Context fields:**

| Field      | Type     | Notes                              |
|------------|----------|------------------------------------|
| `PadId`    | `string` | Pad identifier                     |
| `AuthorId` | `string` | Session author                     |
| `Token`    | `string` | Session auth token (may be empty)  |

Informational; no accumulator method.

---

#### `clientVars`

| | |
|---|---|
| Enqueue | `EnqueueClientVarsHook(cb func(*events.ClientVarsContext))` |
| Context type | `events.ClientVarsContext` |
| Fires | Just before the `CLIENT_VARS` payload is sent to a joining client |

**Context fields:**

| Field        | Type                         | Notes                                          |
|--------------|------------------------------|------------------------------------------------|
| `PadId`      | `string`                     | Pad identifier                                 |
| `AuthorId`   | `string`                     | Session author                                 |
| `ClientVars` | `*clientVars.ClientVars`     | Full typed payload; fields may be mutated      |
| `Extra`      | `map[string]any`             | Additional top-level keys to merge into the payload |

A callback may mutate fields on `ctx.ClientVars` directly (e.g.
`ctx.ClientVars.UserName`), and/or add arbitrary keys via `ctx.Extra`. On key
collision, the typed field in `ClientVars` wins — `Extra` cannot overwrite
engine-owned keys.

---

#### `chatNewMessage`

| | |
|---|---|
| Enqueue | `EnqueueChatNewMessageHook(cb func(*events.ChatNewMessageContext))` |
| Context type | `events.ChatNewMessageContext` |
| Fires | Before a user-originated chat message is stored and broadcast |

**Context fields:**

| Field      | Type      | Notes                                                    |
|------------|-----------|----------------------------------------------------------|
| `PadId`    | `string`  | Pad identifier                                           |
| `AuthorId` | `string`  | Author of the chat message                               |
| `Text`     | `*string` | Message text; mutate via `*ctx.Text = "..."` to rewrite  |
| `Message`  | `any`     | The chat message; type-assert to `ws.ChatMessageData`    |

**Accumulator methods:**

| Method          | Effect                                                |
|-----------------|-------------------------------------------------------|
| `DropMessage()` | Suppresses the message — not stored, not broadcast    |
| `Dropped() bool`| Reports whether any callback dropped the message      |

To rewrite the text, assign to `*ctx.Text`. The canonical form is
`*ctx.Text = newText`; reassigning the pointer (`ctx.Text = &newText`) also
works because the fire site reads `ctx.Text` back after all hooks run.

---

#### `userJoin` / `userLeave`

| | |
|---|---|
| Enqueue | `EnqueueUserJoinHook(cb func(*events.UserJoinLeaveContext))` |
| Enqueue | `EnqueueUserLeaveHook(cb func(*events.UserJoinLeaveContext))` |
| Context type | `events.UserJoinLeaveContext` |
| `userJoin` fires | After `CLIENT_VARS` is sent and the client has fully joined |
| `userLeave` fires | After a client disconnects and user-leave notifications have been sent |

**Context fields:**

| Field           | Type                        | Notes                                                                  |
|-----------------|-----------------------------|------------------------------------------------------------------------|
| `PadId`         | `string`                    | Pad identifier                                                         |
| `AuthorId`      | `string`                    | The joining or leaving author                                          |
| `BroadcastChat` | `func(message map[string]any)` | Helper — sends a chat message to all clients in the room without persisting it |

Both hooks share the same `events.UserJoinLeaveContext` type. `BroadcastChat`
is useful for posting join/leave announcements to the chat sidebar.

---

### Pre-existing hooks

These hooks were implemented before Phase A/B and are available for completeness:

| Hook name          | Enqueue method                    | Purpose                                                             |
|--------------------|-----------------------------------|---------------------------------------------------------------------|
| `preAuthorize`     | `EnqueuePreAuthorizeHook`         | Permit or deny a request before authentication runs; see `events.PreAuthorizeContext` |
| `preAuthzFailure`  | `EnqueuePreAuthzFailureHook`      | Override the default 403 after a `preAuthorize` deny; see `events.PreAuthzFailureContext` |
| `loadTranslations` | `EnqueueGetPluginTranslationHooks`| Supply plugin-specific i18n strings; see `events.LocaleLoadContext` |
| `getLineHTMLForExport` | `EnqueueGetLineHtmlForExportHook` | Customise per-line HTML during export (context passed as `any`)  |

Export format hooks (`getLinePDFForExport`, `getLineDocxForExport`,
`getLineOdtForExport`, `getLineMarkdownForExport`, `getLineTxtForExport`) are
defined in `lib/hooks/events/exportEvents.go`.

### Node.js / Express-only hooks

Hooks that depend on Express middleware, the Node.js `require` system, or the
etherpad-lite plugin manager (e.g. `expressConfigure`, `expressCreateServer`,
`pluginUninstall`) are intentionally **not** supported — there is no JavaScript
runtime.

---

## Code examples

### Adding an Extra key to `clientVars`

```go
package myplugin

import (
    "github.com/ether/etherpad-go/lib/hooks/events"
    "github.com/ether/etherpad-go/lib/plugins/interfaces"
)

type MyPlugin struct{ enabled bool }

func (p *MyPlugin) Name() string        { return "ep_myplugin" }
func (p *MyPlugin) Description() string { return "Example plugin" }
func (p *MyPlugin) SetEnabled(v bool)   { p.enabled = v }
func (p *MyPlugin) IsEnabled() bool     { return p.enabled }

func (p *MyPlugin) Init(store *interfaces.EpPluginStore) {
    // clientVars: attach plugin metadata for the browser client
    store.HookSystem.EnqueueClientVarsHook(func(ctx *events.ClientVarsContext) {
        ctx.Extra["ep_myplugin"] = map[string]any{
            "version": "1.0.0",
            "feature": true,
        }
    })

    // chatNewMessage: prefix every chat message with "[bot]"
    store.HookSystem.EnqueueChatNewMessageHook(func(ctx *events.ChatNewMessageContext) {
        if ctx.Text != nil {
            rewritten := "[bot] " + *ctx.Text
            *ctx.Text = rewritten
        }
    })
}
```

### Dropping a chat message

```go
store.HookSystem.EnqueueChatNewMessageHook(func(ctx *events.ChatNewMessageContext) {
    if ctx.Text != nil && strings.Contains(*ctx.Text, "spam") {
        ctx.DropMessage()
    }
})
```

### Reacting to user join with a chat announcement

```go
store.HookSystem.EnqueueUserJoinHook(func(ctx *events.UserJoinLeaveContext) {
    ctx.BroadcastChat(map[string]any{
        "text": ctx.AuthorId + " joined the pad",
    })
})
```

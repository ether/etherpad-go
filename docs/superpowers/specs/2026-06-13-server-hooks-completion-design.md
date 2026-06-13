# Server-side hook system completion — design

**Date:** 2026-06-13
**Status:** Approved (pending spec review)
**Scope:** Bring etherpad-go's server-side hook system up to meaningful parity with
etherpad-lite for **Go-native (compiled-in) plugins**. Node.js plugins are explicitly
out of scope — there is no JS runtime in Go and no attempt to support one.

## Goal

etherpad-lite exposes 20+ server-side hooks plus client-side JS hooks. etherpad-go
currently *fires* a subset via raw `ExecuteHooks("key", ctx)` calls but only provides
typed wrappers for a few. This work:

1. Gives every already-fired hook a typed, ergonomic, compile-checked wrapper.
2. Adds the genuinely missing high-value hooks (collab/client, auth/access,
   import/export, lifecycle) so Go-native plugins can actually influence behavior.
3. Makes hook execution **deterministic** (registration order).

Node/Express/EJS/dynamic-plugin-specific hooks are intentionally **excluded** because
they have no meaning in Go's Fiber + templ + compiled-plugin architecture:
`eejsBlock_*`, `pluginInstall`, `pluginUninstall`, `init_<plugin>`, `expressPreSession`,
`expressConfigure`, `expressCreateServer`, `expressCloseServer`, `createServer`,
`restartServer`, `socketio`.

## Current state (verified)

- `lib/hooks/hook.go` — `Hook` struct stores callbacks in
  `map[string]map[string]func(ctx any)` (keyed by hook name → uuid → callback).
  `EnqueueHook`/`DequeueHook`/`ExecuteHooks` are the generic primitives; a handful of
  typed wrappers exist (`preAuthorize`, `preAuthzFailure`, `getLineHTMLForExport`,
  `loadTranslations`).
- Hook contexts live in `lib/hooks/events/`.
- Plugins implement `interfaces.EpPlugin` and register in
  `Init(store *interfaces.EpPluginStore)` via `store.HookSystem.Enqueue…Hook(...)`.
- Already *fired* via raw `ExecuteHooks` (no typed wrapper): `padDefaultContent`,
  `padCreate`, `padLoad`, `padUpdate`, `padCopy`, `padRemove`, `userJoin`, `userLeave`.
- The auth pipeline in `lib/pad/webaccess.go` (`CheckAccessWithHooks`) already has
  commented insertion points for the missing auth hooks; the 401/403 fallbacks mark
  exactly where `authnFailure`/`authzFailure` slot in.
- Single message dispatch choke point: `PadMessageHandler.HandleMessage(message any, …)`
  at `lib/ws/PadMessageHandler.go:413`; the type switch is at line 536; chat handling is
  the `case ws.ChatMessage` branch.

### Import-cycle constraints (verified)

- `lib/ws` imports `lib/hooks` → `lib/hooks/events` **must not** import `lib/ws`.
  Therefore `handleMessage`, `handleMessageSecurity`, `clientReady`, and
  `chatNewMessage` contexts expose the message/socket objects as `any`; plugins
  type-assert (mirrors how lite passes the raw message).
- `lib/models/pad` imports `lib/hooks` → pad-lifecycle contexts expose the pad as
  `any`; plugins type-assert to `*pad.Pad` (plugins are leaf packages and may import it).
- `lib/settings/clientVars` does **not** import `lib/hooks` → the `clientVars` context
  **may** hold a concrete `*clientVars.ClientVars`.

## Design

### Conventions (uniform across all hooks)

- **Context location:** every hook context type lives in `lib/hooks/events/`.
- **Cycle rule:** engine objects that would create an import cycle (`ws.*` messages,
  `models/pad.Pad`) are exposed as `any`; cycle-safe types (`clientVars.ClientVars`)
  are exposed concretely.
- **Decision pattern:** read-only input fields + accumulator methods
  (`Permit`/`Deny`/`Respond`/`DropMessage`/`GrantWriteAccess`/…) + a `Decision()` or
  getter, exactly like the existing `events.PreAuthorizeContext`. Aggregation is
  **"deny/drop wins"**: a single deny or drop overrides any number of permits.
- **Typed wrappers:** every hook gets `EnqueueXxxHook(cb func(ctx *events.XxxContext))`
  and `ExecuteXxxHooks(ctx *events.XxxContext)` on `*hooks.Hook`.
- **Naming:** hook string constants live in `lib/hooks/HookConstants.go`.

### 1. Core change — deterministic ordering

Replace the inner `map[string]func` with an ordered slice so registration order is the
execution order:

```go
type hookEntry struct {
    id string
    fn func(ctx any)
}
type Hook struct {
    hooks map[string][]hookEntry
}
```

- `EnqueueHook(key, fn)` — append a new `hookEntry` (generate uuid), return the id.
- `DequeueHook(key, id)` — filter the slice removing the matching id.
- `ExecuteHooks(key, ctx)` — iterate the slice in order, calling each `fn`.

All existing typed wrappers call these primitives and require **no changes**. This is the
only change to the core `Hook` type; it removes the current nondeterministic
map-iteration order, which matters for `handleMessage` (drop/stop), `clientVars` (mutating
a shared struct/map), and `chatNewMessage`.

### 2. Category A — typed wrappers for already-fired hooks

No new fire sites. For each of `padDefaultContent`, `padCreate`, `padLoad`, `padUpdate`,
`padCopy`, `padRemove`, `userJoin`, `userLeave`:

- Define an `events.<Name>Context` carrying the existing data fields, with the pad exposed
  as `any` (field name `Pad any`, plus a typed `PadId string`).
- Add `EnqueueXxxHook` / `ExecuteXxxHooks` typed wrappers.
- Update the existing raw fire site to construct and pass the `events` context type instead
  of the inline `models/pad` struct.
- `padDefaultContent` keeps its current behavior: the callback mutates `ctx.Content`, which
  the caller reads back after `Execute`.

`userJoin`/`userLeave` already have `events.UserJoinLeaveContext`; this just adds the typed
`Enqueue`/`Execute` wrappers (currently fired via raw string key).

### 3. Category B — collab/client hooks (new fire sites)

All fire sites in `lib/ws/PadMessageHandler.go`. Messages/sockets exposed as `any`.

| Hook | Fire site | Context (key fields) | Plugin can |
|---|---|---|---|
| `handleMessage` | top of `HandleMessage` (line 413), before the type switch | `Message any`, `Client any`, `PadId string`, `AuthorId string` | `DropMessage()` to stop further dispatch |
| `handleMessageSecurity` | the write-access check for edit messages | `Message any`, `PadId string`, `AuthorId string` | `GrantWriteAccess()` for this single message |
| `clientReady` | inside `HandleClientReadyMessage` | `PadId string`, `AuthorId string`, `Token string` (read-only) | react (informational only) |
| `clientVars` | just before the clientVars payload is sent (~1425–1443) | `ClientVars *clientVars.ClientVars`, `Extra map[string]any`, `PadId string`, `AuthorId string` | mutate typed `ClientVars` fields and/or add arbitrary keys via `Extra` |
| `chatNewMessage` | the `case ws.ChatMessage` branch, before store/broadcast | `Message any`, `Text *string` (mutable), `PadId string`, `AuthorId string` | edit chat text; `DropMessage()` to suppress |

**`clientVars` Extra-map merge:** the clientVars JSON payload is built from the typed
`*clientVars.ClientVars`. Keys present in `ctx.Extra` are merged into the outgoing JSON
object alongside the typed fields at serialization time. On key collision, the typed field
wins (Extra cannot silently clobber engine-owned keys). This requires a small change to the
clientVars send path to perform the merge (e.g. marshal struct → map, overlay Extra for
keys not already present, marshal map).

`handleMessage` drop semantics: if any callback calls `DropMessage()`, `HandleMessage`
returns early without running the type switch.

### 4. Category C — auth/access hooks

Wired into `lib/pad/webaccess.go` (`CheckAccessWithHooks`) at the already-commented points,
plus the socket pad-access path. All use the established `Decision()` / `Respond()` idiom.

| Hook | Purpose | Context / decision |
|---|---|---|
| `onAccessCheck` | pad-level access decision (http + socket) | `Permit()`/`Deny()`, deny-wins, like `preAuthorize` |
| `getAuthorId` | let a plugin supply/override the author id from a token | input `Token string`; plugin sets `AuthorId` (first non-empty wins) |
| `authenticate` | custom authentication | inputs: username, password, headers; `Authenticate(username string)` sets the authenticated user (first wins); no answer defers to built-in basic-auth |
| `authorize` | custom authorization / grant level | `Grant(level string)` / `Deny()`, deny-wins; level normalized via `NormalizeAuthzLevel` |
| `authnFailure` | override the 401 when authentication fails | `Respond(status, body)` + `SetHeader`, like `preAuthzFailure`; default 401 if unhandled |
| `authzFailure` | override the 403 when authorization fails | `Respond(status, body)` + `SetHeader`; default 403 if unhandled |

**Dropped:** `authFailure` (lite's combined legacy fallback) — redundant given
`authnFailure`/`authzFailure`.

### 5. Category D — import/export & lifecycle extras

| Hook | Fire site | Notes |
|---|---|---|
| `exportConvert` | export pipeline | let a plugin take over format conversion |
| `stylesForExport` | HTML/export style assembly | plugin contributes CSS |
| `exportFileName` | export filename construction | plugin overrides the download filename |
| `exportHTMLAdditionalContent` | HTML export body assembly | plugin injects extra HTML |
| `exportHTMLSend` | just before HTML export response | plugin can take over the send |
| `import` | importer entry | plugin handles a format |
| `importEtherpad` | `.etherpad` import path | plugin contributes extra records |
| `loadSettings` | after settings are loaded at startup | plugin reacts to / inspects settings |
| `shutdown` | graceful shutdown path | plugin cleanup |

**Dropped:** `padCheck` — out of scope by decision.

### 6. Testing

Unit tests in `lib/hooks` (and adjacent packages for fire-site behavior), following the
existing test style:

- Registration + ordered execution (registration order is honored).
- Decision aggregation: deny-wins for `onAccessCheck`/`authorize`/`preAuthorize`-style;
  drop-stops for `handleMessage`/`chatNewMessage`.
- `clientVars` `Extra` merge, including the collision rule (typed field wins).
- `DequeueHook` removes the right entry and preserves order of the rest.
- Auth-hook fire-site behavior: `authenticate`/`authorize` permit/deny paths, and
  `authnFailure`/`authzFailure` overriding the default 401/403.

### 7. Docs

- Update `doc/` with the list of supported server-side hooks, their contexts, and the
  decision semantics.
- Document the convention that engine objects (`ws` messages, `*pad.Pad`) are exposed as
  `any` and plugins type-assert to the concrete type.

## Implementation phasing

Each phase is independently shippable and testable:

- **Phase 0** — core ordering change (`hook.go`) + tests.
- **Phase A** — typed wrappers for already-fired hooks; migrate fire sites to `events` types.
- **Phase B** — collab/client hooks (`handleMessage`, `handleMessageSecurity`,
  `clientReady`, `clientVars` + Extra merge, `chatNewMessage`).
- **Phase C** — auth/access hooks wired into `webaccess.go` + socket access path.
- **Phase D** — import/export + lifecycle extras.

## Non-goals

- Running Node.js/JS plugins.
- Express/EJS/dynamic-plugin-install hooks (listed under "excluded" above).
- Client-side JS hooks (this spec is server-side only).
- `authFailure`, `padCheck`.

# Server Hooks — Phase D (import/export + lifecycle) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) tracking.

**Goal:** Add the document-level export/import hooks and the lifecycle hooks, completing the server-hook system. Build graceful shutdown infrastructure (which etherpad-go lacks today) so the `shutdown` hook can fire.

**Architecture:** Same conventions as Phases A–C — contexts in `lib/hooks/events/`, constants in `HookConstants.go`, typed `Enqueue/Execute` wrappers on `*hooks.Hook`, engine objects exposed as `any`. The Go hook system uses mutable-context accumulators (no return values), so lite's `aCallAll`/`aCallFirst` map to: accumulate-all (concat) or first-answer-wins, deterministic by registration order (Phase 0).

Design spec: `docs/superpowers/specs/2026-06-13-server-hooks-completion-design.md`. Builds on Phases 0/A (#293), B (#294), C (#295), all merged or open.

**Dropped:** `exportConvert` — etherpad-go builds every export format in-memory (no soffice/external converter), so there is no conversion step to intercept.

## Per-hook contract (Go adaptation of lite semantics)

| Hook | Fires in | Aggregation | Context (events/*) |
|---|---|---|---|
| `exportFileName` | `lib/io/exportEtherpad.go` `DoExport`, before `ctx.Attachment(...)` | first-answer | `ExportFileNameContext{PadId, ReadOnlyId, ExportType string}`; `SetFileName(name)` (first wins) / `FileName()` |
| `stylesForExport` | `lib/io/exportHtml.go` `GetPadHTMLDocument`, before `ExportTemplate` | accumulate-all | `StylesForExportContext{PadId string}`; `AddStyle(css)` / `Styles() string` (joined) |
| `exportHTMLAdditionalContent` | same, appended to body before template | accumulate-all | `ExportHTMLAdditionalContentContext{PadId string}`; `Add(html)` / `Content() string` |
| `exportHTMLSend` | `DoExport`, just before `ctx.SendString(htmlContent)` for the HTML branch | first-answer (replace) | `ExportHTMLSendContext{PadId string, HTML *string}`; mutate `*HTML` (read back after) |
| `import` | `lib/api/io/importHandler.go` `doImport`, before the file-extension switch | accumulate-all, any-handled skips built-in | `ImportContext{FileEnding, PadId, AuthorId string, Content []byte}`; `Handle()` + optional `SetHTML(html)`/`SetText(text)` / `Handled()` |
| `importEtherpad` | `lib/io/importer.go` `SetPadRaw`, after parsing the `.etherpad` JSON, before/around DB writes | accumulate-all | `ImportEtherpadContext{PadId, SrcPadId string, Data map[string]any}` (parsed records; plugins may inspect/augment the map) |
| `loadSettings` | `lib/server/server.go` `InitServer`, **after `plugins.InitPlugins`** (line 96) | accumulate-all (notify) | `LoadSettingsContext{Settings any}` (type-assert `*settings.Settings`) |
| `shutdown` | `InitServer`, on SIGINT/SIGTERM before `app.Shutdown` | accumulate-all (notify) | `ShutdownContext{}` |

## Files

- `lib/hooks/HookConstants.go` — 8 constants.
- `lib/hooks/events/exportImport.go` — **new**: 6 export/import contexts.
- `lib/hooks/events/lifecycle.go` — **new**: `LoadSettingsContext`, `ShutdownContext`.
- `lib/hooks/hook.go` — 16 typed wrappers.
- `lib/hooks/hook_test.go` — unit tests for the accumulator semantics.
- `lib/io/exportEtherpad.go` — fire `exportFileName` + `exportHTMLSend` in `DoExport`.
- `lib/io/exportHtml.go` — fire `stylesForExport` + `exportHTMLAdditionalContent` in `GetPadHTMLDocument` (route collected CSS into `ExportTemplate`'s styles slot — read its signature; the 2nd arg is currently `""`).
- `lib/api/io/importHandler.go` — plumb a `*hooks.Hook` into `ImportHandler` (`NewImportHandler` + its caller in `lib/api/io/init.go`); fire `import` before the extension switch (any-handled → skip built-in + import the plugin-provided HTML/text).
- `lib/io/importer.go` — plumb `*hooks.Hook` into `Importer` (`NewImporter` + caller in `server.go:79`); fire `importEtherpad` in `SetPadRaw`.
- `lib/server/server.go` — fire `loadSettings` after `plugins.InitPlugins`; add graceful-shutdown (run `app.Listen` in a goroutine, wait on `signal.Notify(SIGINT,SIGTERM)`, fire `shutdown`, then `app.ShutdownWithTimeout(3s)`).
- `lib/test/...` — tests (export_handler_test.go for export hooks; importHandler tests; a focused server/lifecycle test where feasible).
- `doc/hooks_server-side.md` — document the 8 hooks.

## Reuse (don't reinvent)

- Existing per-line export hooks (`getLineHTMLForExport` etc.) show the in-exporter firing pattern; `ExportEtherpad`/`ExportHtml` already hold `*hooks.Hook`.
- The accumulate-all `AddStyle`/`Add`/joined-getter and first-answer `SetFileName` mirror the `clientVars.Extra`/`PreAuthorizeContext` idioms.
- `ExportHTMLSendContext.HTML *string` mirrors `ChatNewMessageContext.Text` (mutate-and-read-back; nil-guard on read).
- `LoadSettingsContext.Settings any` avoids an events→settings import cycle (consistent with the "engine objects as any" rule).
- Fiber v3 `app.ShutdownWithTimeout(d)` for graceful shutdown.

## Tasks (TDD → spec review → code-quality review → commit; subagent-driven)

1. **lib/hooks layer (additive):** 8 constants, the 2 new events files (8 contexts), 16 wrappers, unit tests (first-answer for exportFileName/exportHTMLSend; accumulate/concat for styles/additional-content; any-handled for import; notify for load/shutdown). No fire sites.
2. **Export hooks:** wire `exportFileName` + `exportHTMLSend` (exportEtherpad.go `DoExport`) and `stylesForExport` + `exportHTMLAdditionalContent` (exportHtml.go `GetPadHTMLDocument`). Tests via `lib/test/api/io/export_handler_test.go` (register hooks; assert filename header, injected CSS/content present, send-replacement applied).
3. **Import hooks:** plumb `*hooks.Hook` into `ImportHandler` + `Importer` (constructors + their callers); fire `import` (before the extension switch; any-handled → use plugin HTML/text + skip built-in) and `importEtherpad` (in `SetPadRaw`). Tests: a plugin `import` hook handles a custom extension; an `importEtherpad` hook observes the parsed data.
4. **Lifecycle:** fire `loadSettings` after `plugins.InitPlugins`; add graceful shutdown (signal handling + `app.ShutdownWithTimeout`) firing `shutdown`. Tests: a focused test that registering a loadSettings hook receives the settings (call the same firing path), and that the shutdown hook is invoked by the shutdown routine (extract the shutdown sequence into a testable function if needed).
5. **Docs + full verification:** document the 8 hooks in `doc/hooks_server-side.md`; run the full affected suite (`lib/hooks`, `lib/io`, `lib/api/io`, `lib/test/api/io`, `lib/test/ws`).

## Risk notes / decisions

- **`import` Go adaptation:** lite converts the upload to an HTML temp file and returns true. Go has no temp-file/HTML-file step, so the hook lets a plugin either fully handle the import or hand back converted HTML/text (`SetHTML`/`SetText`) that core imports via the existing `importHTML`/`importText`. Keep it minimal and testable; this is the fuzziest mapping.
- **`importEtherpad` is observational/light-mutation** of the parsed `Data` map. lite's full prefix-based extra-record contribution model (`exportEtherpadAdditionalContent` + temporary `pad.db`) is **out of scope** (no Go plugin needs it; `exportEtherpadAdditionalContent` isn't in this hook set). Document the limitation.
- **Graceful shutdown is new infrastructure**, reusable by the future self-update/drain feature. Keep the signal-handling minimal: `app.Listen` in a goroutine, block on signal, fire `shutdown`, `app.ShutdownWithTimeout(3*time.Second)` (matches lite's 3s budget), then return.
- **Hook-map synchronization** (flagged across A/B/C reviews) remains a separate follow-up; still safe because registration is startup-only.

## Branch

New branch `feat/server-hooks-phase-d` off `main` (Phase C #295 is independent of Phase D files except `lib/hooks/*` and `doc/hooks_server-side.md`; if #295 hasn't merged, stack on `feat/server-hooks-phase-c` to avoid overlap conflicts in those two files). PR targets the appropriate base so the diff shows only Phase D.

## Verification

- `go build ./...` + `go vet` clean.
- `go test ./lib/hooks/` (accumulator semantics).
- `go test ./lib/test/api/io/` (export filename/styles/content/send; import custom-format + importEtherpad).
- Lifecycle: loadSettings fires after plugin registration; shutdown hook invoked on the shutdown path.
- `go test ./lib/test/ws/` green (no regression).

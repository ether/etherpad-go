# Excel-comparable collaborative spreadsheet — design

**Date:** 2026-07-01
**Status:** Approved (scope + sequencing confirmed by user)
**Builds on:** collaborative spreadsheet (PR #306) and sheet live-presence (PR #312).

## Goal

Take the existing collaborative spreadsheet from "cells + formulas + presence" to
something a user would call comparable with Excel: multi-cell selection and
clipboard, real cell formatting, a formula bar, and structural editing (multiple
sheets, resize, freeze, sort/filter).

The work is decomposed into four dependency-ordered milestones, each independently
shippable and reviewable. This document is the shared design; each milestone gets
its own section of the implementation plan.

## What already exists (do not rebuild)

- **OT collab core** (Go `lib/sheet` + TS mirror `ui/src/js/sheet`): ops `setCell`,
  `setStyle`, `clearRange`, `insertRows`, `deleteRows`, `insertCols`, `deleteCols`,
  with rebase/transform, per-document serialization goroutine, snapshot + op-log
  persistence, reconnect via `OpsSince`.
- **Formula engine**: HyperFormula (client-side, GPLv3), ~400 functions, wrapped in
  `formulaEngine.ts`. Raw string is the source of truth; computed value derived.
- **Presence**: per-user cursor + live-edit overlay via one ephemeral `SHEET_PRESENCE`
  frame each direction (`sheetPresence.ts`, `SheetHandler.HandlePresence`).
- **Style model**: `StylePool` (dedup by canonical key, id 0 = empty) + `Cell.StyleId`
  + `setStyle` op. **Gap:** no wire path to *define* a style's props — see M2.
- **Multi-sheet model**: `Workbook.Sheets []*Sheet` with `AddSheet`; snapshot carries
  all sheets. **Gap:** no ops to add/rename/delete/reorder sheets; UI shows only the
  first sheet.
- **View**: `DomSheetView` — a 50×20 contenteditable HTML `<table>`, deliberately
  swappable behind the collab layer. Renders plain text only (no styling).

## Architectural decisions

1. **Keep the contenteditable HTML-table view; extend it.** Do not rewrite as a
   canvas/virtualized grid in this initiative. Virtualization is the logged upgrade
   path (see M4 ceiling). Rationale: the view is already isolated behind the collab
   contract, and a rewrite risks the working collab/presence wiring for no near-term
   payoff.

2. **Server is the sole style-id authority.** Clients send style *props*; the server
   interns them and echoes the resolved `{styleId, props}`. Clients never assign ids.
   This keeps `StylePool` deterministic across peers without a separate sync channel.

3. **Out of scope** (each its own future initiative; upgrade paths noted, nothing
   built now): charts, pivot tables, conditional formatting, data validation, macros,
   cell comments, cross-sheet formula references beyond what HyperFormula gives for
   free on a single active sheet.

---

## M1 — Range selection + clipboard

The interaction foundation every later milestone operates on.

### Components
- **`sheetSelection.ts`** (new): a `Selection { anchor: {row,col}, focus: {row,col} }`
  model with helpers `normalize()` (top-left/bottom-right), `contains(r,c)`,
  `cells()`, `isSingle()`. Pure, unit-tested.
- **`DomSheetView`**: track selection; render a highlight (background tint on
  in-range cells + a heavier border on the range outline + the anchor cell keeps the
  focus ring). Selection is driven by:
  - mousedown + drag (mousemove while button held) → anchor fixed, focus follows;
  - shift+click → extend focus to clicked cell;
  - shift+arrow → extend focus; plain arrow → move both (single-cell);
  - Ctrl/Cmd+A → select the used range.
- **Clipboard** (`sheetClipboard.ts`, new):
  - Copy/Cut: serialize the selected range's *raw* values as TSV (tab between cols,
    newline between rows) and write via `navigator.clipboard.writeText`. Cut also
    clears the range after a successful copy (via `clearRange`).
  - Paste: read `navigator.clipboard.readText`, parse TSV into a grid, write as a
    batch of `setCell` ops anchored at the active cell. TSV means real-Excel
    interop both directions.
  - A fallback path via a hidden textarea + `copy`/`paste` events for browsers/HTTP
    contexts where the async Clipboard API is unavailable.
- **Fill handle**: a small square at the bottom-right of the selection outline.
  Dragging it extends the selection and fills: copy pattern from the source range,
  adjusting relative formula refs (column letters / row numbers) by the offset.
  Absolute refs (`$A$1`) are left unchanged. Fill emits a batch of `setCell` ops.
- **Delete/Backspace** on a multi-cell selection → single `clearRange` op; on a
  single cell → `setCell` with empty raw (existing behavior).

### Collaboration
Copy/paste/fill produce ordinary `setCell`/`clearRange` ops through the existing
pipeline — no new op types, no server change. A large paste is many `setCell` ops
sent back-to-back; the client applies them optimistically and the server serializes
them in order (acceptable for M1; batching is a noted optimization, not built).

Selection range is added to the existing `SHEET_PRESENCE` frame as optional
`focusRow/focusCol` (anchor stays `row/col`); peers render a translucent range
outline in the sender's color. Read-only sessions still send selection. No new frame.

### Testing
- `sheetSelection.test.ts`: normalize, contains, cells enumeration, single detection.
- `sheetClipboard.test.ts`: TSV round-trip (serialize→parse), paste-grid → op batch,
  fill relative-ref adjustment (`=A1` filled down becomes `=A2`; `=$A$1` unchanged).

---

## M2 — Cell formatting

The visual payoff. Turns the existing (inert) style model into real formatting.

### Wire change (the core of this milestone)
- Add an optional `Props map[string]string` to the Go `Op` (`op.go`) and TS `Op`
  (`op.ts`), used by `setCell` and `setStyle`.
- Server (`apply.go` / the submit path): when an op carries `Props`, intern them via
  `StylePool.Put` to obtain a `styleId`, set that on the op's resolved form, and set
  the cell's `StyleId`. The **rebased broadcast op** carries BOTH the resolved
  `styleId` and the `props` map. New clients get the full pool in the snapshot;
  live clients receiving a broadcast op record `pool.IdToStyle[styleId] = props`
  directly (no local `Put`, so ids never diverge from the server's).
- Merge semantics: applying a partial style (e.g. just `{bold:"1"}`) to a cell that
  already has a style **merges** onto the existing props (bold added, existing color
  kept), then interns the merged result. This is what a toolbar toggle expects.
  Clearing a property sends the merged map without that key.
- `setStyle`/`setCell` validation updated: `setStyle` needs `styleId` **or** `props`.

### Style prop vocabulary (string→string, format-agnostic pool unchanged)
`bold`, `italic`, `underline` (`"1"`/absent); `color` (font, hex); `bg` (fill, hex);
`align` (`left`/`center`/`right`); `border` (`all`/`top`/`bottom`/`left`/`right`/
`none`, one edge-set string); `numFmt` (`general`/`number`/`currency`/`percent`/
`date`/`text`, optionally with decimals like `number:2`).

### UI
- **Toolbar** (`sheetToolbar.ts`, new) above the grid: B / I / U toggle buttons, font
  color + fill color swatches (native `<input type="color">`), align group, border
  dropdown, number-format dropdown. Acts on the current selection (M1): builds the
  merged props and sends one `setStyle` op per cell in range (or a future batched op;
  per-cell for M2).
- **Rendering** (`sheetView.render`): resolve each populated cell's `styleId` →
  props → inline CSS (fontWeight, fontStyle, textDecoration, color, background,
  textAlign, borders). Applied alongside existing text/decoration painting.
- **Number formatting on display**: a `formatValue(value, valueType, numFmt)` helper
  using `Intl.NumberFormat` (number/currency/percent) and a small date formatter;
  `general` keeps HyperFormula's native output; `text` shows raw. Only affects
  display, never the stored raw.

### Testing
- Go `style_test.go` / `apply_test.go`: op with props interns + assigns id; partial
  props merge onto existing style; broadcast op carries resolved id + props;
  convergence test with two clients applying overlapping style ops.
- TS: `workbookState` records broadcast `{styleId,props}` into pool; `formatValue`
  cases (currency, percent, decimals, date, text).

---

## M3 — Formula bar + formula UX

### Components
- **Formula bar** (`sheetFormulaBar.ts`, new) above the toolbar: a **Name Box**
  (shows the active cell ref, e.g. `B7`, or range `A1:C5`) + a text input showing the
  active cell's raw content. Two-way sync with the view:
  - selecting a cell fills the bar with its raw value;
  - typing in the bar + Enter commits via the same `setCell` path as in-cell edit,
    and drives the live-edit presence frame so peers see bar edits too;
  - Escape reverts.
- **Autocomplete**: as the user types `=FUN`, suggest from HyperFormula's registered
  function names (`getRegisteredFunctionNames`), arrow-key select, Tab/Enter accept.
  A lightweight dropdown anchored to the bar/cell; no dependency added.
- **Error display**: HyperFormula already returns error values (`#DIV/0!`, `#NAME?`,
  `#REF!`, `#VALUE!`, …). `formulaEngine` exposes the error type; the view renders
  error cells in a distinct style and shows the error detail on hover (title attr).

### Testing
- `sheetFormulaBar.test.ts`: bar reflects active cell; commit path calls onEdit with
  the typed raw; Escape reverts.
- Autocomplete filter: prefix match against a stubbed function-name list.

---

## M4 — Structural / grid polish

### Multiple sheets
- New ops (Go + TS): `addSheet` (id, name, index), `renameSheet` (id, name),
  `deleteSheet` (id), `moveSheet` (id, toIndex). Add to `OpType`, `Validate`,
  `Apply`/`applyOp`, and `Transform` (structural-vs-structural: sheet ops commute
  with cell ops trivially since they touch the sheet list, not cells; concurrent
  add/delete of the same id resolved last-writer/no-op).
- **Tabs bar** (`sheetTabs.ts`, new) at the bottom: one tab per sheet, active
  highlight, `+` to add, double-click to rename, right-click/context to delete,
  drag to reorder. Switching tabs re-points `activeSheetId` and re-renders (the
  workbook already holds all sheets client-side).
- Deleting the last sheet is disallowed (validation).

### Column/row resize
- New op `setDimension` (sheet, axis=`col`/`row`, index, sizePx). Stored as sheet
  metadata `ColWidths map[int]int` / `RowHeights map[int]int` (snapshot-extended,
  sparse). Structural insert/delete row/col ops shift these maps too.
- View: draggable borders on the header cells; drag emits `setDimension`; render
  applies widths/heights as inline styles.

### Freeze panes
- Freeze row 1 and/or col A (a simple, common case; arbitrary freeze is the upgrade
  path). Stored as per-doc metadata (`FrozenRows int`, `FrozenCols int`) via a
  `setFreeze` op so it converges for all viewers. View uses `position: sticky` on the
  frozen header row/col — a native CSS feature, no scroll-sync JS.

### Sort / filter
- **Sort**: a header/context action on a selected range → reorder rows by a chosen
  column, emitted as a batch of `setCell` ops (data moves; formulas adjust via the
  same relative-ref logic as fill). Client-computed, server just sees cell ops.
- **Filter**: a view-only row-hide toggle per column (auto-filter dropdown of
  distinct values). Filter state is client-local (not collaborative) in M4;
  collaborative filter is the upgrade path.

### Grid size + ceiling
- Grow the rendered grid to ~200 rows × 52 cols (A–AZ).
- **Ceiling (ponytail):** every cell is a DOM node, so very large grids get heavy.
  Virtualization (render only the visible viewport) is the upgrade path when row
  counts hurt; not built here. Logged as a `ponytail:` comment in the view.

### Testing
- Go: sheet-list ops (`addSheet`/`rename`/`delete`/`move`) apply + validate +
  transform + convergence; `setDimension`/`setFreeze` apply; dimension maps shift
  under row/col insert/delete.
- TS: tabs switch active sheet; resize emits `setDimension`; freeze renders sticky;
  sort produces the expected `setCell` batch; filter hides rows without emitting ops.

## Cross-cutting concerns

- **Read-only sessions**: may select, copy, scroll, switch sheets, and see formatting
  — but every op-producing action (paste, fill, format, structural, sort) is blocked
  client-side and rejected server-side (the existing `session.ReadOnly` guard already
  covers ops; extend the guard list for the new ops).
- **Persistence**: new ops flow through the existing op-log + snapshot; snapshot
  gains `colWidths`/`rowHeights`/`frozenRows`/`frozenCols` fields (all `omitempty`).
  Style pool already persists.
- **xlsx import/export**: existing import/export (`sheetdoc`) should preserve the new
  style props and sheet/dimension metadata where the xlsx library supports it;
  verifying and extending that mapping is part of M2 (styles) and M4 (sheets/dims).
- **Build**: sheet bundle still built with `vite build --mode sheet` (the rolldown +
  commonjs workaround for HyperFormula stays); new files are plain TS modules.

## Milestone independence

M1 ships without M2–M4 (selection + clipboard on the current plain grid). M2 ships on
top of M1 (formatting the selection). M3 is largely independent but assumes the active
cell/selection from M1. M4 is additive. Each milestone is a reviewable unit with its
own tests, and the plan will treat them as separate phases with checkpoints.

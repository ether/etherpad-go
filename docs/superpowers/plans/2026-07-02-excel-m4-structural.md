# M4 — Structural / grid polish (multiple sheets, resize, freeze, sort/filter)

Implements the M4 section of `docs/superpowers/specs/2026-07-01-excel-spreadsheet-design.md`.
Branch `feat/excel-m4-structural`, stacked on `fix/sheet-typing-ws` (PR #324).

## Ops (Go `lib/sheet` + TS mirror)

- **Sheet list**: `addSheet` (sheet=new id, name, index), `renameSheet`, `deleteSheet`,
  `moveSheet` (toIndex). Convergence rules: duplicate add = first-wins no-op, deleting
  the last sheet = no-op, and **any op on a missing sheet is a silent no-op** (a late op
  after a concurrent `deleteSheet` must not poison the ordered-log replay — this changed
  `Apply`'s old unknown-sheet error).
- **`setDimension`** (axis `col`/`row`, index, sizePx 1..4096): sparse
  `ColWidths`/`RowHeights` maps on `Sheet`; structural insert/delete shifts the maps
  (in-band deletes drop overrides); `Transform` shifts `setDimension.Index` on the
  matching axis.
- **`setFreeze`** (frozenRows/frozenCols, 0|1): per-sheet metadata; arbitrary freeze is
  the upgrade path.
- Snapshot round-trips all new metadata (`colWidths`/`rowHeights` JSON objects with
  stringified indices, `frozenRows`/`frozenCols`).

## View / UI

- Grid grown to **200×52** (`ponytail:` DOM-node-per-cell; virtualization is the upgrade
  path, comment in `sheetView.ts`).
- **Resize**: drag grips on the header cells (`.sheet-resizer-col/-row`), live preview,
  one `setDimension` op on mouseup. Column overrides also relax the per-cell
  `min-width: 80px` default.
- **Freeze**: `border-collapse: separate` (sticky drops collapsed borders) with
  right/bottom-only 1px borders; `.sheet-frozen-r/-c` classes + `--fr-top`/`--fc-left`
  CSS vars measured at render.
- **Tabs bar** (`sheetTabs.ts`): click switch, dblclick rename (native prompt),
  right-click delete (native confirm, disabled for the last sheet), HTML5 drag reorder,
  `+` add. The client-local filter resets on sheet switch.
- **Sort** (`sheetSortFilter.ts`): A→Z / Z→A toolbar buttons sort the selected range by
  the focused column as a batch of `setCell` ops; moved formulas shift row refs via the
  fill heuristic (`adjustFormula`). Numbers sort numerically, empties always last.
- **Filter**: toolbar dropdown of the focused column's distinct values; hides
  non-matching rows client-side (blank rows stay visible). Not collaborative in v1 —
  collaborative filter is the upgrade path.

## Testing

- Go: `lib/sheet/structural_test.go` (apply/validate/transform/convergence/snapshot,
  dim shifting). `TestApplyUnknownSheet` now asserts the no-op semantics.
- TS: `structural.test.ts` (op mirror), `sheetSortFilter.test.ts` (sort batch, formula
  shift, distinct/hide predicates).
- E2E: `playwright/specs/sheet_structural.spec.ts` — tabs add/switch/rename with
  per-sheet data isolation, column resize persisting across reload, sticky frozen row,
  A→Z sort, filter hide/clear. Ran live 5/5 green (plus the 10 existing sheet specs).

## Known ceilings (deliberate)

- Filter: one active column filter, client-local.
- Freeze: first row / first col only.
- No virtualization; 200×52 is the practical grid bound for the DOM view.

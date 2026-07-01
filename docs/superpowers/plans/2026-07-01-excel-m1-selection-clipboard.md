# Excel M1 — Range Selection + Clipboard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add multi-cell range selection, TSV copy/cut/paste (Excel-interop both directions), a fill handle with relative-formula adjustment, and range-clear to the collaborative spreadsheet.

**Architecture:** A pure `Selection` model (`sheetSelection.ts`) drives a highlight in the existing `DomSheetView`. Clipboard/fill logic lives in pure helpers (`sheetClipboard.ts`) that produce plain `setCell`/`clearRange` op payloads through the existing collab pipeline — **no new op types, no server change**. The selection range is added to the existing ephemeral `SHEET_PRESENCE` frame so peers see each other's selection.

**Tech Stack:** TypeScript (`ui/src/js/sheet`, vitest), existing HyperFormula/collab/presence layers. Go changes are limited to two optional fields on the presence frame.

## Global Constraints

- **No new dependencies.** TSV parsing, fill-ref adjustment, and clipboard fallback are hand-written.
- All data mutations go through existing ops: `setCell` (paste/fill) and `clearRange` (delete range). No new op types in M1.
- **Read-only** sessions may select, copy, and scroll, but every op-producing action (paste, cut, fill, delete-range) is blocked client-side. The server already rejects ops from read-only sessions.
- Selection range travels on the existing `SHEET_PRESENCE` frame as **optional** `focusRow`/`focusCol` (anchor stays in the existing `row`/`col`). No new frame type.
- Zero-based `(row, col)` internally, matching `CellRef`/`Op`. Column letters (`A`, `AA`) only appear in UI/clipboard, never in the model.
- Vitest run command: `cd ui && npm test` (or a single file: `cd ui && npx vitest run src/js/sheet/<file>.test.ts`).

---

## File Structure

- `ui/src/js/sheet/sheetSelection.ts` (create) — pure `Selection` range model + helpers.
- `ui/src/js/sheet/sheetSelection.test.ts` (create) — model unit tests.
- `ui/src/js/sheet/sheetClipboard.ts` (create) — pure TSV serialize/parse, paste→op-batch, fill→op-batch, relative-ref adjustment.
- `ui/src/js/sheet/sheetClipboard.test.ts` (create) — clipboard + fill unit tests.
- `ui/src/js/sheet/sheetView.ts` (modify) — selection tracking, highlight render, fill handle, range keyboard, remote selection outline.
- `ui/src/js/sheet/sheetEditor.ts` (modify) — wire clipboard/fill callbacks to the collab client, send/receive selection presence.
- `ui/src/js/sheet/sheetPresence.ts` (modify) — carry `focusRow`/`focusCol` on cursors + frame.
- `lib/models/ws/sheetMessages.go` (modify) — add optional `FocusRow`/`FocusCol` to the presence in/out structs.
- `lib/ws/SheetHandler.go` (modify) — relay `FocusRow`/`FocusCol` in `HandlePresence`.

---

## Task 1: Selection model (`sheetSelection.ts`)

**Files:**
- Create: `ui/src/js/sheet/sheetSelection.ts`
- Test: `ui/src/js/sheet/sheetSelection.test.ts`

**Interfaces:**
- Produces:
  - `interface CellPos { row: number; col: number }`
  - `interface Selection { anchor: CellPos; focus: CellPos }`
  - `function normalize(s: Selection): { r0: number; c0: number; r1: number; c1: number }` — top-left/bottom-right inclusive.
  - `function selContains(s: Selection, row: number, col: number): boolean`
  - `function selCells(s: Selection): CellPos[]` — all cells in the rectangle, row-major.
  - `function selIsSingle(s: Selection): boolean`
  - `function selFromSingle(row: number, col: number): Selection`

- [ ] **Step 1: Write the failing test**

```typescript
// ui/src/js/sheet/sheetSelection.test.ts
import { describe, it, expect } from 'vitest';
import { normalize, selContains, selCells, selIsSingle, selFromSingle } from './sheetSelection';

describe('Selection model', () => {
  it('normalize orders anchor/focus into top-left..bottom-right', () => {
    const s = { anchor: { row: 3, col: 5 }, focus: { row: 1, col: 2 } };
    expect(normalize(s)).toEqual({ r0: 1, c0: 2, r1: 3, c1: 5 });
  });

  it('selContains covers the full rectangle regardless of direction', () => {
    const s = { anchor: { row: 3, col: 5 }, focus: { row: 1, col: 2 } };
    expect(selContains(s, 2, 3)).toBe(true);
    expect(selContains(s, 1, 2)).toBe(true);
    expect(selContains(s, 3, 5)).toBe(true);
    expect(selContains(s, 0, 2)).toBe(false);
    expect(selContains(s, 2, 6)).toBe(false);
  });

  it('selCells enumerates row-major, inclusive of both corners', () => {
    const s = { anchor: { row: 0, col: 0 }, focus: { row: 1, col: 1 } };
    expect(selCells(s)).toEqual([
      { row: 0, col: 0 }, { row: 0, col: 1 },
      { row: 1, col: 0 }, { row: 1, col: 1 },
    ]);
  });

  it('selIsSingle is true only for a 1x1 selection', () => {
    expect(selIsSingle(selFromSingle(2, 2))).toBe(true);
    expect(selIsSingle({ anchor: { row: 0, col: 0 }, focus: { row: 0, col: 1 } })).toBe(false);
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd ui && npx vitest run src/js/sheet/sheetSelection.test.ts`
Expected: FAIL — cannot resolve `./sheetSelection`.

- [ ] **Step 3: Write minimal implementation**

```typescript
// ui/src/js/sheet/sheetSelection.ts
// Pure range model for grid selection. Zero-based (row, col). anchor is the
// fixed corner (where a drag/extend started); focus is the moving corner.

export interface CellPos {
  row: number;
  col: number;
}

export interface Selection {
  anchor: CellPos;
  focus: CellPos;
}

export function selFromSingle(row: number, col: number): Selection {
  return { anchor: { row, col }, focus: { row, col } };
}

export function normalize(s: Selection): { r0: number; c0: number; r1: number; c1: number } {
  return {
    r0: Math.min(s.anchor.row, s.focus.row),
    c0: Math.min(s.anchor.col, s.focus.col),
    r1: Math.max(s.anchor.row, s.focus.row),
    c1: Math.max(s.anchor.col, s.focus.col),
  };
}

export function selContains(s: Selection, row: number, col: number): boolean {
  const { r0, c0, r1, c1 } = normalize(s);
  return row >= r0 && row <= r1 && col >= c0 && col <= c1;
}

export function selCells(s: Selection): CellPos[] {
  const { r0, c0, r1, c1 } = normalize(s);
  const out: CellPos[] = [];
  for (let r = r0; r <= r1; r++) {
    for (let c = c0; c <= c1; c++) out.push({ row: r, col: c });
  }
  return out;
}

export function selIsSingle(s: Selection): boolean {
  return s.anchor.row === s.focus.row && s.anchor.col === s.focus.col;
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd ui && npx vitest run src/js/sheet/sheetSelection.test.ts`
Expected: PASS (4 tests).

- [ ] **Step 5: Commit**

```bash
git add ui/src/js/sheet/sheetSelection.ts ui/src/js/sheet/sheetSelection.test.ts
git commit -m "feat(sheet): pure range selection model"
```

---

## Task 2: Clipboard TSV + paste/fill op builders (`sheetClipboard.ts`)

**Files:**
- Create: `ui/src/js/sheet/sheetClipboard.ts`
- Test: `ui/src/js/sheet/sheetClipboard.test.ts`

**Interfaces:**
- Consumes: `Selection`, `normalize`, `selCells` from `./sheetSelection`; `Op` from `./op`.
- Produces:
  - `function rangeToTSV(sel: Selection, rawAt: (r: number, c: number) => string): string`
  - `function parseTSV(text: string): string[][]` — rows of columns; trailing `\r` stripped; a single trailing newline ignored.
  - `function pasteOps(grid: string[][], anchor: CellPos, sheet: string, baseRev: number): Op[]` — one `setCell` per grid entry, placed with top-left at `anchor`.
  - `function fillOps(src: Selection, target: Selection, sheet: string, baseRev: number, rawAt: (r: number, c: number) => string): Op[]` — fill `target` from the `src` pattern, adjusting relative refs.
  - `function adjustFormula(raw: string, dRow: number, dCol: number): string` — shift relative A1 refs by (dRow, dCol); leave `$`-anchored parts unchanged; non-formula raw returned as-is.

Note on `Op`: use `{ type: 'setCell', sheet, baseRev, row, col, raw }`. `baseRev` is a required field on `Op`; the collab client overwrites it with the live rev when it flushes (see `SheetCollabClient.flush`), so any value passed here is a placeholder — pass the given `baseRev`.

- [ ] **Step 1: Write the failing test**

```typescript
// ui/src/js/sheet/sheetClipboard.test.ts
import { describe, it, expect } from 'vitest';
import { rangeToTSV, parseTSV, pasteOps, fillOps, adjustFormula } from './sheetClipboard';

const raw = (grid: Record<string, string>) => (r: number, c: number) => grid[`${r}:${c}`] ?? '';

describe('TSV clipboard', () => {
  it('rangeToTSV joins cols with tab, rows with newline (row-major)', () => {
    const sel = { anchor: { row: 0, col: 0 }, focus: { row: 1, col: 1 } };
    const g = raw({ '0:0': 'a', '0:1': 'b', '1:0': 'c', '1:1': 'd' });
    expect(rangeToTSV(sel, g)).toBe('a\tb\nc\td');
  });

  it('parseTSV splits rows/cols and tolerates CRLF + one trailing newline', () => {
    expect(parseTSV('a\tb\r\nc\td\r\n')).toEqual([['a', 'b'], ['c', 'd']]);
    expect(parseTSV('x')).toEqual([['x']]);
  });

  it('pasteOps places a grid with top-left at the anchor', () => {
    const ops = pasteOps([['a', 'b'], ['c', 'd']], { row: 2, col: 3 }, 's1', 0);
    expect(ops).toEqual([
      { type: 'setCell', sheet: 's1', baseRev: 0, row: 2, col: 3, raw: 'a' },
      { type: 'setCell', sheet: 's1', baseRev: 0, row: 2, col: 4, raw: 'b' },
      { type: 'setCell', sheet: 's1', baseRev: 0, row: 3, col: 3, raw: 'c' },
      { type: 'setCell', sheet: 's1', baseRev: 0, row: 3, col: 4, raw: 'd' },
    ]);
  });
});

describe('adjustFormula relative-ref shift', () => {
  it('shifts relative refs by the offset', () => {
    expect(adjustFormula('=A1', 1, 0)).toBe('=A2');
    expect(adjustFormula('=A1+B1', 0, 1)).toBe('=B1+C1');
    expect(adjustFormula('=SUM(A1:A3)', 2, 0)).toBe('=SUM(A3:A5)');
  });
  it('leaves absolute ($) parts unchanged', () => {
    expect(adjustFormula('=$A$1', 5, 5)).toBe('=$A$1');
    expect(adjustFormula('=$A1', 1, 1)).toBe('=$A2'); // col absolute, row relative
    expect(adjustFormula('=A$1', 1, 1)).toBe('=B$1'); // row absolute, col relative
  });
  it('returns non-formula raw untouched', () => {
    expect(adjustFormula('hello', 3, 3)).toBe('hello');
    expect(adjustFormula('42', 3, 3)).toBe('42');
  });
});

describe('fillOps', () => {
  it('fills a target range downward from a single-cell formula source', () => {
    // source A1 (r0c0) = "=B1"; target A1:A3 -> A2, A3 get =B2, =B3
    const src = { anchor: { row: 0, col: 0 }, focus: { row: 0, col: 0 } };
    const target = { anchor: { row: 0, col: 0 }, focus: { row: 2, col: 0 } };
    const g = raw({ '0:0': '=B1' });
    const ops = fillOps(src, target, 's1', 0, g);
    // A1 unchanged (already the source); A2, A3 filled.
    expect(ops).toEqual([
      { type: 'setCell', sheet: 's1', baseRev: 0, row: 1, col: 0, raw: '=B2' },
      { type: 'setCell', sheet: 's1', baseRev: 0, row: 2, col: 0, raw: '=B3' },
    ]);
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd ui && npx vitest run src/js/sheet/sheetClipboard.test.ts`
Expected: FAIL — cannot resolve `./sheetClipboard`.

- [ ] **Step 3: Write minimal implementation**

```typescript
// ui/src/js/sheet/sheetClipboard.ts
// Pure clipboard + fill helpers. They build plain setCell op payloads; the
// collab client assigns the real baseRev at flush time.

import type { Op } from './op';
import type { CellPos, Selection } from './sheetSelection';
import { normalize } from './sheetSelection';

export function rangeToTSV(sel: Selection, rawAt: (r: number, c: number) => string): string {
  const { r0, c0, r1, c1 } = normalize(sel);
  const rows: string[] = [];
  for (let r = r0; r <= r1; r++) {
    const cols: string[] = [];
    for (let c = c0; c <= c1; c++) cols.push(rawAt(r, c));
    rows.push(cols.join('\t'));
  }
  return rows.join('\n');
}

export function parseTSV(text: string): string[][] {
  const trimmed = text.replace(/\r/g, '').replace(/\n$/, '');
  return trimmed.split('\n').map((line) => line.split('\t'));
}

export function pasteOps(grid: string[][], anchor: CellPos, sheet: string, baseRev: number): Op[] {
  const ops: Op[] = [];
  for (let r = 0; r < grid.length; r++) {
    for (let c = 0; c < grid[r].length; c++) {
      ops.push({ type: 'setCell', sheet, baseRev, row: anchor.row + r, col: anchor.col + c, raw: grid[r][c] });
    }
  }
  return ops;
}

// A1-style ref token: optional $ before col letters, optional $ before row digits.
const REF = /(\$?)([A-Z]+)(\$?)(\d+)/g;

function colToNum(letters: string): number {
  let n = 0;
  for (const ch of letters) n = n * 26 + (ch.charCodeAt(0) - 64);
  return n - 1; // zero-based
}

function numToCol(n: number): string {
  let s = '';
  let x = n + 1;
  while (x > 0) {
    const rem = (x - 1) % 26;
    s = String.fromCharCode(65 + rem) + s;
    x = Math.floor((x - 1) / 26);
  }
  return s;
}

export function adjustFormula(raw: string, dRow: number, dCol: number): string {
  if (!raw.startsWith('=')) return raw;
  return raw.replace(REF, (_m, colAbs: string, colLetters: string, rowAbs: string, rowDigits: string) => {
    const col = colAbs ? colLetters : numToCol(colToNum(colLetters) + dCol);
    const row = rowAbs ? rowDigits : String(Number(rowDigits) + dRow);
    return `${colAbs}${col}${rowAbs}${row}`;
  });
}

export function fillOps(
  src: Selection,
  target: Selection,
  sheet: string,
  baseRev: number,
  rawAt: (r: number, c: number) => string,
): Op[] {
  const s = normalize(src);
  const t = normalize(target);
  const srcH = s.r1 - s.r0 + 1;
  const srcW = s.c1 - s.c0 + 1;
  const ops: Op[] = [];
  for (let r = t.r0; r <= t.r1; r++) {
    for (let c = t.c0; c <= t.c1; c++) {
      if (r >= s.r0 && r <= s.r1 && c >= s.c0 && c <= s.c1) continue; // skip the source cells
      const sr = s.r0 + ((r - t.r0) % srcH);
      const sc = s.c0 + ((c - t.c0) % srcW);
      const raw = adjustFormula(rawAt(sr, sc), r - sr, c - sc);
      ops.push({ type: 'setCell', sheet, baseRev, row: r, col: c, raw });
    }
  }
  return ops;
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd ui && npx vitest run src/js/sheet/sheetClipboard.test.ts`
Expected: PASS (all describe blocks green).

- [ ] **Step 5: Commit**

```bash
git add ui/src/js/sheet/sheetClipboard.ts ui/src/js/sheet/sheetClipboard.test.ts
git commit -m "feat(sheet): TSV clipboard + fill op builders"
```

---

## Task 3: Presence frame carries selection focus (Go + TS reducer)

**Files:**
- Modify: `lib/models/ws/sheetMessages.go`
- Modify: `lib/ws/SheetHandler.go:177-188` (the `SheetPresence` output struct construction in `HandlePresence`)
- Modify: `ui/src/js/sheet/sheetPresence.ts`
- Modify: `ui/src/js/sheet/sheetPresence.test.ts`

**Interfaces:**
- Produces (TS): `RemoteCursor` and `PresenceFrame` gain optional `focusRow?: number; focusCol?: number`.
- Produces (Go): the incoming presence data struct and the outgoing `SheetPresenceData` gain `FocusRow`/`FocusCol` with `json:"focusRow,omitempty"` / `json:"focusCol,omitempty"`.

**Context:** First read `lib/models/ws/sheetMessages.go` to find the incoming presence struct (the one `HandlePresence` reads via `msg.Data.Data.Row`) and the outgoing `SheetPresenceData`. Add the two fields to both. This step only threads the fields through; the view uses them in Task 5.

- [ ] **Step 1: Write the failing test (TS reducer keeps focus on the cursor)**

Add to `ui/src/js/sheet/sheetPresence.test.ts` inside `describe('SheetPresence reducer', …)`:

```typescript
  it('keeps selection focus (focusRow/focusCol) on the cursor', () => {
    const p = new SheetPresence('me');
    p.applyPresence(frame({ userId: 'a', row: 1, col: 1, focusRow: 3, focusCol: 4 }));
    const cur = p.cursorsForSheet('s1')[0];
    expect(cur.focusRow).toBe(3);
    expect(cur.focusCol).toBe(4);
  });
```

Also extend the `frame` helper's type usage — `focusRow`/`focusCol` are optional on `PresenceFrame`, so `frame({ ..., focusRow: 3, focusCol: 4 })` type-checks once the interface is updated.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd ui && npx vitest run src/js/sheet/sheetPresence.test.ts`
Expected: FAIL — `focusRow`/`focusCol` not on the cursor (and a TS error until the interfaces are updated).

- [ ] **Step 3: Implement — TS side**

In `ui/src/js/sheet/sheetPresence.ts`:

Add to `RemoteCursor`:
```typescript
export interface RemoteCursor {
  userId: string;
  name: string;
  color: string;
  sheet: string;
  row: number;
  col: number;
  focusRow?: number;
  focusCol?: number;
}
```

Add to `PresenceFrame`:
```typescript
export interface PresenceFrame {
  userId: string;
  name: string;
  color: string;
  sheet: string;
  row: number;
  col: number;
  editing: boolean;
  raw?: string;
  focusRow?: number;
  focusCol?: number;
}
```

In `applyPresence`, include the focus fields when setting the cursor:
```typescript
    this.cursors.set(f.userId, {
      userId: f.userId, name: f.name, color: f.color, sheet: f.sheet,
      row: f.row, col: f.col, focusRow: f.focusRow, focusCol: f.focusCol,
    });
```

- [ ] **Step 4: Implement — Go side**

In `lib/models/ws/sheetMessages.go`, add to the incoming presence data struct (fields alongside `Row`/`Col`) and to `SheetPresenceData`:
```go
	FocusRow int `json:"focusRow,omitempty"`
	FocusCol int `json:"focusCol,omitempty"`
```

In `lib/ws/SheetHandler.go`, in `HandlePresence`, copy them onto the outgoing frame (next to `Row`/`Col`):
```go
		FocusRow: msg.Data.Data.FocusRow,
		FocusCol: msg.Data.Data.FocusCol,
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd ui && npx vitest run src/js/sheet/sheetPresence.test.ts`
Expected: PASS.
Run: `go build ./...`
Expected: builds clean.

- [ ] **Step 6: Commit**

```bash
git add lib/models/ws/sheetMessages.go lib/ws/SheetHandler.go ui/src/js/sheet/sheetPresence.ts ui/src/js/sheet/sheetPresence.test.ts
git commit -m "feat(sheet): carry selection focus on the presence frame"
```

---

## Task 4: Selection state + highlight in `DomSheetView`

**Files:**
- Modify: `ui/src/js/sheet/sheetView.ts`

**Interfaces:**
- Consumes: `Selection`, `selFromSingle`, `normalize`, `selContains` from `./sheetSelection`.
- Produces (new `SheetViewOptions` callback + methods used by Task 6):
  - `onSelectionChange?: (sel: Selection) => void` — fires whenever the local selection changes (mouse/keyboard). Replaces the reason to keep `onSelect` for network; `onSelect(r,c)` still fires for the active (focus) cell.
  - `getSelection(): Selection` — current local selection.
  - `setRemoteSelections(list: Array<{ userId: string; color: string; sel: Selection }>): void` — remote outlines; painted in `render()`.

**Behavior to implement:**
- Track `private selection: Selection` (init `selFromSingle(0,0)`).
- On cell `mousedown`: set `selection = selFromSingle(r,c)`, begin a drag (listen for `mouseover` on cells to move `focus` while the mouse button is held; end on `mouseup`). On `shift`+mousedown: keep anchor, set `focus`.
- Keyboard on the focused cell (extend the existing `keydown` handler): plain arrows move both anchor+focus by one (single-cell move + `onSelect`); `shift`+arrows move only `focus` (extend). `Ctrl/Cmd+A` selects `(0,0)..(rows-1,cols-1)`.
- After any selection change: call `onSelectionChange?.(selection)`, then `render()`.
- CSS: add a `.sheet-sel` background tint class applied to in-range cells and a `.sheet-sel-focus` heavier outline on the focus cell. Add a `.sheet-remote-sel` outline (colored, translucent) for remote selections.
- In `render()`: after existing text/decoration painting, apply `.sheet-sel` to every cell where `selContains(this.selection, r, c)` and the active/focus ring to the focus cell; then paint remote selection outlines from `setRemoteSelections`.

- [ ] **Step 1: Write the failing test**

Create `ui/src/js/sheet/sheetView.selection.test.ts`. This test uses jsdom (vitest default env is node; add the env pragma):

```typescript
// @vitest-environment jsdom
import { describe, it, expect, vi } from 'vitest';
import { DomSheetView } from './sheetView';

function mkView(onSelectionChange = vi.fn()) {
  const root = document.createElement('div');
  document.body.appendChild(root);
  const view = new DomSheetView(root, {
    rows: 5, cols: 5,
    rawValue: () => '',
    displayValue: () => '',
    onEdit: () => {},
    onSelectionChange,
  });
  return { root, view };
}

describe('DomSheetView selection', () => {
  it('shift+click extends the selection from the anchor', () => {
    const onSel = vi.fn();
    const { root, view } = mkView(onSel);
    const cell = (r: number, c: number) =>
      root.querySelectorAll('tbody tr')[r].querySelectorAll('td')[c] as HTMLElement;

    cell(1, 1).dispatchEvent(new MouseEvent('mousedown', { bubbles: true }));
    cell(3, 3).dispatchEvent(new MouseEvent('mousedown', { bubbles: true, shiftKey: true }));

    const sel = view.getSelection();
    expect(sel.anchor).toEqual({ row: 1, col: 1 });
    expect(sel.focus).toEqual({ row: 3, col: 3 });
    // the last change fired with the extended range
    expect(onSel).toHaveBeenLastCalledWith(sel);
  });

  it('marks in-range cells with the selection class', () => {
    const { root, view } = mkView();
    const cell = (r: number, c: number) =>
      root.querySelectorAll('tbody tr')[r].querySelectorAll('td')[c] as HTMLElement;
    cell(0, 0).dispatchEvent(new MouseEvent('mousedown', { bubbles: true }));
    cell(1, 1).dispatchEvent(new MouseEvent('mousedown', { bubbles: true, shiftKey: true }));
    view.render();
    expect(cell(0, 0).classList.contains('sheet-sel')).toBe(true);
    expect(cell(1, 1).classList.contains('sheet-sel')).toBe(true);
    expect(cell(2, 2).classList.contains('sheet-sel')).toBe(false);
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd ui && npx vitest run src/js/sheet/sheetView.selection.test.ts`
Expected: FAIL — `getSelection`/`onSelectionChange` don't exist; no `.sheet-sel` class applied.

- [ ] **Step 3: Implement**

In `ui/src/js/sheet/sheetView.ts`:
1. Import the model: `import { type Selection, selFromSingle, normalize, selContains } from './sheetSelection';`
2. Add to `SheetViewOptions`: `onSelectionChange?: (sel: Selection) => void;`
3. Add fields: `private selection: Selection = selFromSingle(0, 0);` `private dragging = false;` `private remoteSel: Array<{ userId: string; color: string; sel: Selection }> = [];`
4. Extend the `.sheet-grid` CSS string (`CSS`) with:
```css
.sheet-grid td.sheet-sel { background: rgba(100, 210, 155, 0.15); }
.sheet-grid td.sheet-sel-focus { box-shadow: inset 0 0 0 2px #2f9e6b; }
.sheet-grid td.sheet-remote-sel { box-shadow: inset 0 0 0 2px var(--rsel, #888); }
```
5. In `attach(td, r, c)`, add:
```typescript
    td.addEventListener('mousedown', (e: MouseEvent) => {
      if (e.shiftKey) {
        this.selection = { anchor: this.selection.anchor, focus: { row: r, col: c } };
      } else {
        this.selection = selFromSingle(r, c);
        this.dragging = true;
      }
      this.opts.onSelectionChange?.(this.selection);
      this.render();
    });
    td.addEventListener('mouseover', () => {
      if (!this.dragging) return;
      this.selection = { anchor: this.selection.anchor, focus: { row: r, col: c } };
      this.opts.onSelectionChange?.(this.selection);
      this.render();
    });
```
6. In the constructor (after building the table) add a one-time `mouseup` on `document`: `document.addEventListener('mouseup', () => { this.dragging = false; });`
7. In the `keydown` handler, before the Enter/Escape branches, add arrow + select-all handling:
```typescript
      const move = (dr: number, dc: number, extend: boolean) => {
        e.preventDefault();
        const f = this.selection.focus;
        const nr = Math.min(this.opts.rows - 1, Math.max(0, f.row + dr));
        const nc = Math.min(this.opts.cols - 1, Math.max(0, f.col + dc));
        this.selection = extend
          ? { anchor: this.selection.anchor, focus: { row: nr, col: nc } }
          : selFromSingle(nr, nc);
        if (!extend) this.opts.onSelect?.(nr, nc);
        this.opts.onSelectionChange?.(this.selection);
        this.render();
      };
      if (e.key === 'ArrowUp') return move(-1, 0, e.shiftKey);
      if (e.key === 'ArrowDown') return move(1, 0, e.shiftKey);
      if (e.key === 'ArrowLeft') return move(0, -1, e.shiftKey);
      if (e.key === 'ArrowRight') return move(0, 1, e.shiftKey);
      if ((e.ctrlKey || e.metaKey) && (e.key === 'a' || e.key === 'A')) {
        e.preventDefault();
        this.selection = { anchor: { row: 0, col: 0 }, focus: { row: this.opts.rows - 1, col: this.opts.cols - 1 } };
        this.opts.onSelectionChange?.(this.selection);
        return this.render();
      }
```
   Note: these run while the cell is focused (contenteditable). Arrow keys inside a contenteditable normally move the caret; `e.preventDefault()` in `move` suppresses that so arrows navigate cells. Keep the existing Enter/Escape handling after this block.
8. Add methods:
```typescript
  getSelection(): Selection {
    return this.selection;
  }

  setRemoteSelections(list: Array<{ userId: string; color: string; sel: Selection }>): void {
    this.remoteSel = list;
  }
```
9. In `render()`, after the existing decoration loop, add selection painting:
```typescript
    // local selection
    for (let r = 0; r < this.opts.rows; r++) {
      for (let c = 0; c < this.opts.cols; c++) {
        const td = this.cells[r][c];
        td.classList.toggle('sheet-sel', selContains(this.selection, r, c));
        td.classList.toggle(
          'sheet-sel-focus',
          r === this.selection.focus.row && c === this.selection.focus.col,
        );
      }
    }
    // remote selections (outline only, multi-cell)
    for (const rs of this.remoteSel) {
      const { r0, c0, r1, c1 } = normalize(rs.sel);
      if (r1 - r0 === 0 && c1 - c0 === 0) continue; // single cell handled by cursor deco
      for (let r = r0; r <= r1; r++) {
        for (let c = c0; c <= c1; c++) {
          const td = this.cells[r]?.[c];
          if (!td) continue;
          td.style.setProperty('--rsel', rs.color);
          td.classList.add('sheet-remote-sel');
          this.decorated.add(td);
        }
      }
    }
```
   Ensure the `render()` reset loop also clears these: in the loop that resets `this.decorated`, also remove `sheet-remote-sel` and the `--rsel` property. Update the reset block at the top of `render()`:
```typescript
    for (const td of this.decorated) {
      td.style.boxShadow = '';
      td.classList.remove('sheet-remote-sel');
      td.style.removeProperty('--rsel');
      td.querySelector('.sheet-remote-tag')?.remove();
    }
    this.decorated.clear();
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd ui && npx vitest run src/js/sheet/sheetView.selection.test.ts`
Expected: PASS.

- [ ] **Step 5: Run the full sheet suite (no regressions)**

Run: `cd ui && npx vitest run src/js/sheet`
Expected: all existing sheet tests still PASS.

- [ ] **Step 6: Commit**

```bash
git add ui/src/js/sheet/sheetView.ts ui/src/js/sheet/sheetView.selection.test.ts
git commit -m "feat(sheet): multi-cell selection state + highlight in the grid view"
```

---

## Task 5: Fill handle in `DomSheetView`

**Files:**
- Modify: `ui/src/js/sheet/sheetView.ts`

**Interfaces:**
- Produces (new `SheetViewOptions` callback used by Task 6):
  - `onFill?: (src: Selection, target: Selection) => void` — fires when the user finishes dragging the fill handle. `src` is the selection at drag start; `target` is the extended range.

**Behavior:**
- Render a small handle element (an 8×8 box) at the bottom-right corner of the focus/selection outline. Simplest placement: append a `<span class="sheet-fill-handle">` to the bottom-right cell of the normalized selection during `render()` (remove it on the next render like other decorations).
- On `mousedown` on the handle: begin a fill drag (`this.filling = true`), record `fillSrc = this.selection`.
- While filling, `mouseover` on cells extends a `fillTarget` selection (anchor = fillSrc top-left, focus = hovered cell), painted with a dashed outline class.
- On `mouseup` while filling: call `onFill?.(fillSrc, fillTarget)`, clear fill state, and set `this.selection = fillTarget`.

- [ ] **Step 1: Write the failing test**

Add to `ui/src/js/sheet/sheetView.selection.test.ts`:

```typescript
import type { Selection } from './sheetSelection';

describe('DomSheetView fill handle', () => {
  it('dragging the fill handle fires onFill with src and target', () => {
    const onFill = vi.fn();
    const root = document.createElement('div');
    document.body.appendChild(root);
    const view = new DomSheetView(root, {
      rows: 5, cols: 5, rawValue: () => '', displayValue: () => '', onEdit: () => {}, onFill,
    });
    const cell = (r: number, c: number) =>
      root.querySelectorAll('tbody tr')[r].querySelectorAll('td')[c] as HTMLElement;

    cell(0, 0).dispatchEvent(new MouseEvent('mousedown', { bubbles: true }));
    view.render();
    const handle = root.querySelector('.sheet-fill-handle') as HTMLElement;
    expect(handle).toBeTruthy();
    handle.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }));
    cell(2, 0).dispatchEvent(new MouseEvent('mouseover', { bubbles: true }));
    document.dispatchEvent(new MouseEvent('mouseup', { bubbles: true }));

    expect(onFill).toHaveBeenCalledTimes(1);
    const [src, target] = onFill.mock.calls[0] as [Selection, Selection];
    expect(src.anchor).toEqual({ row: 0, col: 0 });
    expect(target.focus).toEqual({ row: 2, col: 0 });
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd ui && npx vitest run src/js/sheet/sheetView.selection.test.ts`
Expected: FAIL — no `.sheet-fill-handle`, `onFill` never called.

- [ ] **Step 3: Implement**

In `sheetView.ts`:
1. Add to `SheetViewOptions`: `onFill?: (src: Selection, target: Selection) => void;`
2. Add fields: `private filling = false;` `private fillSrc: Selection | null = null;` `private fillTarget: Selection | null = null;`
3. Extend CSS:
```css
.sheet-fill-handle { position: absolute; right: -4px; bottom: -4px; width: 8px; height: 8px; background: #2f9e6b; border: 1px solid #fff; cursor: crosshair; z-index: 6; }
.sheet-grid td.sheet-fill-target { box-shadow: inset 0 0 0 1px #2f9e6b; }
```
4. In `attach`, extend the `mouseover` handler so it also drives fill:
```typescript
    td.addEventListener('mouseover', () => {
      if (this.filling && this.fillSrc) {
        this.fillTarget = { anchor: this.fillSrc.anchor, focus: { row: r, col: c } };
        this.render();
        return;
      }
      if (!this.dragging) return;
      this.selection = { anchor: this.selection.anchor, focus: { row: r, col: c } };
      this.opts.onSelectionChange?.(this.selection);
      this.render();
    });
```
5. In the constructor's `document` `mouseup` listener, handle fill completion:
```typescript
    document.addEventListener('mouseup', () => {
      if (this.filling && this.fillSrc && this.fillTarget) {
        this.opts.onFill?.(this.fillSrc, this.fillTarget);
        this.selection = this.fillTarget;
        this.opts.onSelectionChange?.(this.selection);
      }
      this.dragging = false;
      this.filling = false;
      this.fillSrc = null;
      this.fillTarget = null;
      this.render();
    });
```
6. In `render()`, after painting the local selection, append the fill handle to the bottom-right cell of the normalized selection and paint any fill target:
```typescript
    const { r1, c1 } = normalize(this.selection);
    const brCell = this.cells[r1]?.[c1];
    if (brCell && !this.opts.readOnly) {
      const h = document.createElement('span');
      h.className = 'sheet-fill-handle';
      h.addEventListener('mousedown', (e: MouseEvent) => {
        e.stopPropagation();
        e.preventDefault();
        this.filling = true;
        this.fillSrc = this.selection;
        this.fillTarget = this.selection;
      });
      brCell.appendChild(h);
      this.decorated.add(brCell);
    }
    if (this.fillTarget) {
      const t = normalize(this.fillTarget);
      for (let r = t.r0; r <= t.r1; r++) {
        for (let c = t.c0; c <= t.c1; c++) {
          const td = this.cells[r]?.[c];
          if (td) { td.classList.add('sheet-fill-target'); this.decorated.add(td); }
        }
      }
    }
```
   Update the `render()` reset block to also remove `sheet-fill-target` and any `.sheet-fill-handle` child:
```typescript
    for (const td of this.decorated) {
      td.style.boxShadow = '';
      td.classList.remove('sheet-remote-sel', 'sheet-fill-target');
      td.style.removeProperty('--rsel');
      td.querySelector('.sheet-remote-tag')?.remove();
      td.querySelector('.sheet-fill-handle')?.remove();
    }
    this.decorated.clear();
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd ui && npx vitest run src/js/sheet/sheetView.selection.test.ts`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add ui/src/js/sheet/sheetView.ts ui/src/js/sheet/sheetView.selection.test.ts
git commit -m "feat(sheet): fill handle drag on the grid view"
```

---

## Task 6: Wire selection, clipboard, fill, and range-delete in `sheetEditor.ts`

**Files:**
- Modify: `ui/src/js/sheet/sheetEditor.ts`

**Interfaces:**
- Consumes: `getSelection`, `onSelectionChange`, `onFill`, `setRemoteSelections` from the view (Tasks 4-5); `rangeToTSV`, `parseTSV`, `pasteOps`, `fillOps` from `./sheetClipboard`; `normalize`, `selCells`, `selIsSingle`, type `Selection` from `./sheetSelection`; presence `focusRow`/`focusCol` (Task 3).

**Behavior:**
- Keep a local `let selection: Selection` updated from the view's `onSelectionChange`. When it changes, broadcast presence with focus: `sendPresence(anchor.row, anchor.col, false, undefined, focus.row, focus.col)` — extend `sendPresence` to take optional focus coords.
- Copy/Cut/Paste: attach `keydown` on `document` (or the sheet root) for Ctrl/Cmd+C / X / V. Guard: ignore when a cell is mid-edit (contenteditable focus) — let native clipboard handle in-cell text. Use `document.activeElement`/a flag to know if editing.
  - Copy: `navigator.clipboard.writeText(rangeToTSV(selection, rawValue))`.
  - Cut: same write, then if not read-only send a `clearRange` op over the selection.
  - Paste (not read-only): `const text = await navigator.clipboard.readText(); const grid = parseTSV(text); for (const op of pasteOps(grid, normalize→top-left, activeSheetId, collab.rev)) collab.applyLocal(op);`
- Fill: on the view's `onFill(src, target)` (not read-only), send `fillOps(src, target, activeSheetId, collab.rev, rawValue)` each via `collab.applyLocal`.
- Delete/Backspace on a multi-cell selection (not mid-edit, not read-only): send one `clearRange` op `{ type:'clearRange', sheet, baseRev, row:r0, col:c0, endRow:r1, endCol:c1 }` via `collab.applyLocal`; for a single cell, keep the existing in-cell behavior.
- Receiving presence: in `onChange`, also compute remote selections from `presence.cursorsForSheet` (those with `focusRow`/`focusCol` set) and call `view.setRemoteSelections([...])`.
- Read-only guard: gate paste/cut/fill/clearRange on `!data.readonly` (store the flag from `SheetVarsData`).

**Note on clipboard availability:** `navigator.clipboard` requires a secure context (HTTPS or localhost). Etherpad dev/prod both qualify. A hidden-textarea fallback is the documented upgrade path if a plain-HTTP deployment needs it — not built in M1 (`ponytail:` comment at the copy/paste site naming the fallback).

- [ ] **Step 1: Add the local selection + focus presence wiring**

In `sheetEditor.ts`:
1. Extend `sendPresence` signature:
```typescript
  const sendPresence = (
    row: number, col: number, editing: boolean, raw?: string, focusRow?: number, focusCol?: number,
  ): void =>
    socket.emit('message', {
      type: 'COLLABROOM',
      component: 'sheet',
      data: { type: 'SHEET_PRESENCE', sheet: activeSheetId, row, col, editing, raw, focusRow, focusCol },
    });
```
2. Track selection and store readonly:
```typescript
  let selection: import('./sheetSelection').Selection = { anchor: { row: 0, col: 0 }, focus: { row: 0, col: 0 } };
  let readOnly = false;
```
3. Add the view option `onSelectionChange`:
```typescript
      onSelectionChange: (sel) => {
        selection = sel;
        sendPresence(sel.anchor.row, sel.anchor.col, false, undefined, sel.focus.row, sel.focus.col);
      },
```
   (Set `readOnly = data.readonly;` in `initSheet`.)

- [ ] **Step 2: Add clipboard, fill, and delete handlers**

Add near the top of `startSheetEditor` (after `transport`), a helper import line:
```typescript
import { rangeToTSV, parseTSV, pasteOps, fillOps } from './sheetClipboard';
import { normalize, selIsSingle } from './sheetSelection';
```
Add the `onFill` view option:
```typescript
      onFill: (src, target) => {
        if (readOnly || !collab) return;
        for (const op of fillOps(src, target, activeSheetId, collab.rev, rawValue)) collab.applyLocal(op);
      },
```
Add a document keydown handler (registered once, after the view is created in `initSheet`):
```typescript
    const editingNow = (): boolean => {
      const el = document.activeElement as HTMLElement | null;
      return !!el && el.tagName === 'TD' && el.isContentEditable;
    };
    document.addEventListener('keydown', (e) => {
      if (!collab) return;
      const mod = e.ctrlKey || e.metaKey;
      if (mod && (e.key === 'c' || e.key === 'C') && !editingNow()) {
        void navigator.clipboard.writeText(rangeToTSV(selection, rawValue)); // ponytail: async Clipboard API only; hidden-textarea fallback if plain-HTTP deploys need it
        return;
      }
      if (mod && (e.key === 'x' || e.key === 'X') && !editingNow()) {
        void navigator.clipboard.writeText(rangeToTSV(selection, rawValue));
        if (readOnly) return;
        const { r0, c0, r1, c1 } = normalize(selection);
        collab.applyLocal({ type: 'clearRange', sheet: activeSheetId, baseRev: collab.rev, row: r0, col: c0, endRow: r1, endCol: c1 });
        return;
      }
      if (mod && (e.key === 'v' || e.key === 'V') && !editingNow() && !readOnly) {
        e.preventDefault();
        void navigator.clipboard.readText().then((text) => {
          if (!collab) return;
          const grid = parseTSV(text);
          const { r0, c0 } = normalize(selection);
          for (const op of pasteOps(grid, { row: r0, col: c0 }, activeSheetId, collab.rev)) collab.applyLocal(op);
        });
        return;
      }
      if ((e.key === 'Delete' || e.key === 'Backspace') && !editingNow() && !readOnly && !selIsSingle(selection)) {
        e.preventDefault();
        const { r0, c0, r1, c1 } = normalize(selection);
        collab.applyLocal({ type: 'clearRange', sheet: activeSheetId, baseRev: collab.rev, row: r0, col: c0, endRow: r1, endCol: c1 });
      }
    });
```

- [ ] **Step 3: Paint remote selections in `onChange`**

Extend the existing `onChange` in `sheetEditor.ts`, after `setRemoteLiveEdits`:
```typescript
      view.setRemoteSelections(
        presence
          .cursorsForSheet(activeSheetId)
          .filter((c) => c.focusRow !== undefined && c.focusCol !== undefined)
          .map((c) => ({
            userId: c.userId,
            color: c.color,
            sel: { anchor: { row: c.row, col: c.col }, focus: { row: c.focusRow as number, col: c.focusCol as number } },
          })),
      );
```

- [ ] **Step 4: Type-check and run the full sheet suite**

Run: `cd ui && npx vitest run src/js/sheet`
Expected: PASS (no unit regressions).
Run: `cd ui && npx tsc --noEmit`
Expected: no type errors in the sheet files. (If the pre-existing `plugin/chat.ts` errors appear, they are unrelated to M1 — confirm no `src/js/sheet/*` errors.)

- [ ] **Step 5: Commit**

```bash
git add ui/src/js/sheet/sheetEditor.ts
git commit -m "feat(sheet): wire selection, TSV copy/cut/paste, fill, and range-delete"
```

---

## Task 7: E2E — selection, copy/paste, fill (Playwright)

**Files:**
- Create: `playwright/specs/sheet_selection.spec.ts`

**Context:** Read `playwright/specs/sheet_presence.spec.ts` first for the established sheet E2E harness (how it opens a sheet pad, waits for the grid, the `webServer`/`reuseExistingServer` setup noted in the memory). Reuse its bootstrap. Clipboard E2E: grant clipboard permissions via the browser context (`permissions: ['clipboard-read', 'clipboard-write']`) in the test or project config; if permissions are flaky on the CI browser, assert paste by driving fill instead (fill needs no OS clipboard) and mark the clipboard assertion `test.fixme` with a note — do not silently skip.

- [ ] **Step 1: Write the E2E spec**

```typescript
import { test, expect } from '@playwright/test';

// Assumes the same sheet-pad bootstrap as sheet_presence.spec.ts.
test('drag selection then fill copies a formula with adjusted refs', async ({ page }) => {
  // open a fresh sheet pad (reuse helper/pattern from sheet_presence.spec.ts)
  await page.goto('/p/sheet-e2e-' + Date.now() + '?sheet=true'); // adjust to the real sheet route
  const cell = (r: number, c: number) => page.locator('tbody tr').nth(r).locator('td').nth(c);

  // A1 = 10, B1 = =A1*2
  await cell(0, 0).dblclick();
  await page.keyboard.type('10');
  await page.keyboard.press('Enter');
  await cell(0, 1).click();
  await page.keyboard.type('=A1*2');
  await page.keyboard.press('Enter');
  await expect(cell(0, 1)).toHaveText('20');

  // A2 = 5, then fill B1 down to B2 -> B2 = =A2*2 = 10
  await cell(1, 0).dblclick();
  await page.keyboard.type('5');
  await page.keyboard.press('Enter');

  await cell(0, 1).click();
  const handle = page.locator('.sheet-fill-handle');
  const target = cell(1, 1);
  await handle.dragTo(target);
  await expect(cell(1, 1)).toHaveText('10');
});
```

- [ ] **Step 2: Run the E2E**

Run (per the memory's Windows note, start `etherpad-go.exe` manually first so `reuseExistingServer` reuses it, then): `cd playwright && npx playwright test specs/sheet_selection.spec.ts`
Expected: PASS. If the sheet route/bootstrap differs, fix the `goto`/selectors to match `sheet_presence.spec.ts` — do not change the assertions.

- [ ] **Step 3: Commit**

```bash
git add playwright/specs/sheet_selection.spec.ts
git commit -m "test(sheet): E2E for selection + fill"
```

---

## Self-Review notes (addressed)

- **Spec coverage (M1 section):** selection model (T1), TSV copy/paste + Excel interop (T2, T6), fill handle + relative-ref adjustment (T2, T5, T6), range delete via `clearRange` (T6), selection presence for peers (T3, T4, T6), read-only guards (T6), no new op types / no new dependency (constraints + T2/T6). All covered.
- **No server op change:** confirmed — paste/fill/delete reuse `setCell`/`clearRange`; the only Go change is two optional presence fields (T3).
- **Type consistency:** `Selection`/`CellPos`/`normalize`/`selCells`/`selContains`/`selIsSingle`/`selFromSingle` used identically across T1–T6; `Op` payloads match `op.ts` (`type`,`sheet`,`baseRev`,`row`,`col`,`raw`,`endRow`,`endCol`); presence `focusRow`/`focusCol` names identical across Go and TS (T3).
- **Deferred (noted in-code):** batched paste op, hidden-textarea clipboard fallback, collaborative (vs per-cell) style later — all logged as upgrade paths, not silently dropped.
```

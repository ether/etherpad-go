# Excel M2 — Cell Formatting Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn the existing (inert) style model into real cell formatting: a toolbar for bold/italic/underline, font & fill color, horizontal align, borders, and number format; styled rendering in the grid; and number formatting on display — all collaborative through the OT pipeline.

**Architecture:** Style ops carry a `props map[string]string`. Both the Go `Workbook.Apply` and the TS `WorkbookState.applyOp` intern the props into their `StylePool` (dedup by content) and set the cell's `styleId`. Because every client's confirmed state (`serverWb`) replays the same totally-ordered op log starting from the same snapshot pool, the incrementing pool ids stay in lockstep server-side and client-side — no server-assigned id needs to travel on the wire. The toolbar reads a cell's current props, merges the requested change, and sends one `setStyle` op per selected cell. Rendering resolves `styleId → props → inline CSS`; display values are formatted via a pure `formatValue` helper (Intl).

**Tech Stack:** Go (`lib/sheet`), TypeScript (`ui/src/js/sheet`, vitest), HyperFormula, `Intl` (built-in), Playwright.

## Global Constraints

- **No new dependencies.** Number formatting uses the built-in `Intl` API; the toolbar uses native `<input type="color">`/`<select>`/buttons.
- **Style ops carry `props` (a string→string map); the pool interns them.** The wire never carries a server-assigned style id for style ops — each side interns independently and converges because `serverWb` replays the same ordered log from the same snapshot pool.
- **Reuse the existing `StylePool`** (`lib/sheet/style.go`): `Put(Style)` dedups by canonical key, id 0 = empty. Do NOT change its id-assignment scheme.
- **Merge is client-side:** a toolbar toggle reads the cell's current props, applies the change, and sends the merged props. The server/pool just interns whatever props arrive.
- **Prop vocabulary (string→string):** `bold`,`italic`,`underline` = `"1"` or absent; `color` (font hex, e.g. `#cc0000`); `bg` (fill hex); `align` = `left|center|right`; `border` = `all|none` (v1: outer/all only); `numFmt` = `general|number|currency|percent|date|text`, optionally `number:<decimals>` / `currency:<decimals>` / `percent:<decimals>`.
- **Read-only sessions** may not apply styles (toolbar disabled client-side; server already rejects ops from read-only sessions).
- Zero-based `(row,col)`. Vitest: `cd ui && npx vitest run src/js/sheet/<file>`. Go: `go test ./lib/sheet/...`.
- DOM-view behavior is covered by the Playwright E2E (jsdom was rejected in M1); extract pure helpers so logic stays unit-testable without a DOM.

---

## File Structure

- `lib/sheet/op.go` (modify) — add `Props map[string]string`; extend `Validate`.
- `lib/sheet/apply.go` (modify) — intern `op.Props` into the pool and set the cell's `StyleId` for `setCell`/`setStyle`.
- `lib/sheet/apply_test.go` / `style_test.go` / `convergence_test.go` (modify) — cover interning, dedup, merge, convergence.
- `ui/src/js/sheet/op.ts` (modify) — add `props?: Record<string, string>`.
- `ui/src/js/sheet/stylePool.ts` (create) — TS mirror of `StylePool` (content-dedup `put`, seed from snapshot).
- `ui/src/js/sheet/stylePool.test.ts` (create) — dedup / seed / put tests.
- `ui/src/js/sheet/workbookState.ts` (modify) — hold a `StylePoolMirror`, parse `snapshot.styles`, intern `op.props` in `applyOp`, expose `getStyleProps`.
- `ui/src/js/sheet/workbookState.test.ts` (modify) — style op interning + snapshot parse.
- `ui/src/js/sheet/format.ts` (create) — pure `formatValue(value, valueType, numFmt)`.
- `ui/src/js/sheet/format.test.ts` (create) — formatting cases.
- `ui/src/js/sheet/styleCss.ts` (create) — pure `styleToCss(props)` → inline style object; `mergeProps`/`toggleProp` helpers.
- `ui/src/js/sheet/styleCss.test.ts` (create) — css mapping + merge/toggle tests.
- `ui/src/js/sheet/sheetView.ts` (modify) — `styleOf(r,c)` option; apply per-cell style CSS in `render()` (with reset).
- `ui/src/js/sheet/sheetToolbar.ts` (create) — the toolbar UI + apply callback.
- `ui/src/js/sheet/sheetEditor.ts` (modify) — mount toolbar above the grid; wire style lookups, `styleOf`, numFmt display; gate on read-only.
- `playwright/specs/sheet_formatting.spec.ts` (create) — E2E: bold a range, number format, peer sees style.

---

## Task 1: Go wire — `Op.Props` + interning in `Apply`

**Files:**
- Modify: `lib/sheet/op.go`
- Modify: `lib/sheet/apply.go`
- Test: `lib/sheet/apply_test.go`

**Interfaces:**
- Produces: `Op.Props map[string]string` (json `props,omitempty`). `Apply` interns `op.Props` via `w.Styles.Put(Style{Props: op.Props})` and assigns the resulting id to the target cell's `StyleId`, for both `setCell` and `setStyle`.

- [ ] **Step 1: Write the failing test**

Add to `lib/sheet/apply_test.go`:

```go
func TestApplySetStyleInternsProps(t *testing.T) {
	w := NewWorkbook()
	w.AddSheet("s1", "Sheet1")

	props := map[string]string{"bold": "1", "color": "#cc0000"}
	if err := w.Apply(Op{Type: OpSetStyle, Sheet: "s1", Row: 1, Col: 2, Props: props}); err != nil {
		t.Fatal(err)
	}
	cell := w.SheetByID("s1").GetCell(CellRef{1, 2})
	if cell.StyleId == 0 {
		t.Fatalf("expected non-zero styleId after interning props")
	}
	got, ok := w.Styles.Get(cell.StyleId)
	if !ok || got.Props["bold"] != "1" || got.Props["color"] != "#cc0000" {
		t.Fatalf("pool did not store props: %+v ok=%v", got, ok)
	}

	// Dedup: identical props reused on another cell -> same id.
	if err := w.Apply(Op{Type: OpSetStyle, Sheet: "s1", Row: 3, Col: 4, Props: props}); err != nil {
		t.Fatal(err)
	}
	if w.SheetByID("s1").GetCell(CellRef{3, 4}).StyleId != cell.StyleId {
		t.Fatalf("identical props should dedup to the same id")
	}

	// Different props -> different id.
	if err := w.Apply(Op{Type: OpSetStyle, Sheet: "s1", Row: 5, Col: 6, Props: map[string]string{"italic": "1"}}); err != nil {
		t.Fatal(err)
	}
	if w.SheetByID("s1").GetCell(CellRef{5, 6}).StyleId == cell.StyleId {
		t.Fatalf("different props must not share an id")
	}
}

func TestApplySetCellWithPropsInterns(t *testing.T) {
	w := NewWorkbook()
	w.AddSheet("s1", "Sheet1")
	raw := "42"
	if err := w.Apply(Op{Type: OpSetCell, Sheet: "s1", Row: 0, Col: 0, Raw: &raw, Props: map[string]string{"align": "right"}}); err != nil {
		t.Fatal(err)
	}
	cell := w.SheetByID("s1").GetCell(CellRef{0, 0})
	if cell.Raw != "42" || cell.StyleId == 0 {
		t.Fatalf("setCell should set raw AND intern props: %+v", cell)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./lib/sheet/ -run 'TestApplySetStyleInternsProps|TestApplySetCellWithPropsInterns' -v`
Expected: FAIL (compile error: `Op` has no field `Props`).

- [ ] **Step 3: Implement**

In `lib/sheet/op.go`, add the field to the `Op` struct (near `StyleId`):
```go
	// setCell + setStyle: style properties to intern into the workbook StylePool.
	// When present, Apply interns them and sets the cell's StyleId to the result.
	Props map[string]string `json:"props,omitempty"`
```
Update `Validate`:
```go
	case OpSetCell:
		if o.Raw == nil && o.StyleId == nil && o.Props == nil {
			return fmt.Errorf("setCell needs raw, styleId, and/or props")
		}
		if o.Row < 0 || o.Col < 0 {
			return fmt.Errorf("setCell negative coord")
		}
	case OpSetStyle:
		if o.StyleId == nil && o.Props == nil {
			return fmt.Errorf("setStyle needs styleId or props")
		}
		if o.Row < 0 || o.Col < 0 {
			return fmt.Errorf("setStyle negative coord")
		}
```

In `lib/sheet/apply.go`, add a helper and use it in both cases. Add at the top of the `switch` handling, replace the `OpSetCell` and `OpSetStyle` cases:
```go
	case OpSetCell:
		cur := s.GetCell(CellRef{op.Row, op.Col})
		if op.Raw != nil {
			cur.Raw = *op.Raw
			cur.Value = ""
			cur.ValueType = ""
		}
		if op.Value != nil {
			cur.Value = *op.Value
		}
		if op.ValueType != nil {
			cur.ValueType = *op.ValueType
		}
		if op.Props != nil {
			cur.StyleId = w.Styles.Put(Style{Props: op.Props})
		} else if op.StyleId != nil {
			cur.StyleId = *op.StyleId
		}
		s.SetCell(CellRef{op.Row, op.Col}, cur)
	case OpSetStyle:
		cur := s.GetCell(CellRef{op.Row, op.Col})
		if op.Props != nil {
			cur.StyleId = w.Styles.Put(Style{Props: op.Props})
		} else {
			cur.StyleId = *op.StyleId
		}
		s.SetCell(CellRef{op.Row, op.Col}, cur)
```
(Note: `w.Styles` is the workbook's `*StylePool`, already present. `Style{Props: ...}` and `Put` exist in `style.go`. A cell with only a non-zero `StyleId` is not empty per `Cell.IsEmpty()`, so `SetCell` keeps it.)

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./lib/sheet/ -run 'TestApplySetStyleInternsProps|TestApplySetCellWithPropsInterns' -v`
Expected: PASS.

- [ ] **Step 5: Convergence test**

Add to `lib/sheet/convergence_test.go` (follow the file's existing helper style — read it first):

```go
func TestConvergenceStyleProps(t *testing.T) {
	// Two documents applying the same two style ops in the same order must end
	// with identical pool ids on the affected cells (ids stay in lockstep).
	mk := func() *Workbook { w := NewWorkbook(); w.AddSheet("s1", "Sheet1"); return w }
	a, b := mk(), mk()
	ops := []Op{
		{Type: OpSetStyle, Sheet: "s1", Row: 0, Col: 0, Props: map[string]string{"bold": "1"}},
		{Type: OpSetStyle, Sheet: "s1", Row: 1, Col: 1, Props: map[string]string{"italic": "1"}},
	}
	for _, op := range ops {
		if err := a.Apply(op); err != nil { t.Fatal(err) }
		if err := b.Apply(op); err != nil { t.Fatal(err) }
	}
	if a.SheetByID("s1").GetCell(CellRef{0, 0}).StyleId != b.SheetByID("s1").GetCell(CellRef{0, 0}).StyleId {
		t.Fatalf("style ids diverged between documents")
	}
}
```

- [ ] **Step 6: Run full package + commit**

Run: `go test ./lib/sheet/...`
Expected: PASS (all existing + new).

```bash
git add lib/sheet/op.go lib/sheet/apply.go lib/sheet/apply_test.go lib/sheet/convergence_test.go
git commit -m "feat(sheet): style ops carry props; Apply interns into the StylePool"
```

---

## Task 2: TS StylePool mirror + workbookState interning

**Files:**
- Modify: `ui/src/js/sheet/op.ts`
- Create: `ui/src/js/sheet/stylePool.ts`
- Create: `ui/src/js/sheet/stylePool.test.ts`
- Modify: `ui/src/js/sheet/workbookState.ts`
- Modify: `ui/src/js/sheet/workbookState.test.ts`

**Interfaces:**
- Produces (`op.ts`): `Op.props?: Record<string, string>`.
- Produces (`stylePool.ts`):
  - `type StyleProps = Record<string, string>`
  - `class StylePoolMirror { put(props: StyleProps): number; get(id: number): StyleProps | undefined; seed(snap: unknown): void }`
  - id 0 → `{}` (empty). `put` dedups by canonical key (sorted-key JSON). `seed` loads `{ idToStyle: { [id]: { props } }, nextId }` from the snapshot.
- Produces (`workbookState.ts`): `WorkbookState.styles: StylePoolMirror`; `applyOp` interns `op.props`; `getStyleProps(sheetId, row, col): StyleProps` (resolved props for a cell, `{}` if none).

**Why the ids converge:** each client seeds its mirror from the snapshot pool, then interns `op.props` from confirmed ops in the same total order the server did — so `nextId` advances identically. The wire never carries a style id for style ops. (Rendering keys off props, so a transient optimistic id never shows.)

- [ ] **Step 1: Write the failing test (stylePool)**

```typescript
// ui/src/js/sheet/stylePool.test.ts
import { describe, it, expect } from 'vitest';
import { StylePoolMirror } from './stylePool';

describe('StylePoolMirror', () => {
  it('id 0 is the empty style', () => {
    const p = new StylePoolMirror();
    expect(p.get(0)).toEqual({});
  });
  it('put dedups identical props regardless of key order', () => {
    const p = new StylePoolMirror();
    const a = p.put({ bold: '1', color: '#f00' });
    const b = p.put({ color: '#f00', bold: '1' });
    expect(a).toBe(b);
    expect(a).toBeGreaterThan(0);
    expect(p.get(a)).toEqual({ bold: '1', color: '#f00' });
  });
  it('different props get different ids; empty props is id 0', () => {
    const p = new StylePoolMirror();
    expect(p.put({})).toBe(0);
    expect(p.put({ bold: '1' })).not.toBe(p.put({ italic: '1' }));
  });
  it('seed restores id->props and continues nextId', () => {
    const p = new StylePoolMirror();
    p.seed({ idToStyle: { '1': { props: { bold: '1' } } }, nextId: 2 });
    expect(p.get(1)).toEqual({ bold: '1' });
    expect(p.put({ bold: '1' })).toBe(1);          // dedups to seeded id
    expect(p.put({ italic: '1' })).toBe(2);        // continues from nextId
  });
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd ui && npx vitest run src/js/sheet/stylePool.test.ts`
Expected: FAIL — cannot resolve `./stylePool`.

- [ ] **Step 3: Implement `stylePool.ts`**

```typescript
// ui/src/js/sheet/stylePool.ts
// TS mirror of lib/sheet.StylePool. Dedups styles by content so client-side
// interning of confirmed ops stays in lockstep with the Go server (both start
// from the same snapshot pool and intern the same ordered ops). The canonical
// key need only be injective on props content — it need not byte-match Go's.

export type StyleProps = Record<string, string>;

function canonicalKey(props: StyleProps): string {
  const keys = Object.keys(props).sort();
  if (keys.length === 0) return '';
  return JSON.stringify(keys.map((k) => [k, props[k]]));
}

export class StylePoolMirror {
  private idToStyle = new Map<number, StyleProps>();
  private keyToId = new Map<string, number>([['', 0]]);
  private nextId = 1;

  put(props: StyleProps): number {
    const key = canonicalKey(props);
    const existing = this.keyToId.get(key);
    if (existing !== undefined) return existing;
    const id = this.nextId++;
    this.idToStyle.set(id, { ...props });
    this.keyToId.set(key, id);
    return id;
  }

  get(id: number): StyleProps | undefined {
    if (id === 0) return {};
    return this.idToStyle.get(id);
  }

  // seed loads a serialized Go StylePool ({ idToStyle: {id: {props}}, nextId }).
  seed(snap: unknown): void {
    this.idToStyle = new Map();
    this.keyToId = new Map([['', 0]]);
    this.nextId = 1;
    if (!snap || typeof snap !== 'object') return;
    const s = snap as { idToStyle?: Record<string, { props?: StyleProps }>; nextId?: number };
    for (const [idStr, style] of Object.entries(s.idToStyle ?? {})) {
      const id = Number(idStr);
      const props = style?.props ?? {};
      this.idToStyle.set(id, props);
      this.keyToId.set(canonicalKey(props), id);
    }
    if (typeof s.nextId === 'number' && s.nextId > 0) this.nextId = s.nextId;
  }
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `cd ui && npx vitest run src/js/sheet/stylePool.test.ts`
Expected: PASS.

- [ ] **Step 5: Wire into `workbookState.ts` (write the failing test first)**

Add to `ui/src/js/sheet/workbookState.test.ts` (read the file first for its existing imports/helpers):

```typescript
import { WorkbookState } from './workbookState';
// ... existing imports/tests ...

describe('WorkbookState style props', () => {
  it('setStyle op interns props and getStyleProps resolves them', () => {
    const wb = new WorkbookState();
    wb.loadSnapshot({ sheets: [{ id: 's1', name: 'S', cells: [] }] });
    wb.applyOp({ type: 'setStyle', sheet: 's1', baseRev: 0, row: 0, col: 0, props: { bold: '1' } });
    expect(wb.getStyleProps('s1', 0, 0)).toEqual({ bold: '1' });
  });
  it('setCell op can carry raw and props together', () => {
    const wb = new WorkbookState();
    wb.loadSnapshot({ sheets: [{ id: 's1', name: 'S', cells: [] }] });
    wb.applyOp({ type: 'setCell', sheet: 's1', baseRev: 0, row: 1, col: 1, raw: '5', props: { align: 'right' } });
    expect(wb.getCell('s1', 1, 1)?.raw).toBe('5');
    expect(wb.getStyleProps('s1', 1, 1)).toEqual({ align: 'right' });
  });
});
```

- [ ] **Step 6: Run to verify it fails**

Run: `cd ui && npx vitest run src/js/sheet/workbookState.test.ts`
Expected: FAIL — `applyOp` ignores `props`; `getStyleProps` missing.

- [ ] **Step 7: Implement in `workbookState.ts`**

1. Import: `import { StylePoolMirror, type StyleProps } from './stylePool';`
2. Add `op.ts` field — in `ui/src/js/sheet/op.ts`, add to the `Op` interface:
```typescript
  // setCell / setStyle style properties, interned into the workbook style pool.
  props?: Record<string, string>;
```
3. Add a field and seed it in `loadSnapshot`:
```typescript
  styles = new StylePoolMirror();
```
   In `loadSnapshot(snap)`, after building sheets, add: `this.styles.seed(snap.styles);`
4. In `clone()`, the mirror can be shared by reference for display rebuilds (the display is rebuilt from serverWb+pending each time and only READS props); to stay safe, keep a single mirror on the confirmed state. Simplest correct approach: DO NOT deep-clone the pool per clone — share the same `StylePoolMirror` instance so all interning accretes in one pool. Change `clone()` to copy the reference:
```typescript
  clone(): WorkbookState {
    const cp = new WorkbookState();
    cp.sheets = this.sheets.map((s) => ({ id: s.id, name: s.name, cells: new Map(s.cells) }));
    cp.styles = this.styles; // shared pool: interning is monotonic + content-deduped
    return cp;
  }
```
   (Sharing is safe: `put` is idempotent by content and only appends new ids; the display clone never removes pool entries.)
5. In `applyOp`, `setCell` case — after the existing raw/value handling, before `this.setCell(...)`:
```typescript
        if (op.props !== undefined) {
          cur.styleId = this.styles.put(op.props);
        } else if (op.styleId !== undefined) {
          cur.styleId = op.styleId;
        }
```
   `setStyle` case — replace the body:
```typescript
      case 'setStyle': {
        const cur: Cell = { ...(sheet.cells.get(key(row, col)) ?? { raw: '' }) };
        cur.styleId = op.props !== undefined ? this.styles.put(op.props) : op.styleId;
        this.setCell(sheet, row, col, cur);
        break;
      }
```
6. Add the accessor:
```typescript
  getStyleProps(sheetId: string, row: number, col: number): StyleProps {
    const id = this.getCell(sheetId, row, col)?.styleId ?? 0;
    return this.styles.get(id) ?? {};
  }
```

- [ ] **Step 8: Run tests + commit**

Run: `cd ui && npx vitest run src/js/sheet`
Expected: PASS (all sheet tests).

```bash
git add ui/src/js/sheet/op.ts ui/src/js/sheet/stylePool.ts ui/src/js/sheet/stylePool.test.ts ui/src/js/sheet/workbookState.ts ui/src/js/sheet/workbookState.test.ts
git commit -m "feat(sheet): TS style pool mirror; workbookState interns style props"
```

---

## Task 3: `formatValue` — number/date formatting (pure)

**Files:**
- Create: `ui/src/js/sheet/format.ts`
- Create: `ui/src/js/sheet/format.test.ts`

**Interfaces:**
- Produces: `function formatValue(value: string, valueType: string, numFmt: string | undefined): string`
  - `numFmt` forms: `general`/undefined (return `value` unchanged), `text` (return `value`), `number[:d]`, `currency[:d]` (USD `$`), `percent[:d]`, `date` (value is an ISO date or a number-of-days serial → locale date). Non-numeric `value` under a numeric fmt returns `value` unchanged (never throws).

- [ ] **Step 1: Write the failing test**

```typescript
// ui/src/js/sheet/format.test.ts
import { describe, it, expect } from 'vitest';
import { formatValue } from './format';

describe('formatValue', () => {
  it('general / text / undefined return the value unchanged', () => {
    expect(formatValue('1234.5', 'number', 'general')).toBe('1234.5');
    expect(formatValue('1234.5', 'number', undefined)).toBe('1234.5');
    expect(formatValue('=A1', 'text', 'text')).toBe('=A1');
  });
  it('number applies grouping and optional decimals', () => {
    expect(formatValue('1234.5', 'number', 'number')).toBe('1,234.5');
    expect(formatValue('1234.5', 'number', 'number:2')).toBe('1,234.50');
  });
  it('currency and percent', () => {
    expect(formatValue('1234.5', 'number', 'currency:2')).toBe('$1,234.50');
    expect(formatValue('0.125', 'number', 'percent:1')).toBe('12.5%');
  });
  it('non-numeric value under a numeric format is returned unchanged', () => {
    expect(formatValue('hello', 'text', 'number')).toBe('hello');
    expect(formatValue('', 'empty', 'currency')).toBe('');
  });
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd ui && npx vitest run src/js/sheet/format.test.ts`
Expected: FAIL — cannot resolve `./format`.

- [ ] **Step 3: Implement**

```typescript
// ui/src/js/sheet/format.ts
// Pure display formatting. raw stays the source of truth; this only affects how
// the computed value is shown. Uses the built-in Intl API (no dependency).

function parseFmt(numFmt: string): { kind: string; decimals: number | undefined } {
  const [kind, d] = numFmt.split(':');
  return { kind, decimals: d === undefined ? undefined : Number(d) };
}

export function formatValue(value: string, _valueType: string, numFmt: string | undefined): string {
  if (!numFmt || numFmt === 'general' || numFmt === 'text') return value;
  const { kind, decimals } = parseFmt(numFmt);

  if (kind === 'date') {
    const d = /^\d+(\.\d+)?$/.test(value)
      ? new Date(Date.UTC(1899, 11, 30) + Number(value) * 86400000) // spreadsheet serial
      : new Date(value);
    return isNaN(d.getTime()) ? value : d.toLocaleDateString();
  }

  const n = Number(value);
  if (value === '' || Number.isNaN(n)) return value; // non-numeric: leave as-is

  const opts: Intl.NumberFormatOptions =
    decimals === undefined ? {} : { minimumFractionDigits: decimals, maximumFractionDigits: decimals };
  switch (kind) {
    case 'number':
      return new Intl.NumberFormat(undefined, { useGrouping: true, ...opts }).format(n);
    case 'currency':
      return new Intl.NumberFormat(undefined, { style: 'currency', currency: 'USD', ...opts }).format(n);
    case 'percent':
      return new Intl.NumberFormat(undefined, { style: 'percent', ...opts }).format(n);
    default:
      return value;
  }
}
```
Note: `percent` multiplies by 100 (Intl behavior) so `0.125` → `12.5%` — matches the test.

- [ ] **Step 4: Run to verify it passes**

Run: `cd ui && npx vitest run src/js/sheet/format.test.ts`
Expected: PASS. (If a locale difference makes grouping/currency assertions flaky in CI, pin the locale to `'en-US'` in the `Intl.NumberFormat` calls and update the test expectations accordingly — do this only if the default-locale run fails.)

- [ ] **Step 5: Commit**

```bash
git add ui/src/js/sheet/format.ts ui/src/js/sheet/format.test.ts
git commit -m "feat(sheet): pure formatValue for number/currency/percent/date display"
```

---

## Task 4: Style→CSS mapping + merge/toggle helpers (pure), and grid rendering

**Files:**
- Create: `ui/src/js/sheet/styleCss.ts`
- Create: `ui/src/js/sheet/styleCss.test.ts`
- Modify: `ui/src/js/sheet/sheetView.ts`

**Interfaces:**
- Produces (`styleCss.ts`):
  - `type CellCss = { fontWeight?: string; fontStyle?: string; textDecoration?: string; color?: string; background?: string; textAlign?: string; border?: string }`
  - `function styleToCss(props: Record<string,string>): CellCss`
  - `function mergeProps(base: Record<string,string>, change: Record<string,string>): Record<string,string>` — returns a new map; a key whose value is `''` is REMOVED (so toggles can clear).
  - `function toggleProp(props: Record<string,string>, key: string, on: boolean, value = '1'): Record<string,string>` — sets `key=value` when `on`, removes it when `!on`.
- Produces (`sheetView.ts`): new `SheetViewOptions.styleOf?: (row, col) => Record<string,string>`; `render()` applies `styleToCss` to each cell and clears style props on cells that have none.

- [ ] **Step 1: Write the failing test (styleCss)**

```typescript
// ui/src/js/sheet/styleCss.test.ts
import { describe, it, expect } from 'vitest';
import { styleToCss, mergeProps, toggleProp } from './styleCss';

describe('styleToCss', () => {
  it('maps known props to CSS', () => {
    expect(styleToCss({ bold: '1', italic: '1', underline: '1', color: '#c00', bg: '#eee', align: 'right' }))
      .toEqual({ fontWeight: 'bold', fontStyle: 'italic', textDecoration: 'underline', color: '#c00', background: '#eee', textAlign: 'right' });
  });
  it('border all -> a border css; empty props -> empty css', () => {
    expect(styleToCss({ border: 'all' }).border).toBe('1px solid #333');
    expect(styleToCss({})).toEqual({});
  });
});

describe('mergeProps / toggleProp', () => {
  it('merge overlays and empty-string removes', () => {
    expect(mergeProps({ bold: '1', color: '#c00' }, { italic: '1' })).toEqual({ bold: '1', color: '#c00', italic: '1' });
    expect(mergeProps({ bold: '1', color: '#c00' }, { color: '' })).toEqual({ bold: '1' });
  });
  it('toggleProp adds/removes', () => {
    expect(toggleProp({ color: '#c00' }, 'bold', true)).toEqual({ color: '#c00', bold: '1' });
    expect(toggleProp({ color: '#c00', bold: '1' }, 'bold', false)).toEqual({ color: '#c00' });
  });
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd ui && npx vitest run src/js/sheet/styleCss.test.ts`
Expected: FAIL — cannot resolve `./styleCss`.

- [ ] **Step 3: Implement `styleCss.ts`**

```typescript
// ui/src/js/sheet/styleCss.ts
// Pure mapping from style props (the wire vocabulary) to inline CSS, plus merge
// helpers used by the toolbar. Keeps the DOM view free of formatting policy.

export type CellCss = {
  fontWeight?: string; fontStyle?: string; textDecoration?: string;
  color?: string; background?: string; textAlign?: string; border?: string;
};

export function styleToCss(props: Record<string, string>): CellCss {
  const css: CellCss = {};
  if (props.bold === '1') css.fontWeight = 'bold';
  if (props.italic === '1') css.fontStyle = 'italic';
  if (props.underline === '1') css.textDecoration = 'underline';
  if (props.color) css.color = props.color;
  if (props.bg) css.background = props.bg;
  if (props.align) css.textAlign = props.align;
  if (props.border === 'all') css.border = '1px solid #333';
  return css;
}

export function mergeProps(base: Record<string, string>, change: Record<string, string>): Record<string, string> {
  const out: Record<string, string> = { ...base };
  for (const [k, v] of Object.entries(change)) {
    if (v === '') delete out[k];
    else out[k] = v;
  }
  return out;
}

export function toggleProp(props: Record<string, string>, key: string, on: boolean, value = '1'): Record<string, string> {
  return mergeProps(props, { [key]: on ? value : '' });
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `cd ui && npx vitest run src/js/sheet/styleCss.test.ts`
Expected: PASS.

- [ ] **Step 5: Apply styles in `sheetView.ts` render()**

1. Import: `import { styleToCss } from './styleCss';`
2. Add to `SheetViewOptions`: `styleOf?: (row: number, col: number) => Record<string, string>;`
3. In `render()`, inside the FIRST cell loop (the one that sets `td.textContent`), after setting text, apply the style. Because `styleToCss` returns only the props that are set, you MUST reset the full set of style properties each render so cleared styles disappear. Add, right after the `td.textContent = ...` line and before the `deco` handling:
```typescript
        // Reset then apply cell formatting (props resolved by the editor).
        td.style.fontWeight = '';
        td.style.fontStyle = '';
        td.style.textDecoration = '';
        td.style.color = '';
        td.style.background = '';
        td.style.textAlign = '';
        td.style.border = '';
        if (this.opts.styleOf) {
          const css = styleToCss(this.opts.styleOf(r, c));
          if (css.fontWeight) td.style.fontWeight = css.fontWeight;
          if (css.fontStyle) td.style.fontStyle = css.fontStyle;
          if (css.textDecoration) td.style.textDecoration = css.textDecoration;
          if (css.color) td.style.color = css.color;
          if (css.background) td.style.background = css.background;
          if (css.textAlign) td.style.textAlign = css.textAlign;
          if (css.border) td.style.border = css.border;
        }
```
   Note: this runs for every non-editing cell (the loop already `continue`s on the editing cell). Setting `td.style.border=''` reverts to the grid's CSS border (the `.sheet-grid td` rule), so cleared borders look normal.

- [ ] **Step 6: Run the sheet suite (no regression) + commit**

Run: `cd ui && npx vitest run src/js/sheet`
Expected: PASS (existing sheet tests unaffected; styleCss tests green).

```bash
git add ui/src/js/sheet/styleCss.ts ui/src/js/sheet/styleCss.test.ts ui/src/js/sheet/sheetView.ts
git commit -m "feat(sheet): style->css helpers and styled cell rendering"
```

---

## Task 5: Toolbar + editor wiring

**Files:**
- Create: `ui/src/js/sheet/sheetToolbar.ts`
- Modify: `ui/src/js/sheet/sheetEditor.ts`

**Interfaces:**
- Consumes: `mergeProps`/`toggleProp` from `./styleCss`; `selCells`, type `Selection` from `./sheetSelection`.
- Produces (`sheetToolbar.ts`):
  - `interface ToolbarCallbacks { getProps: (row: number, col: number) => Record<string,string>; applyToSelection: (change: Record<string,string>) => void; readOnly: boolean }`
  - `function createToolbar(cb: ToolbarCallbacks): HTMLElement` — returns the toolbar element. Buttons emit a `change` map via `cb.applyToSelection`: B→`{bold:'1'}` or `{bold:''}` (toggle based on the focus cell's current props via `cb.getProps`), similarly I/U; color inputs → `{color:hex}`/`{bg:hex}`; align select → `{align}`; border button → `{border:'all'}`/`{border:''}`; numFmt select → `{numFmt}`.
- Produces (`sheetEditor.ts`): mounts the toolbar above the grid; `applyToSelection(change)` computes, per cell in the current selection, `mergeProps(currentProps, change)` and sends one `setStyle` op; `styleOf` passed to the view resolves props from `collab.display`; `displayValue` applies `numFmt` via `formatValue`.

- [ ] **Step 1: Implement `sheetToolbar.ts`**

(No unit test — DOM component; covered by the Task 6 E2E. The pure logic it uses — `mergeProps`/`toggleProp` — is already tested in Task 4.)

```typescript
// ui/src/js/sheet/sheetToolbar.ts
// Minimal formatting toolbar. Emits style-prop *changes* for the current
// selection; the editor merges them onto each cell's existing props and sends
// setStyle ops. Uses native inputs (no dependency).

export interface ToolbarCallbacks {
  getProps: (row: number, col: number) => Record<string, string>;
  focusCell: () => { row: number; col: number };
  applyToSelection: (change: Record<string, string>) => void;
  readOnly: boolean;
}

const CSS = `
.sheet-toolbar { display: flex; gap: 4px; align-items: center; padding: 4px; border-bottom: 1px solid #d2d2d2; font: 13px system-ui, sans-serif; flex-wrap: wrap; }
.sheet-toolbar button, .sheet-toolbar select { height: 24px; min-width: 24px; cursor: pointer; }
.sheet-toolbar button.on { background: #cfeede; }
.sheet-toolbar input[type=color] { width: 26px; height: 24px; padding: 0; border: 1px solid #ccc; }
.sheet-toolbar[aria-disabled=true] { opacity: 0.5; pointer-events: none; }
`;

export function createToolbar(cb: ToolbarCallbacks): HTMLElement {
  if (!document.getElementById('sheet-toolbar-style')) {
    const s = document.createElement('style');
    s.id = 'sheet-toolbar-style';
    s.textContent = CSS;
    document.head.appendChild(s);
  }
  const bar = document.createElement('div');
  bar.className = 'sheet-toolbar';
  if (cb.readOnly) bar.setAttribute('aria-disabled', 'true');

  const curProps = () => { const f = cb.focusCell(); return cb.getProps(f.row, f.col); };

  const toggleBtn = (label: string, key: string) => {
    const b = document.createElement('button');
    b.textContent = label;
    b.title = key;
    b.dataset.key = key;
    b.addEventListener('click', () => {
      const on = curProps()[key] === '1';
      cb.applyToSelection({ [key]: on ? '' : '1' });
    });
    bar.appendChild(b);
    return b;
  };
  toggleBtn('B', 'bold').style.fontWeight = 'bold';
  toggleBtn('I', 'italic').style.fontStyle = 'italic';
  toggleBtn('U', 'underline').style.textDecoration = 'underline';

  const color = document.createElement('input');
  color.type = 'color'; color.title = 'Font color';
  color.addEventListener('input', () => cb.applyToSelection({ color: color.value }));
  bar.appendChild(color);

  const bg = document.createElement('input');
  bg.type = 'color'; bg.title = 'Fill color'; bg.value = '#ffffff';
  bg.addEventListener('input', () => cb.applyToSelection({ bg: bg.value }));
  bar.appendChild(bg);

  const align = document.createElement('select');
  align.title = 'Align';
  for (const a of ['left', 'center', 'right']) {
    const o = document.createElement('option'); o.value = a; o.textContent = a; align.appendChild(o);
  }
  align.addEventListener('change', () => cb.applyToSelection({ align: align.value }));
  bar.appendChild(align);

  const border = document.createElement('button');
  border.textContent = '▦'; border.title = 'Borders';
  border.addEventListener('click', () => {
    const on = curProps().border === 'all';
    cb.applyToSelection({ border: on ? '' : 'all' });
  });
  bar.appendChild(border);

  const numFmt = document.createElement('select');
  numFmt.title = 'Number format';
  for (const [v, label] of [['general', 'General'], ['number:2', 'Number'], ['currency:2', 'Currency'], ['percent:0', 'Percent'], ['date', 'Date'], ['text', 'Text']] as const) {
    const o = document.createElement('option'); o.value = v; o.textContent = label; numFmt.appendChild(o);
  }
  numFmt.addEventListener('change', () => cb.applyToSelection({ numFmt: numFmt.value }));
  bar.appendChild(numFmt);

  return bar;
}
```

- [ ] **Step 2: Wire into `sheetEditor.ts`**

1. Imports:
```typescript
import { createToolbar } from './sheetToolbar';
import { mergeProps } from './styleCss';
import { formatValue } from './format';
import { selCells } from './sheetSelection';
```
2. `styleOf` + numFmt display. Add a helper and use it:
```typescript
  const propsOf = (r: number, c: number): Record<string, string> =>
    collab ? collab.display.getStyleProps(activeSheetId, r, c) : {};
```
   Change `displayValue` so numFmt is applied to the computed value:
```typescript
  const displayValue = (r: number, c: number): string => {
    const cell = collab?.display.getCell(activeSheetId, r, c);
    if (!cell || cell.raw === '') return '';
    const raw = cell.raw.startsWith('=') ? engine.getValue(r, c).value : cell.raw;
    return formatValue(raw, '', propsOf(r, c).numFmt);
  };
```
3. `applyToSelection` — merge per cell, send one setStyle op each:
```typescript
  const applyStyleToSelection = (change: Record<string, string>): void => {
    if (readOnly || !collab) return;
    for (const { row, col } of selCells(selection)) {
      const merged = mergeProps(propsOf(row, col), change);
      collab.applyLocal({ type: 'setStyle', sheet: activeSheetId, baseRev: collab.rev, row, col, props: merged });
    }
  };
```
4. Mount the toolbar ABOVE the grid. The view does `root.innerHTML=''`, so give the view its own sub-container. In `initSheet`, BEFORE constructing the view, restructure `root`:
```typescript
    root.innerHTML = '';
    const toolbar = createToolbar({
      getProps: (r, c) => propsOf(r, c),
      focusCell: () => selection.focus,
      applyToSelection: applyStyleToSelection,
      readOnly: data.readonly,
    });
    const gridHost = document.createElement('div');
    root.appendChild(toolbar);
    root.appendChild(gridHost);
```
   Then construct the view against `gridHost` instead of `root`: `view = new DomSheetView(gridHost, { ... })`.
5. Pass `styleOf` in the view options object:
```typescript
      styleOf: (r, c) => propsOf(r, c),
```

- [ ] **Step 3: Type-check + run sheet suite**

Run: `cd ui && npx vitest run src/js/sheet`
Expected: PASS (no unit regressions).
Run: `cd ui && npx tsc --noEmit`
Expected: no errors in any `src/js/sheet/*` file (pre-existing `plugin/chat.ts` / `ep_*` errors are unrelated).

- [ ] **Step 4: Commit**

```bash
git add ui/src/js/sheet/sheetToolbar.ts ui/src/js/sheet/sheetEditor.ts
git commit -m "feat(sheet): formatting toolbar wired to selection + number-format display"
```

---

## Task 6: E2E — formatting

**Files:**
- Create: `playwright/specs/sheet_formatting.spec.ts`

**Context:** Reuse the sheet-pad bootstrap from `playwright/specs/sheet_selection.spec.ts` (route `/s/${padId}`, `.sheet-grid` wait, `tbody tr:nth-child / td:nth-child` cell locator). The toolbar is `.sheet-toolbar` with buttons `[data-key=bold]` etc. Validate with `--list`; the controller runs the full live E2E.

- [ ] **Step 1: Write the E2E**

```typescript
import { test, expect } from '@playwright/test';

test.describe('Sheet formatting', () => {
  test('bold applies to the selection and renders bold', async ({ page }) => {
    await page.goto('/s/fmt-e2e-' + Date.now());
    await page.locator('.sheet-grid').waitFor();
    const cell = (r: number, c: number) =>
      page.locator(`.sheet-grid tbody tr:nth-child(${r + 1}) td:nth-child(${c + 2})`);

    await cell(0, 0).click();
    await page.keyboard.type('hi');
    await page.keyboard.press('Enter');

    await cell(0, 0).click(); // select A1 (not editing)
    await page.locator('.sheet-toolbar button[data-key=bold]').click();

    await expect(cell(0, 0)).toHaveCSS('font-weight', /700|bold/);
  });

  test('number format renders grouped number', async ({ page }) => {
    await page.goto('/s/fmt-num-' + Date.now());
    await page.locator('.sheet-grid').waitFor();
    const cell = (r: number, c: number) =>
      page.locator(`.sheet-grid tbody tr:nth-child(${r + 1}) td:nth-child(${c + 2})`);

    await cell(0, 0).click();
    await page.keyboard.type('1234.5');
    await page.keyboard.press('Enter');

    await cell(0, 0).click();
    await page.locator('.sheet-toolbar select[title="Number format"]').selectOption('number:2');
    await expect(cell(0, 0)).toHaveText('1,234.50');
  });
});
```

- [ ] **Step 2: Validate (no server run)**

Run: `cd playwright && npx playwright test specs/sheet_formatting.spec.ts --list`
Expected: lists 2 tests, no parse/TS errors. (If node_modules missing: `pnpm install --frozen-lockfile` first — must NOT modify the lockfile.)

- [ ] **Step 3: Commit**

```bash
git add playwright/specs/sheet_formatting.spec.ts
git commit -m "test(sheet): E2E for cell formatting (bold + number format)"
```

---

## Self-Review notes (addressed)

- **Spec coverage (M2):** wire change with props + pool interning (T1, T2); prop vocabulary (constraints + T4 styleToCss); toolbar B/I/U/color/bg/align/border/numFmt (T5); styled rendering (T4); number formatting via Intl (T3, wired T5); read-only gating (T5); merge semantics client-side (T4 mergeProps + T5). All covered.
- **Convergence:** style ids stay in lockstep because every `serverWb` replays the same ordered op log from the same snapshot pool (T1 convergence test + T2 shared-pool design). No server id on the wire.
- **Type consistency:** `props`/`Props` map<string,string> on both Op types; `StylePoolMirror.put/get/seed`, `getStyleProps`, `styleToCss/mergeProps/toggleProp`, `formatValue`, `styleOf`, `applyToSelection` names used identically across tasks.
- **No new dependency:** Intl is built-in; toolbar uses native inputs; jsdom NOT reintroduced (view/toolbar covered by E2E).
- **Deferred (noted):** border edges beyond all/none; per-range batched style op (per-cell for M2); conditional formatting / data validation (out of scope per design spec).

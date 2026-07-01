# Excel M3 — Formula Bar Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a formula bar (Name Box + editable input) above the grid, two-way synced with the active cell; function-name autocomplete from HyperFormula; and distinct rendering of error cells with a hover message.

**Architecture:** A `sheetFormulaBar.ts` DOM component (Name Box span + text input) mounts between the M2 toolbar and the grid. The editor syncs it from the active cell on selection change, and a commit from the bar goes through the SAME `setCell` op path as in-cell editing. Autocomplete filters HyperFormula's registered function names via a pure helper. Error cells already show their error text (the engine returns `#DIV/0!` etc. with `type:'error'`); M3 adds a distinct class + `title` via a new view option.

**Tech Stack:** TypeScript (`ui/src/js/sheet`, vitest), HyperFormula, Playwright. No Go changes (formula bar edits are ordinary `setCell` ops).

## Global Constraints

- **No new dependencies.** Autocomplete dropdown + name box are native DOM.
- **No new op types.** A formula-bar commit is an ordinary `setCell` op via `collab.applyLocal` — identical to in-cell editing.
- **Read-only sessions:** the formula-bar input is disabled and never emits an op (server also rejects).
- Cell refs in the UI use A1 notation (`colName(col)+（row+1)`); the model stays zero-based.
- DOM behavior is covered by the Playwright E2E (jsdom rejected in M1); keep logic in pure, unit-tested helpers where possible.
- Vitest: `cd ui && npx vitest run src/js/sheet/<file>`. Build stays `vite build --mode sheet`.

---

## File Structure

- `ui/src/js/sheet/formulaEngine.ts` (modify) — add `functionNames(): string[]`.
- `ui/src/js/sheet/formulaEngine.test.ts` (modify) — assert some known functions are listed.
- `ui/src/js/sheet/a1.ts` (create) — pure `cellRefA1(row,col)` / `rangeRefA1(sel)` naming helpers (extract the `colName` logic so both the view and the bar share one implementation).
- `ui/src/js/sheet/a1.test.ts` (create) — ref-naming tests.
- `ui/src/js/sheet/autocomplete.ts` (create) — pure `functionPrefix(raw, caret)` + `filterFunctions(names, prefix)`.
- `ui/src/js/sheet/autocomplete.test.ts` (create) — prefix extraction + filter tests.
- `ui/src/js/sheet/sheetFormulaBar.ts` (create) — the Name Box + input + autocomplete dropdown component.
- `ui/src/js/sheet/sheetView.ts` (modify) — add `errorOf?(row,col)` option; apply `.sheet-cell-error` class + `title` in render().
- `ui/src/js/sheet/sheetEditor.ts` (modify) — mount the formula bar; sync on selection; commit/revert; wire `errorOf`.
- `playwright/specs/sheet_formula_bar.spec.ts` (create) — E2E.

---

## Task 1: Expose function names from the engine + A1 ref helpers

**Files:**
- Modify: `ui/src/js/sheet/formulaEngine.ts`
- Modify: `ui/src/js/sheet/formulaEngine.test.ts`
- Create: `ui/src/js/sheet/a1.ts`
- Create: `ui/src/js/sheet/a1.test.ts`

**Interfaces:**
- Produces: `FormulaEngine.functionNames(): string[]` — the registered HyperFormula function names (e.g. includes `SUM`, `IF`, `VLOOKUP`).
- Produces (`a1.ts`):
  - `function cellRefA1(row: number, col: number): string` — e.g. `(0,0)→"A1"`, `(6,1)→"B7"`.
  - `function rangeRefA1(r0: number, c0: number, r1: number, c1: number): string` — single cell → `"A1"`; range → `"A1:C5"`.

- [ ] **Step 1: Write the failing tests**

`ui/src/js/sheet/a1.test.ts`:
```typescript
import { describe, it, expect } from 'vitest';
import { cellRefA1, rangeRefA1 } from './a1';

describe('A1 refs', () => {
  it('cellRefA1 is 1-based row, letter col', () => {
    expect(cellRefA1(0, 0)).toBe('A1');
    expect(cellRefA1(6, 1)).toBe('B7');
    expect(cellRefA1(0, 26)).toBe('AA1');
  });
  it('rangeRefA1 collapses a single cell and orders corners', () => {
    expect(rangeRefA1(0, 0, 0, 0)).toBe('A1');
    expect(rangeRefA1(0, 0, 4, 2)).toBe('A1:C5');
  });
});
```
Add to `ui/src/js/sheet/formulaEngine.test.ts` (inside the existing describe or a new one):
```typescript
import { FormulaEngine } from './formulaEngine';
it('functionNames lists common spreadsheet functions', () => {
  const names = new FormulaEngine().functionNames();
  expect(names).toContain('SUM');
  expect(names).toContain('IF');
  expect(names.length).toBeGreaterThan(50);
});
```

- [ ] **Step 2: Run to verify they fail**

Run: `cd ui && npx vitest run src/js/sheet/a1.test.ts src/js/sheet/formulaEngine.test.ts`
Expected: FAIL — `./a1` missing; `functionNames` not a function.

- [ ] **Step 3: Implement `a1.ts`**

```typescript
// ui/src/js/sheet/a1.ts
// A1-notation helpers shared by the grid view and the formula bar's name box.

export function colName(col: number): string {
  let s = '';
  let n = col + 1;
  while (n > 0) {
    const rem = (n - 1) % 26;
    s = String.fromCharCode(65 + rem) + s;
    n = Math.floor((n - 1) / 26);
  }
  return s;
}

export function cellRefA1(row: number, col: number): string {
  return `${colName(col)}${row + 1}`;
}

export function rangeRefA1(r0: number, c0: number, r1: number, c1: number): string {
  const top = { r: Math.min(r0, r1), c: Math.min(c0, c1) };
  const bot = { r: Math.max(r0, r1), c: Math.max(c0, c1) };
  const a = cellRefA1(top.r, top.c);
  if (top.r === bot.r && top.c === bot.c) return a;
  return `${a}:${cellRefA1(bot.r, bot.c)}`;
}
```

- [ ] **Step 4: Implement `functionNames` in `formulaEngine.ts`**

Add a method to the `FormulaEngine` class:
```typescript
  // functionNames returns the HyperFormula-registered function names (for the
  // formula bar's autocomplete). Uses the default 'enGB' language pack that
  // buildEmpty registers.
  functionNames(): string[] {
    return HyperFormula.getRegisteredFunctionNames('enGB');
  }
```
Note: `getRegisteredFunctionNames` is a static method on `HyperFormula`. If the installed version returns an empty list for `'enGB'`, try `this.hf.getRegisteredFunctionNames()` (instance) and, if that also fails, `HyperFormula.getAllFunctionPlugins()`-derived names — but verify the enGB static call first; it is the documented API. Report which call you used.

- [ ] **Step 5: Run to verify they pass**

Run: `cd ui && npx vitest run src/js/sheet/a1.test.ts src/js/sheet/formulaEngine.test.ts`
Expected: PASS.

- [ ] **Step 6: Refactor the view to reuse `colName` (optional but DRY)**

In `ui/src/js/sheet/sheetView.ts`, replace its local `colName` function with an import from `./a1` (`import { colName } from './a1';`) and delete the local copy. Run `cd ui && npx vitest run src/js/sheet` to confirm no regression.

- [ ] **Step 7: Commit**

```bash
git add ui/src/js/sheet/formulaEngine.ts ui/src/js/sheet/formulaEngine.test.ts ui/src/js/sheet/a1.ts ui/src/js/sheet/a1.test.ts ui/src/js/sheet/sheetView.ts
git commit -m "feat(sheet): expose engine function names + shared A1 ref helpers"
```

---

## Task 2: Autocomplete pure helpers

**Files:**
- Create: `ui/src/js/sheet/autocomplete.ts`
- Create: `ui/src/js/sheet/autocomplete.test.ts`

**Interfaces:**
- Produces:
  - `function functionPrefix(raw: string, caret: number): string | null` — if the text before `caret` in a formula (`raw` starting with `=`) ends in an identifier being typed (letters, after `=`/operator/`(`/`,`), return the uppercased partial name; else `null`. E.g. `("=SU", 3) → "SU"`, `("=A1+VLoo", 8) → "VLOO"`, `("=SUM(A1", 7) → null` (inside args, `A1` is a ref not a function head — treat a token immediately preceded by a digit-or-`:` as not a function; simplest: only return a prefix when the partial token is pure letters AND the char after it is not `(`).
  - `function filterFunctions(names: string[], prefix: string): string[]` — names starting with `prefix` (case-insensitive), sorted, max 8.

- [ ] **Step 1: Write the failing test**

```typescript
// ui/src/js/sheet/autocomplete.test.ts
import { describe, it, expect } from 'vitest';
import { functionPrefix, filterFunctions } from './autocomplete';

describe('functionPrefix', () => {
  it('returns the partial function name being typed', () => {
    expect(functionPrefix('=SU', 3)).toBe('SU');
    expect(functionPrefix('=A1+VLoo', 8)).toBe('VLOO');
  });
  it('returns null when not typing a function name', () => {
    expect(functionPrefix('hello', 5)).toBeNull();   // not a formula
    expect(functionPrefix('=A1', 3)).toBe('A');       // "A" is a partial name until a digit follows... see note
    expect(functionPrefix('=SUM(', 5)).toBeNull();    // just after '(' — nothing typed
  });
});

describe('filterFunctions', () => {
  it('prefix-matches case-insensitively, sorted, capped at 8', () => {
    const names = ['SUM', 'SUMIF', 'SUMIFS', 'SUMPRODUCT', 'SUMSQ', 'SUMX2MY2', 'SUMX2PY2', 'SUMXMY2', 'SIN', 'IF'];
    const out = filterFunctions(names, 'sum');
    expect(out[0]).toBe('SUM');
    expect(out).not.toContain('SIN');
    expect(out.length).toBeLessThanOrEqual(8);
  });
});
```
Note on `=A1` → `"A"`: the prefix extractor is intentionally simple — it returns the trailing letter-run. The dropdown just won't match many functions for `"A"` and disappears once a digit is typed (`=A1` → the token becomes `A1`, not pure letters → `null`). This is acceptable; the test encodes it.

- [ ] **Step 2: Run to verify it fails**

Run: `cd ui && npx vitest run src/js/sheet/autocomplete.test.ts`
Expected: FAIL — `./autocomplete` missing.

- [ ] **Step 3: Implement**

```typescript
// ui/src/js/sheet/autocomplete.ts
// Pure helpers for formula function-name autocomplete. No DOM.

// functionPrefix returns the uppercased partial function name immediately left
// of `caret` in a formula, or null. A function head is a run of letters that is
// NOT immediately followed by a digit (which would make it a cell ref like A1)
// and not already closed by '('.
export function functionPrefix(raw: string, caret: number): string | null {
  if (!raw.startsWith('=')) return null;
  const left = raw.slice(0, caret);
  const m = /([A-Za-z]+)$/.exec(left);
  if (!m) return null;
  const after = raw.slice(caret);
  if (after.startsWith('(')) return null; // already a completed call head
  // If the letter run is immediately followed (in the full raw) by a digit, it
  // is a cell ref being typed, not a function name.
  if (/^[0-9]/.test(after)) return null;
  return m[1].toUpperCase();
}

export function filterFunctions(names: string[], prefix: string): string[] {
  const p = prefix.toUpperCase();
  return names
    .filter((n) => n.toUpperCase().startsWith(p))
    .sort()
    .slice(0, 8);
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `cd ui && npx vitest run src/js/sheet/autocomplete.test.ts`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add ui/src/js/sheet/autocomplete.ts ui/src/js/sheet/autocomplete.test.ts
git commit -m "feat(sheet): pure autocomplete prefix + filter helpers"
```

---

## Task 3: Formula bar component + autocomplete dropdown

**Files:**
- Create: `ui/src/js/sheet/sheetFormulaBar.ts`

**Interfaces:**
- Consumes: `functionPrefix`, `filterFunctions` from `./autocomplete`.
- Produces:
  - `interface FormulaBarCallbacks { onCommit: (raw: string) => void; getFunctionNames: () => string[]; readOnly: boolean }`
  - `interface FormulaBarHandle { el: HTMLElement; setActive: (ref: string, raw: string) => void }`
  - `function createFormulaBar(cb: FormulaBarCallbacks): FormulaBarHandle`
- Behavior: a name-box span (`.sheet-namebox`) + a text input (`.sheet-fx-input`). `setActive(ref, raw)` sets the name box text and the input value (unless the input is focused/mid-edit — don't stomp the user's typing). Enter in the input → `onCommit(input.value)` + blur. Escape → revert to the last `setActive` value + blur. As the user types a formula, show an autocomplete dropdown (`.sheet-fx-ac`) of `filterFunctions(getFunctionNames(), functionPrefix(value, caret))`; ArrowUp/Down move the highlighted item; Tab/Enter accept (insert the function name + `(` at the caret, replacing the partial); Escape closes the dropdown first (before reverting the field). Read-only → input `disabled`.

- [ ] **Step 1: Implement (no unit test — DOM component; covered by Task 5 E2E; the pure logic it uses is tested in Task 2)**

```typescript
// ui/src/js/sheet/sheetFormulaBar.ts
// Formula bar: a Name Box (active ref) + an editable input two-way-synced with
// the active cell, plus function-name autocomplete. Commits go through the same
// setCell path as in-cell editing (via onCommit).

import { functionPrefix, filterFunctions } from './autocomplete';

export interface FormulaBarCallbacks {
  onCommit: (raw: string) => void;
  getFunctionNames: () => string[];
  readOnly: boolean;
}

export interface FormulaBarHandle {
  el: HTMLElement;
  setActive: (ref: string, raw: string) => void;
}

const CSS = `
.sheet-formula-bar { display: flex; align-items: stretch; gap: 6px; padding: 3px 4px; border-bottom: 1px solid #d2d2d2; font: 13px system-ui, sans-serif; position: relative; }
.sheet-namebox { min-width: 72px; padding: 2px 6px; border: 1px solid #ccc; background: #f8f9fa; text-align: center; align-self: center; border-radius: 3px; }
.sheet-fx-input { flex: 1; padding: 2px 6px; border: 1px solid #ccc; font: 13px/1.4 ui-monospace, monospace; }
.sheet-fx-ac { position: absolute; top: 100%; left: 84px; z-index: 20; background: #fff; border: 1px solid #bbb; box-shadow: 0 2px 6px rgba(0,0,0,.15); min-width: 180px; max-height: 180px; overflow-y: auto; }
.sheet-fx-ac div { padding: 2px 8px; cursor: pointer; font: 12px/1.5 ui-monospace, monospace; }
.sheet-fx-ac div.hl { background: #cfeede; }
`;

export function createFormulaBar(cb: FormulaBarCallbacks): FormulaBarHandle {
  if (!document.getElementById('sheet-formula-bar-style')) {
    const s = document.createElement('style');
    s.id = 'sheet-formula-bar-style';
    s.textContent = CSS;
    document.head.appendChild(s);
  }
  const bar = document.createElement('div');
  bar.className = 'sheet-formula-bar';

  const nameBox = document.createElement('span');
  nameBox.className = 'sheet-namebox';
  nameBox.textContent = 'A1';

  const input = document.createElement('input');
  input.className = 'sheet-fx-input';
  input.type = 'text';
  input.disabled = cb.readOnly;

  const ac = document.createElement('div');
  ac.className = 'sheet-fx-ac';
  ac.style.display = 'none';

  bar.append(nameBox, input, ac);

  let lastRaw = '';
  let acItems: string[] = [];
  let acIndex = -1;

  const closeAc = (): void => { ac.style.display = 'none'; acItems = []; acIndex = -1; };

  const renderAc = (): void => {
    const prefix = functionPrefix(input.value, input.selectionStart ?? input.value.length);
    acItems = prefix ? filterFunctions(cb.getFunctionNames(), prefix) : [];
    if (acItems.length === 0) { closeAc(); return; }
    acIndex = 0;
    ac.innerHTML = '';
    acItems.forEach((name, i) => {
      const d = document.createElement('div');
      d.textContent = name;
      if (i === acIndex) d.className = 'hl';
      d.addEventListener('mousedown', (e) => { e.preventDefault(); accept(name); });
      ac.appendChild(d);
    });
    ac.style.display = 'block';
  };

  const highlight = (): void => {
    [...ac.children].forEach((c, i) => (c as HTMLElement).className = i === acIndex ? 'hl' : '');
  };

  const accept = (name: string): void => {
    const caret = input.selectionStart ?? input.value.length;
    const left = input.value.slice(0, caret).replace(/[A-Za-z]+$/, '');
    const right = input.value.slice(caret);
    input.value = `${left}${name}(${right}`;
    const pos = left.length + name.length + 1;
    input.setSelectionRange(pos, pos);
    closeAc();
    input.focus();
  };

  input.addEventListener('input', renderAc);
  input.addEventListener('blur', () => closeAc());
  input.addEventListener('keydown', (e: KeyboardEvent) => {
    if (ac.style.display === 'block' && acItems.length) {
      if (e.key === 'ArrowDown') { e.preventDefault(); acIndex = (acIndex + 1) % acItems.length; return highlight(); }
      if (e.key === 'ArrowUp') { e.preventDefault(); acIndex = (acIndex - 1 + acItems.length) % acItems.length; return highlight(); }
      if (e.key === 'Tab' || e.key === 'Enter') { e.preventDefault(); return accept(acItems[acIndex]); }
      if (e.key === 'Escape') { e.preventDefault(); return closeAc(); }
    }
    if (e.key === 'Enter') { e.preventDefault(); cb.onCommit(input.value); input.blur(); }
    else if (e.key === 'Escape') { e.preventDefault(); input.value = lastRaw; input.blur(); }
  });

  return {
    el: bar,
    setActive(ref: string, raw: string): void {
      nameBox.textContent = ref;
      lastRaw = raw;
      // Don't stomp the user's in-progress typing in the bar.
      if (document.activeElement !== input) input.value = raw;
    },
  };
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd ui && npx tsc --noEmit` — no NEW errors in `src/js/sheet/*` (pre-existing `plugin/chat.ts`/`ep_*` errors are unrelated).
Run: `cd ui && npx vitest run src/js/sheet` — existing suite still passes (this file adds no tests but must not break compilation of the suite).

- [ ] **Step 3: Commit**

```bash
git add ui/src/js/sheet/sheetFormulaBar.ts
git commit -m "feat(sheet): formula bar component with name box + autocomplete"
```

---

## Task 4: Error-cell rendering + wire the formula bar in the editor

**Files:**
- Modify: `ui/src/js/sheet/sheetView.ts`
- Modify: `ui/src/js/sheet/sheetEditor.ts`

**Interfaces:**
- Consumes: `createFormulaBar` from `./sheetFormulaBar`; `cellRefA1`/`rangeRefA1` from `./a1`.
- Produces (`sheetView.ts`): `SheetViewOptions.errorOf?: (row, col) => string | undefined`; render() adds `.sheet-cell-error` class + `title` when `errorOf` returns a string, and clears both otherwise.

- [ ] **Step 1: Error rendering in the view**

In `ui/src/js/sheet/sheetView.ts`:
1. Add to `SheetViewOptions`: `errorOf?: (row: number, col: number) => string | undefined;`
2. Add the CSS to the `CSS` string: `.sheet-grid td.sheet-cell-error { color: #c0392b; }`
3. In render()'s first cell loop, right after the style-apply block (from M2), add:
```typescript
        const err = this.opts.errorOf?.(r, c);
        td.classList.toggle('sheet-cell-error', !!err);
        if (err) td.title = err; else td.removeAttribute('title');
```
   (This runs on every non-editing cell each render, so it self-corrects — no separate reset needed.)

- [ ] **Step 2: Wire the formula bar + errorOf in `sheetEditor.ts`**

1. Imports:
```typescript
import { createFormulaBar, type FormulaBarHandle } from './sheetFormulaBar';
import { rangeRefA1 } from './a1';
import { normalize } from './sheetSelection';
```
   (`normalize` is already imported in M1 — don't duplicate.)
2. Add a handle var near `view`: `let formulaBar: FormulaBarHandle | null = null;`
3. In `initSheet`, when building the toolbar/gridHost structure (M2 added this), also create and mount the formula bar BELOW the toolbar and ABOVE the grid:
```typescript
    formulaBar = createFormulaBar({
      readOnly: data.readonly,
      getFunctionNames: () => engine.functionNames(),
      onCommit: (raw) => {
        if (readOnly || !collab) return;
        const { row, col } = selection.focus;
        collab.applyLocal({ type: 'setCell', sheet: activeSheetId, baseRev: collab.rev, row, col, raw });
      },
    });
    // order inside root: toolbar, formula bar, grid host
    root.appendChild(formulaBar.el);
    root.appendChild(gridHost);
```
   (Insert `root.appendChild(formulaBar.el)` between the toolbar append and the gridHost append that M2 introduced.)
4. Sync the bar from the active cell. In the view's `onSelectionChange`, after updating `selection`, refresh the bar:
```typescript
      onSelectionChange: (sel) => {
        selection = sel;
        sendPresence(sel.anchor.row, sel.anchor.col, false, undefined, sel.focus.row, sel.focus.col);
        const { r0, c0, r1, c1 } = normalize(sel);
        formulaBar?.setActive(rangeRefA1(r0, c0, r1, c1), rawValue(sel.focus.row, sel.focus.col));
      },
```
5. Also refresh the bar in `onChange` so a committed value / remote edit updates the bar's input for the active cell:
```typescript
      // at the end of onChange, after view?.render():
      if (formulaBar) {
        const { r0, c0, r1, c1 } = normalize(selection);
        formulaBar.setActive(rangeRefA1(r0, c0, r1, c1), rawValue(selection.focus.row, selection.focus.col));
      }
```
   (`setActive` won't stomp the input while it's focused, so typing in the bar is safe.)
6. Pass `errorOf` to the view options:
```typescript
      errorOf: (r, c) => {
        const cell = collab?.display.getCell(activeSheetId, r, c);
        if (!cell || !cell.raw.startsWith('=')) return undefined;
        const res = engine.getValue(r, c);
        return res.type === 'error' ? res.value : undefined;
      },
```

- [ ] **Step 3: Verify**

Run: `cd ui && npx vitest run src/js/sheet` — existing suite passes (no regression).
Run: `cd ui && npx tsc --noEmit` — no NEW `src/js/sheet/*` errors.

- [ ] **Step 4: Commit**

```bash
git add ui/src/js/sheet/sheetView.ts ui/src/js/sheet/sheetEditor.ts
git commit -m "feat(sheet): wire formula bar to active cell + render error cells"
```

---

## Task 5: E2E — formula bar, autocomplete, error display

**Files:**
- Create: `playwright/specs/sheet_formula_bar.spec.ts`

**Context:** Reuse the sheet bootstrap from `sheet_selection.spec.ts` (route `/s/${padId}`, `.sheet-grid` wait, cell locator). Formula-bar selectors: `.sheet-namebox`, `.sheet-fx-input`, dropdown `.sheet-fx-ac`. Do NOT use clipboard permissions (avoids the firefox context issue). If a browser-specific need arises, use a describe-level `test.skip(({browserName}) => ...)` (never an inner test.fixme). Validate with `--list`; the controller runs the live E2E.

- [ ] **Step 1: Write the E2E**

```typescript
import { test, expect } from '@playwright/test';

test.describe('Sheet formula bar', () => {
  const cell = (page, r: number, c: number) =>
    page.locator(`.sheet-grid tbody tr:nth-child(${r + 1}) td:nth-child(${c + 2})`);

  test('name box shows the active ref; committing from the bar sets the cell', async ({ page }) => {
    await page.goto('/s/fx-e2e-' + Date.now());
    await page.locator('.sheet-grid').waitFor();

    await cell(page, 2, 1).click(); // C? -> row2 col1 = B3
    await expect(page.locator('.sheet-namebox')).toHaveText('B3');

    const fx = page.locator('.sheet-fx-input');
    await fx.click();
    await fx.fill('=1+2');
    await fx.press('Enter');
    await expect(cell(page, 2, 1)).toHaveText('3');
  });

  test('an invalid formula renders as a styled error cell', async ({ page }) => {
    await page.goto('/s/fx-err-' + Date.now());
    await page.locator('.sheet-grid').waitFor();

    await cell(page, 0, 0).click();
    const fx = page.locator('.sheet-fx-input');
    await fx.click();
    await fx.fill('=1/0');
    await fx.press('Enter');

    await expect(cell(page, 0, 0)).toHaveText('#DIV/0!');
    await expect(cell(page, 0, 0)).toHaveClass(/sheet-cell-error/);
  });
});
```

- [ ] **Step 2: Validate (no server run)**

Run: `cd playwright && npx playwright test specs/sheet_formula_bar.spec.ts --list`
Expected: lists 2 tests, no parse/TS errors. (If node_modules missing: `pnpm install --frozen-lockfile`.)

- [ ] **Step 3: Commit**

```bash
git add playwright/specs/sheet_formula_bar.spec.ts
git commit -m "test(sheet): E2E for formula bar + error rendering"
```

---

## Self-Review notes (addressed)

- **Spec coverage (M3):** Name Box + editable bar synced to active cell (T3, T4); commit via setCell path (T4); autocomplete from HyperFormula function names (T1 engine, T2 pure filter, T3 dropdown); error display with distinct style + hover (T4). All covered.
- **No Go / no new op types:** formula-bar commit reuses `setCell`; confirmed.
- **Type consistency:** `functionNames`, `cellRefA1`/`rangeRefA1`/`colName`, `functionPrefix`/`filterFunctions`, `createFormulaBar`/`FormulaBarHandle.setActive`, `errorOf` names used identically across tasks.
- **Reuse:** `colName` extracted to `a1.ts` and the view refactored to import it (no duplicate).
- **Deferred (noted):** live-broadcast of in-progress bar typing to peers (the spec mentions it; M3 commits on Enter and relies on M1's in-cell live-edit for cell typing — bar-driven live-edit is a follow-up); argument hints/signature tooltips; multi-cell Name Box navigation (type a ref to jump). Cross-sheet refs remain out of scope per the design spec.

// Pure sort + filter helpers. Sort is collaborative (a batch of setCell ops:
// data moves, the server only sees cell ops). Filter is client-local row
// hiding — no ops, not shared (collaborative filter is the upgrade path).

import type { Op } from './op';
import type { Selection } from './sheetSelection';
import { normalize } from './sheetSelection';
import { adjustFormula } from './sheetClipboard';

// compareVals: numbers numerically, everything else lexically; empty always
// sorts last regardless of direction (Excel behavior).
export function compareVals(a: string, b: string, asc: boolean): number {
  if (a === '' && b === '') return 0;
  if (a === '') return 1;
  if (b === '') return -1;
  const na = Number(a);
  const nb = Number(b);
  const base =
    !Number.isNaN(na) && !Number.isNaN(nb) ? na - nb : a.localeCompare(b, undefined, { numeric: true });
  return asc ? base : -base;
}

// sortRangeOps reorders the rows of the selected range by the given column and
// emits one setCell per cell of the range (moved formulas shift row refs like
// fill does). byCol must lie inside the selection.
export function sortRangeOps(
  sel: Selection,
  byCol: number,
  asc: boolean,
  sheet: string,
  baseRev: number,
  rawAt: (r: number, c: number) => string,
): Op[] {
  const { r0, c0, r1, c1 } = normalize(sel);
  const keyIdx = Math.min(Math.max(byCol, c0), c1) - c0;
  const rows: string[][] = [];
  for (let r = r0; r <= r1; r++) {
    const cells: string[] = [];
    for (let c = c0; c <= c1; c++) cells.push(rawAt(r, c));
    rows.push(cells);
  }
  const order = rows
    .map((cells, i) => ({ cells, srcRow: r0 + i }))
    .sort((a, b) => compareVals(a.cells[keyIdx], b.cells[keyIdx], asc)); // stable

  const ops: Op[] = [];
  for (let i = 0; i < order.length; i++) {
    const destRow = r0 + i;
    const { cells, srcRow } = order[i];
    if (srcRow === destRow) continue; // row did not move
    for (let c = c0; c <= c1; c++) {
      ops.push({
        type: 'setCell', sheet, baseRev, row: destRow, col: c,
        raw: adjustFormula(cells[c - c0], destRow - srcRow, 0),
      });
    }
  }
  return ops;
}

// distinctValues lists the non-empty values present in a column (sorted),
// for the filter dropdown.
export function distinctValues(
  col: number,
  rowCount: number,
  rawAt: (r: number, c: number) => string,
): string[] {
  const seen = new Set<string>();
  for (let r = 0; r < rowCount; r++) {
    const v = rawAt(r, col);
    if (v !== '') seen.add(v);
  }
  return [...seen].sort((a, b) => compareVals(a, b, true));
}

// hiddenRowsForFilter: rows whose column value is non-empty and differs from
// `keep`. Blank rows stay visible (the empty grid must not collapse).
export function hiddenRowsForFilter(
  col: number,
  keep: string,
  rowCount: number,
  rawAt: (r: number, c: number) => string,
): Set<number> {
  const hidden = new Set<number>();
  for (let r = 0; r < rowCount; r++) {
    const v = rawAt(r, col);
    if (v !== '' && v !== keep) hidden.add(r);
  }
  return hidden;
}

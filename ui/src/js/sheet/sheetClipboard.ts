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

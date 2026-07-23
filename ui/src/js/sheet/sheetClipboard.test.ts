import { describe, it, expect } from 'vitest';
import { rangeToTSV, rangeToCSV, parseTSV, pasteOps, fillOps, adjustFormula } from './sheetClipboard';

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

describe('CSV export', () => {
  it('rangeToCSV joins cols with comma, rows with CRLF', () => {
    const sel = { anchor: { row: 0, col: 0 }, focus: { row: 1, col: 1 } };
    const g = raw({ '0:0': 'a', '0:1': 'b', '1:0': 'c', '1:1': 'd' });
    expect(rangeToCSV(sel, g)).toBe('a,b\r\nc,d');
  });

  it('quotes fields containing comma, quote, CR or LF and doubles quotes', () => {
    const sel = { anchor: { row: 0, col: 0 }, focus: { row: 0, col: 3 } };
    const g = raw({ '0:0': 'a,b', '0:1': 'say "hi"', '0:2': 'line1\nline2', '0:3': 'plain' });
    expect(rangeToCSV(sel, g)).toBe('"a,b","say ""hi""","line1\nline2",plain');
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
  it('does not corrupt function names ending in digits', () => {
    expect(adjustFormula('=LOG10(A1)', 1, 0)).toBe('=LOG10(A2)');
    expect(adjustFormula('=ATAN2(A1,B1)', 0, 1)).toBe('=ATAN2(B1,C1)');
  });
  it('shifts lowercase refs, normalizing column letters to uppercase', () => {
    expect(adjustFormula('=a1', 1, 0)).toBe('=A2');
    expect(adjustFormula('=sum(a1:a3)', 2, 0)).toBe('=sum(A3:A5)');
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

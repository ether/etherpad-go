import { describe, it, expect } from 'vitest';
import { sortRangeOps, distinctValues, hiddenRowsForFilter, compareVals } from './sheetSortFilter';
import type { Selection } from './sheetSelection';

const sel = (r0: number, c0: number, r1: number, c1: number): Selection => ({
  anchor: { row: r0, col: c0 },
  focus: { row: r1, col: c1 },
});

const gridRaw =
  (grid: string[][]) =>
  (r: number, c: number): string =>
    grid[r]?.[c] ?? '';

describe('compareVals', () => {
  it('numbers numerically, strings lexically, empty last', () => {
    expect(compareVals('2', '10', true)).toBeLessThan(0);
    expect(compareVals('b', 'a', true)).toBeGreaterThan(0);
    expect(compareVals('', 'a', true)).toBeGreaterThan(0); // empty last even asc
    expect(compareVals('', 'a', false)).toBeGreaterThan(0); // and desc
  });
});

describe('sortRangeOps', () => {
  it('sorts rows by the key column and moves whole rows', () => {
    const grid = [
      ['3', 'c'],
      ['1', 'a'],
      ['2', 'b'],
    ];
    const ops = sortRangeOps(sel(0, 0, 2, 1), 0, true, 's1', 0, gridRaw(grid));
    // row1 (1,a) -> row0; row2 (2,b) -> row1; row0 (3,c) -> row2
    const at = (r: number, c: number) => ops.find((o) => o.row === r && o.col === c)?.raw;
    expect(at(0, 0)).toBe('1');
    expect(at(0, 1)).toBe('a');
    expect(at(2, 0)).toBe('3');
    expect(at(2, 1)).toBe('c');
  });

  it('adjusts moved formula row refs like fill does', () => {
    const grid = [
      ['2', '=A1*2'],
      ['1', '=A2*2'],
    ];
    const ops = sortRangeOps(sel(0, 0, 1, 1), 0, true, 's1', 0, gridRaw(grid));
    const at = (r: number, c: number) => ops.find((o) => o.row === r && o.col === c)?.raw;
    expect(at(0, 1)).toBe('=A1*2'); // was =A2*2 on row 1, moved up one row
    expect(at(1, 1)).toBe('=A2*2');
  });

  it('emits nothing when already sorted', () => {
    const grid = [
      ['1', 'a'],
      ['2', 'b'],
    ];
    expect(sortRangeOps(sel(0, 0, 1, 1), 0, true, 's1', 0, gridRaw(grid))).toEqual([]);
  });
});

describe('filter helpers', () => {
  const grid = [['x'], ['y'], ['x'], [''], ['z']];
  it('distinctValues sorted + deduped, blanks excluded', () => {
    expect(distinctValues(0, 5, gridRaw(grid))).toEqual(['x', 'y', 'z']);
  });
  it('hiddenRowsForFilter hides non-matching, keeps blanks visible', () => {
    const hidden = hiddenRowsForFilter(0, 'x', 5, gridRaw(grid));
    expect([...hidden].sort()).toEqual([1, 4]);
  });
});

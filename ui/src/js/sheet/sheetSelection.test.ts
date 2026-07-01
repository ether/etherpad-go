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

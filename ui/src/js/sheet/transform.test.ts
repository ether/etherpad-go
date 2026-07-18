import { describe, it, expect } from 'vitest';
import { transform } from './transform';
import type { Op } from './op';

const setCell = (row: number, col: number): Op => ({ type: 'setCell', sheet: 's1', baseRev: 0, row, col, raw: 'x' });

describe('transform (port of Go Transform)', () => {
  it('cell below insertRows shifts down; above stays', () => {
    const applied: Op = { type: 'insertRows', sheet: 's1', baseRev: 0, index: 2, count: 3 };
    expect(transform(setCell(4, 0), applied).row).toBe(7);
    expect(transform(setCell(1, 0), applied).row).toBe(1);
  });

  it('cell below deleteRows shifts up; inside band clamps', () => {
    const applied: Op = { type: 'deleteRows', sheet: 's1', baseRev: 0, index: 2, count: 2 };
    expect(transform(setCell(5, 0), applied).row).toBe(3);
    expect(transform(setCell(3, 0), applied).row).toBe(2);
  });

  it('different sheet is a no-op', () => {
    const applied: Op = { type: 'insertRows', sheet: 'other', baseRev: 0, index: 0, count: 5 };
    const out = transform(setCell(1, 1), applied);
    expect(out.row).toBe(1);
    expect(out.col).toBe(1);
  });

  it('insert index shifts against a prior insert', () => {
    const applied: Op = { type: 'insertRows', sheet: 's1', baseRev: 0, index: 2, count: 2 };
    const inOp: Op = { type: 'insertRows', sheet: 's1', baseRev: 0, index: 4, count: 1 };
    expect(transform(inOp, applied).index).toBe(6);
  });

  it('cell right of insertCols shifts right', () => {
    const applied: Op = { type: 'insertCols', sheet: 's1', baseRev: 0, index: 1, count: 2 };
    expect(transform(setCell(0, 3), applied).col).toBe(5);
  });

  it('mergeCells rectangle shifts like clearRange', () => {
    const merge: Op = { type: 'mergeCells', sheet: 's1', baseRev: 0, row: 2, col: 1, endRow: 4, endCol: 2 };
    const out = transform(merge, { type: 'insertRows', sheet: 's1', baseRev: 0, index: 3, count: 2 });
    expect(out.row).toBe(2);
    expect(out.endRow).toBe(6);
    // delete of the whole range collapses to a degenerate rectangle (applyOp no-ops it)
    const gone = transform(merge, { type: 'deleteRows', sheet: 's1', baseRev: 0, index: 2, count: 3 });
    expect(gone.row).toBe(2);
    expect(gone.endRow).toBe(2);
  });
});

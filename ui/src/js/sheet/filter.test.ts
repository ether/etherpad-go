import { describe, expect, it } from 'vitest';
import { WorkbookState } from './workbookState';
import { invertOp } from './undo';
import type { Op } from './op';

const op = (o: Partial<Op>): Op => ({ type: 'setFilter', sheet: 's1', baseRev: 0, ...o }) as Op;

describe('shared filters', () => {
  it('apply, shift with columns, undo (mirrors lib/sheet/filter_test.go)', () => {
    const wb = new WorkbookState();
    wb.loadSnapshot({ sheets: [{ id: 's1', name: 'Sheet1', cells: [] }] });
    const s = wb.sheetById('s1')!;
    wb.applyOp(op({ row: 2, col: 1, values: ['x'] }));
    wb.applyOp(op({ row: 2, col: 3, values: ['a', 'b'] }));
    wb.applyOp({ type: 'insertCols', sheet: 's1', baseRev: 0, index: 0, count: 1 });
    wb.applyOp({ type: 'insertRows', sheet: 's1', baseRev: 0, index: 0, count: 2 });
    expect(s.filterRow).toBe(4);
    expect([...s.filters]).toEqual([[2, ['x']], [4, ['a', 'b']]]);

    const del: Op = { type: 'deleteCols', sheet: 's1', baseRev: 0, index: 2, count: 1 };
    const inverse = invertOp(wb, del);
    wb.applyOp(del);
    expect([...s.filters]).toEqual([[3, ['a', 'b']]]);
    for (const inv of inverse) wb.applyOp(inv);
    expect(new Map(s.filters)).toEqual(new Map([[2, ['x']], [4, ['a', 'b']]]));

    const clear = op({ row: 0, col: 0, clear: true });
    const undoClear = invertOp(wb, clear);
    wb.applyOp(clear);
    expect(s.filters.size).toBe(0);
    for (const inv of undoClear) wb.applyOp(inv);
    expect(s.filters.size).toBe(2);
    expect(s.filterRow).toBe(4);
  });
});

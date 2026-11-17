import { describe, expect, it } from 'vitest';
import { WorkbookState } from './workbookState';
import { invertOp } from './undo';
import { transform } from './transform';
import { SheetCollabClient } from './sheetCollabClient';
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

describe('filter review fixes', () => {
  const fresh = () => {
    const wb = new WorkbookState();
    wb.loadSnapshot({ sheets: [{ id: 's1', name: 'Sheet1', cells: [] }] });
    return wb;
  };
  it('undo of one column filter leaves other columns alone', () => {
    const wb = fresh();
    const first = op({ row: 0, col: 1, values: ['x'] });
    const inverse = invertOp(wb, first);
    wb.applyOp(first);
    wb.applyOp(op({ row: 0, col: 2, values: ['y'] })); // someone else
    for (const inv of inverse) wb.applyOp(inv);
    expect([...wb.sheetById('s1')!.filters]).toEqual([[2, ['y']]]);
  });
  it('undo of a row delete puts the header back', () => {
    const wb = fresh();
    wb.applyOp(op({ row: 3, col: 0, values: ['x'] }));
    const del: Op = { type: 'deleteRows', sheet: 's1', baseRev: 0, index: 2, count: 3 };
    const inverse = invertOp(wb, del);
    wb.applyOp(del);
    for (const inv of inverse) wb.applyOp(inv);
    expect(wb.sheetById('s1')!.filterRow).toBe(3);
  });
  it('a filter on a concurrently deleted column becomes a no-op', () => {
    const got = transform(op({ col: 2, values: ['x'] }), { type: 'deleteCols', sheet: 's1', baseRev: 0, index: 2, count: 1 });
    expect(got.noop).toBe(true);
    const wb = fresh();
    wb.applyOp(op({ row: 0, col: 2, values: ['keep'] }));
    wb.applyOp(got);
    expect(wb.sheetById('s1')!.filters.get(2)).toEqual(['keep']);
  });
  it('a refused op is dropped and the next one is sent', () => {
    const sent: Op[] = [];
    const c = new SheetCollabClient({ sheets: [{ id: 's1', name: 'Sheet1', cells: [] }] }, 0, { send: (o) => sent.push(o) });
    c.applyLocal({ type: 'setCell', sheet: 's1', baseRev: 0, row: 0, col: 0, raw: 'bad' });
    c.applyLocal({ type: 'setCell', sheet: 's1', baseRev: 0, row: 1, col: 0, raw: 'good' });
    expect(sent).toHaveLength(1);
    c.onReject(sent[0].cid);
    expect(sent).toHaveLength(2);
    expect(sent[1].raw).toBe('good');
    expect(c.display.getCell('s1', 0, 0)).toBeUndefined();
  });
});

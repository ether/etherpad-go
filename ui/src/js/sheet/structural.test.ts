import { describe, it, expect } from 'vitest';
import { WorkbookState } from './workbookState';
import { transform } from './transform';
import type { Op } from './op';

const wb2 = (): WorkbookState => {
  const w = new WorkbookState();
  w.addSheet('s1', 'Sheet1');
  w.addSheet('s2', 'Sheet2');
  return w;
};
const ids = (w: WorkbookState) => w.sheets.map((s) => s.id);

describe('sheet-list ops (mirrors Go structural_test)', () => {
  it('add/rename/move/delete', () => {
    const w = wb2();
    w.applyOp({ type: 'addSheet', sheet: 's3', name: 'Drei', index: 1, baseRev: 0 });
    expect(ids(w)).toEqual(['s1', 's3', 's2']);
    w.applyOp({ type: 'addSheet', sheet: 's3', name: 'Nochmal', index: 0, baseRev: 0 });
    expect(w.sheets.length).toBe(3); // duplicate add is a no-op
    expect(w.sheetById('s3')?.name).toBe('Drei');
    w.applyOp({ type: 'renameSheet', sheet: 's3', name: 'Umbenannt', baseRev: 0 });
    expect(w.sheetById('s3')?.name).toBe('Umbenannt');
    w.applyOp({ type: 'moveSheet', sheet: 's3', toIndex: 2, baseRev: 0 });
    expect(ids(w)).toEqual(['s1', 's2', 's3']);
    w.applyOp({ type: 'deleteSheet', sheet: 's3', baseRev: 0 });
    expect(ids(w)).toEqual(['s1', 's2']);
  });

  it('never deletes the last sheet; ops on deleted sheets are no-ops', () => {
    const w = new WorkbookState();
    w.addSheet('s1', 'Sheet1');
    w.applyOp({ type: 'deleteSheet', sheet: 's1', baseRev: 0 });
    expect(w.sheets.length).toBe(1);
    expect(() =>
      w.applyOp({ type: 'setCell', sheet: 'gone', row: 0, col: 0, raw: 'x', baseRev: 0 }),
    ).not.toThrow();
  });
});

describe('dimensions + freeze', () => {
  it('setDimension/setFreeze store metadata', () => {
    const w = wb2();
    w.applyOp({ type: 'setDimension', sheet: 's1', axis: 'col', index: 2, size: 140, baseRev: 0 });
    w.applyOp({ type: 'setDimension', sheet: 's1', axis: 'row', index: 5, size: 40, baseRev: 0 });
    w.applyOp({ type: 'setFreeze', sheet: 's1', frozenRows: 1, frozenCols: 1, baseRev: 0 });
    const s = w.sheetById('s1')!;
    expect(s.colWidths.get(2)).toBe(140);
    expect(s.rowHeights.get(5)).toBe(40);
    expect(s.frozenRows).toBe(1);
    expect(s.frozenCols).toBe(1);
  });

  it('dims shift under structural ops; in-band deletes drop the override', () => {
    const w = wb2();
    w.applyOp({ type: 'setDimension', sheet: 's1', axis: 'col', index: 3, size: 120, baseRev: 0 });
    w.applyOp({ type: 'insertCols', sheet: 's1', index: 0, count: 2, baseRev: 0 });
    expect(w.sheetById('s1')!.colWidths.get(5)).toBe(120);
    w.applyOp({ type: 'deleteCols', sheet: 's1', index: 4, count: 2, baseRev: 0 });
    expect(w.sheetById('s1')!.colWidths.size).toBe(0);
  });

  it('snapshot round-trip loads dims/freeze', () => {
    const w = new WorkbookState();
    w.loadSnapshot({
      sheets: [
        { id: 's1', name: 'Sheet1', cells: [], colWidths: { '1': 99 }, rowHeights: { '2': 33 }, frozenRows: 1 },
      ],
    });
    const s = w.sheetById('s1')!;
    expect(s.colWidths.get(1)).toBe(99);
    expect(s.rowHeights.get(2)).toBe(33);
    expect(s.frozenRows).toBe(1);
  });
});

describe('transform for setDimension (mirrors Go)', () => {
  it('col dimension shifts under col insert; row axis untouched', () => {
    const inOp: Op = { type: 'setDimension', sheet: 's1', axis: 'col', index: 3, size: 100, baseRev: 0 };
    const applied: Op = { type: 'insertCols', sheet: 's1', index: 1, count: 2, baseRev: 0 };
    expect(transform(inOp, applied).index).toBe(5);
    expect(transform({ ...inOp, axis: 'row' }, applied).index).toBe(3);
  });
});

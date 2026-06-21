import { describe, it, expect, beforeEach } from 'vitest';
import { WorkbookState } from './workbookState';

let wb: WorkbookState;
beforeEach(() => {
  wb = new WorkbookState();
  wb.addSheet('s1', 'Sheet1');
});

describe('WorkbookState.applyOp (port of Go Apply)', () => {
  it('setCell stores raw', () => {
    wb.applyOp({ type: 'setCell', sheet: 's1', baseRev: 0, row: 1, col: 1, raw: '42' });
    expect(wb.getCell('s1', 1, 1)?.raw).toBe('42');
  });

  it('clearRange clears in-range, keeps outside', () => {
    wb.applyOp({ type: 'setCell', sheet: 's1', baseRev: 0, row: 0, col: 0, raw: 'a' });
    wb.applyOp({ type: 'setCell', sheet: 's1', baseRev: 0, row: 1, col: 1, raw: 'b' });
    wb.applyOp({ type: 'setCell', sheet: 's1', baseRev: 0, row: 5, col: 5, raw: 'keep' });
    wb.applyOp({ type: 'clearRange', sheet: 's1', baseRev: 0, row: 0, col: 0, endRow: 2, endCol: 2 });
    expect(wb.getCell('s1', 0, 0)).toBeUndefined();
    expect(wb.getCell('s1', 1, 1)).toBeUndefined();
    expect(wb.getCell('s1', 5, 5)?.raw).toBe('keep');
  });

  it('insertRows shifts cells down', () => {
    wb.applyOp({ type: 'setCell', sheet: 's1', baseRev: 0, row: 3, col: 0, raw: 'row3' });
    wb.applyOp({ type: 'insertRows', sheet: 's1', baseRev: 0, index: 2, count: 2 });
    expect(wb.getCell('s1', 3, 0)).toBeUndefined();
    expect(wb.getCell('s1', 5, 0)?.raw).toBe('row3');
  });

  it('deleteRows removes and shifts up', () => {
    wb.applyOp({ type: 'setCell', sheet: 's1', baseRev: 0, row: 2, col: 0, raw: 'del' });
    wb.applyOp({ type: 'setCell', sheet: 's1', baseRev: 0, row: 5, col: 0, raw: 'shift' });
    wb.applyOp({ type: 'deleteRows', sheet: 's1', baseRev: 0, index: 2, count: 2 });
    expect(wb.getCell('s1', 2, 0)).toBeUndefined();
    expect(wb.getCell('s1', 3, 0)?.raw).toBe('shift');
  });

  it('insertCols shifts cells right', () => {
    wb.applyOp({ type: 'setCell', sheet: 's1', baseRev: 0, row: 0, col: 3, raw: 'col3' });
    wb.applyOp({ type: 'insertCols', sheet: 's1', baseRev: 0, index: 2, count: 1 });
    expect(wb.getCell('s1', 0, 4)?.raw).toBe('col3');
  });

  it('setCell at row/col 0 works with omitted coords from wire', () => {
    // server may omit zero coords (Go omitempty); applyOp must default to 0.
    wb.applyOp({ type: 'setCell', sheet: 's1', baseRev: 0, raw: 'origin' });
    expect(wb.getCell('s1', 0, 0)?.raw).toBe('origin');
  });

  it('loadSnapshot restores cells', () => {
    const w2 = new WorkbookState();
    w2.loadSnapshot({ sheets: [{ id: 's1', name: 'Sheet1', cells: [{ row: 2, col: 3, raw: 'hi' }] }] });
    expect(w2.getCell('s1', 2, 3)?.raw).toBe('hi');
  });
});

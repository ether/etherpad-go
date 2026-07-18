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

describe('WorkbookState merges (port of Go Apply)', () => {
  it('mergeCells stores span; overlapping merge absorbs', () => {
    wb.applyOp({ type: 'mergeCells', sheet: 's1', baseRev: 0, row: 1, col: 1, endRow: 3, endCol: 2 });
    let s = wb.sheetById('s1')!;
    expect(s.merges.get('1:1')).toEqual({ rows: 3, cols: 2 });
    wb.applyOp({ type: 'mergeCells', sheet: 's1', baseRev: 0, row: 2, col: 2, endRow: 5, endCol: 5 });
    s = wb.sheetById('s1')!;
    expect(s.merges.size).toBe(1);
    expect(s.merges.get('2:2')).toEqual({ rows: 4, cols: 4 });
  });

  it('unmergeCells drops intersecting merges; 1x1 merge is a no-op', () => {
    wb.applyOp({ type: 'mergeCells', sheet: 's1', baseRev: 0, row: 1, col: 1, endRow: 3, endCol: 2 });
    wb.applyOp({ type: 'unmergeCells', sheet: 's1', baseRev: 0, row: 2, col: 2, endRow: 2, endCol: 2 });
    expect(wb.sheetById('s1')!.merges.size).toBe(0);
    wb.applyOp({ type: 'mergeCells', sheet: 's1', baseRev: 0, row: 0, col: 0, endRow: 0, endCol: 0 });
    expect(wb.sheetById('s1')!.merges.size).toBe(0);
  });

  it('insert at the trailing edge does not grow the merge', () => {
    // rows 2-4: inserting at row 5 (right after) must leave the span at 3
    wb.applyOp({ type: 'mergeCells', sheet: 's1', baseRev: 0, row: 2, col: 1, endRow: 4, endCol: 2 });
    wb.applyOp({ type: 'insertRows', sheet: 's1', baseRev: 0, index: 5, count: 2 });
    expect(wb.sheetById('s1')!.merges.get('2:1')).toEqual({ rows: 3, cols: 2 });
  });

  it('structural ops shift, grow, shrink and drop merges', () => {
    // rows 2-4, cols 1-2
    wb.applyOp({ type: 'mergeCells', sheet: 's1', baseRev: 0, row: 2, col: 1, endRow: 4, endCol: 2 });
    wb.applyOp({ type: 'insertRows', sheet: 's1', baseRev: 0, index: 3, count: 1 }); // inside: grows
    expect(wb.sheetById('s1')!.merges.get('2:1')).toEqual({ rows: 4, cols: 2 });
    wb.applyOp({ type: 'deleteRows', sheet: 's1', baseRev: 0, index: 0, count: 2 }); // above: moves up
    expect(wb.sheetById('s1')!.merges.get('0:1')).toEqual({ rows: 4, cols: 2 });
    wb.applyOp({ type: 'deleteCols', sheet: 's1', baseRev: 0, index: 1, count: 2 }); // all cols: dropped
    expect(wb.sheetById('s1')!.merges.size).toBe(0);
  });

  it('loadSnapshot restores merges', () => {
    const w2 = new WorkbookState();
    w2.loadSnapshot({
      sheets: [{ id: 's1', name: 'S', cells: [], merges: [{ row: 0, col: 0, rows: 2, cols: 3 }] }],
    });
    expect(w2.sheetById('s1')!.merges.get('0:0')).toEqual({ rows: 2, cols: 3 });
  });
});

describe('WorkbookState style props', () => {
  it('setStyle op interns props and getStyleProps resolves them', () => {
    const wb = new WorkbookState();
    wb.loadSnapshot({ sheets: [{ id: 's1', name: 'S', cells: [] }] });
    wb.applyOp({ type: 'setStyle', sheet: 's1', baseRev: 0, row: 0, col: 0, props: { bold: '1' } });
    expect(wb.getStyleProps('s1', 0, 0)).toEqual({ bold: '1' });
  });
  it('setCell op can carry raw and props together', () => {
    const wb = new WorkbookState();
    wb.loadSnapshot({ sheets: [{ id: 's1', name: 'S', cells: [] }] });
    wb.applyOp({ type: 'setCell', sheet: 's1', baseRev: 0, row: 1, col: 1, raw: '5', props: { align: 'right' } });
    expect(wb.getCell('s1', 1, 1)?.raw).toBe('5');
    expect(wb.getStyleProps('s1', 1, 1)).toEqual({ align: 'right' });
  });
});

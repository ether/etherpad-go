import { describe, it, expect, beforeEach } from 'vitest';
import { WorkbookState } from './workbookState';
import { invertOp } from './undo';
import { SheetCollabClient } from './sheetCollabClient';
import type { Op } from './op';

let wb: WorkbookState;
beforeEach(() => {
  wb = new WorkbookState();
  wb.addSheet('s1', 'Sheet1');
});

// A comparable fingerprint of everything applyOp can change, so "the inverse
// restored the state" is checked structurally instead of field by field.
const snapshot = (state: WorkbookState): string =>
  JSON.stringify(
    state.sheets.map((s) => ({
      id: s.id,
      name: s.name,
      cells: [...s.cells.entries()]
        .map(([k, c]) => [k, c.raw, state.styles.get(c.styleId ?? 0) ?? {}])
        .sort((a, b) => String(a[0]).localeCompare(String(b[0]))),
      colWidths: [...s.colWidths.entries()].sort((a, b) => a[0] - b[0]),
      rowHeights: [...s.rowHeights.entries()].sort((a, b) => a[0] - b[0]),
      frozen: [s.frozenRows, s.frozenCols],
      merges: [...s.merges.entries()].sort((a, b) => a[0].localeCompare(b[0])),
    })),
  );

// roundTrip applies `op`, then its inverse, and asserts the state is back.
const roundTrip = (op: Op): void => {
  const before = snapshot(wb);
  const inverse = invertOp(wb, op);
  wb.applyOp(op);
  for (const inv of inverse) wb.applyOp(inv);
  expect(snapshot(wb)).toBe(before);
};

const cell = (row: number, col: number, raw: string, props?: Record<string, string>): Op => ({
  type: 'setCell', sheet: 's1', baseRev: 0, row, col, raw, props,
});

describe('invertOp', () => {
  it('restores an overwritten cell, including its style', () => {
    wb.applyOp(cell(1, 1, 'old', { bold: '1' }));
    roundTrip(cell(1, 1, 'new', { italic: '1' }));
    expect(wb.getCell('s1', 1, 1)?.raw).toBe('old');
    expect(wb.getStyleProps('s1', 1, 1)).toEqual({ bold: '1' });
  });

  it('removes a cell that did not exist before', () => {
    roundTrip(cell(2, 2, 'typed'));
    expect(wb.getCell('s1', 2, 2)).toBeUndefined();
  });

  it('restores styling', () => {
    wb.applyOp(cell(0, 0, 'x', { bold: '1' }));
    roundTrip({ type: 'setStyle', sheet: 's1', baseRev: 0, row: 0, col: 0, props: { italic: '1', bg: '#ffcc00' } });
    expect(wb.getStyleProps('s1', 0, 0)).toEqual({ bold: '1' });
  });

  it('restores every cell a clearRange wiped', () => {
    wb.applyOp(cell(0, 0, 'a', { bold: '1' }));
    wb.applyOp(cell(1, 1, 'b'));
    wb.applyOp(cell(9, 9, 'outside'));
    roundTrip({ type: 'clearRange', sheet: 's1', baseRev: 0, row: 0, col: 0, endRow: 2, endCol: 2 });
    expect(wb.getCell('s1', 0, 0)?.raw).toBe('a');
    expect(wb.getStyleProps('s1', 0, 0)).toEqual({ bold: '1' });
    expect(wb.getCell('s1', 1, 1)?.raw).toBe('b');
  });

  it('undoes row and column inserts', () => {
    wb.applyOp(cell(3, 0, 'keep'));
    roundTrip({ type: 'insertRows', sheet: 's1', baseRev: 0, index: 1, count: 2 });
    expect(wb.getCell('s1', 3, 0)?.raw).toBe('keep');
    roundTrip({ type: 'insertCols', sheet: 's1', baseRev: 0, index: 0, count: 1 });
  });

  it('brings back deleted rows with their cells, sizes and merges', () => {
    wb.applyOp(cell(2, 0, 'gone', { bold: '1' }));
    wb.applyOp(cell(2, 1, 'gone2'));
    wb.applyOp(cell(5, 0, 'shifts'));
    wb.applyOp({ type: 'setDimension', sheet: 's1', baseRev: 0, axis: 'row', index: 2, size: 44 });
    wb.applyOp({ type: 'mergeCells', sheet: 's1', baseRev: 0, row: 2, col: 2, endRow: 3, endCol: 3 });
    roundTrip({ type: 'deleteRows', sheet: 's1', baseRev: 0, index: 2, count: 2 });
    expect(wb.getCell('s1', 2, 0)?.raw).toBe('gone');
    expect(wb.getStyleProps('s1', 2, 0)).toEqual({ bold: '1' });
    expect(wb.getCell('s1', 5, 0)?.raw).toBe('shifts');
    expect(wb.sheetById('s1')?.rowHeights.get(2)).toBe(44);
    expect(wb.sheetById('s1')?.merges.get('2:2')).toEqual({ rows: 2, cols: 2 });
  });

  it('brings back deleted columns', () => {
    wb.applyOp(cell(0, 1, 'gone'));
    wb.applyOp({ type: 'setDimension', sheet: 's1', baseRev: 0, axis: 'col', index: 1, size: 150 });
    roundTrip({ type: 'deleteCols', sheet: 's1', baseRev: 0, index: 1, count: 1 });
    expect(wb.getCell('s1', 0, 1)?.raw).toBe('gone');
    expect(wb.sheetById('s1')?.colWidths.get(1)).toBe(150);
  });

  it('undoes merge, including merges it absorbed, and unmerge', () => {
    wb.applyOp({ type: 'mergeCells', sheet: 's1', baseRev: 0, row: 0, col: 0, endRow: 1, endCol: 1 });
    roundTrip({ type: 'mergeCells', sheet: 's1', baseRev: 0, row: 0, col: 0, endRow: 3, endCol: 3 });
    expect(wb.sheetById('s1')?.merges.get('0:0')).toEqual({ rows: 2, cols: 2 });
    roundTrip({ type: 'unmergeCells', sheet: 's1', baseRev: 0, row: 0, col: 0, endRow: 3, endCol: 3 });
    expect(wb.sheetById('s1')?.merges.get('0:0')).toEqual({ rows: 2, cols: 2 });
  });

  it('undoes dimensions, freeze and sheet-list ops', () => {
    wb.applyOp({ type: 'setDimension', sheet: 's1', baseRev: 0, axis: 'col', index: 0, size: 120 });
    roundTrip({ type: 'setDimension', sheet: 's1', baseRev: 0, axis: 'col', index: 0, size: 300 });
    expect(wb.sheetById('s1')?.colWidths.get(0)).toBe(120);
    // A first-ever resize has no previous size to restore, so the inverse writes
    // the grid default (22px) instead of removing the entry — same rendering,
    // just not a byte-identical state, hence no roundTrip() here.
    const fresh: Op = { type: 'setDimension', sheet: 's1', baseRev: 0, axis: 'row', index: 4, size: 90 };
    const inverse = invertOp(wb, fresh);
    wb.applyOp(fresh);
    for (const inv of inverse) wb.applyOp(inv);
    expect(wb.sheetById('s1')?.rowHeights.get(4)).toBe(22);
    wb.sheetById('s1')?.rowHeights.delete(4);
    roundTrip({ type: 'setFreeze', sheet: 's1', baseRev: 0, frozenRows: 1, frozenCols: 1 });
    roundTrip({ type: 'renameSheet', sheet: 's1', baseRev: 0, name: 'Renamed' });
    roundTrip({ type: 'addSheet', sheet: 's2', baseRev: 0, name: 'Sheet2', index: 1 });
  });

  it('restores a deleted sheet with its contents', () => {
    wb.applyOp({ type: 'addSheet', sheet: 's2', baseRev: 0, name: 'Data', index: 1 });
    wb.applyOp({ type: 'setCell', sheet: 's2', baseRev: 0, row: 1, col: 1, raw: 'v', props: { bold: '1' } });
    wb.applyOp({ type: 'mergeCells', sheet: 's2', baseRev: 0, row: 0, col: 0, endRow: 0, endCol: 2 });
    roundTrip({ type: 'deleteSheet', sheet: 's2', baseRev: 0 });
    expect(wb.getCell('s2', 1, 1)?.raw).toBe('v');
    expect(wb.getStyleProps('s2', 1, 1)).toEqual({ bold: '1' });
  });

  it('undoing the refused deletion of the last sheet is a no-op', () => {
    roundTrip({ type: 'deleteSheet', sheet: 's1', baseRev: 0 });
    expect(wb.sheets).toHaveLength(1);
  });
});

describe('SheetCollabClient undo/redo', () => {
  const snap = { sheets: [{ id: 's1', name: 'Sheet1', cells: [] }] };
  const client = (): SheetCollabClient => new SheetCollabClient(snap, 0, { send: () => {} });
  const flushTick = (): Promise<void> => new Promise((r) => queueMicrotask(() => r()));

  it('undoes and redoes a single edit', async () => {
    const c = client();
    c.applyLocal({ type: 'setCell', sheet: 's1', baseRev: 0, row: 0, col: 0, raw: 'hello' });
    await flushTick();
    expect(c.canUndo()).toBe(true);
    c.undo();
    expect(c.display.getCell('s1', 0, 0)).toBeUndefined();
    expect(c.canRedo()).toBe(true);
    c.redo();
    expect(c.display.getCell('s1', 0, 0)?.raw).toBe('hello');
    // Redoing pushes onto the undo stack again, so the step stays reversible.
    c.undo();
    expect(c.display.getCell('s1', 0, 0)).toBeUndefined();
  });

  it('treats ops applied in one tick as one undo step', async () => {
    const c = client();
    for (let i = 0; i < 3; i++) c.applyLocal({ type: 'setCell', sheet: 's1', baseRev: 0, row: i, col: 0, raw: `v${i}` });
    await flushTick();
    c.undo();
    for (let i = 0; i < 3; i++) expect(c.display.getCell('s1', i, 0)).toBeUndefined();
    expect(c.canUndo()).toBe(false);
  });

  it('keeps separate ticks as separate steps', async () => {
    const c = client();
    c.applyLocal({ type: 'setCell', sheet: 's1', baseRev: 0, row: 0, col: 0, raw: 'first' });
    await flushTick();
    c.applyLocal({ type: 'setCell', sheet: 's1', baseRev: 0, row: 0, col: 0, raw: 'second' });
    await flushTick();
    c.undo();
    expect(c.display.getCell('s1', 0, 0)?.raw).toBe('first');
    c.undo();
    expect(c.display.getCell('s1', 0, 0)).toBeUndefined();
  });

  it('a new edit clears the redo branch', async () => {
    const c = client();
    c.applyLocal({ type: 'setCell', sheet: 's1', baseRev: 0, row: 0, col: 0, raw: 'a' });
    await flushTick();
    c.undo();
    expect(c.canRedo()).toBe(true);
    c.applyLocal({ type: 'setCell', sheet: 's1', baseRev: 0, row: 5, col: 5, raw: 'b' });
    await flushTick();
    expect(c.canRedo()).toBe(false);
  });

  it('rebases the history against remote structural ops', async () => {
    const c = client();
    c.applyLocal({ type: 'setCell', sheet: 's1', baseRev: 0, row: 3, col: 0, raw: 'mine' });
    await flushTick();
    c.onAccept(1);
    // Somebody else inserts two rows above: my cell now lives at row 5, and the
    // undo has to clear row 5, not row 3.
    c.onRemote({ type: 'insertRows', sheet: 's1', baseRev: 1, index: 0, count: 2 }, 2);
    expect(c.display.getCell('s1', 5, 0)?.raw).toBe('mine');
    c.undo();
    expect(c.display.getCell('s1', 5, 0)).toBeUndefined();
    expect(c.display.getCell('s1', 3, 0)).toBeUndefined();
  });

  it('does not record remote ops', () => {
    const c = client();
    c.onRemote({ type: 'setCell', sheet: 's1', baseRev: 0, row: 0, col: 0, raw: 'theirs' }, 1);
    expect(c.canUndo()).toBe(false);
  });
});

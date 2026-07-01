// @vitest-environment jsdom
import { describe, it, expect, vi } from 'vitest';
import { DomSheetView } from './sheetView';

function mkView(onSelectionChange = vi.fn()) {
  const root = document.createElement('div');
  document.body.appendChild(root);
  const view = new DomSheetView(root, {
    rows: 5, cols: 5,
    rawValue: () => '',
    displayValue: () => '',
    onEdit: () => {},
    onSelectionChange,
  });
  return { root, view };
}

describe('DomSheetView selection', () => {
  it('shift+click extends the selection from the anchor', () => {
    const onSel = vi.fn();
    const { root, view } = mkView(onSel);
    const cell = (r: number, c: number) =>
      root.querySelectorAll('tbody tr')[r].querySelectorAll('td')[c] as HTMLElement;

    cell(1, 1).dispatchEvent(new MouseEvent('mousedown', { bubbles: true }));
    cell(3, 3).dispatchEvent(new MouseEvent('mousedown', { bubbles: true, shiftKey: true }));

    const sel = view.getSelection();
    expect(sel.anchor).toEqual({ row: 1, col: 1 });
    expect(sel.focus).toEqual({ row: 3, col: 3 });
    // the last change fired with the extended range
    expect(onSel).toHaveBeenLastCalledWith(sel);
  });

  it('marks in-range cells with the selection class', () => {
    const { root, view } = mkView();
    const cell = (r: number, c: number) =>
      root.querySelectorAll('tbody tr')[r].querySelectorAll('td')[c] as HTMLElement;
    cell(0, 0).dispatchEvent(new MouseEvent('mousedown', { bubbles: true }));
    cell(1, 1).dispatchEvent(new MouseEvent('mousedown', { bubbles: true, shiftKey: true }));
    view.render();
    expect(cell(0, 0).classList.contains('sheet-sel')).toBe(true);
    expect(cell(1, 1).classList.contains('sheet-sel')).toBe(true);
    expect(cell(2, 2).classList.contains('sheet-sel')).toBe(false);
  });
});

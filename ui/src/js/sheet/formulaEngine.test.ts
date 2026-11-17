import { describe, it, expect } from 'vitest';
import { FormulaEngine } from './formulaEngine';

describe('FormulaEngine (HyperFormula wrapper)', () => {
  it('computes a SUM formula', () => {
    const e = new FormulaEngine();
    e.setCell(0, 0, '2'); // A1
    e.setCell(1, 0, '3'); // A2
    const r = e.setCell(0, 1, '=SUM(A1:A2)'); // B1
    expect(r.value).toBe('5');
    expect(r.type).toBe('number');
  });

  it('recomputes dependents when an input changes', () => {
    const e = new FormulaEngine();
    e.setCell(0, 0, '2');
    e.setCell(1, 0, '3');
    e.setCell(0, 1, '=SUM(A1:A2)');
    const change = e.setCell(0, 0, '10'); // A1 -> 10
    expect(e.getValue(0, 1).value).toBe('13'); // B1 recomputed
    // the dependent B1 (row0,col1) is reported as changed
    expect(change.changed.some((c) => c.row === 0 && c.col === 1)).toBe(true);
  });

  it('reports text and value types', () => {
    const e = new FormulaEngine();
    expect(e.setCell(0, 0, 'hello').type).toBe('text');
    expect(e.setCell(1, 0, '42').type).toBe('number');
  });

  it('functionNames lists common spreadsheet functions', () => {
    const names = new FormulaEngine().functionNames();
    expect(names).toContain('SUM');
    expect(names).toContain('IF');
    expect(names.length).toBeGreaterThan(50);
  });
});

describe('FormulaEngine workbook', () => {
  it('computes references to other sheets and follows the active sheet', () => {
    const e = new FormulaEngine();
    e.setWorkbook([
      { name: 'Sheet1', cells: [{ row: 0, col: 0, raw: '5' }] },
      { name: 'My Data', cells: [{ row: 0, col: 0, raw: '=Sheet1!A1*2' }, { row: 1, col: 0, raw: '#' }] },
      { name: 'Sheet3', cells: [{ row: 0, col: 0, raw: "='My Data'!A1+1" }] },
    ], 'Sheet3');
    expect(e.getValue(0, 0).value).toBe('11');
    e.setWorkbook([
      { name: 'Sheet1', cells: [{ row: 0, col: 0, raw: '7' }] },
      { name: 'My Data', cells: [{ row: 0, col: 0, raw: '=Sheet1!A1*2' }, { row: 1, col: 0, raw: '#' }] },
      { name: 'Sheet3', cells: [{ row: 0, col: 0, raw: "='My Data'!A1+1" }] },
    ], 'My Data');
    expect(e.getValue(0, 0).value).toBe('14');
  });

  it('shows #REF! for a broken reference', () => {
    const e = new FormulaEngine();
    e.setWorkbook([{ name: 'Sheet1', cells: [{ row: 0, col: 0, raw: '=#REF!+1' }] }], 'Sheet1');
    expect(e.getValue(0, 0).type).toBe('error');
    expect(e.getValue(0, 0).value).toContain('REF');
  });
});

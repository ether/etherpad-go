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
});

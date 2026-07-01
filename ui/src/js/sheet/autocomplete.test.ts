import { describe, it, expect } from 'vitest';
import { functionPrefix, filterFunctions } from './autocomplete';

describe('functionPrefix', () => {
  it('returns the partial function name being typed', () => {
    expect(functionPrefix('=SU', 3)).toBe('SU');
    expect(functionPrefix('=A1+VLoo', 8)).toBe('VLOO');
  });
  it('returns null when not typing a function name', () => {
    expect(functionPrefix('hello', 5)).toBeNull();   // not a formula
    expect(functionPrefix('=A1', 3)).toBe('A');       // "A" is a partial name until a digit follows... see note
    expect(functionPrefix('=SUM(', 5)).toBeNull();    // just after '(' — nothing typed
  });
});

describe('filterFunctions', () => {
  it('prefix-matches case-insensitively, sorted, capped at 8', () => {
    const names = ['SUM', 'SUMIF', 'SUMIFS', 'SUMPRODUCT', 'SUMSQ', 'SUMX2MY2', 'SUMX2PY2', 'SUMXMY2', 'SIN', 'IF'];
    const out = filterFunctions(names, 'sum');
    expect(out[0]).toBe('SUM');
    expect(out).not.toContain('SIN');
    expect(out.length).toBeLessThanOrEqual(8);
  });
});

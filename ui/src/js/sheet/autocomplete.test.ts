import { describe, it, expect } from 'vitest';
import { functionPrefix, filterFunctions } from './autocomplete';

describe('functionPrefix', () => {
  it('returns the partial function name being typed', () => {
    expect(functionPrefix('=SU', 3)).toBe('SU');
    expect(functionPrefix('=A1+VLoo', 8)).toBe('VLOO');
  });
  it('returns null when not typing a function name', () => {
    expect(functionPrefix('hello', 5)).toBeNull();       // not a formula
    expect(functionPrefix('=A1', 3)).toBeNull();         // caret after a digit: a ref was typed, no dropdown
    expect(functionPrefix('=A1', 2)).toBeNull();         // caret between 'A' and '1': digit follows -> ref
    expect(functionPrefix('=SUM(', 5)).toBeNull();       // just after '(' — nothing typed
    expect(functionPrefix('=SUM(A1', 7)).toBeNull();     // typing args: ref inside call
    expect(functionPrefix('=SUM(A1,2', 9)).toBeNull();   // typing numeric arg
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

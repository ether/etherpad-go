import { describe, it, expect } from 'vitest';
import { cellRefA1, rangeRefA1 } from './a1';

describe('A1 refs', () => {
  it('cellRefA1 is 1-based row, letter col', () => {
    expect(cellRefA1(0, 0)).toBe('A1');
    expect(cellRefA1(6, 1)).toBe('B7');
    expect(cellRefA1(0, 26)).toBe('AA1');
  });
  it('rangeRefA1 collapses a single cell and orders corners', () => {
    expect(rangeRefA1(0, 0, 0, 0)).toBe('A1');
    expect(rangeRefA1(0, 0, 4, 2)).toBe('A1:C5');
  });
});

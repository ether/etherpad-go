import { describe, it, expect } from 'vitest';
import { findAll, findNext, matches, replaceAll, replaceInRaw, type RawCell } from './findReplace';

const CELLS: RawCell[] = [
  { row: 0, col: 0, raw: 'Apple' },
  { row: 0, col: 1, raw: 'pineapple' },
  { row: 1, col: 0, raw: 'Banana' },
  { row: 2, col: 3, raw: '=SUM(A1:A2)' },
  { row: 3, col: 0, raw: 'apple pie apple' },
];

describe('matches', () => {
  it('is case-insensitive by default and substring-based', () => {
    expect(matches('Apple', 'apple')).toBe(true);
    expect(matches('pineapple', 'APPLE')).toBe(true);
    expect(matches('Apple', 'apple', { matchCase: true })).toBe(false);
  });

  it('honours whole-cell matching', () => {
    expect(matches('Apple', 'apple', { wholeCell: true })).toBe(true);
    expect(matches('pineapple', 'apple', { wholeCell: true })).toBe(false);
  });

  it('never matches an empty query', () => {
    expect(matches('anything', '')).toBe(false);
  });
});

describe('findAll / findNext', () => {
  it('returns matches in row-major order', () => {
    expect(findAll(CELLS, 'apple')).toEqual([
      { row: 0, col: 0 },
      { row: 0, col: 1 },
      { row: 3, col: 0 },
    ]);
  });

  it('searches formula text, like Excel looking in formulas', () => {
    expect(findAll(CELLS, 'sum')).toEqual([{ row: 2, col: 3 }]);
  });

  it('steps forward from the current cell and wraps around', () => {
    expect(findNext(CELLS, 'apple', { row: 0, col: 0 })).toEqual({ row: 0, col: 1 });
    expect(findNext(CELLS, 'apple', { row: 0, col: 1 })).toEqual({ row: 3, col: 0 });
    expect(findNext(CELLS, 'apple', { row: 3, col: 0 })).toEqual({ row: 0, col: 0 }); // wrap
    expect(findNext(CELLS, 'nothing', { row: 0, col: 0 })).toBeNull();
  });

  it('finds the only match even when standing on it', () => {
    expect(findNext(CELLS, 'banana', { row: 1, col: 0 })).toEqual({ row: 1, col: 0 });
  });
});

describe('replaceInRaw', () => {
  it('replaces every occurrence and keeps the surrounding casing', () => {
    expect(replaceInRaw('apple pie apple', 'apple', 'pear')).toBe('pear pie pear');
    expect(replaceInRaw('Apple and APPLE', 'apple', 'pear')).toBe('pear and pear');
    expect(replaceInRaw('Apple and APPLE', 'apple', 'pear', { matchCase: true })).toBe('Apple and APPLE');
  });

  it('swaps the whole content in whole-cell mode', () => {
    expect(replaceInRaw('Apple', 'apple', 'Pear', { wholeCell: true })).toBe('Pear');
    expect(replaceInRaw('pineapple', 'apple', 'Pear', { wholeCell: true })).toBe('pineapple');
  });

  it('leaves non-matching content untouched', () => {
    expect(replaceInRaw('Banana', 'apple', 'pear')).toBe('Banana');
  });

  it('does not loop forever when the replacement contains the query', () => {
    expect(replaceInRaw('a a', 'a', 'aa')).toBe('aa aa');
  });
});

describe('replaceAll', () => {
  it('returns only the cells that actually change', () => {
    expect(replaceAll(CELLS, 'apple', 'pear')).toEqual([
      { row: 0, col: 0, raw: 'pear' },
      { row: 0, col: 1, raw: 'pinepear' },
      { row: 3, col: 0, raw: 'pear pie pear' },
    ]);
  });

  it('rewrites formulas too', () => {
    expect(replaceAll(CELLS, 'A1:A2', 'B1:B2')).toEqual([{ row: 2, col: 3, raw: '=SUM(B1:B2)' }]);
  });

  it('returns nothing when the query matches nothing', () => {
    expect(replaceAll(CELLS, 'kiwi', 'x')).toEqual([]);
  });
});

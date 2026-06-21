import { describe, it, expect } from 'vitest';
import { serializeOp, isStructural, type Op } from './op';

describe('Op serialization parity with Go', () => {
  it('serializes a setCell op with only the set fields', () => {
    const op: Op = { type: 'setCell', sheet: 's1', baseRev: 0, row: 1, col: 2, raw: 'x' };
    const parsed = JSON.parse(serializeOp(op));
    expect(parsed).toEqual({ type: 'setCell', sheet: 's1', baseRev: 0, row: 1, col: 2, raw: 'x' });
    // no undefined fields leak into the wire payload
    expect(Object.values(parsed).every((v) => v !== undefined)).toBe(true);
    expect('value' in parsed).toBe(false);
    expect('styleId' in parsed).toBe(false);
  });

  it('serializes an insertRows op', () => {
    const op: Op = { type: 'insertRows', sheet: 's1', baseRev: 3, index: 5, count: 2 };
    expect(JSON.parse(serializeOp(op))).toEqual({
      type: 'insertRows', sheet: 's1', baseRev: 3, index: 5, count: 2,
    });
  });

  it('serializes a clearRange op', () => {
    const op: Op = { type: 'clearRange', sheet: 's1', baseRev: 0, row: 0, col: 0, endRow: 3, endCol: 3 };
    expect(JSON.parse(serializeOp(op))).toEqual({
      type: 'clearRange', sheet: 's1', baseRev: 0, row: 0, col: 0, endRow: 3, endCol: 3,
    });
  });

  it('classifies structural ops', () => {
    expect(isStructural({ type: 'insertRows', sheet: 's', baseRev: 0 })).toBe(true);
    expect(isStructural({ type: 'deleteCols', sheet: 's', baseRev: 0 })).toBe(true);
    expect(isStructural({ type: 'setCell', sheet: 's', baseRev: 0 })).toBe(false);
    expect(isStructural({ type: 'clearRange', sheet: 's', baseRev: 0 })).toBe(false);
  });
});

import { describe, it, expect } from 'vitest';
import { StylePoolMirror } from './stylePool';

describe('StylePoolMirror', () => {
  it('id 0 is the empty style', () => {
    const p = new StylePoolMirror();
    expect(p.get(0)).toEqual({});
  });
  it('put dedups identical props regardless of key order', () => {
    const p = new StylePoolMirror();
    const a = p.put({ bold: '1', color: '#f00' });
    const b = p.put({ color: '#f00', bold: '1' });
    expect(a).toBe(b);
    expect(a).toBeGreaterThan(0);
    expect(p.get(a)).toEqual({ bold: '1', color: '#f00' });
  });
  it('different props get different ids; empty props is id 0', () => {
    const p = new StylePoolMirror();
    expect(p.put({})).toBe(0);
    expect(p.put({ bold: '1' })).not.toBe(p.put({ italic: '1' }));
  });
  it('seed restores id->props and continues nextId', () => {
    const p = new StylePoolMirror();
    p.seed({ idToStyle: { '1': { props: { bold: '1' } } }, nextId: 2 });
    expect(p.get(1)).toEqual({ bold: '1' });
    expect(p.put({ bold: '1' })).toBe(1);          // dedups to seeded id
    expect(p.put({ italic: '1' })).toBe(2);        // continues from nextId
  });
});

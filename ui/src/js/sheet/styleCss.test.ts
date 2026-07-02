// ui/src/js/sheet/styleCss.test.ts
import { describe, it, expect } from 'vitest';
import { styleToCss, mergeProps, toggleProp } from './styleCss';

describe('styleToCss', () => {
  it('maps known props to CSS', () => {
    expect(styleToCss({ bold: '1', italic: '1', underline: '1', color: '#c00', bg: '#eee', align: 'right' }))
      .toEqual({ fontWeight: 'bold', fontStyle: 'italic', textDecoration: 'underline', color: '#c00', background: '#eee', textAlign: 'right' });
  });
  it('border all -> a border css; empty props -> empty css', () => {
    expect(styleToCss({ border: 'all' }).border).toBe('1px solid #333');
    expect(styleToCss({})).toEqual({});
  });
  it('ignores non-allowlisted values (defense against CSS injection)', () => {
    expect(styleToCss({ bg: 'url(https://evil.example/x)' })).toEqual({});
    expect(styleToCss({ color: 'red' })).toEqual({});
    expect(styleToCss({ align: 'justify; background: url(https://x)' })).toEqual({});
  });
});

describe('mergeProps / toggleProp', () => {
  it('merge overlays and empty-string removes', () => {
    expect(mergeProps({ bold: '1', color: '#c00' }, { italic: '1' })).toEqual({ bold: '1', color: '#c00', italic: '1' });
    expect(mergeProps({ bold: '1', color: '#c00' }, { color: '' })).toEqual({ bold: '1' });
  });
  it('toggleProp adds/removes', () => {
    expect(toggleProp({ color: '#c00' }, 'bold', true)).toEqual({ color: '#c00', bold: '1' });
    expect(toggleProp({ color: '#c00', bold: '1' }, 'bold', false)).toEqual({ color: '#c00' });
  });
});

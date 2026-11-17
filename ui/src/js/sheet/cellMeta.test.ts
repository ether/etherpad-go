import { describe, expect, it } from 'vitest';
import { byteLength, condStyle, inList, listValues, normalizeLink, ruleMatches, validLink, type CondRule } from './cellMeta';

describe('conditional formatting rules', () => {
  const red: CondRule = { op: 'gt', v1: '10', bg: '#ffc7ce', color: '#9c0006' };
  it('compares numbers as numbers', () => {
    expect(ruleMatches(red, '11')).toBe(true);
    expect(ruleMatches(red, '9')).toBe(false);
    expect(ruleMatches(red, '100')).toBe(true); // not "100" < "10" as text
  });
  it('treats an empty cell as 0 against a number, like Excel', () => {
    expect(ruleMatches({ op: 'lt', v1: '5' }, '')).toBe(true);
  });
  it('compares text without case and puts it after numbers', () => {
    expect(ruleMatches({ op: 'eq', v1: 'Ja' }, 'ja')).toBe(true);
    expect(ruleMatches({ op: 'gt', v1: '1000' }, 'abc')).toBe(true);
  });
  it('between accepts both orders', () => {
    expect(ruleMatches({ op: 'between', v1: '5', v2: '1' }, '3')).toBe(true);
    expect(ruleMatches({ op: 'notBetween', v1: '1', v2: '5' }, '3')).toBe(false);
  });
  it('text rules', () => {
    expect(ruleMatches({ op: 'contains', v1: 'ERR' }, 'an error')).toBe(true);
    expect(ruleMatches({ op: 'beginsWith', v1: 'an' }, 'An error')).toBe(true);
    expect(ruleMatches({ op: 'blank' }, ' ')).toBe(true);
  });
  it('the first matching rule wins a property', () => {
    const s = condStyle([red, { op: 'gt', v1: '5', bg: '#c6efce', bold: '1' }], '20');
    expect(s).toEqual({ bg: '#ffc7ce', color: '#9c0006', bold: true });
  });
});

describe('list validation', () => {
  it('splits and matches without case', () => {
    const l = listValues(' rot, grün ,,blau');
    expect(l).toEqual(['rot', 'grün', 'blau']);
    expect(inList(l, 'GRÜN')).toBe(true);
    expect(inList(l, 'gelb')).toBe(false);
    expect(inList(l, '')).toBe(true);
  });
});

describe('links', () => {
  it('accepts web and mail addresses only', () => {
    expect(validLink('https://etherpad.org')).toBe(true);
    expect(validLink('mailto:a@b.de')).toBe(true);
    expect(validLink('javascript:alert(1)')).toBe(false);
    expect(validLink('https://a b')).toBe(false);
  });
  it('completes what users type', () => {
    expect(normalizeLink('www.etherpad.org')).toBe('https://www.etherpad.org');
    expect(normalizeLink('a@b.de')).toBe('mailto:a@b.de');
    expect(normalizeLink('http://x.de')).toBe('http://x.de');
    expect(normalizeLink('etherpad.org:8080/p')).toBe('https://etherpad.org:8080/p');
    expect(normalizeLink('localhost:9001')).toBe('https://localhost:9001');
    expect(validLink(normalizeLink('localhost:9001'))).toBe(true);
    expect(validLink(normalizeLink('javascript:alert(1)'))).toBe(false);
  });
});

describe('limits count bytes like the server', () => {
  it('counts UTF-8 bytes', () => {
    expect(byteLength('ä')).toBe(2);
    expect(byteLength('€')).toBe(3);
  });
});

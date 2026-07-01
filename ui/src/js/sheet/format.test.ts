import { describe, it, expect } from 'vitest';
import { formatValue } from './format';

describe('formatValue', () => {
  it('general / text / undefined return the value unchanged', () => {
    expect(formatValue('1234.5', 'number', 'general')).toBe('1234.5');
    expect(formatValue('1234.5', 'number', undefined)).toBe('1234.5');
    expect(formatValue('=A1', 'text', 'text')).toBe('=A1');
  });
  it('number applies grouping and optional decimals', () => {
    expect(formatValue('1234.5', 'number', 'number')).toBe('1,234.5');
    expect(formatValue('1234.5', 'number', 'number:2')).toBe('1,234.50');
  });
  it('currency and percent', () => {
    expect(formatValue('1234.5', 'number', 'currency:2')).toBe('$1,234.50');
    expect(formatValue('0.125', 'number', 'percent:1')).toBe('12.5%');
  });
  it('non-numeric value under a numeric format is returned unchanged', () => {
    expect(formatValue('hello', 'text', 'number')).toBe('hello');
    expect(formatValue('', 'empty', 'currency')).toBe('');
  });
  it('date formats ISO strings and spreadsheet serials, passes garbage through', () => {
    expect(formatValue('2023-03-15', 'text', 'date')).toBe('3/15/2023');
    // Excel serial 44000 -> 2020-06-18 (epoch 1899-12-30). Assert it is a valid en-US date string, not the raw serial.
    expect(formatValue('44000', 'number', 'date')).toMatch(/^\d{1,2}\/\d{1,2}\/\d{4}$/);
    expect(formatValue('not-a-date', 'text', 'date')).toBe('not-a-date');
  });
  it('malformed numFmt decimals never throws (returns a formatted or unchanged value)', () => {
    expect(() => formatValue('1234.5', 'number', 'number:abc')).not.toThrow();
    // With NaN decimals treated as "no explicit fraction digits", grouping still applies:
    expect(formatValue('1234.5', 'number', 'number:abc')).toBe('1,234.5');
  });
});

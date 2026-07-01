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
});

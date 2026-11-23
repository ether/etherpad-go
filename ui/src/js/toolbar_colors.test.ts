import {describe, expect, it} from 'vitest';
import {toolbarColorForTokens} from './toolbar_colors';

// Must match lib/skin/colors.go and the theme CSS.
describe('toolbarColorForTokens', () => {
  it('colibris colours', () => {
    expect(toolbarColorForTokens(['super-light-toolbar'], 'colibris')).toBe('#ffffff');
    expect(toolbarColorForTokens(['super-dark-toolbar'], 'colibris')).toBe('#485365');
    expect(toolbarColorForTokens([], 'colibris')).toBe('#ffffff');
  });

  it('etherpad-go theme colours', () => {
    expect(toolbarColorForTokens(['super-light-toolbar', 'light-background'], 'etherpad-go')).toBe('#ffffff');
    expect(toolbarColorForTokens(['light-toolbar'], 'etherpad-go')).toBe('#f7f9fb');
    expect(toolbarColorForTokens(['super-dark-toolbar', 'dark-background'], 'etherpad-go')).toBe('#111827');
    expect(toolbarColorForTokens(['dark-toolbar'], 'etherpad-go')).toBe('#1f2937');
  });

  it('the rule defined last wins', () => {
    expect(toolbarColorForTokens(['dark-toolbar', 'super-light-toolbar'], 'etherpad-go')).toBe('#1f2937');
  });
});

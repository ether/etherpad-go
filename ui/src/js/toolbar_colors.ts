// Kept apart from skin_variants.ts, which touches the page on import.

// Toolbar background colors that the colibris skin variants resolve to.
// Mirrors --bg-color in assets/css/skin/colibris/pad-variants.css. Order
// matters: when skinVariants contains multiple *-toolbar tokens the CSS
// cascade picks the rule defined last, so iterate in source order and let
// the last matching token win. Upstream #7606 / #7690.
const TOOLBAR_COLORS_IN_CSS_ORDER: Array<[string, string]> = [
  ['super-light-toolbar', '#ffffff'],
  ['light-toolbar', '#f2f3f4'],
  ['super-dark-toolbar', '#485365'],
  ['dark-toolbar', '#576273'],
];
const COLIBRIS_DEFAULT_TOOLBAR_COLOR = '#ffffff';

// The same for the etherpad-go theme (assets/css/theme/etherpad-go.css,
// lib/skin/colors.go), used while <html> has the class theme-etherpad-go.
const ETHERPAD_GO_TOOLBAR_COLORS: Record<string, string> = {
  'super-light-toolbar': '#ffffff',
  'light-toolbar': '#f7f9fb',
  'super-dark-toolbar': '#111827',
  'dark-toolbar': '#1f2937',
};

export const toolbarColorForTokens = (tokens: string[], theme?: string): string => {
  const set = new Set(tokens);
  const goTheme = (theme ?? (typeof document !== 'undefined' && document.documentElement.classList.contains('theme-etherpad-go') ? 'etherpad-go' : '')) === 'etherpad-go';
  let color = COLIBRIS_DEFAULT_TOOLBAR_COLOR;
  for (const [variant, c] of TOOLBAR_COLORS_IN_CSS_ORDER) {
    if (set.has(variant)) color = goTheme ? ETHERPAD_GO_TOOLBAR_COLORS[variant] : c;
  }
  return color;
};

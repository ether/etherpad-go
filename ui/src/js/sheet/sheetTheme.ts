// Theme tokens of the spreadsheet page.
//
// All sheet CSS reads --sheet-* custom properties whose fallbacks are the
// classic Excel-like look (green accent), used with padTheme.name =
// "colibris". With the etherpad-go theme (<html class="theme-etherpad-go">)
// the page takes the look of the pad and admin UI: grey chrome and the pad
// accent colour (--primary-color, replaced by padTheme.accentColor), and
// with enableDarkMode (class sheet-dark-allowed) it follows the system's
// dark mode. Colours that are part of the data (cell formats, conditional
// formatting, charts) are not themed.
export const THEME_CSS = `
html.theme-etherpad-go {
  --primary-color: #4a6cf7;
  --sheet-accent: var(--primary-color);
  --sheet-on-accent: var(--primary-text-color, #ffffff);
  --sheet-accent-hover: color-mix(in srgb, var(--sheet-accent) 88%, black);
  --sheet-accent-border: color-mix(in srgb, var(--sheet-accent) 40%, var(--sheet-bg));
  --sheet-danger: #b91c1c;
  --sheet-link: #1d4ed8;
  --sheet-text-disabled: #9ca3af;
  --sheet-bg: #ffffff;
  --sheet-chrome: #f7f9fb;
  --sheet-grid: #e5e7eb;
  --sheet-border: #e5e7eb;
  --sheet-control-border: #d1d5db;
  --sheet-text: #1a2230;
  --sheet-text-soft: #6b7280;
  --sheet-accent-text: color-mix(in srgb, var(--sheet-accent) 75%, var(--sheet-text));
  --sheet-accent-soft: color-mix(in srgb, var(--sheet-accent) 10%, var(--sheet-bg));
  --sheet-accent-soft-strong: color-mix(in srgb, var(--sheet-accent) 22%, var(--sheet-bg));
  --sheet-selection: color-mix(in srgb, var(--sheet-accent) 12%, transparent);
  color-scheme: light;
}
/* Classic look: the green fallbacks, unless padTheme.accentColor sets
   --primary-color; the tints are derived from the accent so they match. */
html.theme-colibris {
  --sheet-accent: var(--primary-color, #107c41);
  --sheet-on-accent: var(--primary-text-color, #ffffff);
  --sheet-accent-text: color-mix(in srgb, var(--sheet-accent) 85%, black);
  --sheet-accent-hover: color-mix(in srgb, var(--sheet-accent) 88%, black);
  --sheet-accent-soft: color-mix(in srgb, var(--sheet-accent) 10%, #ffffff);
  --sheet-accent-soft-strong: color-mix(in srgb, var(--sheet-accent) 20%, #ffffff);
  --sheet-accent-border: color-mix(in srgb, var(--sheet-accent) 40%, #ffffff);
  --sheet-selection: color-mix(in srgb, var(--sheet-accent) 10%, transparent);
}
@media (prefers-color-scheme: dark) {
  html.theme-etherpad-go.sheet-dark-allowed {
    --sheet-bg: #111827;
    --sheet-chrome: #1f2937;
    --sheet-grid: #374151;
    --sheet-border: #374151;
    --sheet-control-border: #4b5563;
    --sheet-text: #e5e7eb;
    --sheet-text-soft: #9ca3af;
    --sheet-accent-text: color-mix(in srgb, var(--sheet-accent) 55%, #ffffff);
    --sheet-danger: #f87171;
    --sheet-link: #93c5fd;
    --sheet-text-disabled: #6b7280;
    color-scheme: dark;
  }
}
html.theme-etherpad-go body {
  background: var(--sheet-bg);
  color: var(--sheet-text);
  font-family: system-ui, -apple-system, "Segoe UI", Roboto, "Helvetica Neue", sans-serif;
}
/* Native controls follow the theme. :where() keeps these at element
   specificity, so the sheet's own .class rules still win. */
:where(html.theme-etherpad-go) :is(select, input, textarea, button) {
  color: inherit;
}
:where(html.theme-etherpad-go) :is(select, textarea, button),
:where(html.theme-etherpad-go) input:not([type="checkbox"], [type="radio"], [type="color"]) {
  background-color: var(--sheet-bg);
  border-color: var(--sheet-control-border);
}
`;

/** Black or white text, whichever contrasts more with a #rgb / #rrggbb fill. */
export function readableTextOn(hex: string): string {
  const m = /^#([0-9a-f]{3}|[0-9a-f]{6})$/i.exec(hex.trim());
  if (!m) return '';
  const h = m[1].length === 3 ? m[1].replace(/./g, (x) => x + x) : m[1];
  const [r, g, b] = [0, 2, 4].map((i) => {
    const v = parseInt(h.slice(i, i + 2), 16) / 255;
    return v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4;
  });
  const lum = 0.2126 * r + 0.7152 * g + 0.0722 * b;
  return (lum + 0.05) / 0.05 >= 1.05 / (lum + 0.05) ? '#000000' : '#ffffff';
}

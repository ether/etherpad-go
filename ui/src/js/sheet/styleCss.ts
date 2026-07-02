// ui/src/js/sheet/styleCss.ts
// Pure mapping from style props (the wire vocabulary) to inline CSS, plus merge
// helpers used by the toolbar. Keeps the DOM view free of formatting policy.

export type CellCss = {
  fontWeight?: string; fontStyle?: string; textDecoration?: string;
  color?: string; background?: string; textAlign?: string; border?: string;
  fontFamily?: string; fontSize?: string; whiteSpace?: string;
};

// Defense in depth: props are validated server-side (lib/sheet/op.go), but a
// value like bg: "url(...)" must never reach td.style even if bad data slips in.
const HEX_COLOR = /^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$/;
const ALIGNS = new Set(['left', 'center', 'right']);
const FONT_FAMILIES = new Set(['Calibri', 'Arial', 'Times New Roman', 'Courier New', 'Georgia', 'Verdana']);
const FONT_SIZE = /^[1-9]\d?$/; // 6..96, range-checked below

export function styleToCss(props: Record<string, string>): CellCss {
  const css: CellCss = {};
  if (props.bold === '1') css.fontWeight = 'bold';
  if (props.italic === '1') css.fontStyle = 'italic';
  if (props.underline === '1') css.textDecoration = 'underline';
  if (props.color && HEX_COLOR.test(props.color)) css.color = props.color;
  if (props.bg && HEX_COLOR.test(props.bg)) css.background = props.bg;
  if (props.align && ALIGNS.has(props.align)) css.textAlign = props.align;
  if (props.border === 'all') css.border = '1px solid #333';
  if (props.fontFamily && FONT_FAMILIES.has(props.fontFamily)) css.fontFamily = props.fontFamily;
  if (props.fontSize && FONT_SIZE.test(props.fontSize)) {
    const n = Number(props.fontSize);
    if (n >= 6 && n <= 96) css.fontSize = `${n}pt`;
  }
  if (props.wrap === '1') css.whiteSpace = 'normal';
  return css;
}

export function mergeProps(base: Record<string, string>, change: Record<string, string>): Record<string, string> {
  const out: Record<string, string> = { ...base };
  for (const [k, v] of Object.entries(change)) {
    if (v === '') delete out[k];
    else out[k] = v;
  }
  return out;
}

export function toggleProp(props: Record<string, string>, key: string, on: boolean, value = '1'): Record<string, string> {
  return mergeProps(props, { [key]: on ? value : '' });
}

// ui/src/js/sheet/styleCss.ts
// Pure mapping from style props (the wire vocabulary) to inline CSS, plus merge
// helpers used by the toolbar. Keeps the DOM view free of formatting policy.

export type CellCss = {
  fontWeight?: string; fontStyle?: string; textDecoration?: string;
  color?: string; background?: string; textAlign?: string; border?: string;
};

export function styleToCss(props: Record<string, string>): CellCss {
  const css: CellCss = {};
  if (props.bold === '1') css.fontWeight = 'bold';
  if (props.italic === '1') css.fontStyle = 'italic';
  if (props.underline === '1') css.textDecoration = 'underline';
  if (props.color) css.color = props.color;
  if (props.bg) css.background = props.bg;
  if (props.align) css.textAlign = props.align;
  if (props.border === 'all') css.border = '1px solid #333';
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

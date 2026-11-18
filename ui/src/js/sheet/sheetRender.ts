// Renders a sheet of a workbook as a static HTML table: for printing and for
// the preview of an earlier version. The workbook gets its own formula
// engine, so neither touches what the editor shows.

import type { Chart } from './chart';
import { chartData, type ChartData } from './chartRender';
import { filterKeeps } from './cellFilter';
import { condStyle, parseCondRules, PROP_COND_FMT, PROP_LINK, validLink } from './cellMeta';
import { formatValue } from './format';
import { FormulaEngine } from './formulaEngine';
import { styleToCss } from './styleCss';
import type { SheetState, WorkbookState } from './workbookState';

export const DEFAULT_COL_WIDTH = 80; // lib/sheet/restore.go, undo.ts
export const DEFAULT_ROW_HEIGHT = 22;
// Printing more cells than this would hang the browser.
export const MAX_RENDER_CELLS = 200000;
// An imported sheet can be 16384 columns wide; more than this is cut off.
export const MAX_RENDER_COLS = 256;

export interface RenderRange {
  r0: number;
  c0: number;
  r1: number;
  c1: number;
}

export interface RenderOptions {
  // The cells to render; the used range of the sheet when absent.
  range?: RenderRange;
  gridlines?: boolean;
  // Draws a chart as SVG markup. Without it charts are left out.
  chartSvg?: (chart: Chart, data: ChartData | null) => string;
  // Shown above the table (the sheet name when printing a workbook).
  caption?: string;
}

const escapeHtml = (s: string): string =>
  s.replace(/[&<>"']/g, (ch) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[ch] as string);

const parseKey = (k: string): [number, number] => {
  const i = k.indexOf(':');
  return [Number(k.slice(0, i)), Number(k.slice(i + 1))];
};

// usedRange is the part of the sheet with cells, merges or chart anchors;
// null for an empty sheet.
export function usedRange(sheet: SheetState): RenderRange | null {
  let r1 = -1;
  let c1 = -1;
  for (const [k, cell] of sheet.cells) {
    if (cell.raw === '' && !cell.styleId) continue;
    const [r, c] = parseKey(k);
    r1 = Math.max(r1, r);
    c1 = Math.max(c1, c);
  }
  for (const [k, sp] of sheet.merges) {
    const [r, c] = parseKey(k);
    r1 = Math.max(r1, r + sp.rows - 1);
    c1 = Math.max(c1, c + sp.cols - 1);
  }
  for (const ch of sheet.charts) {
    r1 = Math.max(r1, ch.row);
    c1 = Math.max(c1, ch.col);
  }
  return r1 < 0 ? null : { r0: 0, c0: 0, r1, c1 };
}

// workbookEngine computes the formulas of every sheet of wb.
export function workbookEngine(wb: WorkbookState): FormulaEngine {
  const engine = new FormulaEngine();
  loadWorkbook(engine, wb, wb.sheets[0]?.name ?? '');
  return engine;
}

const loadWorkbook = (engine: FormulaEngine, wb: WorkbookState, active: string): void => {
  engine.setWorkbook(
    wb.sheets.map((sh) => ({
      name: sh.name,
      key: String(sh.stamp),
      cells: () => [...sh.cells].map(([k, cell]) => {
        const [row, col] = parseKey(k);
        return { row, col, raw: cell.raw };
      }),
    })),
    active,
  );
};

// renderSheetHtml renders one sheet as a <div> holding a table and its
// charts. All cell text is escaped.
export function renderSheetHtml(wb: WorkbookState, sheetId: string, engine: FormulaEngine, opts: RenderOptions = {}): string {
  const sheet = wb.sheetById(sheetId);
  if (!sheet) return '';
  loadWorkbook(engine, wb, sheet.name);
  const props = (r: number, c: number): Record<string, string> => wb.getStyleProps(sheetId, r, c);
  const display = (r: number, c: number): string => {
    const cell = sheet.cells.get(`${r}:${c}`);
    const v = engine.getValue(r, c);
    if (!cell || cell.raw === '') return v.type === 'empty' ? '' : formatValue(v.value, '', props(r, c).numFmt);
    return formatValue(cell.raw.startsWith('=') ? v.value : cell.raw, '', props(r, c).numFmt);
  };

  const caption = opts.caption ? `<div class="sheet-render-caption">${escapeHtml(opts.caption)}</div>` : '';
  const range = opts.range ?? usedRange(sheet);
  if (!range) return `<div class="sheet-render" data-sheet="${escapeHtml(sheetId)}">${caption}<p class="sheet-render-empty">This sheet is empty.</p></div>`;
  const { r0, c0 } = range;
  // Never more than MAX_RENDER_COLS columns and MAX_RENDER_CELLS cells:
  // the rest is cut off at the right and the bottom.
  const c1 = Math.min(range.c1, c0 + MAX_RENDER_COLS - 1);
  const width = c1 - c0 + 1;
  const r1 = Math.min(range.r1, r0 + Math.max(1, Math.floor(MAX_RENDER_CELLS / width)) - 1);

  // Rows the sheet's filters hide (as in the editor).
  const hidden = new Set<number>();
  for (const [c, values] of sheet.filters) {
    const hide = sheet.filterHide.has(c);
    for (let r = Math.max(sheet.filterRow + 1, r0); r <= r1; r++) {
      if (!hidden.has(r) && !filterKeeps(values, hide, display(r, c))) hidden.add(r);
    }
  }

  // Merges, clipped to the range: anchors get spans, covered cells none.
  const spanAt = new Map<string, { rows: number; cols: number }>();
  const covered = new Set<string>();
  for (const [k, sp] of sheet.merges) {
    const [mr, mc] = parseKey(k);
    const ar = Math.max(mr, r0);
    const ac = Math.max(mc, c0);
    const er = Math.min(mr + sp.rows - 1, r1);
    const ec = Math.min(mc + sp.cols - 1, c1);
    if (ar > er || ac > ec) continue;
    let rows = 0;
    for (let r = ar; r <= er; r++) if (!hidden.has(r)) rows++;
    let top = ar;
    while (top <= er && hidden.has(top)) top++;
    if (rows === 0) continue;
    spanAt.set(`${top}:${ac}`, { rows, cols: ec - ac + 1 });
    for (let r = ar; r <= er; r++) for (let c = ac; c <= ec; c++) if (r !== top || c !== ac) covered.add(`${r}:${c}`);
  }

  const colW = (c: number): number => sheet.colWidths.get(c) ?? DEFAULT_COL_WIDTH;
  const rowH = (r: number): number => sheet.rowHeights.get(r) ?? DEFAULT_ROW_HEIGHT;
  let tableW = 0;
  const cols: string[] = [];
  for (let c = c0; c <= c1; c++) {
    tableW += colW(c);
    cols.push(`<col style="width:${colW(c)}px">`);
  }

  const rows: string[] = [];
  let tableH = 0;
  for (let r = r0; r <= r1; r++) {
    if (hidden.has(r)) continue;
    tableH += rowH(r);
    const tds: string[] = [];
    for (let c = c0; c <= c1; c++) {
      const k = `${r}:${c}`;
      if (covered.has(k)) continue;
      const p = props(r, c);
      const css = styleToCss(p);
      const style: string[] = [];
      if (css.fontWeight) style.push(`font-weight:${css.fontWeight}`);
      if (css.fontStyle) style.push(`font-style:${css.fontStyle}`);
      if (css.textDecoration) style.push(`text-decoration:${css.textDecoration}`);
      if (css.color) style.push(`color:${css.color}`);
      if (css.background) style.push(`background:${css.background}`);
      if (css.border) style.push(`border:${css.border}`);
      if (css.fontFamily) style.push(`font-family:'${css.fontFamily}'`);
      if (css.fontSize) style.push(`font-size:${css.fontSize}`);
      if (css.whiteSpace) style.push(`white-space:${css.whiteSpace}`);
      if (css.verticalAlign) style.push(`vertical-align:${css.verticalAlign}`);
      let align = css.textAlign;
      if (!align && p.numFmt !== 'text') {
        const type = engine.getValue(r, c).type;
        if (type === 'number') align = 'right';
        else if (type === 'bool' || type === 'error') align = 'center';
      }
      if (align) style.push(`text-align:${align}`);
      const text = display(r, c);
      if (p[PROP_COND_FMT]) {
        const raw = sheet.cells.get(k)?.raw ?? '';
        const cs = condStyle(parseCondRules(p[PROP_COND_FMT]), raw.startsWith('=') ? engine.getValue(r, c).value : raw);
        if (cs.bg) style.push(`background:${cs.bg}`);
        if (cs.color) style.push(`color:${cs.color}`);
        if (cs.bold) style.push('font-weight:bold');
      }
      const span = spanAt.get(k);
      const attrs = [
        span && span.rows > 1 ? ` rowspan="${span.rows}"` : '',
        span && span.cols > 1 ? ` colspan="${span.cols}"` : '',
        style.length > 0 ? ` style="${escapeHtml(style.join(';'))}"` : '',
      ].join('');
      const link = p[PROP_LINK];
      const content = link && validLink(link) && text !== ''
        ? `<a href="${escapeHtml(link)}">${escapeHtml(text)}</a>`
        : escapeHtml(text);
      tds.push(`<td${attrs}>${content}</td>`);
    }
    rows.push(`<tr style="height:${rowH(r)}px">${tds.join('')}</tr>`);
  }

  // Charts anchored inside the range, placed over the table like in the grid.
  let boxW = tableW;
  let boxH = tableH;
  const charts: string[] = [];
  if (opts.chartSvg) {
    for (const ch of sheet.charts) {
      if (ch.row < r0 || ch.row > r1 || ch.col < c0 || ch.col > c1 || hidden.has(ch.row)) continue;
      let left = 0;
      for (let c = c0; c < ch.col; c++) left += colW(c);
      let top = 0;
      for (let r = r0; r < ch.row; r++) if (!hidden.has(r)) top += rowH(r);
      const data = chartData(ch.range, (r, c) => {
        const v = engine.getValue(r, c);
        return { text: display(r, c), num: v.type === 'number' && v.value !== '' ? Number(v.value) : null };
      });
      boxW = Math.max(boxW, left + ch.width);
      boxH = Math.max(boxH, top + ch.height);
      charts.push(`<div class="sheet-render-chart" style="left:${left}px;top:${top}px;width:${ch.width}px;height:${ch.height}px">${opts.chartSvg(ch, data)}</div>`);
    }
  }
  const cls = `sheet-render${opts.gridlines === false ? ' sheet-render-nogrid' : ''}`;
  const cut = [
    r1 < range.r1 ? `Only the first ${r1 - r0 + 1} rows are shown.` : '',
    c1 < range.c1 ? `Only the first ${width} columns are shown.` : '',
  ].filter(Boolean).map((t) => `<p class="sheet-render-cut">${t}</p>`).join('');
  return `<div class="${cls}" data-sheet="${escapeHtml(sheetId)}">${caption}` +
    `<div class="sheet-render-box" style="width:${boxW}px;height:${boxH}px">` +
    `<table style="width:${tableW}px"><colgroup>${cols.join('')}</colgroup><tbody>${rows.join('')}</tbody></table>` +
    `${charts.join('')}</div>${cut}</div>`;
}

// RENDER_CSS styles renderSheetHtml's markup, on screen and on paper.
export const RENDER_CSS = `
.sheet-render { font: 11pt Calibri, 'Segoe UI', system-ui, sans-serif; color: #000; }
.sheet-render-caption { font-weight: 600; margin: 0 0 6px; }
.sheet-render-box { position: relative; }
.sheet-render table { border-collapse: collapse; table-layout: fixed; }
.sheet-render td { border: 1px solid #d4d4d4; padding: 1px 3px; overflow: hidden; white-space: nowrap; text-overflow: clip; vertical-align: bottom; }
.sheet-render-nogrid td { border-color: transparent; }
.sheet-render tr { break-inside: avoid; }
.sheet-render-chart { position: absolute; }
.sheet-render-chart svg { display: block; }
.sheet-render-empty, .sheet-render-cut { color: #666; font-style: italic; }
.sheet-render a { color: #0563c1; }
`;

// printDocument wraps rendered sheets into a page for printing: each sheet
// starts on a new page.
export function printDocument(title: string, sheets: string[]): string {
  return '<!DOCTYPE html><html><head><meta charset="utf-8">' +
    `<title>${escapeHtml(title)}</title><style>${RENDER_CSS}` +
    '@page { margin: 12mm; } body { margin: 0; } ' +
    '.sheet-render + .sheet-render { break-before: page; } ' +
    '.sheet-render td, .sheet-render-chart { -webkit-print-color-adjust: exact; print-color-adjust: exact; }' +
    `</style></head><body>${sheets.join('')}</body></html>`;
}

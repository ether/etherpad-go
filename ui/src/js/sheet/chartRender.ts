// Draws a chart as SVG from the values of its range. Built with DOM calls
// (textContent), so cell text never becomes markup.
import { MAX_CHART_POINTS, MAX_CHART_SERIES, type Chart } from './chart';

export interface ChartSeries {
  name: string;
  values: Array<number | null>;
}

export interface ChartData {
  categories: string[];
  series: ChartSeries[];
}

export interface CellValue {
  text: string;
  num: number | null; // the value when the cell holds a number
}

// Excel's default palette.
export const CHART_COLORS = ['#4472c4', '#ed7d31', '#a5a5a5', '#ffc000', '#5b9bd5', '#70ad47', '#264478', '#9e480e'];


// parseRange reads "A1:C10" into zero-based bounds; null for #REF! and junk.
export function parseRange(range: string): { r0: number; c0: number; r1: number; c1: number } | null {
  const m = /^\$?([A-Z]{1,3})\$?(\d+)(?::\$?([A-Z]{1,3})\$?(\d+))?$/.exec(range);
  if (!m) return null;
  const col = (s: string): number => [...s].reduce((n, ch) => n * 26 + ch.charCodeAt(0) - 64, 0) - 1;
  const a = { r: Number(m[2]) - 1, c: col(m[1]) };
  const b = m[3] ? { r: Number(m[4]) - 1, c: col(m[3]) } : a;
  return { r0: Math.min(a.r, b.r), c0: Math.min(a.c, b.c), r1: Math.max(a.r, b.r), c1: Math.max(a.c, b.c) };
}

// chartData reads a range like Excel does by default: a first column of
// text gives the categories, a first row of text the series names, every
// other column is a series.
export function chartData(range: string, at: (r: number, c: number) => CellValue): ChartData | null {
  const b = parseRange(range);
  if (!b) return null;
  const isText = (v: CellValue): boolean => v.text.trim() !== '' && v.num === null;
  const multiCol = b.c1 > b.c0;
  let catCol = false;
  if (multiCol) {
    for (let r = b.r0 + 1; r <= Math.min(b.r1, b.r0 + MAX_CHART_POINTS); r++) if (isText(at(r, b.c0))) catCol = true;
    if (at(b.r0, b.c0).text.trim() === '') catCol = true; // Excel: empty corner = labels both ways
  }
  const firstSeriesCol = catCol ? b.c0 + 1 : b.c0;
  let header = false;
  for (let c = firstSeriesCol; c <= Math.min(b.c1, firstSeriesCol + MAX_CHART_SERIES - 1); c++) if (isText(at(b.r0, c))) header = true;
  const firstRow = header ? b.r0 + 1 : b.r0;
  // What a chart shows, also for one stored before these limits.
  const r1 = Math.min(b.r1, firstRow + MAX_CHART_POINTS - 1);
  const c1 = Math.min(b.c1, firstSeriesCol + MAX_CHART_SERIES - 1);
  const categories: string[] = [];
  for (let r = firstRow; r <= r1; r++) categories.push(catCol ? at(r, b.c0).text : String(r - firstRow + 1));
  const series: ChartSeries[] = [];
  for (let c = firstSeriesCol; c <= c1; c++) {
    const values: Array<number | null> = [];
    for (let r = firstRow; r <= r1; r++) values.push(at(r, c).num);
    series.push({ name: header ? at(b.r0, c).text : `Series ${series.length + 1}`, values });
  }
  return { categories, series };
}

const SVG = 'http://www.w3.org/2000/svg';
const el = <K extends keyof SVGElementTagNameMap>(tag: K, attrs: Record<string, string | number>, parent?: Element): SVGElementTagNameMap[K] => {
  const e = document.createElementNS(SVG, tag);
  for (const [k, v] of Object.entries(attrs)) e.setAttribute(k, String(v));
  parent?.appendChild(e);
  return e;
};
const text = (parent: Element, x: number, y: number, s: string, attrs: Record<string, string | number> = {}): SVGTextElement => {
  const t = el('text', { x, y, 'font-size': 11, fill: '#444', ...attrs }, parent);
  t.textContent = s;
  return t;
};
const clip = (s: string, n: number): string => (s.length > n ? `${s.slice(0, n - 1)}…` : s);

// niceScale picks round axis steps covering [min, max].
export function niceScale(min: number, max: number, ticks = 5): { lo: number; hi: number; step: number } {
  if (min === max) {
    max = min === 0 ? 1 : min + Math.abs(min);
    min = Math.min(0, min);
  }
  const raw = (max - min) / ticks;
  const mag = 10 ** Math.floor(Math.log10(raw));
  const step = [1, 2, 2.5, 5, 10].map((m) => m * mag).find((s) => s >= raw) ?? 10 * mag;
  return { lo: Math.floor(min / step) * step, hi: Math.ceil(max / step) * step, step };
}

const fmt = (n: number): string => {
  const a = Math.abs(n);
  if (a >= 1e9) return `${+(n / 1e9).toFixed(1)}G`;
  if (a >= 1e6) return `${+(n / 1e6).toFixed(1)}M`;
  if (a >= 1e4) return `${+(n / 1e3).toFixed(1)}k`;
  return String(+n.toFixed(6));
};

export function renderChart(chart: Chart, data: ChartData | null, w: number, h: number): SVGSVGElement {
  const svg = el('svg', { width: w, height: h, viewBox: `0 0 ${w} ${h}`, role: 'img', 'font-family': 'system-ui, sans-serif' });
  el('rect', { x: 0, y: 0, width: w, height: h, fill: '#fff' }, svg);
  const title = chart.title ?? '';
  svg.setAttribute('aria-label', title || `${chart.type} chart`);
  let top = 10;
  if (title) {
    text(svg, w / 2, 22, clip(title, 80), { 'text-anchor': 'middle', 'font-size': 14, fill: '#222', 'font-weight': 600 });
    top = 34;
  }
  if (!data || data.series.length === 0 || data.categories.length === 0) {
    text(svg, w / 2, h / 2, data ? 'No data in the range' : `Range ${chart.range} is not valid`, { 'text-anchor': 'middle', fill: '#888' });
    return svg;
  }
  const names = chart.type === 'pie' ? data.categories : data.series.map((s) => s.name);
  const legend = chart.type === 'pie' || data.series.length > 1;
  const bottomLegend = legend ? 22 : 0;
  if (legend) {
    let x = 10;
    const y = h - 8;
    names.slice(0, 12).forEach((n, i) => {
      el('rect', { x, y: y - 9, width: 10, height: 10, fill: CHART_COLORS[i % CHART_COLORS.length] }, svg);
      const label = clip(n, 18);
      text(svg, x + 14, y, label);
      x += 24 + label.length * 6.2;
    });
  }

  if (chart.type === 'pie') {
    const values = data.series[0].values.map((v) => (v !== null && v > 0 ? v : 0));
    const total = values.reduce((a, b) => a + b, 0);
    const cx = w / 2;
    const cy = top + (h - top - bottomLegend) / 2;
    const r = Math.max(10, Math.min(w, h - top - bottomLegend) / 2 - 12);
    if (total <= 0) {
      text(svg, cx, cy, 'No positive values', { 'text-anchor': 'middle', fill: '#888' });
      return svg;
    }
    let a = -Math.PI / 2;
    values.forEach((v, i) => {
      if (v <= 0) return;
      const da = (v / total) * Math.PI * 2;
      const color = CHART_COLORS[i % CHART_COLORS.length];
      if (da >= Math.PI * 2 - 1e-9) {
        el('circle', { cx, cy, r, fill: color }, svg);
      } else {
        const x0 = cx + r * Math.cos(a);
        const y0 = cy + r * Math.sin(a);
        const x1 = cx + r * Math.cos(a + da);
        const y1 = cy + r * Math.sin(a + da);
        el('path', { d: `M${cx},${cy} L${x0},${y0} A${r},${r} 0 ${da > Math.PI ? 1 : 0} 1 ${x1},${y1} Z`, fill: color, stroke: '#fff', 'stroke-width': 1 }, svg);
      }
      if (da > 0.25) {
        const m = a + da / 2;
        text(svg, cx + r * 0.65 * Math.cos(m), cy + r * 0.65 * Math.sin(m) + 4, `${Math.round((v / total) * 100)}%`, { 'text-anchor': 'middle', fill: '#fff', 'font-weight': 600 });
      }
      a += da;
    });
    return svg;
  }

  const all = data.series.flatMap((s) => s.values).filter((v): v is number => v !== null);
  const scale = niceScale(Math.min(0, ...all), Math.max(0, ...all));
  const horizontal = chart.type === 'bar';
  const left = horizontal ? 70 : 44;
  const plot = { x: left, y: top, w: Math.max(10, w - left - 14), h: Math.max(10, h - top - bottomLegend - 26) };
  const valuePos = (v: number): number =>
    horizontal
      ? plot.x + ((v - scale.lo) / (scale.hi - scale.lo)) * plot.w
      : plot.y + plot.h - ((v - scale.lo) / (scale.hi - scale.lo)) * plot.h;
  // grid lines and value labels
  for (let v = scale.lo; v <= scale.hi + scale.step / 2; v += scale.step) {
    const p = valuePos(v);
    if (horizontal) {
      el('line', { x1: p, y1: plot.y, x2: p, y2: plot.y + plot.h, stroke: '#e5e5e5' }, svg);
      text(svg, p, plot.y + plot.h + 14, fmt(v), { 'text-anchor': 'middle' });
    } else {
      el('line', { x1: plot.x, y1: p, x2: plot.x + plot.w, y2: p, stroke: '#e5e5e5' }, svg);
      text(svg, plot.x - 6, p + 4, fmt(v), { 'text-anchor': 'end' });
    }
  }
  const n = data.categories.length;
  const band = (horizontal ? plot.h : plot.w) / n;
  const zero = valuePos(Math.max(scale.lo, Math.min(0, scale.hi)));
  const labelEvery = Math.max(1, Math.ceil((n * 40) / (horizontal ? plot.h * 0.6 : plot.w)));
  data.categories.forEach((c, i) => {
    if (i % labelEvery !== 0) return;
    if (horizontal) text(svg, plot.x - 6, plot.y + band * (i + 0.5) + 4, clip(c, 10), { 'text-anchor': 'end' });
    else text(svg, plot.x + band * (i + 0.5), plot.y + plot.h + 14, clip(c, Math.max(3, Math.floor(band * labelEvery / 6.5))), { 'text-anchor': 'middle' });
  });

  if (chart.type === 'line') {
    data.series.forEach((s, si) => {
      const color = CHART_COLORS[si % CHART_COLORS.length];
      let d = '';
      s.values.forEach((v, i) => {
        if (v === null) return;
        const x = plot.x + band * (i + 0.5);
        const y = valuePos(v);
        d += `${d === '' || s.values[i - 1] === null ? 'M' : 'L'}${x},${y} `;
        if (n <= 60) el('circle', { cx: x, cy: y, r: 2.5, fill: color }, svg);
      });
      if (d) el('path', { d, fill: 'none', stroke: color, 'stroke-width': 2 }, svg);
    });
  } else {
    const k = data.series.length;
    const inner = band * 0.75;
    const bar = inner / k;
    data.series.forEach((s, si) => {
      const color = CHART_COLORS[si % CHART_COLORS.length];
      s.values.forEach((v, i) => {
        if (v === null) return;
        const p = valuePos(v);
        const along = band * i + (band - inner) / 2 + bar * si;
        if (horizontal) {
          el('rect', { x: Math.min(p, zero), y: plot.y + along, width: Math.max(0.5, Math.abs(p - zero)), height: Math.max(1, bar - 1), fill: color }, svg);
        } else {
          el('rect', { x: plot.x + along, y: Math.min(p, zero), width: Math.max(1, bar - 1), height: Math.max(0.5, Math.abs(p - zero)), fill: color }, svg);
        }
      });
    });
  }
  // axis line through zero
  if (horizontal) el('line', { x1: zero, y1: plot.y, x2: zero, y2: plot.y + plot.h, stroke: '#999' }, svg);
  else el('line', { x1: plot.x, y1: zero, x2: plot.x + plot.w, y2: zero, stroke: '#999' }, svg);
  return svg;
}

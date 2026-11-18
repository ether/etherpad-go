// Charts floating over a sheet (mirrors lib/sheet/chart.go).
import { rewriteFormula, type RefEdit } from './formulaRefs';

export type ChartType = 'column' | 'bar' | 'line' | 'pie';
export const CHART_TYPES: ChartType[] = ['column', 'bar', 'line', 'pie'];

export interface Chart {
  id: string;
  type: ChartType;
  range: string; // A1:C10 on the same sheet, #REF! once its cells are deleted
  title?: string;
  row: number; // top-left anchor cell
  col: number;
  width: number; // px
  height: number;
}

export const MAX_CHARTS_PER_SHEET = 50;
export const MAX_CHART_TITLE = 255;
export const CHART_RANGE = /^\$?[A-Z]{1,3}\$?[0-9]{1,7}(:\$?[A-Z]{1,3}\$?[0-9]{1,7})?$/;

const shiftIdx = (v: number, index: number, delta: number): number => (v < index ? v : v + delta);

// shiftChartAnchor: like a cell, but one inside a deleted band lands on its
// first row/column.
export function shiftChartAnchor(v: number, index: number, delta: number): number {
  if (delta < 0 && v >= index && v < index - delta) return index;
  return shiftIdx(v, index, delta);
}

// moveChartRange rewrites a chart's range as a formula on its own sheet.
export function moveChartRange(range: string, e: Omit<RefEdit, 'onTarget'>): string {
  if (range === '#REF!') return range;
  const out = rewriteFormula(`=${range}`, { ...e, onTarget: true });
  return out.startsWith('=') ? out.slice(1) : out;
}

export function moveCharts(charts: Chart[], e: Omit<RefEdit, 'onTarget'>): Chart[] {
  return charts.map((c) => ({
    ...c,
    row: e.axis === 'row' ? shiftChartAnchor(c.row, e.index ?? 0, e.delta ?? 0) : c.row,
    col: e.axis === 'col' ? shiftChartAnchor(c.col, e.index ?? 0, e.delta ?? 0) : c.col,
    range: moveChartRange(c.range, e),
  }));
}

export const newChartId = (): string => `c${Date.now().toString(36)}${Math.random().toString(36).slice(2, 8)}`;

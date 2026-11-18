import { describe, expect, it } from 'vitest';
import { chartData, niceScale, parseRange } from './chartRender';
import { WorkbookState } from './workbookState';
import { invertOp } from './undo';
import { transform } from './transform';
import type { Op } from './op';
import type { Chart } from './chart';

const grid: Record<string, string> = {
  '0:0': 'Monat', '0:1': 'Umsatz', '0:2': 'Kosten',
  '1:0': 'Jan', '1:1': '120', '1:2': '80',
  '2:0': 'Feb', '2:1': '150', '2:2': '',
};
const at = (r: number, c: number) => {
  const t = grid[`${r}:${c}`] ?? '';
  const n = t.trim() === '' ? null : Number(t);
  return { text: t, num: n !== null && Number.isFinite(n) ? n : null };
};

describe('chart data', () => {
  it('takes categories from a text column and names from a text row', () => {
    expect(chartData('A1:C3', at)).toEqual({
      categories: ['Jan', 'Feb'],
      series: [{ name: 'Umsatz', values: [120, 150] }, { name: 'Kosten', values: [80, null] }],
    });
  });
  it('numbers only: series 1.. and categories 1..', () => {
    const d = chartData('B2:B3', at)!;
    expect(d.categories).toEqual(['1', '2']);
    expect(d.series[0].name).toBe('Series 1');
  });
  it('broken ranges and scales', () => {
    expect(chartData('#REF!', at)).toBeNull();
    expect(parseRange('$B$3:A1')).toEqual({ r0: 0, c0: 0, r1: 2, c1: 1 });
    expect(niceScale(0, 187)).toEqual({ lo: 0, hi: 200, step: 50 });
  });
});

describe('charts in the workbook (mirrors lib/sheet/chart_test.go)', () => {
  const chart: Chart = { id: 'c1', type: 'column', range: 'A1:C10', title: 'Umsatz', row: 2, col: 4, width: 400, height: 300 };
  const fresh = () => {
    const wb = new WorkbookState();
    wb.loadSnapshot({ sheets: [{ id: 's1', name: 'Sheet1', cells: [] }] });
    wb.applyOp({ type: 'setChart', sheet: 's1', baseRev: 0, row: 2, col: 4, chart });
    return wb;
  };
  it('moves with rows and columns', () => {
    const wb = fresh();
    wb.applyOp({ type: 'insertRows', sheet: 's1', baseRev: 0, index: 0, count: 2 });
    wb.applyOp({ type: 'deleteCols', sheet: 's1', baseRev: 0, index: 1, count: 1 });
    expect(wb.sheetById('s1')!.charts[0]).toMatchObject({ row: 4, col: 3, range: 'A3:B12' });
  });
  it('undo of a delete that broke it puts it back', () => {
    const wb = fresh();
    const del: Op = { type: 'deleteRows', sheet: 's1', baseRev: 0, index: 0, count: 20 };
    const inverse = invertOp(wb, del);
    wb.applyOp(del);
    expect(wb.sheetById('s1')!.charts[0].range).toBe('#REF!');
    for (const inv of inverse) wb.applyOp(inv);
    expect(wb.sheetById('s1')!.charts[0]).toEqual(chart);
  });
  it('transform moves a concurrent chart edit', () => {
    const op: Op = { type: 'setChart', sheet: 's1', baseRev: 0, row: 5, col: 1, chart: { ...chart, range: 'A2:B9' } };
    const got = transform(op, { type: 'insertRows', sheet: 's1', baseRev: 0, index: 3, count: 2 });
    expect(got.row).toBe(7);
    expect(got.chart?.range).toBe('A2:B11');
  });
});

describe('chart review fixes', () => {
  const chart: Chart = { id: 'c1', type: 'column', range: 'A1:C10', row: 2, col: 4, width: 400, height: 300 };
  it('limits ranges like the server', async () => {
    const { chartRangeError } = await import('./chart');
    expect(chartRangeError('A0:B5')).not.toBeNull();
    expect(chartRangeError('A1:ZZZ9999999')).not.toBeNull();
    expect(chartRangeError('A1:AY10000')).toBeNull();
  });
  it('reads at most 500 points and 50 series', () => {
    const d = chartData('A1:ZZ900', (r, c) => ({ text: String(r + c), num: r + c }))!;
    expect(d.categories).toHaveLength(500);
    expect(d.series).toHaveLength(50);
  });
  it('a concurrent edit of a deleted chart is dropped', () => {
    const got = transform({ type: 'setChart', sheet: 's1', baseRev: 0, row: 1, col: 1, chart }, { type: 'deleteChart', sheet: 's1', baseRev: 0, chart: { id: 'c1' } as Chart });
    expect(got.noop).toBe(true);
  });
  it('undo of a delete keeps the drawing order', () => {
    const wb = new WorkbookState();
    wb.loadSnapshot({ sheets: [{ id: 's1', name: 'Sheet1', cells: [] }] });
    for (const id of ['a', 'b', 'c']) wb.applyOp({ type: 'setChart', sheet: 's1', baseRev: 0, row: 0, col: 0, chart: { ...chart, id } });
    const del: Op = { type: 'deleteChart', sheet: 's1', baseRev: 0, chart: { id: 'b' } as Chart };
    const inverse = invertOp(wb, del);
    wb.applyOp(del);
    for (const inv of inverse) wb.applyOp(inv);
    expect(wb.sheetById('s1')!.charts.map((c) => c.id)).toEqual(['a', 'b', 'c']);
  });
  it('undo of a row delete restores only the charts it broke, guarded', () => {
    const wb = new WorkbookState();
    wb.loadSnapshot({ sheets: [{ id: 's1', name: 'Sheet1', cells: [], charts: [
      { ...chart, id: 'far', range: 'A20:B30', row: 40 },
      { ...chart, id: 'hit', range: 'A1:B3', row: 1 },
    ] }] });
    const inverse = invertOp(wb, { type: 'deleteRows', sheet: 's1', baseRev: 0, index: 0, count: 5 });
    const restores = inverse.filter((o) => o.type === 'setChart');
    expect(restores.map((o) => o.chart?.id)).toEqual(['hit']);
    expect(restores[0].expectChart).toBeDefined();
  });
});

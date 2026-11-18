import { describe, expect, it } from 'vitest';
import { printDocument, renderSheetHtml, usedRange, workbookEngine, MAX_RENDER_CELLS } from './sheetRender';
import { WorkbookState } from './workbookState';
import type { Op } from './op';

const book = (ops: Op[]): WorkbookState => {
  const wb = new WorkbookState();
  wb.loadSnapshot({ sheets: [{ id: 's1', name: 'Sheet1', cells: [] }, { id: 's2', name: 'Two', cells: [] }] });
  for (const op of ops) wb.applyOp(op);
  return wb;
};
const set = (row: number, col: number, raw: string, props?: Record<string, string>, sheet = 's1'): Op =>
  ({ type: 'setCell', sheet, baseRev: 0, row, col, raw, ...(props ? { props } : {}) });

const render = (wb: WorkbookState, opts = {}, sheet = 's1') => renderSheetHtml(wb, sheet, workbookEngine(wb), opts);
const cells = (html: string): string[] => [...html.matchAll(/<td[^>]*>(.*?)<\/td>/g)].map((m) => m[1]);

describe('sheet render', () => {
  it('renders the used range with computed, formatted values', () => {
    const wb = book([
      set(0, 0, 'Item'), set(0, 1, 'Price', { bold: '1', bg: '#ffff00' }),
      set(1, 0, 'Tea'), set(1, 1, '2.5', { numFmt: 'number:2' }),
      set(2, 0, 'Sum'), set(2, 1, '=B2*2+Two!A1'),
      set(0, 0, '10', undefined, 's2'),
    ]);
    const html = render(wb);
    expect(cells(html)).toEqual(['Item', 'Price', 'Tea', '2.50', 'Sum', '15']);
    expect(html).toContain('font-weight:bold;background:#ffff00');
    expect(html).toMatch(/<td style="text-align:right">15<\/td>/);
    expect(html.match(/<col /g)).toHaveLength(2);
  });

  it('escapes cell text and keeps only valid links', () => {
    const wb = book([
      set(0, 0, '<img src=x onerror=alert(1)>'),
      set(1, 0, 'site', { link: 'https://example.org/?a=1&b="2"' }),
      set(2, 0, 'bad', { link: 'javascript:alert(1)' }),
    ]);
    const html = render(wb);
    expect(html).not.toContain('<img');
    expect(html).toContain('&lt;img src=x onerror=alert(1)&gt;');
    expect(html).toContain('<a href="https://example.org/?a=1&amp;b=&quot;2&quot;">site</a>');
    expect(html).not.toContain('javascript:');
  });

  it('spans merges and leaves out the covered cells', () => {
    const wb = book([
      set(0, 0, 'Title'), set(0, 1, 'hidden under the merge'), set(1, 2, 'x'),
      { type: 'mergeCells', sheet: 's1', baseRev: 0, row: 0, col: 0, endRow: 0, endCol: 2 },
    ]);
    const html = render(wb);
    expect(html).toContain('<td colspan="3">Title</td>');
    expect(cells(html)).toEqual(['Title', '', '', 'x']);
  });

  it('leaves out rows the filters hide and clips merges to the shown rows', () => {
    const wb = book([
      set(0, 0, 'Fruit'), set(1, 0, 'Apple'), set(2, 0, 'Pear'), set(3, 0, 'Apple'),
      set(1, 1, 'm'),
      { type: 'mergeCells', sheet: 's1', baseRev: 0, row: 1, col: 1, endRow: 2, endCol: 1 },
      { type: 'setFilter', sheet: 's1', baseRev: 0, row: 0, col: 0, values: ['Apple'] },
    ]);
    const html = render(wb);
    expect(cells(html)).toEqual(['Fruit', '', 'Apple', 'm', 'Apple', '']);
    expect(html).not.toContain('rowspan');
  });

  it('renders a selection, widths and heights', () => {
    const wb = book([
      set(0, 0, 'a'), set(1, 1, 'b'), set(5, 5, 'far'),
      { type: 'setDimension', sheet: 's1', baseRev: 0, axis: 'col', index: 1, size: 150 },
      { type: 'setDimension', sheet: 's1', baseRev: 0, axis: 'row', index: 1, size: 40 },
    ]);
    const html = render(wb, { range: { r0: 1, c0: 1, r1: 2, c1: 2 } });
    expect(cells(html)).toEqual(['b', '', '', '']);
    expect(html).toContain('<col style="width:150px"><col style="width:80px">');
    expect(html).toContain('<tr style="height:40px">');
    expect(html).toContain('<table style="width:230px">');
  });

  it('places charts over the table', () => {
    const wb = book([
      set(0, 0, 'x'), set(1, 0, '1'),
      { type: 'setChart', sheet: 's1', baseRev: 0, row: 2, col: 1, chart: { id: 'c1', type: 'column', range: 'A1:A2', row: 2, col: 1, width: 300, height: 200 } },
    ]);
    const seen: string[] = [];
    const html = render(wb, { chartSvg: (ch: { id: string }, data: unknown) => { seen.push(ch.id + (data ? '+' : '-')); return '<svg id="chart"></svg>'; } });
    expect(seen).toEqual(['c1+']);
    expect(html).toContain('<div class="sheet-render-chart" style="left:80px;top:44px;width:300px;height:200px"><svg id="chart"></svg></div>');
    expect(html).toContain('style="width:380px;height:244px"');
    expect(render(wb)).not.toContain('sheet-render-chart');
  });

  it('handles empty sheets, gridlines and the size limit', () => {
    const wb = book([]);
    expect(usedRange(wb.sheetById('s1')!)).toBeNull();
    expect(render(wb)).toContain('This sheet is empty.');
    const one = book([set(0, 0, 'a')]);
    expect(render(one, { gridlines: false })).toContain('sheet-render-nogrid');
    expect(render(one, { caption: 'Sheet <1>' })).toContain('<div class="sheet-render-caption">Sheet &lt;1&gt;</div>');
    const big = render(one, { range: { r0: 0, c0: 0, r1: MAX_RENDER_CELLS, c1: 1 } });
    expect(big.match(/<tr /g)).toHaveLength(MAX_RENDER_CELLS / 2);
    expect(big).toContain(`Only the first ${MAX_RENDER_CELLS / 2} rows are shown.`);
  });

  it('wraps sheets into a printable page', () => {
    const doc = printDocument('Q <1>', ['<div class="sheet-render">a</div>', '<div class="sheet-render">b</div>']);
    expect(doc).toMatch(/^<!DOCTYPE html>/);
    expect(doc).toContain('<title>Q &lt;1&gt;</title>');
    expect(doc).toContain('.sheet-render + .sheet-render { break-before: page; }');
  });
});

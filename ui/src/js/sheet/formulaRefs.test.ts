import casesJson from '../../../../lib/sheet/testdata/formularefs.json?raw';
import { describe, expect, it } from 'vitest';
import { rewriteFormula, quoteSheetName, type RefEdit } from './formulaRefs';
import { WorkbookState } from './workbookState';
import { transform } from './transform';
import { invertOp } from './undo';
import type { Op } from './op';

interface RefCase {
  name: string;
  raw: string;
  edit: RefEdit;
  want: string;
}

// Shared with lib/sheet/formularefs_test.go: server and browser must rewrite
// formulas the same way.
const cases: RefCase[] = JSON.parse(casesJson);

describe('rewriteFormula', () => {
  for (const c of cases) {
    it(c.name, () => {
      expect(rewriteFormula(c.raw, c.edit)).toBe(c.want);
    });
  }

  it('quotes names that need it', () => {
    expect(quoteSheetName('Sheet1')).toBe('Sheet1');
    expect(quoteSheetName('My Data')).toBe("'My Data'");
    expect(quoteSheetName("It's")).toBe("'It''s'");
    expect(quoteSheetName('B2')).toBe("'B2'");
  });
});

const wbWith = (): WorkbookState => {
  const wb = new WorkbookState();
  wb.loadSnapshot({
    sheets: [
      { id: 's1', name: 'Sheet1', cells: [
        { row: 0, col: 0, raw: '5' },
        { row: 1, col: 0, raw: '=A1*2' },
        { row: 0, col: 1, raw: '=A2+1' },
      ] },
      { id: 's2', name: 'Sheet2', cells: [{ row: 0, col: 0, raw: '=Sheet1!A2+A2' }] },
    ],
  } as never);
  return wb;
};

describe('applyOp moves formula references', () => {
  it('on insert, rename and delete', () => {
    const wb = wbWith();
    const before = wb.clone();
    wb.applyOp({ type: 'insertRows', sheet: 's1', baseRev: 0, index: 0, count: 1 });
    expect(wb.getCell('s1', 2, 0)?.raw).toBe('=A2*2');
    expect(wb.getCell('s1', 1, 1)?.raw).toBe('=A3+1');
    expect(wb.getCell('s2', 0, 0)?.raw).toBe('=Sheet1!A3+A2');
    // a clone sharing the cell objects is not changed
    expect(before.getCell('s1', 1, 0)?.raw).toBe('=A1*2');

    wb.applyOp({ type: 'renameSheet', sheet: 's1', baseRev: 0, name: 'Daten 1' });
    expect(wb.getCell('s2', 0, 0)?.raw).toBe("='Daten 1'!A3+A2");
    wb.applyOp({ type: 'deleteSheet', sheet: 's1', baseRev: 0 });
    expect(wb.getCell('s2', 0, 0)?.raw).toBe('=#REF!+A2');
  });

  it('transform moves a concurrent formula', () => {
    const op: Op = { type: 'setCell', sheet: 's1', baseRev: 0, row: 7, col: 1, raw: '=A5+Sheet1!A5' };
    const got = transform(op, { type: 'insertRows', sheet: 's1', baseRev: 0, index: 2, count: 1 });
    expect(got.row).toBe(8);
    expect(got.raw).toBe('=A6+Sheet1!A5');
  });
});

describe('undo restores rewritten formulas', () => {
  const roundTrip = (op: Op): void => {
    const wb = wbWith();
    const before = JSON.stringify(wb.sheets.map((s) => [s.id, s.name, [...s.cells].map(([k, c]) => [k, c.raw]).sort()]));
    const inverse = invertOp(wb, op);
    wb.applyOp(op);
    for (const inv of inverse) wb.applyOp(inv);
    const after = JSON.stringify(wb.sheets.map((s) => [s.id, s.name, [...s.cells].map(([k, c]) => [k, c.raw]).sort()]));
    expect(after).toBe(before);
  };
  it('delete row', () => roundTrip({ type: 'deleteRows', sheet: 's1', baseRev: 0, index: 0, count: 1 }));
  it('insert row', () => roundTrip({ type: 'insertRows', sheet: 's1', baseRev: 0, index: 0, count: 2 }));
  it('delete column', () => roundTrip({ type: 'deleteCols', sheet: 's1', baseRev: 0, index: 0, count: 1 }));
  it('rename sheet', () => roundTrip({ type: 'renameSheet', sheet: 's1', baseRev: 0, name: 'X Y' }));
  it('delete sheet', () => roundTrip({ type: 'deleteSheet', sheet: 's1', baseRev: 0 }));
});

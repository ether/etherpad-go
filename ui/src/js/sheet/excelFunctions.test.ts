import { describe, it, expect, beforeAll } from 'vitest';
import { FormulaEngine } from './formulaEngine';
import { excelExtraFunctionNames } from './excelFunctions';

// One engine for all cases: formulas go into column Z and read fixtures from
// A:D, so each case is a single setCell + read.
let e: FormulaEngine;

const FIXTURE = [
  // A     B        C       D
  ['3', 'apple', '10', '2020-01-01'],
  ['1', 'banana', '20', '2021-01-01'],
  ['3', 'apple', '30', '2022-01-01'],
  ['7', 'cherry', '40', '2023-01-01'],
];

const evalAt = (formula: string, row = 0, col = 25): string => e.setCell(row, col, formula).value;

beforeAll(() => {
  e = new FormulaEngine();
  FIXTURE.forEach((cells, r) => cells.forEach((raw, c) => e.setCell(r, c, raw)));
});

describe('registration', () => {
  it('exposes the new functions to autocomplete', () => {
    const names = new FormulaEngine().functionNames();
    for (const n of excelExtraFunctionNames) expect(names).toContain(n);
  });
});

describe('text', () => {
  it('CONCAT joins scalars and ranges', () => {
    expect(evalAt('=CONCAT("a",1,TRUE(),B1:B2)')).toBe('a1TRUEapplebanana');
  });

  it('TEXTBEFORE / TEXTAFTER pick by instance', () => {
    expect(evalAt('=TEXTBEFORE("a-b-c","-")')).toBe('a');
    expect(evalAt('=TEXTBEFORE("a-b-c","-",2)')).toBe('a-b');
    expect(evalAt('=TEXTBEFORE("a-b-c","-",-1)')).toBe('a-b');
    expect(evalAt('=TEXTAFTER("a-b-c","-")')).toBe('b-c');
    expect(evalAt('=TEXTAFTER("a-b-c","-",-1)')).toBe('c');
    expect(evalAt('=TEXTAFTER("a-b-c","X",1,0,0,"none")')).toBe('none');
    expect(evalAt('=TEXTBEFORE("aXb","x",1,1)')).toBe('a'); // case-insensitive
    expect(evalAt('=TEXTBEFORE("abc","-")')).toBe('#N/A');
    expect(evalAt('=TEXTBEFORE("abc","-",1,0,1)')).toBe('abc'); // match_end
  });

  it('NUMBERVALUE honours separators and percent signs', () => {
    expect(evalAt('=NUMBERVALUE("1.234,56",",",".")')).toBe('1234.56');
    expect(evalAt('=NUMBERVALUE("50%")')).toBe('0.5');
    expect(evalAt('=NUMBERVALUE("abc")')).toBe('#VALUE!');
  });

  it('FIXED and DOLLAR format numbers', () => {
    expect(evalAt('=FIXED(1234.567)')).toBe('1,234.57');
    expect(evalAt('=FIXED(1234.567,1,TRUE())')).toBe('1234.6');
    expect(evalAt('=FIXED(1234.567,-2)')).toBe('1,200');
    expect(evalAt('=DOLLAR(1234.567)')).toBe('$1,234.57');
    expect(evalAt('=DOLLAR(-1234.567)')).toBe('($1,234.57)');
  });
});

describe('lookup', () => {
  it('XMATCH finds exact, closest and wildcard matches', () => {
    expect(evalAt('=XMATCH(7,A1:A4)')).toBe('4');
    expect(evalAt('=XMATCH(3,A1:A4)')).toBe('1');
    expect(evalAt('=XMATCH(3,A1:A4,0,-1)')).toBe('3'); // search from the end
    expect(evalAt('=XMATCH(5,A1:A4,-1)')).toBe('1'); // next smaller (3)
    expect(evalAt('=XMATCH(5,A1:A4,1)')).toBe('4'); // next larger (7)
    expect(evalAt('=XMATCH("ban*",B1:B4,2)')).toBe('2');
    expect(evalAt('=XMATCH(99,A1:A4)')).toBe('#N/A');
  });

  it('LOOKUP handles the vector and the array form', () => {
    expect(evalAt('=LOOKUP(3,A1:A4,C1:C4)')).toBe('30');
    expect(evalAt('=LOOKUP(4,A1:A4,C1:C4)')).toBe('30'); // largest value <= 4
    expect(evalAt('=LOOKUP(3,A1:C4)')).toBe('30'); // taller than wide: last column
    expect(evalAt('=LOOKUP(0,A1:A4,C1:C4)')).toBe('#N/A');
  });
});

describe('dynamic arrays', () => {
  // Array results spill; read the spilled cells directly.
  const spill = (formula: string, row: number, col: number): string => {
    e.setCell(10, 10, formula); // K11
    return e.getValue(row, col).value;
  };

  it('UNIQUE drops duplicate rows', () => {
    expect(spill('=UNIQUE(B1:B4)', 10, 10)).toBe('apple');
    expect(spill('=UNIQUE(B1:B4)', 11, 10)).toBe('banana');
    expect(spill('=UNIQUE(B1:B4)', 12, 10)).toBe('cherry');
    expect(spill('=UNIQUE(B1:B4,FALSE(),TRUE())', 10, 10)).toBe('banana'); // exactly once
  });

  it('SORT orders rows by a column', () => {
    expect(spill('=SORT(A1:A4)', 10, 10)).toBe('1');
    expect(spill('=SORT(A1:A4,1,-1)', 10, 10)).toBe('7');
  });

  it('SORTBY orders one range by another', () => {
    expect(spill('=SORTBY(B1:B4,C1:C4,-1)', 10, 10)).toBe('cherry');
  });

  it('TAKE and DROP slice from either end', () => {
    expect(spill('=TAKE(A1:A4,2)', 11, 10)).toBe('1');
    expect(spill('=TAKE(A1:A4,-1)', 10, 10)).toBe('7');
    expect(spill('=DROP(A1:A4,3)', 10, 10)).toBe('7');
    expect(spill('=DROP(A1:A4,-3)', 10, 10)).toBe('3');
  });

  it('VSTACK and HSTACK combine ranges', () => {
    expect(spill('=VSTACK(A1:A2,C1:C2)', 12, 10)).toBe('10');
    expect(spill('=HSTACK(A1:A2,C1:C2)', 10, 11)).toBe('10');
  });

  it('TOCOL and TOROW flatten', () => {
    expect(spill('=TOROW(A1:A4)', 10, 13)).toBe('7');
    expect(spill('=TOCOL(A1:C1)', 12, 10)).toBe('10');
  });

  it('CHOOSECOLS and CHOOSEROWS pick by index', () => {
    expect(spill('=CHOOSECOLS(A1:C1,3)', 10, 10)).toBe('10');
    expect(spill('=CHOOSEROWS(A1:A4,-1)', 10, 10)).toBe('7');
    expect(spill('=CHOOSECOLS(A1:C1,9)', 10, 10)).toBe('#VALUE!');
  });

  it('EXPAND pads to a larger size', () => {
    expect(spill('=EXPAND(A1:A2,3,1,0)', 12, 10)).toBe('0');
    expect(spill('=EXPAND(A1:A2,1)', 10, 10)).toBe('#VALUE!'); // cannot shrink
  });

  it('FREQUENCY buckets values', () => {
    e.setCell(20, 0, '5'); // A21 bin
    e.setCell(21, 0, '25'); // A22 bin
    expect(spill('=FREQUENCY(C1:C4,A21:A22)', 10, 10)).toBe('0'); // <=5
    expect(spill('=FREQUENCY(C1:C4,A21:A22)', 11, 10)).toBe('2'); // <=25
    expect(spill('=FREQUENCY(C1:C4,A21:A22)', 12, 10)).toBe('2'); // rest
  });
});

describe('statistics', () => {
  it('AVERAGEIFS averages with multiple criteria', () => {
    expect(evalAt('=AVERAGEIFS(C1:C4,A1:A4,3)')).toBe('20'); // (10+30)/2
    expect(evalAt('=AVERAGEIFS(C1:C4,A1:A4,3,C1:C4,">15")')).toBe('30');
    expect(evalAt('=AVERAGEIFS(C1:C4,B1:B4,"a*")')).toBe('20');
    expect(evalAt('=AVERAGEIFS(C1:C4,B1:B4,"<>apple")')).toBe('30'); // (20+40)/2
    expect(evalAt('=AVERAGEIFS(C1:C4,A1:A4,99)')).toBe('#DIV/0!');
  });

  it('RANK and RANK.AVG rank ties', () => {
    expect(evalAt('=RANK(7,A1:A4)')).toBe('1');
    expect(evalAt('=RANK(3,A1:A4)')).toBe('2');
    expect(evalAt('=RANK(3,A1:A4,1)')).toBe('2'); // ascending: 1 is smaller
    expect(evalAt('=RANK.AVG(3,A1:A4)')).toBe('2.5');
    expect(evalAt('=RANK.EQ(3,A1:A4)')).toBe('2');
    expect(evalAt('=RANK(99,A1:A4)')).toBe('#N/A');
  });

  it('MODE returns the most frequent value', () => {
    expect(evalAt('=MODE(A1:A4)')).toBe('3');
    expect(evalAt('=MODE.SNGL(A1:A4)')).toBe('3');
    expect(evalAt('=MODE(C1:C4)')).toBe('#N/A'); // all distinct
  });

  it('TRIMMEAN drops the extremes', () => {
    expect(evalAt('=TRIMMEAN(C1:C4,0.5)')).toBe('25'); // trims 10 and 40
    expect(evalAt('=TRIMMEAN(C1:C4,0)')).toBe('25');
  });

  it('PERMUT and PERMUTATIONA count arrangements', () => {
    expect(evalAt('=PERMUT(5,2)')).toBe('20');
    expect(evalAt('=PERMUTATIONA(5,2)')).toBe('25');
    expect(evalAt('=PERMUT(2,5)')).toBe('#NUM!');
  });

  it('INTERCEPT and FORECAST fit a line', () => {
    // C = 10, 20, 30, 40 against x = 1..4 in B21:B24
    e.setCell(20, 1, '1');
    e.setCell(21, 1, '2');
    e.setCell(22, 1, '3');
    e.setCell(23, 1, '4');
    expect(evalAt('=INTERCEPT(C1:C4,B21:B24)')).toBe('0');
    expect(evalAt('=FORECAST(5,C1:C4,B21:B24)')).toBe('50');
    expect(evalAt('=FORECAST.LINEAR(5,C1:C4,B21:B24)')).toBe('50');
  });
});

describe('information', () => {
  it('ERROR.TYPE maps error values to codes', () => {
    expect(evalAt('=ERROR.TYPE(1/0)')).toBe('2');
    expect(evalAt('=ERROR.TYPE(NA())')).toBe('7');
    expect(evalAt('=ERROR.TYPE(1)')).toBe('#N/A');
  });

  it('TYPE classifies values', () => {
    expect(evalAt('=TYPE(1)')).toBe('1');
    expect(evalAt('=TYPE("x")')).toBe('2');
    expect(evalAt('=TYPE(TRUE())')).toBe('4');
    expect(evalAt('=TYPE(NA())')).toBe('16');
    // ponytail: a range argument collapses to its first value (no array type 64) —
    // accepting errors (16) matters more in practice than TYPE of an array.
    expect(evalAt('=TYPE(A1:A2)')).toBe('1');
  });
});

describe('financial', () => {
  it('XIRR solves for the irregular-interval rate', () => {
    e.setCell(30, 0, '-1000');
    e.setCell(31, 0, '1100');
    e.setCell(30, 1, '=DATE(2020,1,1)');
    e.setCell(31, 1, '=DATE(2021,1,1)'); // 366 days later
    const r = Number(evalAt('=XIRR(A31:A32,B31:B32)'));
    expect(r).toBeCloseTo(0.0997, 3); // ~10% over 366/365 years
  });

  it('reports #NUM! without a sign change', () => {
    e.setCell(30, 0, '-1000');
    e.setCell(31, 0, '-1100');
    expect(evalAt('=XIRR(A31:A32,B31:B32)')).toBe('#NUM!');
  });
});

import { describe, it, expect, beforeAll } from 'vitest';
import { FormulaEngine } from './formulaEngine';
import { bondFunctionNames } from './bondFunctions';

// Expected values are Microsoft's documented examples for each function, so a
// regression here means we drifted away from Excel, not from ourselves.
let e: FormulaEngine;

const num = (formula: string): number => Number(e.setCell(0, 25, formula).value);
const text = (formula: string): string => e.setCell(0, 25, formula).value;

beforeAll(() => {
  e = new FormulaEngine();
});

describe('registration', () => {
  it('exposes the bond functions to autocomplete', () => {
    const names = new FormulaEngine().functionNames();
    for (const n of bondFunctionNames) expect(names).toContain(n);
  });
});

describe('day-count conventions', () => {
  // Each basis is cross-checked against YEARFRAC, which HyperFormula implements
  // independently, so a wrong denominator cannot hide.
  it('agree with YEARFRAC for every basis', () => {
    for (const basis of [0, 1, 2, 3, 4]) {
      // Not an end-of-February start date: there HyperFormula's YEARFRAC applies
      // the US 30/360 rules in the wrong order and we deliberately differ (see
      // days360 in bondFunctions.ts and the end-of-month case below).
      const yf = num(`=YEARFRAC(DATE(2008,2,15),DATE(2011,7,31),${basis})`);
      // ACCRINTM is par * rate * yearfrac, so with par=100 and rate=1 it is the
      // day-count fraction itself.
      const viaBond = num(`=ACCRINTM(DATE(2008,2,15),DATE(2011,7,31),1,100,${basis})`) / 100;
      expect(viaBond).toBeCloseTo(yf, 10);
    }
  });

  it('handles the 30/360 end-of-month rules', () => {
    // US 30/360: 31st collapses to the 30th, February's last day counts as 30.
    expect(num('=ACCRINTM(DATE(2008,1,31),DATE(2008,3,31),1,360,0)')).toBeCloseTo(60, 9);
    // Excel gives 180 here; HyperFormula's YEARFRAC gives 181 (rule ordering).
    expect(num('=ACCRINTM(DATE(2008,2,29),DATE(2008,8,31),1,360,0)')).toBeCloseTo(180, 9);
    expect(num('=YEARFRAC(DATE(2008,2,29),DATE(2008,8,31),0)') * 360).toBeCloseTo(181, 6);
    // Both ends in February: the end date also becomes the 30th.
    expect(num('=ACCRINTM(DATE(2008,2,29),DATE(2009,2,28),1,360,0)')).toBeCloseTo(360, 9);
    // European 30/360 shortens both 31sts unconditionally.
    expect(num('=ACCRINTM(DATE(2008,1,31),DATE(2008,3,31),1,360,4)')).toBeCloseTo(60, 9);
  });
});

describe('coupon dates', () => {
  const settle = 'DATE(2011,1,25)';
  const mature = 'DATE(2011,11,15)';

  it('COUPDAYBS / COUPDAYS / COUPDAYSNC split the coupon period', () => {
    expect(num(`=COUPDAYBS(${settle},${mature},2,1)`)).toBe(71);
    expect(num(`=COUPDAYS(${settle},${mature},2,1)`)).toBe(181);
    expect(num(`=COUPDAYSNC(${settle},${mature},2,1)`)).toBe(110);
    // The three always add up to the period length.
    for (const basis of [0, 1, 2, 3, 4]) {
      const period = num(`=COUPDAYS(${settle},${mature},2,${basis})`);
      const before = num(`=COUPDAYBS(${settle},${mature},2,${basis})`);
      const after = num(`=COUPDAYSNC(${settle},${mature},2,${basis})`);
      if (basis === 0 || basis === 4) expect(before + after).toBe(period);
      else expect(before + after).toBeGreaterThan(0);
    }
  });

  it('COUPNCD / COUPPCD bracket the settlement date', () => {
    expect(num(`=COUPNCD(${settle},${mature},2)`)).toBe(num('=DATE(2011,5,15)'));
    expect(num(`=COUPPCD(${settle},${mature},2)`)).toBe(num('=DATE(2010,11,15)'));
    // Quarterly and annual schedules step from maturity, not from settlement.
    expect(num(`=COUPNCD(${settle},${mature},4)`)).toBe(num('=DATE(2011,2,15)'));
    expect(num(`=COUPPCD(${settle},${mature},1)`)).toBe(num('=DATE(2010,11,15)'));
  });

  it('COUPNUM counts the outstanding coupons', () => {
    expect(num('=COUPNUM(DATE(2007,1,25),DATE(2008,11,15),2,1)')).toBe(4);
    expect(num('=COUPNUM(DATE(2007,1,25),DATE(2008,11,15),4,1)')).toBe(8);
    expect(num('=COUPNUM(DATE(2007,1,25),DATE(2008,11,15),1,1)')).toBe(2);
  });

  it('keeps month-end maturities on schedule', () => {
    // A 31 August maturity must not drift to the 30th via February.
    expect(num('=COUPPCD(DATE(2011,1,25),DATE(2012,8,31),2)')).toBe(num('=DATE(2010,8,31)'));
    expect(num('=COUPNCD(DATE(2011,1,25),DATE(2012,8,31),2)')).toBe(num('=DATE(2011,2,28)'));
  });
});

describe('coupon bonds', () => {
  it('PRICE matches the documented example', () => {
    expect(num('=PRICE(DATE(2008,2,15),DATE(2017,11,15),0.0575,0.065,100,2,0)')).toBeCloseTo(94.63436, 4);
  });

  it('YIELD inverts PRICE', () => {
    expect(num('=YIELD(DATE(2008,2,15),DATE(2016,11,15),0.0575,95.04287,100,2,0)')).toBeCloseTo(0.065, 5);
    // Round trip at an arbitrary yield, including a bond in its final period.
    const price = num('=PRICE(DATE(2008,2,15),DATE(2008,11,15),0.05,0.08,100,2,1)');
    expect(num(`=YIELD(DATE(2008,2,15),DATE(2008,11,15),0.05,${price},100,2,1)`)).toBeCloseTo(0.08, 6);
  });

  it('DURATION and MDURATION match the documented example', () => {
    expect(num('=DURATION(DATE(2018,7,1),DATE(2048,1,1),0.08,0.09,2,1)')).toBeCloseTo(10.9191453, 5);
    // MDURATION is DURATION discounted by one period's yield, by definition.
    expect(num('=MDURATION(DATE(2018,7,1),DATE(2048,1,1),0.08,0.09,2,1)')).toBeCloseTo(10.9191453 / 1.045, 6);
  });
});

describe('accrued interest', () => {
  it('ACCRINT matches the documented example', () => {
    expect(num('=ACCRINT(DATE(2008,3,1),DATE(2008,8,31),DATE(2008,5,1),0.1,1000,2,0)')).toBeCloseTo(16.66667, 4);
  });

  it('ACCRINTM matches the documented example', () => {
    expect(num('=ACCRINTM(DATE(2008,4,1),DATE(2008,6,15),0.1,1000,3)')).toBeCloseTo(20.54795, 4);
  });
});

describe('discounted securities', () => {
  it('INTRATE and RECEIVED match the documented examples', () => {
    expect(num('=INTRATE(DATE(2008,2,15),DATE(2008,5,15),1000000,1014420,2)')).toBeCloseTo(0.05768, 5);
    expect(num('=RECEIVED(DATE(2008,2,15),DATE(2008,5,15),1000000,0.0575,2)')).toBeCloseTo(1014584.654, 2);
  });

  it('PRICEDISC and YIELDDISC match the documented examples', () => {
    expect(num('=PRICEDISC(DATE(2008,2,16),DATE(2008,3,1),0.0525,100,2)')).toBeCloseTo(99.79583, 4);
    expect(num('=YIELDDISC(DATE(2008,2,16),DATE(2008,3,1),99.795,100,2)')).toBeCloseTo(0.052823, 5);
  });

  it('DISC is the inverse of PRICEDISC', () => {
    const price = num('=PRICEDISC(DATE(2008,2,16),DATE(2009,3,1),0.0525,100,2)');
    expect(num(`=DISC(DATE(2008,2,16),DATE(2009,3,1),${price},100,2)`)).toBeCloseTo(0.0525, 8);
  });
});

describe('securities paying interest at maturity', () => {
  it('PRICEMAT matches the documented example', () => {
    expect(num('=PRICEMAT(DATE(2008,2,15),DATE(2008,4,13),DATE(2007,11,11),0.061,0.061,0)')).toBeCloseTo(99.98449888, 6);
  });

  it('YIELDMAT matches the documented example', () => {
    expect(num('=YIELDMAT(DATE(2008,3,15),DATE(2008,11,3),DATE(2007,11,8),0.0625,100.0123,0)')).toBeCloseTo(0.060954, 5);
  });

  it('YIELDMAT inverts PRICEMAT', () => {
    const price = num('=PRICEMAT(DATE(2008,2,15),DATE(2009,4,13),DATE(2007,11,11),0.061,0.07,1)');
    expect(num(`=YIELDMAT(DATE(2008,2,15),DATE(2009,4,13),DATE(2007,11,11),0.061,${price},1)`)).toBeCloseTo(0.07, 8);
  });
});

describe('argument validation', () => {
  it('rejects settlement on or after maturity', () => {
    expect(text('=PRICE(DATE(2008,2,15),DATE(2008,2,15),0.05,0.06,100,2,0)')).toBe('#NUM!');
    expect(text('=COUPNUM(DATE(2009,1,1),DATE(2008,1,1),2,0)')).toBe('#NUM!');
    expect(text('=ACCRINTM(DATE(2008,6,15),DATE(2008,4,1),0.1,1000,3)')).toBe('#NUM!');
  });

  it('rejects an unsupported coupon frequency and basis', () => {
    expect(text('=PRICE(DATE(2008,2,15),DATE(2017,11,15),0.0575,0.065,100,3,0)')).toBe('#NUM!');
    expect(text('=COUPDAYS(DATE(2011,1,25),DATE(2011,11,15),2,5)')).toBe('#NUM!');
  });
});

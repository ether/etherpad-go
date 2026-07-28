// Excel's bond/security functions, which HyperFormula does not implement. They
// all sit on top of two pieces of shared machinery: the day-count conventions
// (basis 0-4) and the coupon schedule derived backwards from the maturity date.
//
// Basis: 0 = US 30/360, 1 = actual/actual, 2 = actual/360, 3 = actual/365,
// 4 = European 30/360. The fractions match YEARFRAC for all bases except the
// end-of-February case in basis 0, where HyperFormula's YEARFRAC deviates from
// Excel and these functions follow Excel (see days360).
import { CellError, ErrorType, FunctionArgumentType as T, FunctionPlugin, type ImplementedFunctions } from 'hyperformula';

// eslint-disable-next-line @typescript-eslint/no-explicit-any -- HyperFormula's
// interpreter types (ProcedureAst, InterpreterState) are not public API.
type Val = any;

interface Date3 {
  year: number;
  month: number;
  day: number;
}

const numErr = (m: string): CellError => new CellError(ErrorType.NUM, m);

// NumberType is not re-exported by HyperFormula, so the date-typed return value
// (COUPNCD/COUPPCD render as dates, not serial numbers) is spelled out here.
const DATE_RESULT = 'NUMBER_DATE' as unknown as ImplementedFunctions[string]['returnNumberType'];

const VALID_FREQUENCIES = [1, 2, 4];

export class BondFunctionsPlugin extends FunctionPlugin {
  // --- day counts -----------------------------------------------------------

  private date(serial: number): Date3 {
    return this.dateTimeHelper.numberToSimpleDate(Math.trunc(serial));
  }

  private isEndOfFebruary(d: Date3): boolean {
    return d.month === 2 && d.day === this.dateTimeHelper.daysInMonth(d.year, d.month);
  }

  // days360 counts 30/360 days; `european` picks basis 4 over basis 0.
  //
  // The US rules are applied in Excel's documented order. HyperFormula's own
  // toBasisUS() checks "end is the 31st" before it normalises an end-of-February
  // start to the 30th, so its YEARFRAC returns 181 days for 29-Feb-2008 ->
  // 31-Aug-2008 where Excel returns 180. The bond math follows Excel.
  private days360(start: number, end: number, european: boolean): number {
    const s = this.date(start);
    const e = this.date(end);
    if (european) {
      if (s.day === 31) s.day = 30;
      if (e.day === 31) e.day = 30;
    } else {
      if (this.isEndOfFebruary(s) && this.isEndOfFebruary(e)) e.day = 30;
      if (this.isEndOfFebruary(s)) s.day = 30;
      if (e.day === 31 && s.day >= 30) e.day = 30;
      if (s.day === 31) s.day = 30;
    }
    return 360 * (e.year - s.year) + 30 * (e.month - s.month) + e.day - s.day;
  }

  // dayDiff is the basis-aware day count between two dates: 30/360 arithmetic
  // for bases 0 and 4, plain calendar days otherwise.
  private dayDiff(start: number, end: number, basis: number): number {
    if (basis === 0) return this.days360(start, end, false);
    if (basis === 4) return this.days360(start, end, true);
    return Math.trunc(end) - Math.trunc(start);
  }

  // yearFrac matches YEARFRAC(start, end, basis) exactly — same helper, same
  // actual/actual year-length rule.
  private yearFrac(start: number, end: number, basis: number): number {
    let s = Math.trunc(start);
    let e = Math.trunc(end);
    if (s > e) [s, e] = [e, s];
    switch (basis) {
      case 0:
        return this.days360(s, e, false) / 360;
      case 1:
        return (e - s) / this.dateTimeHelper.yearLengthForBasis(this.date(s), this.date(e));
      case 2:
        return (e - s) / 360;
      case 3:
        return (e - s) / 365;
      default:
        return this.days360(s, e, true) / 360;
    }
  }

  // --- coupon schedule ------------------------------------------------------

  // stepMonths shifts the maturity date by whole months, clamping the day to the
  // target month's length (EDATE's rule), so a 31st or Feb-29 maturity keeps its
  // schedule instead of drifting.
  private stepMonths(anchor: Date3, months: number): number {
    const total = anchor.year * 12 + (anchor.month - 1) + months;
    const year = Math.floor(total / 12);
    const month = (total % 12) + 1;
    return this.dateTimeHelper.dateToNumber({
      year,
      month,
      day: Math.min(anchor.day, this.dateTimeHelper.daysInMonth(year, month)),
    });
  }

  // coupons returns the coupon period around the settlement date: the previous
  // coupon date (pcd, on or before settlement), the next one (ncd) and how many
  // coupons are still outstanding (num).
  private coupons(settlement: number, maturity: number, frequency: number): { pcd: number; ncd: number; num: number } {
    const anchor = this.date(maturity);
    const step = 12 / frequency;
    let num = 1;
    // Bounded by ~200 years of coupons; guards against a pathological input
    // turning this into an endless walk.
    while (num < 4000 && this.stepMonths(anchor, -step * num) > Math.trunc(settlement)) num++;
    return { pcd: this.stepMonths(anchor, -step * num), ncd: this.stepMonths(anchor, -step * (num - 1)), num };
  }

  // coupDays is the length of the coupon period containing settlement. Only
  // actual/actual measures the real period; the other bases use a nominal year.
  private coupDays(settlement: number, maturity: number, frequency: number, basis: number): number {
    if (basis === 1) {
      const { pcd, ncd } = this.coupons(settlement, maturity, frequency);
      return ncd - pcd;
    }
    return (basis === 3 ? 365 : 360) / frequency;
  }

  private coupDayBS(settlement: number, maturity: number, frequency: number, basis: number): number {
    const { pcd } = this.coupons(settlement, maturity, frequency);
    return this.dayDiff(pcd, Math.trunc(settlement), basis);
  }

  private coupDaysNC(settlement: number, maturity: number, frequency: number, basis: number): number {
    if (basis === 0 || basis === 4) {
      // 30/360 bases must add up to the nominal period length.
      return this.coupDays(settlement, maturity, frequency, basis) - this.coupDayBS(settlement, maturity, frequency, basis);
    }
    const { ncd } = this.coupons(settlement, maturity, frequency);
    return ncd - Math.trunc(settlement);
  }

  // securityArgs validates what every one of these functions requires.
  private check(settlement: number, maturity: number, frequency?: number): CellError | undefined {
    if (Math.trunc(settlement) >= Math.trunc(maturity)) return numErr('Settlement must be before maturity.');
    if (frequency !== undefined && !VALID_FREQUENCIES.includes(frequency)) return numErr('Frequency must be 1, 2 or 4.');
    return undefined;
  }

  // priceOf is the Excel PRICE formula, also used to solve YIELD.
  private priceOf(
    settlement: number,
    maturity: number,
    rate: number,
    yld: number,
    redemption: number,
    frequency: number,
    basis: number,
  ): number {
    const e = this.coupDays(settlement, maturity, frequency, basis);
    const dsc = this.coupDaysNC(settlement, maturity, frequency, basis);
    const a = this.coupDayBS(settlement, maturity, frequency, basis);
    const n = this.coupons(settlement, maturity, frequency).num;
    const coupon = (100 * rate) / frequency;
    const f = dsc / e;
    let price = redemption / (1 + yld / frequency) ** (n - 1 + f);
    for (let k = 1; k <= n; k++) price += coupon / (1 + yld / frequency) ** (k - 1 + f);
    return price - (coupon * a) / e;
  }

  // --- coupon date functions ------------------------------------------------

  coupdaybs(ast: Val, state: Val): Val {
    return this.runFunction(ast.args, state, this.metadata('COUPDAYBS'), (s: number, m: number, f: number, b: number) => {
      return this.check(s, m, f) ?? this.coupDayBS(s, m, f, b);
    });
  }

  coupdays(ast: Val, state: Val): Val {
    return this.runFunction(ast.args, state, this.metadata('COUPDAYS'), (s: number, m: number, f: number, b: number) => {
      return this.check(s, m, f) ?? this.coupDays(s, m, f, b);
    });
  }

  coupdaysnc(ast: Val, state: Val): Val {
    return this.runFunction(ast.args, state, this.metadata('COUPDAYSNC'), (s: number, m: number, f: number, b: number) => {
      return this.check(s, m, f) ?? this.coupDaysNC(s, m, f, b);
    });
  }

  coupncd(ast: Val, state: Val): Val {
    return this.runFunction(ast.args, state, this.metadata('COUPNCD'), (s: number, m: number, f: number) => {
      return this.check(s, m, f) ?? this.coupons(s, m, f).ncd;
    });
  }

  couppcd(ast: Val, state: Val): Val {
    return this.runFunction(ast.args, state, this.metadata('COUPPCD'), (s: number, m: number, f: number) => {
      return this.check(s, m, f) ?? this.coupons(s, m, f).pcd;
    });
  }

  coupnum(ast: Val, state: Val): Val {
    return this.runFunction(ast.args, state, this.metadata('COUPNUM'), (s: number, m: number, f: number) => {
      return this.check(s, m, f) ?? this.coupons(s, m, f).num;
    });
  }

  // --- coupon bonds ---------------------------------------------------------

  price(ast: Val, state: Val): Val {
    return this.runFunction(
      ast.args,
      state,
      this.metadata('PRICE'),
      (s: number, m: number, rate: number, yld: number, redemption: number, f: number, b: number) =>
        this.check(s, m, f) ?? this.priceOf(s, m, rate, yld, redemption, f, b),
    );
  }

  yield_(ast: Val, state: Val): Val {
    return this.runFunction(
      ast.args,
      state,
      this.metadata('YIELD'),
      (s: number, m: number, rate: number, pr: number, redemption: number, f: number, b: number) => {
        const bad = this.check(s, m, f);
        if (bad) return bad;
        // ponytail: one numeric solve for all cases instead of Excel's separate
        // closed form for the final coupon period — same root, less code.
        const target = (y: number): number => this.priceOf(s, m, rate, y, redemption, f, b) - pr;
        let lo = -0.99;
        let hi = 10;
        if (target(lo) * target(hi) > 0) return numErr('Yield could not be determined.');
        for (let i = 0; i < 200; i++) {
          const mid = (lo + hi) / 2;
          if (target(lo) * target(mid) <= 0) hi = mid;
          else lo = mid;
        }
        return (lo + hi) / 2;
      },
    );
  }

  duration(ast: Val, state: Val): Val {
    return this.runFunction(
      ast.args,
      state,
      this.metadata('DURATION'),
      (s: number, m: number, coupon: number, yld: number, f: number, b: number) =>
        this.check(s, m, f) ?? this.durationOf(s, m, coupon, yld, f, b),
    );
  }

  mduration(ast: Val, state: Val): Val {
    return this.runFunction(
      ast.args,
      state,
      this.metadata('MDURATION'),
      (s: number, m: number, coupon: number, yld: number, f: number, b: number) => {
        const bad = this.check(s, m, f);
        if (bad) return bad;
        return this.durationOf(s, m, coupon, yld, f, b) / (1 + yld / f);
      },
    );
  }

  // durationOf is Macaulay duration: the cash-flow-weighted average time to
  // payment, in years. The first cash flow is DSC/E periods away — the same
  // coupon fraction PRICE discounts with, not a year fraction of the whole term.
  private durationOf(settlement: number, maturity: number, coupon: number, yld: number, frequency: number, basis: number): number {
    const n = this.coupons(settlement, maturity, frequency).num;
    const stub = this.coupDaysNC(settlement, maturity, frequency, basis) / this.coupDays(settlement, maturity, frequency, basis) - 1;
    const pay = (100 * coupon) / frequency;
    const discount = 1 + yld / frequency;
    let weighted = 0;
    let present = 0;
    for (let t = 1; t <= n; t++) {
      const time = t + stub;
      const cash = t === n ? pay + 100 : pay;
      const pv = cash / discount ** time;
      weighted += time * pv;
      present += pv;
    }
    return weighted / present / frequency;
  }

  // --- accrued interest -----------------------------------------------------

  accrint(ast: Val, state: Val): Val {
    return this.runFunction(
      ast.args,
      state,
      this.metadata('ACCRINT'),
      (issue: number, first: number, settlement: number, rate: number, par: number, f: number, b: number, fromIssue: boolean) => {
        if (Math.trunc(issue) >= Math.trunc(settlement)) return numErr('Issue must be before settlement.');
        if (!VALID_FREQUENCIES.includes(f)) return numErr('Frequency must be 1, 2 or 4.');
        // calc_method FALSE accrues from the last coupon date before settlement
        // instead of from issue, but only once settlement passed first_interest.
        const from =
          !fromIssue && Math.trunc(settlement) > Math.trunc(first) ? this.coupons(settlement, first, f).pcd : Math.trunc(issue);
        return par * rate * this.yearFrac(from, settlement, b);
      },
    );
  }

  accrintm(ast: Val, state: Val): Val {
    return this.runFunction(
      ast.args,
      state,
      this.metadata('ACCRINTM'),
      (issue: number, settlement: number, rate: number, par: number, b: number) => {
        if (Math.trunc(issue) >= Math.trunc(settlement)) return numErr('Issue must be before settlement.');
        return par * rate * this.yearFrac(issue, settlement, b);
      },
    );
  }

  // --- discounted securities ------------------------------------------------

  disc(ast: Val, state: Val): Val {
    return this.runFunction(
      ast.args,
      state,
      this.metadata('DISC'),
      (s: number, m: number, pr: number, redemption: number, b: number) =>
        this.check(s, m) ?? ((redemption - pr) / redemption) / this.yearFrac(s, m, b),
    );
  }

  intrate(ast: Val, state: Val): Val {
    return this.runFunction(
      ast.args,
      state,
      this.metadata('INTRATE'),
      (s: number, m: number, investment: number, redemption: number, b: number) =>
        this.check(s, m) ?? ((redemption - investment) / investment) / this.yearFrac(s, m, b),
    );
  }

  received(ast: Val, state: Val): Val {
    return this.runFunction(
      ast.args,
      state,
      this.metadata('RECEIVED'),
      (s: number, m: number, investment: number, discount: number, b: number) => {
        const bad = this.check(s, m);
        if (bad) return bad;
        const factor = 1 - discount * this.yearFrac(s, m, b);
        if (factor === 0) return numErr('Discount too large.');
        return investment / factor;
      },
    );
  }

  pricedisc(ast: Val, state: Val): Val {
    return this.runFunction(
      ast.args,
      state,
      this.metadata('PRICEDISC'),
      (s: number, m: number, discount: number, redemption: number, b: number) =>
        this.check(s, m) ?? redemption - discount * redemption * this.yearFrac(s, m, b),
    );
  }

  yielddisc(ast: Val, state: Val): Val {
    return this.runFunction(
      ast.args,
      state,
      this.metadata('YIELDDISC'),
      (s: number, m: number, pr: number, redemption: number, b: number) =>
        this.check(s, m) ?? ((redemption - pr) / pr) / this.yearFrac(s, m, b),
    );
  }

  // --- securities paying interest at maturity -------------------------------

  pricemat(ast: Val, state: Val): Val {
    return this.runFunction(
      ast.args,
      state,
      this.metadata('PRICEMAT'),
      (s: number, m: number, issue: number, rate: number, yld: number, b: number) => {
        const bad = this.check(s, m);
        if (bad) return bad;
        if (Math.trunc(issue) >= Math.trunc(s)) return numErr('Issue must be before settlement.');
        const dim = this.yearFrac(issue, m, b);
        const a = this.yearFrac(issue, s, b);
        const dsm = this.yearFrac(s, m, b);
        return (100 + dim * rate * 100) / (1 + dsm * yld) - a * rate * 100;
      },
    );
  }

  yieldmat(ast: Val, state: Val): Val {
    return this.runFunction(
      ast.args,
      state,
      this.metadata('YIELDMAT'),
      (s: number, m: number, issue: number, rate: number, pr: number, b: number) => {
        const bad = this.check(s, m);
        if (bad) return bad;
        if (Math.trunc(issue) >= Math.trunc(s)) return numErr('Issue must be before settlement.');
        const dim = this.yearFrac(issue, m, b);
        const a = this.yearFrac(issue, s, b);
        const dsm = this.yearFrac(s, m, b);
        return ((1 + dim * rate) / (pr / 100 + a * rate) - 1) / dsm;
      },
    );
  }
}

const dateArg = { argumentType: T.NUMBER, minValue: 0 } as const;
const rateArg = { argumentType: T.NUMBER, minValue: 0 } as const;
const cashArg = { argumentType: T.NUMBER, greaterThan: 0 } as const;
const basisArg = { argumentType: T.INTEGER, defaultValue: 0, minValue: 0, maxValue: 4 } as const;

const couponDateParams = [dateArg, dateArg, { argumentType: T.INTEGER }, basisArg];

BondFunctionsPlugin.implementedFunctions = {
  COUPDAYBS: { method: 'coupdaybs', parameters: couponDateParams },
  COUPDAYS: { method: 'coupdays', parameters: couponDateParams },
  COUPDAYSNC: { method: 'coupdaysnc', parameters: couponDateParams },
  COUPNUM: { method: 'coupnum', parameters: couponDateParams },
  COUPNCD: { method: 'coupncd', parameters: couponDateParams, returnNumberType: DATE_RESULT },
  COUPPCD: { method: 'couppcd', parameters: couponDateParams, returnNumberType: DATE_RESULT },
  PRICE: {
    method: 'price',
    parameters: [dateArg, dateArg, rateArg, rateArg, cashArg, { argumentType: T.INTEGER }, basisArg],
  },
  YIELD: {
    method: 'yield_',
    parameters: [dateArg, dateArg, rateArg, cashArg, cashArg, { argumentType: T.INTEGER }, basisArg],
  },
  DURATION: {
    method: 'duration',
    parameters: [dateArg, dateArg, rateArg, rateArg, { argumentType: T.INTEGER }, basisArg],
  },
  MDURATION: {
    method: 'mduration',
    parameters: [dateArg, dateArg, rateArg, rateArg, { argumentType: T.INTEGER }, basisArg],
  },
  ACCRINT: {
    method: 'accrint',
    parameters: [dateArg, dateArg, dateArg, rateArg, cashArg, { argumentType: T.INTEGER }, basisArg, { argumentType: T.BOOLEAN, defaultValue: true }],
  },
  ACCRINTM: { method: 'accrintm', parameters: [dateArg, dateArg, rateArg, cashArg, basisArg] },
  DISC: { method: 'disc', parameters: [dateArg, dateArg, cashArg, cashArg, basisArg] },
  INTRATE: { method: 'intrate', parameters: [dateArg, dateArg, cashArg, cashArg, basisArg] },
  RECEIVED: { method: 'received', parameters: [dateArg, dateArg, cashArg, rateArg, basisArg] },
  PRICEDISC: { method: 'pricedisc', parameters: [dateArg, dateArg, rateArg, cashArg, basisArg] },
  YIELDDISC: { method: 'yielddisc', parameters: [dateArg, dateArg, cashArg, cashArg, basisArg] },
  PRICEMAT: { method: 'pricemat', parameters: [dateArg, dateArg, dateArg, rateArg, rateArg, basisArg] },
  YIELDMAT: { method: 'yieldmat', parameters: [dateArg, dateArg, dateArg, rateArg, cashArg, basisArg] },
};

export const bondFunctionNames = Object.keys(BondFunctionsPlugin.implementedFunctions);

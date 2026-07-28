// Excel functions HyperFormula 3.3 does not ship. Registered as one plugin, so
// they behave exactly like built-ins: same argument coercion, same error
// values, and the formula-bar autocomplete picks them up automatically via
// HyperFormula.getRegisteredFunctionNames().
import {
  ArraySize,
  CellError,
  ErrorType,
  FunctionArgumentType as T,
  FunctionPlugin,
  HyperFormula,
  SimpleRangeValue,
} from 'hyperformula';

// HyperFormula's interpreter types (ProcedureAst, InterpreterState,
// InternalScalarValue) are not part of its public API, so plugin methods and
// cell values are typed loosely on purpose.
// eslint-disable-next-line @typescript-eslint/no-explicit-any
type Val = any;

// HyperFormula wraps typed numbers (dates, currency, percentages) in a
// RichNumber with a `val` field; unwrap before doing arithmetic on them.
const raw = (v: Val): Val => (v !== null && typeof v === 'object' && 'val' in v ? v.val : v);

const na = (m = 'Value not available.'): CellError => new CellError(ErrorType.NA, m);
const numErr = (m = 'Value out of range.'): CellError => new CellError(ErrorType.NUM, m);
const valErr = (m = 'Wrong type of argument.'): CellError => new CellError(ErrorType.VALUE, m);
const divZero = (m = 'Division by zero.'): CellError => new CellError(ErrorType.DIV_BY_ZERO, m);

const flat = (r: SimpleRangeValue): Val[] => r.valuesFromTopLeftCorner().map(raw);
const nums = (vs: Val[]): number[] => vs.filter((v) => typeof v === 'number');
const firstError = (vs: Val[]): CellError | undefined => vs.find((v) => v instanceof CellError) as CellError | undefined;

const str = (v: Val): string => {
  const x = raw(v);
  if (typeof x === 'boolean') return x ? 'TRUE' : 'FALSE';
  if (typeof x === 'number' || typeof x === 'string') return String(x);
  return '';
};

// cmp orders two like-typed scalars; null means incomparable (Excel then fails
// the comparison rather than coercing across types).
const cmp = (a: Val, b: Val): number | null => {
  if (typeof a === 'number' && typeof b === 'number') return a - b;
  if (typeof a === 'string' && typeof b === 'string') {
    const x = a.toLowerCase();
    const y = b.toLowerCase();
    return x < y ? -1 : x > y ? 1 : 0;
  }
  if (typeof a === 'boolean' && typeof b === 'boolean') return (a ? 1 : 0) - (b ? 1 : 0);
  return null;
};
const eq = (a: Val, b: Val): boolean => cmp(a, b) === 0;

// wildcardRe builds Excel's '*'/'?' matcher ('~' escapes them).
const wildcardRe = (pattern: string): RegExp => {
  let out = '';
  for (let i = 0; i < pattern.length; i++) {
    const ch = pattern[i];
    if (ch === '~' && (pattern[i + 1] === '*' || pattern[i + 1] === '?' || pattern[i + 1] === '~')) {
      out += pattern[++i].replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
    } else if (ch === '*') out += '.*';
    else if (ch === '?') out += '.';
    else out += ch.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  }
  return new RegExp(`^${out}$`, 'i');
};

// criterion compiles a SUMIF-style criterion ("<5", "<>x", "a*", 42) into a
// predicate. Shared by AVERAGEIFS.
const criterion = (crit: Val): ((v: Val) => boolean) => {
  const c = raw(crit);
  if (typeof c !== 'string') return (v) => eq(raw(v), c);
  const m = /^(<=|>=|<>|<|>|=)?([\s\S]*)$/.exec(c) as RegExpExecArray;
  const op = m[1] ?? '=';
  const rest = m[2];
  const target: Val =
    rest.trim() !== '' && Number.isFinite(Number(rest)) ? Number(rest) : rest === 'TRUE' ? true : rest === 'FALSE' ? false : rest;
  if (op === '=' || op === '<>') {
    const test =
      typeof target === 'string' && /[*?]/.test(target)
        ? (v: Val) => wildcardRe(target).test(str(v))
        : (v: Val) => eq(raw(v), target) || (typeof target === 'string' && str(v).toLowerCase() === target.toLowerCase());
    return op === '=' ? test : (v) => !test(v);
  }
  return (v) => {
    const d = cmp(raw(v), target);
    if (d === null) return false;
    return op === '<' ? d < 0 : op === '<=' ? d <= 0 : op === '>' ? d > 0 : d >= 0;
  };
};

const grouped = (s: string): string => {
  const neg = s.startsWith('-');
  const body = neg ? s.slice(1) : s;
  const [int, frac] = body.split('.');
  const withSep = int.replace(/\B(?=(\d{3})+(?!\d))/g, ',');
  return `${neg ? '-' : ''}${withSep}${frac === undefined ? '' : `.${frac}`}`;
};

// fixedText is FIXED()'s formatting, reused by DOLLAR(). Negative `decimals`
// rounds to the left of the decimal point, like Excel.
const fixedText = (n: number, decimals: number, commas: boolean): string => {
  const d = Math.trunc(decimals);
  const rounded = d < 0 ? Math.round(n / 10 ** -d) * 10 ** -d : n;
  const s = rounded.toFixed(Math.max(0, d));
  return commas ? grouped(s) : s;
};

const linreg = (ys: number[], xs: number[]): { slope: number; intercept: number } | CellError => {
  const n = Math.min(ys.length, xs.length);
  if (n === 0) return na();
  let sx = 0;
  let sy = 0;
  for (let i = 0; i < n; i++) {
    sx += xs[i];
    sy += ys[i];
  }
  const mx = sx / n;
  const my = sy / n;
  let num = 0;
  let den = 0;
  for (let i = 0; i < n; i++) {
    num += (xs[i] - mx) * (ys[i] - my);
    den += (xs[i] - mx) ** 2;
  }
  if (den === 0) return divZero();
  const slope = num / den;
  return { slope, intercept: my - slope * mx };
};

// numericPairs pulls same-length numeric pairs out of two ranges, dropping
// positions where either side is non-numeric (Excel's behaviour for the
// regression family).
const numericPairs = (a: SimpleRangeValue, b: SimpleRangeValue): [number[], number[]] | CellError => {
  const va = flat(a);
  const vb = flat(b);
  if (va.length !== vb.length) return na('Ranges must be of equal length.');
  const err = firstError(va) ?? firstError(vb);
  if (err) return err;
  const xs: number[] = [];
  const ys: number[] = [];
  for (let i = 0; i < va.length; i++) {
    if (typeof va[i] === 'number' && typeof vb[i] === 'number') {
      ys.push(va[i]);
      xs.push(vb[i]);
    }
  }
  return [ys, xs];
};

const rows = (r: SimpleRangeValue): Val[][] => r.data.map((row) => row.slice());
const NA_PAD = (): CellError => na();

const pad = (data: Val[][], width: number, filler: () => Val): Val[][] =>
  data.map((row) => (row.length >= width ? row : row.concat(Array.from({ length: width - row.length }, filler))));

// takeRange implements TAKE/DROP index math: negative counts come from the end.
const slice1 = (len: number, count: number | undefined, drop: boolean): [number, number] => {
  if (count === undefined) return [0, len];
  const c = Math.trunc(count);
  if (drop) return c >= 0 ? [Math.min(c, len), len] : [0, Math.max(0, len + c)];
  return c >= 0 ? [0, Math.min(c, len)] : [Math.max(0, len + c), len];
};

const idx1 = (len: number, n: number): number | null => {
  const i = Math.trunc(n);
  if (i === 0) return null;
  const zero = i > 0 ? i - 1 : len + i;
  return zero < 0 || zero >= len ? null : zero;
};

const toArray = (data: Val[][]): SimpleRangeValue | CellError =>
  data.length === 0 || data[0].length === 0 ? na('Empty result.') : SimpleRangeValue.onlyValues(data);

export class ExcelExtrasPlugin extends FunctionPlugin {
  // --- text -----------------------------------------------------------------

  concat(ast: Val, state: Val): Val {
    return this.runFunction(ast.args, state, this.metadata('CONCAT'), (...args: Val[]) => {
      let out = '';
      for (const a of args) {
        if (a instanceof CellError) return a;
        if (a instanceof SimpleRangeValue) {
          for (const v of flat(a)) {
            if (v instanceof CellError) return v;
            out += str(v);
          }
        } else out += str(a);
      }
      return out;
    });
  }

  private static split(text: string, delim: string, instance: number, ignoreCase: boolean): number | null {
    if (delim === '') return null;
    const hay = ignoreCase ? text.toLowerCase() : text;
    const needle = ignoreCase ? delim.toLowerCase() : delim;
    const hits: number[] = [];
    for (let i = hay.indexOf(needle); i !== -1; i = hay.indexOf(needle, i + needle.length)) hits.push(i);
    const n = Math.trunc(instance);
    if (n === 0) return null;
    const hit = n > 0 ? hits[n - 1] : hits[hits.length + n];
    return hit === undefined ? null : hit;
  }

  textbefore(ast: Val, state: Val): Val {
    return this.runFunction(
      ast.args,
      state,
      this.metadata('TEXTBEFORE'),
      (text: string, delim: string, instance: number, matchMode: number, matchEnd: number, ifNotFound: Val) => {
        if (delim === '') return '';
        if (instance === 0) return valErr('Instance number must not be 0.');
        const at = ExcelExtrasPlugin.split(text, delim, instance, matchMode !== 0);
        if (at === null) return matchEnd !== 0 ? text : ifNotFound === undefined ? na() : ifNotFound;
        return text.slice(0, at);
      },
    );
  }

  textafter(ast: Val, state: Val): Val {
    return this.runFunction(
      ast.args,
      state,
      this.metadata('TEXTAFTER'),
      (text: string, delim: string, instance: number, matchMode: number, matchEnd: number, ifNotFound: Val) => {
        if (delim === '') return text;
        if (instance === 0) return valErr('Instance number must not be 0.');
        const at = ExcelExtrasPlugin.split(text, delim, instance, matchMode !== 0);
        if (at === null) return matchEnd !== 0 ? '' : ifNotFound === undefined ? na() : ifNotFound;
        return text.slice(at + delim.length);
      },
    );
  }

  numbervalue(ast: Val, state: Val): Val {
    return this.runFunction(ast.args, state, this.metadata('NUMBERVALUE'), (text: string, dec: string, group: string) => {
      if (dec === '' || group === '') return valErr('Separator must not be empty.');
      const d = dec[0];
      const g = group[0];
      if (d === g) return valErr('Separators must differ.');
      let s = text.split(g).join('').trim();
      if (s === '') return 0;
      let percents = 0;
      while (s.endsWith('%')) {
        percents++;
        s = s.slice(0, -1);
      }
      s = s.split(d).join('.');
      if (!/^[+-]?(\d+(\.\d*)?|\.\d+)([eE][+-]?\d+)?$/.test(s)) return valErr('Not a number.');
      return Number(s) / 100 ** percents;
    });
  }

  fixed(ast: Val, state: Val): Val {
    return this.runFunction(ast.args, state, this.metadata('FIXED'), (n: number, decimals: number, noCommas: boolean) =>
      fixedText(n, decimals, !noCommas),
    );
  }

  dollar(ast: Val, state: Val): Val {
    return this.runFunction(ast.args, state, this.metadata('DOLLAR'), (n: number, decimals: number) => {
      const body = fixedText(Math.abs(n), decimals, true);
      return n < 0 ? `($${body})` : `$${body}`;
    });
  }

  // --- lookup ---------------------------------------------------------------

  xmatch(ast: Val, state: Val): Val {
    return this.runFunction(
      ast.args,
      state,
      this.metadata('XMATCH'),
      (needle: Val, range: SimpleRangeValue, matchMode: number, searchMode: number) => {
        const key = raw(needle);
        if (key instanceof CellError) return key;
        const values = flat(range);
        const order = searchMode < 0 ? [...values.keys()].reverse() : [...values.keys()];
        if (matchMode === 2) {
          const re = wildcardRe(str(key));
          for (const i of order) if (typeof values[i] === 'string' && re.test(values[i])) return i + 1;
          return na();
        }
        for (const i of order) if (eq(values[i], key)) return i + 1;
        if (matchMode === 0) return na();
        // ponytail: -1/1 scan linearly for the closest value instead of assuming
        // sorted input; binary search modes (±2) fall back to the same scan.
        let best = -1;
        for (const i of order) {
          const d = cmp(values[i], key);
          if (d === null) continue;
          if (matchMode === -1 ? d > 0 : d < 0) continue;
          if (best === -1 || (matchMode === -1 ? cmp(values[i], values[best])! > 0 : cmp(values[i], values[best])! < 0)) best = i;
        }
        return best === -1 ? na() : best + 1;
      },
    );
  }

  lookup(ast: Val, state: Val): Val {
    return this.runFunction(
      ast.args,
      state,
      this.metadata('LOOKUP'),
      (needle: Val, range: SimpleRangeValue, result: SimpleRangeValue | undefined) => {
        const key = raw(needle);
        if (key instanceof CellError) return key;
        let search: Val[];
        let out: Val[];
        if (result !== undefined) {
          search = flat(range);
          out = flat(result);
        } else if (range.width() > range.height()) {
          // Array form: wider than tall searches the first row, returns the last.
          search = range.data[0].map(raw);
          out = range.data[range.height() - 1].map(raw);
        } else {
          search = range.data.map((r) => raw(r[0]));
          out = range.data.map((r) => raw(r[range.width() - 1]));
        }
        let best = -1;
        for (let i = 0; i < search.length; i++) {
          const d = cmp(search[i], key);
          if (d === null || d > 0) continue;
          if (best === -1 || cmp(search[i], search[best])! >= 0) best = i;
        }
        if (best === -1 || out[best] === undefined) return na();
        return out[best];
      },
    );
  }

  // --- dynamic arrays -------------------------------------------------------

  unique(ast: Val, state: Val): Val {
    return this.runFunction(
      ast.args,
      state,
      this.metadata('UNIQUE'),
      (range: SimpleRangeValue, byCol: boolean, exactlyOnce: boolean) => {
        const data = byCol ? transpose(rows(range)) : rows(range);
        const counts = new Map<string, number>();
        const keys = data.map((row) => row.map((v) => `${typeof raw(v)}:${str(v).toLowerCase()}`).join(' '));
        for (const k of keys) counts.set(k, (counts.get(k) ?? 0) + 1);
        const seen = new Set<string>();
        const kept: Val[][] = [];
        for (let i = 0; i < data.length; i++) {
          if (exactlyOnce ? counts.get(keys[i]) !== 1 : seen.has(keys[i])) continue;
          seen.add(keys[i]);
          kept.push(data[i]);
        }
        return toArray(byCol ? transpose(kept) : kept);
      },
    );
  }

  sort(ast: Val, state: Val): Val {
    return this.runFunction(
      ast.args,
      state,
      this.metadata('SORT'),
      (range: SimpleRangeValue, index: number, order: number, byCol: boolean) => {
        const data = byCol ? transpose(rows(range)) : rows(range);
        const i = Math.trunc(index) - 1;
        if (i < 0 || i >= (data[0]?.length ?? 0)) return valErr('Sort index out of range.');
        const dir = order < 0 ? -1 : 1;
        const sorted = data
          .map((row, pos) => ({ row, pos }))
          .sort((a, b) => (cmp(raw(a.row[i]), raw(b.row[i])) ?? typeRank(a.row[i]) - typeRank(b.row[i])) * dir || a.pos - b.pos)
          .map((e) => e.row);
        return toArray(byCol ? transpose(sorted) : sorted);
      },
    );
  }

  sortby(ast: Val, state: Val): Val {
    return this.runFunction(ast.args, state, this.metadata('SORTBY'), (range: SimpleRangeValue, ...rest: Val[]) => {
      const data = rows(range);
      const keys: Array<{ vals: Val[]; dir: number }> = [];
      for (let i = 0; i < rest.length; i += 2) {
        const by = rest[i];
        if (!(by instanceof SimpleRangeValue)) return valErr('SORTBY expects ranges to sort by.');
        const vals = flat(by);
        if (vals.length !== data.length) return valErr('Sort-by range must match the array height.');
        keys.push({ vals, dir: (raw(rest[i + 1]) ?? 1) < 0 ? -1 : 1 });
      }
      if (keys.length === 0) return valErr('SORTBY needs at least one range to sort by.');
      const sorted = data
        .map((row, pos) => ({ row, pos }))
        .sort((a, b) => {
          for (const k of keys) {
            const d = (cmp(k.vals[a.pos], k.vals[b.pos]) ?? typeRank(k.vals[a.pos]) - typeRank(k.vals[b.pos])) * k.dir;
            if (d !== 0) return d;
          }
          return a.pos - b.pos;
        })
        .map((e) => e.row);
      return toArray(sorted);
    });
  }

  take(ast: Val, state: Val): Val {
    return this.runFunction(ast.args, state, this.metadata('TAKE'), (range: SimpleRangeValue, r: Val, c: Val) =>
      cut(rows(range), raw(r), raw(c), false),
    );
  }

  drop(ast: Val, state: Val): Val {
    return this.runFunction(ast.args, state, this.metadata('DROP'), (range: SimpleRangeValue, r: Val, c: Val) =>
      cut(rows(range), raw(r), raw(c), true),
    );
  }

  vstack(ast: Val, state: Val): Val {
    return this.runFunction(ast.args, state, this.metadata('VSTACK'), (...args: Val[]) => {
      const parts = args.map((a) => (a instanceof SimpleRangeValue ? rows(a) : [[raw(a)]]));
      const width = Math.max(...parts.map((p) => Math.max(...p.map((r) => r.length))));
      return toArray(parts.flatMap((p) => pad(p, width, NA_PAD)));
    });
  }

  hstack(ast: Val, state: Val): Val {
    return this.runFunction(ast.args, state, this.metadata('HSTACK'), (...args: Val[]) => {
      const parts = args.map((a) => (a instanceof SimpleRangeValue ? rows(a) : [[raw(a)]]));
      const height = Math.max(...parts.map((p) => p.length));
      const out: Val[][] = Array.from({ length: height }, () => [] as Val[]);
      for (const p of parts) {
        const width = Math.max(...p.map((r) => r.length));
        for (let i = 0; i < height; i++) out[i].push(...pad([p[i] ?? []], width, NA_PAD)[0]);
      }
      return toArray(out);
    });
  }

  tocol(ast: Val, state: Val): Val {
    return this.runFunction(ast.args, state, this.metadata('TOCOL'), (range: SimpleRangeValue, ignore: number, byCol: boolean) =>
      toArray(linearize(rows(range), ignore, byCol).map((v) => [v])),
    );
  }

  torow(ast: Val, state: Val): Val {
    return this.runFunction(ast.args, state, this.metadata('TOROW'), (range: SimpleRangeValue, ignore: number, byCol: boolean) =>
      toArray([linearize(rows(range), ignore, byCol)]),
    );
  }

  choosecols(ast: Val, state: Val): Val {
    return this.runFunction(ast.args, state, this.metadata('CHOOSECOLS'), (range: SimpleRangeValue, ...cols: Val[]) => {
      const data = rows(range);
      const picked = cols.map((c) => idx1(range.width(), raw(c)));
      if (picked.some((p) => p === null)) return valErr('Column index out of range.');
      return toArray(data.map((row) => picked.map((p) => row[p as number])));
    });
  }

  chooserows(ast: Val, state: Val): Val {
    return this.runFunction(ast.args, state, this.metadata('CHOOSEROWS'), (range: SimpleRangeValue, ...rs: Val[]) => {
      const data = rows(range);
      const picked = rs.map((r) => idx1(range.height(), raw(r)));
      if (picked.some((p) => p === null)) return valErr('Row index out of range.');
      return toArray(picked.map((p) => data[p as number]));
    });
  }

  expand(ast: Val, state: Val): Val {
    return this.runFunction(ast.args, state, this.metadata('EXPAND'), (range: SimpleRangeValue, r: Val, c: Val, padWith: Val) => {
      const data = rows(range);
      const height = raw(r) === undefined ? data.length : Math.trunc(raw(r));
      const width = raw(c) === undefined ? range.width() : Math.trunc(raw(c));
      if (height < data.length || width < range.width()) return valErr('Cannot shrink with EXPAND.');
      const filler = (): Val => (padWith === undefined ? na() : raw(padWith));
      const out = pad(data, width, filler);
      while (out.length < height) out.push(Array.from({ length: width }, filler));
      return toArray(out);
    });
  }

  // --- statistics -----------------------------------------------------------

  averageifs(ast: Val, state: Val): Val {
    return this.runFunction(ast.args, state, this.metadata('AVERAGEIFS'), (avgRange: SimpleRangeValue, ...rest: Val[]) => {
      const values = flat(avgRange);
      const conditions: Array<{ vals: Val[]; test: (v: Val) => boolean }> = [];
      for (let i = 0; i < rest.length; i += 2) {
        const range = rest[i];
        if (!(range instanceof SimpleRangeValue)) return valErr('AVERAGEIFS expects criteria ranges.');
        const vals = flat(range);
        if (vals.length !== values.length) return valErr('Criteria ranges must match the average range.');
        conditions.push({ vals, test: criterion(rest[i + 1]) });
      }
      if (conditions.length === 0) return valErr('AVERAGEIFS needs at least one criterion.');
      let sum = 0;
      let count = 0;
      for (let i = 0; i < values.length; i++) {
        if (values[i] instanceof CellError) return values[i];
        if (typeof values[i] !== 'number') continue;
        if (!conditions.every((c) => c.test(c.vals[i]))) continue;
        sum += values[i];
        count++;
      }
      return count === 0 ? divZero('No cell matched the criteria.') : sum / count;
    });
  }

  rank(ast: Val, state: Val): Val {
    return this.runFunction(ast.args, state, this.metadata('RANK'), (n: number, range: SimpleRangeValue, order: number) =>
      rankOf(n, range, order, false),
    );
  }

  rankavg(ast: Val, state: Val): Val {
    return this.runFunction(ast.args, state, this.metadata('RANK.AVG'), (n: number, range: SimpleRangeValue, order: number) =>
      rankOf(n, range, order, true),
    );
  }

  mode(ast: Val, state: Val): Val {
    return this.runFunction(ast.args, state, this.metadata('MODE'), (...args: Val[]) => {
      const modes = modesOf(args);
      return modes instanceof CellError ? modes : modes[0];
    });
  }

  modemult(ast: Val, state: Val): Val {
    return this.runFunction(ast.args, state, this.metadata('MODE.MULT'), (...args: Val[]) => {
      const modes = modesOf(args);
      return modes instanceof CellError ? modes : toArray(modes.map((m) => [m]));
    });
  }

  trimmean(ast: Val, state: Val): Val {
    return this.runFunction(ast.args, state, this.metadata('TRIMMEAN'), (range: SimpleRangeValue, percent: number) => {
      const values = flat(range);
      const err = firstError(values);
      if (err) return err;
      const sorted = nums(values).sort((a, b) => a - b);
      if (sorted.length === 0) return divZero('Empty range.');
      // Excel trims FLOOR(n*percent, 2)/2 values from each end.
      const k = Math.floor((sorted.length * percent) / 2);
      const kept = sorted.slice(k, sorted.length - k);
      if (kept.length === 0) return divZero('Everything trimmed.');
      return kept.reduce((a, b) => a + b, 0) / kept.length;
    });
  }

  permut(ast: Val, state: Val): Val {
    return this.runFunction(ast.args, state, this.metadata('PERMUT'), (n: number, k: number) => {
      if (k > n) return numErr('Chosen must not exceed total.');
      let out = 1;
      for (let i = 0; i < k; i++) out *= n - i;
      return out;
    });
  }

  permutationa(ast: Val, state: Val): Val {
    return this.runFunction(ast.args, state, this.metadata('PERMUTATIONA'), (n: number, k: number) => n ** k);
  }

  intercept(ast: Val, state: Val): Val {
    return this.runFunction(ast.args, state, this.metadata('INTERCEPT'), (ys: SimpleRangeValue, xs: SimpleRangeValue) => {
      const pairs = numericPairs(ys, xs);
      if (pairs instanceof CellError) return pairs;
      const fit = linreg(pairs[0], pairs[1]);
      return fit instanceof CellError ? fit : fit.intercept;
    });
  }

  forecast(ast: Val, state: Val): Val {
    return this.runFunction(ast.args, state, this.metadata('FORECAST'), (x: number, ys: SimpleRangeValue, xs: SimpleRangeValue) => {
      const pairs = numericPairs(ys, xs);
      if (pairs instanceof CellError) return pairs;
      const fit = linreg(pairs[0], pairs[1]);
      return fit instanceof CellError ? fit : fit.intercept + fit.slope * x;
    });
  }

  frequency(ast: Val, state: Val): Val {
    return this.runFunction(ast.args, state, this.metadata('FREQUENCY'), (data: SimpleRangeValue, bins: SimpleRangeValue) => {
      const values = flat(data);
      const binValues = flat(bins);
      const err = firstError(values) ?? firstError(binValues);
      if (err) return err;
      const edges = nums(binValues).sort((a, b) => a - b);
      const counts = new Array<number>(edges.length + 1).fill(0);
      for (const v of nums(values)) {
        const at = edges.findIndex((e) => v <= e);
        counts[at === -1 ? edges.length : at]++;
      }
      return toArray(counts.map((c) => [c]));
    });
  }

  // --- information ----------------------------------------------------------

  errortype(ast: Val, state: Val): Val {
    return this.runFunction(ast.args, state, this.metadata('ERROR.TYPE'), (v: Val) => {
      if (!(v instanceof CellError)) return na();
      switch (v.type) {
        case ErrorType.DIV_BY_ZERO:
          return 2;
        case ErrorType.VALUE:
          return 3;
        case ErrorType.REF:
          return 4;
        case ErrorType.NAME:
          return 5;
        case ErrorType.NUM:
          return 6;
        case ErrorType.NA:
          return 7;
        default:
          return 8;
      }
    });
  }

  type(ast: Val, state: Val): Val {
    return this.runFunction(ast.args, state, this.metadata('TYPE'), (v: Val) => {
      if (v instanceof CellError) return 16;
      if (v instanceof SimpleRangeValue) return 64;
      const x = raw(v);
      if (typeof x === 'number') return 1;
      if (typeof x === 'boolean') return 4;
      return 2;
    });
  }

  // --- financial ------------------------------------------------------------

  xirr(ast: Val, state: Val): Val {
    return this.runFunction(
      ast.args,
      state,
      this.metadata('XIRR'),
      (values: SimpleRangeValue, dates: SimpleRangeValue, guess: number) => {
        const vs = flat(values);
        const ds = flat(dates);
        const err = firstError(vs) ?? firstError(ds);
        if (err) return err;
        const cash = nums(vs);
        const when = nums(ds);
        if (cash.length !== when.length || cash.length < 2) return numErr('XIRR needs matching values and dates.');
        if (!cash.some((v) => v > 0) || !cash.some((v) => v < 0)) return numErr('XIRR needs a positive and a negative value.');
        const t0 = when[0];
        const npv = (rate: number): number =>
          cash.reduce((acc, v, i) => acc + v / (1 + rate) ** ((when[i] - t0) / 365), 0);
        // Newton first (fast, matches Excel), bisection as the safety net.
        let rate = guess;
        for (let i = 0; i < 50; i++) {
          const f = npv(rate);
          if (Math.abs(f) < 1e-9) return rate;
          const d = (npv(rate + 1e-6) - f) / 1e-6;
          if (!Number.isFinite(d) || d === 0) break;
          const next = rate - f / d;
          if (!Number.isFinite(next) || next <= -1) break;
          rate = next;
        }
        let lo = -0.999999;
        let hi = 1e6;
        if (npv(lo) * npv(hi) > 0) return numErr('XIRR did not converge.');
        for (let i = 0; i < 200; i++) {
          const mid = (lo + hi) / 2;
          if (npv(lo) * npv(mid) <= 0) hi = mid;
          else lo = mid;
        }
        return (lo + hi) / 2;
      },
    );
  }

  // --- array-size predictions ----------------------------------------------
  // HyperFormula reserves the spill range before evaluating, so every
  // array-returning function needs a size hint. A hint that is too large is
  // harmless (extra cells stay empty, like built-in FILTER); too small would
  // truncate, so each hint below is an upper bound.

  private argSizes(ast: Val, state: Val): ArraySize[] {
    return ast.args.map((arg: Val) => this.arraySizeForAst(arg, state));
  }

  inputSize(ast: Val, state: Val): ArraySize {
    const sizes = this.argSizes(ast, state);
    if (sizes.length === 0) return ArraySize.error();
    return new ArraySize(Math.max(...sizes.map((s) => s.width)), Math.max(...sizes.map((s) => s.height)));
  }

  colSize(ast: Val, state: Val): ArraySize {
    const first = this.argSizes(ast, state)[0];
    return first === undefined ? ArraySize.error() : new ArraySize(1, first.width * first.height);
  }

  rowSize(ast: Val, state: Val): ArraySize {
    const first = this.argSizes(ast, state)[0];
    return first === undefined ? ArraySize.error() : new ArraySize(first.width * first.height, 1);
  }

  vstackSize(ast: Val, state: Val): ArraySize {
    const sizes = this.argSizes(ast, state);
    if (sizes.length === 0) return ArraySize.error();
    return new ArraySize(
      Math.max(...sizes.map((s) => s.width)),
      sizes.reduce((a, s) => a + s.height, 0),
    );
  }

  hstackSize(ast: Val, state: Val): ArraySize {
    const sizes = this.argSizes(ast, state);
    if (sizes.length === 0) return ArraySize.error();
    return new ArraySize(
      sizes.reduce((a, s) => a + s.width, 0),
      Math.max(...sizes.map((s) => s.height)),
    );
  }

  chooseColsSize(ast: Val, state: Val): ArraySize {
    const sizes = this.argSizes(ast, state);
    if (sizes.length < 2) return ArraySize.error();
    return new ArraySize(Math.max(sizes[0].width, sizes.length - 1), sizes[0].height);
  }

  chooseRowsSize(ast: Val, state: Val): ArraySize {
    const sizes = this.argSizes(ast, state);
    if (sizes.length < 2) return ArraySize.error();
    return new ArraySize(sizes[0].width, Math.max(sizes[0].height, sizes.length - 1));
  }

  // ponytail: EXPAND only knows its target size when rows/cols are literals;
  // with computed sizes the hint stays at the input size (result then spills
  // only as far as the input). Literal args cover the realistic usage.
  expandSize(ast: Val, state: Val): ArraySize {
    const sizes = this.argSizes(ast, state);
    if (sizes.length < 2) return ArraySize.error();
    const literal = (i: number): number | undefined =>
      ast.args[i] !== undefined && ast.args[i].type === 'NUMBER' ? ast.args[i].value : undefined;
    return new ArraySize(literal(2) ?? sizes[0].width, literal(1) ?? sizes[0].height);
  }

  frequencySize(ast: Val, state: Val): ArraySize {
    const sizes = this.argSizes(ast, state);
    if (sizes.length !== 2) return ArraySize.error();
    return new ArraySize(1, sizes[1].width * sizes[1].height + 1);
  }
}

const transpose = (data: Val[][]): Val[][] => {
  const width = Math.max(0, ...data.map((r) => r.length));
  return Array.from({ length: width }, (_, c) => data.map((r) => r[c]));
};

// typeRank orders across types the way Excel sorts: numbers < text < booleans.
const typeRank = (v: Val): number => {
  const x = raw(v);
  if (typeof x === 'number') return 0;
  if (typeof x === 'string') return 1;
  if (typeof x === 'boolean') return 2;
  return 3;
};

const cut = (data: Val[][], r: Val, c: Val, drop: boolean): SimpleRangeValue | CellError => {
  const [r0, r1] = slice1(data.length, r, drop);
  const width = Math.max(0, ...data.map((row) => row.length));
  const [c0, c1] = slice1(width, c, drop);
  return toArray(data.slice(r0, r1).map((row) => row.slice(c0, c1)));
};

const linearize = (data: Val[][], ignore: number, byCol: boolean): Val[] => {
  const flatVals = (byCol ? transpose(data) : data).flat();
  const skipBlank = ignore === 1 || ignore === 3;
  const skipError = ignore === 2 || ignore === 3;
  return flatVals.filter((v) => {
    const x = raw(v);
    if (skipError && x instanceof CellError) return false;
    if (skipBlank && (x === null || x === undefined || x === '' || typeof x === 'symbol')) return false;
    return true;
  });
};

const rankOf = (n: number, range: SimpleRangeValue, order: number, average: boolean): number | CellError => {
  const values = flat(range);
  const err = firstError(values);
  if (err) return err;
  const pool = nums(values);
  if (!pool.some((v) => v === n)) return na('Value not found in the range.');
  const better = pool.filter((v) => (order === 0 ? v > n : v < n)).length;
  const ties = pool.filter((v) => v === n).length;
  return better + 1 + (average ? (ties - 1) / 2 : 0);
};

const modesOf = (args: Val[]): Val[] | CellError => {
  const values: Val[] = [];
  for (const a of args) {
    if (a instanceof CellError) return a;
    if (a instanceof SimpleRangeValue) {
      const inner = flat(a);
      const err = firstError(inner);
      if (err) return err;
      values.push(...nums(inner));
    } else if (typeof raw(a) === 'number') values.push(raw(a));
  }
  if (values.length === 0) return na('No numeric values.');
  const counts = new Map<number, number>();
  for (const v of values) counts.set(v, (counts.get(v) ?? 0) + 1);
  const top = Math.max(...counts.values());
  if (top < 2) return na('No repeated value.');
  const seen = new Set<number>();
  const out: Val[] = [];
  for (const v of values) {
    if (counts.get(v) === top && !seen.has(v)) {
      seen.add(v);
      out.push(v);
    }
  }
  return out;
};

const optional = { optionalArg: true } as const;
const anyArg = { argumentType: T.ANY } as const;
const rangeArg = { argumentType: T.RANGE } as const;

ExcelExtrasPlugin.implementedFunctions = {
  CONCAT: { method: 'concat', parameters: [anyArg], repeatLastArgs: 1 },
  TEXTBEFORE: {
    method: 'textbefore',
    parameters: [
      { argumentType: T.STRING },
      { argumentType: T.STRING },
      { argumentType: T.NUMBER, defaultValue: 1 },
      { argumentType: T.NUMBER, defaultValue: 0 },
      { argumentType: T.NUMBER, defaultValue: 0 },
      { argumentType: T.SCALAR, ...optional },
    ],
  },
  TEXTAFTER: {
    method: 'textafter',
    parameters: [
      { argumentType: T.STRING },
      { argumentType: T.STRING },
      { argumentType: T.NUMBER, defaultValue: 1 },
      { argumentType: T.NUMBER, defaultValue: 0 },
      { argumentType: T.NUMBER, defaultValue: 0 },
      { argumentType: T.SCALAR, ...optional },
    ],
  },
  NUMBERVALUE: {
    method: 'numbervalue',
    parameters: [
      { argumentType: T.STRING },
      { argumentType: T.STRING, defaultValue: '.' },
      { argumentType: T.STRING, defaultValue: ',' },
    ],
  },
  FIXED: {
    method: 'fixed',
    parameters: [
      { argumentType: T.NUMBER },
      { argumentType: T.NUMBER, defaultValue: 2 },
      { argumentType: T.BOOLEAN, defaultValue: false },
    ],
  },
  DOLLAR: {
    method: 'dollar',
    parameters: [{ argumentType: T.NUMBER }, { argumentType: T.NUMBER, defaultValue: 2 }],
  },
  XMATCH: {
    method: 'xmatch',
    parameters: [
      { argumentType: T.SCALAR },
      rangeArg,
      { argumentType: T.NUMBER, defaultValue: 0 },
      { argumentType: T.NUMBER, defaultValue: 1 },
    ],
  },
  LOOKUP: {
    method: 'lookup',
    parameters: [{ argumentType: T.SCALAR }, rangeArg, { argumentType: T.RANGE, ...optional }],
  },
  UNIQUE: {
    method: 'unique',
    parameters: [rangeArg, { argumentType: T.BOOLEAN, defaultValue: false }, { argumentType: T.BOOLEAN, defaultValue: false }],
    sizeOfResultArrayMethod: 'inputSize',
    vectorizationForbidden: true,
  },
  SORT: {
    method: 'sort',
    parameters: [
      rangeArg,
      { argumentType: T.NUMBER, defaultValue: 1 },
      { argumentType: T.NUMBER, defaultValue: 1 },
      { argumentType: T.BOOLEAN, defaultValue: false },
    ],
    sizeOfResultArrayMethod: 'inputSize',
    vectorizationForbidden: true,
  },
  SORTBY: {
    method: 'sortby',
    parameters: [rangeArg, anyArg, { argumentType: T.SCALAR, ...optional }],
    repeatLastArgs: 2,
    sizeOfResultArrayMethod: 'inputSize',
    vectorizationForbidden: true,
  },
  TAKE: {
    method: 'take',
    parameters: [rangeArg, { argumentType: T.NUMBER, ...optional }, { argumentType: T.NUMBER, ...optional }],
    sizeOfResultArrayMethod: 'inputSize',
    vectorizationForbidden: true,
  },
  DROP: {
    method: 'drop',
    parameters: [rangeArg, { argumentType: T.NUMBER, ...optional }, { argumentType: T.NUMBER, ...optional }],
    sizeOfResultArrayMethod: 'inputSize',
    vectorizationForbidden: true,
  },
  VSTACK: {
    method: 'vstack',
    parameters: [anyArg],
    repeatLastArgs: 1,
    sizeOfResultArrayMethod: 'vstackSize',
    vectorizationForbidden: true,
  },
  HSTACK: {
    method: 'hstack',
    parameters: [anyArg],
    repeatLastArgs: 1,
    sizeOfResultArrayMethod: 'hstackSize',
    vectorizationForbidden: true,
  },
  TOCOL: {
    method: 'tocol',
    parameters: [rangeArg, { argumentType: T.NUMBER, defaultValue: 0 }, { argumentType: T.BOOLEAN, defaultValue: false }],
    sizeOfResultArrayMethod: 'colSize',
    vectorizationForbidden: true,
  },
  TOROW: {
    method: 'torow',
    parameters: [rangeArg, { argumentType: T.NUMBER, defaultValue: 0 }, { argumentType: T.BOOLEAN, defaultValue: false }],
    sizeOfResultArrayMethod: 'rowSize',
    vectorizationForbidden: true,
  },
  CHOOSECOLS: {
    method: 'choosecols',
    parameters: [rangeArg, { argumentType: T.NUMBER }],
    repeatLastArgs: 1,
    sizeOfResultArrayMethod: 'chooseColsSize',
    vectorizationForbidden: true,
  },
  CHOOSEROWS: {
    method: 'chooserows',
    parameters: [rangeArg, { argumentType: T.NUMBER }],
    repeatLastArgs: 1,
    sizeOfResultArrayMethod: 'chooseRowsSize',
    vectorizationForbidden: true,
  },
  EXPAND: {
    method: 'expand',
    parameters: [
      rangeArg,
      { argumentType: T.NUMBER, ...optional },
      { argumentType: T.NUMBER, ...optional },
      { argumentType: T.SCALAR, ...optional },
    ],
    sizeOfResultArrayMethod: 'expandSize',
    vectorizationForbidden: true,
  },
  AVERAGEIFS: {
    method: 'averageifs',
    parameters: [rangeArg, rangeArg, { argumentType: T.NOERROR }],
    repeatLastArgs: 2,
  },
  RANK: {
    method: 'rank',
    parameters: [{ argumentType: T.NUMBER }, rangeArg, { argumentType: T.NUMBER, defaultValue: 0 }],
  },
  'RANK.AVG': {
    method: 'rankavg',
    parameters: [{ argumentType: T.NUMBER }, rangeArg, { argumentType: T.NUMBER, defaultValue: 0 }],
  },
  MODE: { method: 'mode', parameters: [anyArg], repeatLastArgs: 1 },
  'MODE.MULT': {
    method: 'modemult',
    parameters: [anyArg],
    repeatLastArgs: 1,
    sizeOfResultArrayMethod: 'colSize',
    vectorizationForbidden: true,
  },
  TRIMMEAN: {
    method: 'trimmean',
    parameters: [rangeArg, { argumentType: T.NUMBER, minValue: 0, lessThan: 1 }],
  },
  PERMUT: {
    method: 'permut',
    parameters: [
      { argumentType: T.INTEGER, minValue: 0 },
      { argumentType: T.INTEGER, minValue: 0 },
    ],
  },
  PERMUTATIONA: {
    method: 'permutationa',
    parameters: [
      { argumentType: T.INTEGER, minValue: 0 },
      { argumentType: T.INTEGER, minValue: 0 },
    ],
  },
  INTERCEPT: { method: 'intercept', parameters: [rangeArg, rangeArg] },
  FORECAST: { method: 'forecast', parameters: [{ argumentType: T.NUMBER }, rangeArg, rangeArg] },
  FREQUENCY: {
    method: 'frequency',
    parameters: [rangeArg, rangeArg],
    sizeOfResultArrayMethod: 'frequencySize',
    vectorizationForbidden: true,
  },
  'ERROR.TYPE': { method: 'errortype', parameters: [{ argumentType: T.SCALAR }] },
  // SCALAR (not ANY) so error values reach the implementation instead of being
  // propagated; vectorization off so a range argument stays one call (-> 64).
  TYPE: { method: 'type', parameters: [{ argumentType: T.SCALAR }], vectorizationForbidden: true },
  XIRR: {
    method: 'xirr',
    parameters: [rangeArg, rangeArg, { argumentType: T.NUMBER, defaultValue: 0.1 }],
  },
};

ExcelExtrasPlugin.aliases = {
  'RANK.EQ': 'RANK',
  'MODE.SNGL': 'MODE',
  'FORECAST.LINEAR': 'FORECAST',
};

const names = [...Object.keys(ExcelExtrasPlugin.implementedFunctions), ...Object.keys(ExcelExtrasPlugin.aliases)];

let registered = false;

// registerExcelFunctions is idempotent — HyperFormula's registry is global and
// throws on a duplicate function id.
export function registerExcelFunctions(): void {
  if (registered) return;
  registered = true;
  const translations = Object.fromEntries(names.map((n) => [n, n]));
  HyperFormula.registerFunctionPlugin(ExcelExtrasPlugin, { enGB: translations, enUS: translations });
}

export const excelExtraFunctionNames = names;

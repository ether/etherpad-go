// formulaRefs ports lib/sheet/formularefs.go exactly: inserting or deleting
// rows/columns moves the references in formulas, renaming a sheet renames
// them, deleting a sheet breaks them (#REF!), as in Excel. Server and browser
// must agree bit for bit; both run lib/sheet/testdata/formularefs.json.

const MAX_ROWS = 1048576;
const MAX_COLS = 16384;

export interface RefEdit {
  // Name of the sheet the op changes.
  target: string;
  // The formula is on that sheet: references without a sheet name count.
  onTarget: boolean;
  // Insert (delta > 0) or delete (delta < 0) of rows/columns at index.
  axis?: 'row' | 'col';
  index?: number;
  delta?: number;
  // renameSheet: the new name.
  rename?: string;
  // deleteSheet.
  deleted?: boolean;
}

interface RefEnd {
  col: number; // -1: a whole row
  row: number; // -1: a whole column
  absCol: boolean;
  absRow: boolean;
}

const kindOf = (e: RefEnd): number => (e.col >= 0 && e.row >= 0 ? 0 : e.row < 0 ? 1 : 2);

const IDENT = /[\p{L}\p{Nd}_.]/u;
const LETTER = /\p{L}/u;
const isIdent = (c: string | undefined): boolean => c !== undefined && IDENT.test(c);
const isAsciiLetter = (c: string | undefined): boolean => c !== undefined && /[A-Za-z]/.test(c);
const isAsciiDigit = (c: string | undefined): boolean => c !== undefined && c >= '0' && c <= '9';

interface ParsedRef {
  end: number;
  sheet: string;
  hasSheet: boolean;
  prefixEnd: number;
  a: RefEnd;
  b: RefEnd;
  hasB: boolean;
}

function parseRefEnd(rs: string[], p: number): [RefEnd, number, boolean] {
  const e: RefEnd = { col: -1, row: -1, absCol: false, absRow: false };
  if (rs[p] === '$') {
    e.absCol = true;
    p++;
  }
  let j = p;
  let col = 0;
  while (j < rs.length && isAsciiLetter(rs[j]) && j - p < 4) {
    col = col * 26 + (rs[j].toUpperCase().charCodeAt(0) - 64);
    j++;
  }
  const letters = j - p;
  if (letters > 3) return [e, p, false];
  if (letters > 0) {
    e.col = col - 1;
    p = j;
    if (rs[p] === '$') {
      e.absRow = true;
      p++;
    }
  } else if (e.absCol) {
    e.absCol = false;
    e.absRow = true;
  }
  j = p;
  while (j < rs.length && isAsciiDigit(rs[j]) && j - p < 8) j++;
  if (j > p) {
    const n = Number(rs.slice(p, j).join(''));
    if (!Number.isFinite(n) || n < 1) return [e, p, false];
    e.row = n - 1;
    p = j;
  } else if (e.absRow && letters > 0) {
    return [e, p, false];
  }
  if (e.col < 0 && e.row < 0) return [e, p, false];
  if (e.col >= MAX_COLS || e.row >= MAX_ROWS) return [e, p, false];
  return [e, p, true];
}

function parseRef(rs: string[], i: number): ParsedRef | null {
  let p = i;
  let sheet = '';
  let hasSheet = false;
  if (rs[p] === "'") {
    let name = '';
    let j = p + 1;
    let closed = false;
    while (j < rs.length) {
      if (rs[j] === "'") {
        if (rs[j + 1] === "'") {
          name += "'";
          j += 2;
          continue;
        }
        closed = true;
        j++;
        break;
      }
      name += rs[j];
      j++;
    }
    if (!closed || j >= rs.length || rs[j] !== '!') return null;
    sheet = name;
    hasSheet = true;
    p = j + 1;
  } else if (isIdent(rs[p])) {
    let j = p;
    while (j < rs.length && isIdent(rs[j])) j++;
    if (rs[j] === '!') {
      sheet = rs.slice(p, j).join('');
      hasSheet = true;
      p = j + 1;
    }
  }
  const prefixEnd = p;
  const [a, pa, aok] = parseRefEnd(rs, p);
  if (!aok) return null;
  p = pa;
  let b: RefEnd = { col: -1, row: -1, absCol: false, absRow: false };
  let hasB = false;
  if (rs[p] === ':') {
    const [bb, q, bok] = parseRefEnd(rs, p + 1);
    if (bok && kindOf(bb) === kindOf(a)) {
      b = bb;
      hasB = true;
      p = q;
    }
  }
  if (!hasB && kindOf(a) !== 0) return null;
  const next = rs[p];
  if (next !== undefined && (isIdent(next) || next === '(' || next === '!' || next === '$')) return null;
  return { end: p, sheet, hasSheet, prefixEnd, a, b, hasB };
}

export function columnName(col: number): string {
  let s = '';
  for (let n = col + 1; n > 0; n = Math.floor((n - 1) / 26)) {
    s = String.fromCharCode(65 + ((n - 1) % 26)) + s;
  }
  return s;
}

const formatRefEnd = (e: RefEnd): string =>
  (e.col >= 0 ? (e.absCol ? '$' : '') + columnName(e.col) : '') +
  (e.row >= 0 ? (e.absRow ? '$' : '') + String(e.row + 1) : '');

// quoteSheetName writes a sheet name the way a formula needs it.
export function quoteSheetName(name: string): string {
  const cs = Array.from(name);
  let plain = cs.length > 0;
  cs.forEach((c, i) => {
    if ((i === 0 && !(LETTER.test(c) || c === '_')) || !isIdent(c)) plain = false;
  });
  if (plain) {
    if (parseRef(cs, 0) !== null) plain = false;
    const u = name.toUpperCase();
    if (u === 'TRUE' || u === 'FALSE') plain = false;
  }
  return plain ? name : `'${name.split("'").join("''")}'`;
}

function shiftRefCoord(v: number, index: number, delta: number): number | null {
  if (delta > 0) return v >= index ? v + delta : v;
  const band = -delta;
  if (v < index) return v;
  if (v < index + band) return null;
  return v - band;
}

function shiftRangeCoords(lo: number, hi: number, index: number, delta: number): [number, number] {
  if (delta > 0) {
    return [lo >= index ? lo + delta : lo, hi >= index ? hi + delta : hi];
  }
  const band = -delta;
  if (lo >= index + band) lo -= band;
  else if (lo >= index) lo = index;
  if (hi >= index + band) hi -= band;
  else if (hi >= index) hi = index - 1;
  return [lo, hi];
}

function applyRefEdit(e: RefEdit, ref: ParsedRef, prefix: string, orig: string): string {
  if (ref.hasSheet) {
    if (ref.sheet.toLowerCase() !== e.target.toLowerCase()) return orig;
  } else if (!e.onTarget) {
    return orig;
  }
  if (e.deleted) return ref.hasSheet ? '#REF!' : orig;
  if (e.rename) return ref.hasSheet ? `${quoteSheetName(e.rename)}!${orig.slice(prefix.length)}` : orig;
  const delta = e.delta ?? 0;
  const index = e.index ?? 0;
  if ((e.axis !== 'row' && e.axis !== 'col') || delta === 0) return orig;
  const isRow = e.axis === 'row';
  const get = (x: RefEnd): number => (isRow ? x.row : x.col);
  const set = (x: RefEnd, v: number): void => {
    if (isRow) x.row = v;
    else x.col = v;
  };
  const limit = isRow ? MAX_ROWS : MAX_COLS;
  const a = { ...ref.a };
  const b = { ...ref.b };
  if (get(a) < 0) return orig;
  if (!ref.hasB) {
    const v = shiftRefCoord(get(a), index, delta);
    if (v === null || v >= limit) return '#REF!';
    set(a, v);
  } else {
    let lo = get(a);
    let hi = get(b);
    const swapped = lo > hi;
    if (swapped) [lo, hi] = [hi, lo];
    let [nlo, nhi] = shiftRangeCoords(lo, hi, index, delta);
    if (nlo > nhi || nhi >= limit) return '#REF!';
    if (swapped) [nlo, nhi] = [nhi, nlo];
    set(a, nlo);
    set(b, nhi);
  }
  if (get(a) === get(ref.a) && get(b) === get(ref.b)) return orig;
  return prefix + formatRefEnd(a) + (ref.hasB ? `:${formatRefEnd(b)}` : '');
}

// rewriteFormula returns raw with its references changed by e. Text that is
// not a reference is kept as written, as are references e does not touch.
export function rewriteFormula(raw: string, e: RefEdit): string {
  if (!raw.startsWith('=')) return raw;
  const rs = Array.from(raw);
  let out = '';
  let changed = false;
  let i = 0;
  while (i < rs.length) {
    const c = rs[i];
    if (c === '"') {
      let j = i + 1;
      while (j < rs.length) {
        if (rs[j] === '"') {
          if (rs[j + 1] === '"') {
            j += 2;
            continue;
          }
          j++;
          break;
        }
        j++;
      }
      out += rs.slice(i, j).join('');
      i = j;
      continue;
    }
    if (c === "'" || c === '$' || isIdent(c)) {
      const ref = parseRef(rs, i);
      if (ref) {
        const orig = rs.slice(i, ref.end).join('');
        const repl = applyRefEdit(e, ref, rs.slice(i, ref.prefixEnd).join(''), orig);
        if (repl !== orig) changed = true;
        out += repl;
        i = ref.end;
        continue;
      }
      let j = i + 1;
      if (isIdent(c)) while (j < rs.length && isIdent(rs[j])) j++;
      out += rs.slice(i, j).join('');
      i = j;
      continue;
    }
    out += c;
    i++;
  }
  return changed ? out : raw;
}

// Structural ops as a RefEdit (axis, index and signed delta).
export function structuralEdit(type: string, index: number, count: number): Pick<RefEdit, 'axis' | 'index' | 'delta'> | null {
  switch (type) {
    case 'insertRows':
      return { axis: 'row', index, delta: count };
    case 'deleteRows':
      return { axis: 'row', index, delta: -count };
    case 'insertCols':
      return { axis: 'col', index, delta: count };
    case 'deleteCols':
      return { axis: 'col', index, delta: -count };
    default:
      return null;
  }
}

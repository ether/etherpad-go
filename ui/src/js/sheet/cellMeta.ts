// Cell props that are not formatting: hyperlink, note, list validation and
// conditional formatting (mirrors lib/sheet/cellmeta.go). They travel as
// style props, so they move, copy and undo with the cell.

export const PROP_LINK = 'link';
export const PROP_NOTE = 'note';
export const PROP_VALID_LIST = 'validList';
export const PROP_COND_FMT = 'cf';

// Kept by "Clear Formats", as in Excel (conditional formatting goes).
export const META_PROPS = [PROP_LINK, PROP_NOTE, PROP_VALID_LIST];

export const MAX_LINK_LENGTH = 2048;
export const MAX_NOTE_LENGTH = 4000;
export const MAX_VALID_LIST_LENGTH = 255;
export const MAX_COND_RULES = 8;

export type CondOp =
  | 'gt' | 'lt' | 'ge' | 'le' | 'eq' | 'ne' | 'between' | 'notBetween'
  | 'contains' | 'notContains' | 'beginsWith' | 'endsWith' | 'blank' | 'notBlank';

export interface CondRule {
  op: CondOp;
  v1?: string;
  v2?: string;
  bg?: string;
  color?: string;
  bold?: string;
}

// How many values each operator takes (Go: CondOps).
export const COND_OPS: Record<CondOp, number> = {
  gt: 1, lt: 1, ge: 1, le: 1, eq: 1, ne: 1, between: 2, notBetween: 2,
  contains: 1, notContains: 1, beginsWith: 1, endsWith: 1, blank: 0, notBlank: 0,
};

const HEX = /^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$/;

// validLink mirrors the server check: http(s) with a host, or mailto.
export function validLink(v: string): boolean {
  if (v === '' || v.length > MAX_LINK_LENGTH || /[\s\p{Cc}]/u.test(v)) return false;
  try {
    const u = new URL(v);
    if (u.protocol === 'http:' || u.protocol === 'https:') return u.host !== '';
    if (u.protocol === 'mailto:') return u.pathname !== '';
  } catch {
    return false;
  }
  return false;
}

// normalizeLink adds https:// to a bare host, as Excel does for "www.x.de".
export function normalizeLink(v: string): string {
  const t = v.trim();
  if (t === '' || /^[a-z][a-z0-9+.-]*:/i.test(t)) return t;
  if (/^[^@\s]+@[^@\s]+\.[^@\s]+$/.test(t)) return `mailto:${t}`;
  return `https://${t}`;
}

export function parseCondRules(v: string | undefined): CondRule[] {
  if (!v) return [];
  try {
    const rules = JSON.parse(v);
    if (!Array.isArray(rules)) return [];
    return rules.filter((r): r is CondRule => r && typeof r.op === 'string' && r.op in COND_OPS);
  } catch {
    return [];
  }
}

// listValues splits a validList prop into its values.
export const listValues = (v: string | undefined): string[] =>
  v ? v.split(',').map((s) => s.trim()).filter((s) => s !== '') : [];

// inList reports whether raw is allowed by the list (Excel: case-insensitive;
// an empty cell is allowed).
export function inList(list: string[], raw: string): boolean {
  if (raw === '') return true;
  const t = raw.trim().toLowerCase();
  return list.some((v) => v.toLowerCase() === t);
}

const asNumber = (s: string): number | null => {
  const t = s.trim();
  if (t === '') return null;
  const n = Number(t);
  return Number.isFinite(n) ? n : null;
};

// compare orders a cell value against a rule value like Excel's "Cell Value"
// rules: numbers as numbers (an empty cell counts as 0), text without case,
// and text after any number.
function compare(value: string, ruleValue: string): number {
  const rn = asNumber(ruleValue);
  const vn = value.trim() === '' && rn !== null ? 0 : asNumber(value);
  if (vn !== null && rn !== null) return vn - rn;
  if (vn !== null) return -1;
  if (rn !== null) return 1;
  return value.toLowerCase().localeCompare(ruleValue.toLowerCase());
}

export function ruleMatches(r: CondRule, value: string): boolean {
  const v1 = r.v1 ?? '';
  const v2 = r.v2 ?? '';
  const text = value.toLowerCase();
  switch (r.op) {
    case 'gt': return compare(value, v1) > 0;
    case 'lt': return compare(value, v1) < 0;
    case 'ge': return compare(value, v1) >= 0;
    case 'le': return compare(value, v1) <= 0;
    case 'eq': return compare(value, v1) === 0;
    case 'ne': return compare(value, v1) !== 0;
    case 'between':
    case 'notBetween': {
      const [lo, hi] = compare(v1, v2) <= 0 ? [v1, v2] : [v2, v1];
      const inside = compare(value, lo) >= 0 && compare(value, hi) <= 0;
      return r.op === 'between' ? inside : !inside;
    }
    case 'contains': return text.includes(v1.toLowerCase());
    case 'notContains': return !text.includes(v1.toLowerCase());
    case 'beginsWith': return text.startsWith(v1.toLowerCase());
    case 'endsWith': return text.endsWith(v1.toLowerCase());
    case 'blank': return value.trim() === '';
    case 'notBlank': return value.trim() !== '';
    default: return false;
  }
}

// condStyle applies the matching rules in order; an earlier rule wins a
// property (Excel's rule priority).
export function condStyle(rules: CondRule[], value: string): { bg?: string; color?: string; bold?: boolean } {
  const out: { bg?: string; color?: string; bold?: boolean } = {};
  for (const r of rules) {
    if (!ruleMatches(r, value)) continue;
    if (out.bg === undefined && r.bg && HEX.test(r.bg)) out.bg = r.bg;
    if (out.color === undefined && r.color && HEX.test(r.color)) out.color = r.color;
    if (out.bold === undefined && r.bold === '1') out.bold = true;
  }
  return out;
}

// Excel's preset highlight styles.
export const COND_PRESETS: Array<{ label: string; bg?: string; color?: string; bold?: string }> = [
  { label: 'Light red fill with dark red text', bg: '#ffc7ce', color: '#9c0006' },
  { label: 'Yellow fill with dark yellow text', bg: '#ffeb9c', color: '#9c5700' },
  { label: 'Green fill with dark green text', bg: '#c6efce', color: '#006100' },
  { label: 'Light red fill', bg: '#ffc7ce' },
  { label: 'Red text', color: '#9c0006' },
  { label: 'Bold text', bold: '1' },
];

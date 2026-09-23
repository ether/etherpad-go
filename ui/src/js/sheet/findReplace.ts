// Pure find/replace over cell contents. Like Excel's default "Look in:
// Formulas", the search runs against the raw cell content, so a formula is
// found by its text and a replacement rewrites the formula itself.

export interface FindOptions {
  matchCase?: boolean;
  // Excel's "Match entire cell contents": the query must be the whole content
  // instead of appearing somewhere inside it.
  wholeCell?: boolean;
}

export interface CellRef {
  row: number;
  col: number;
}

export interface RawCell extends CellRef {
  raw: string;
}

const fold = (s: string, matchCase: boolean | undefined): string => (matchCase ? s : s.toLowerCase());

export function matches(raw: string, query: string, opts: FindOptions = {}): boolean {
  if (query === '') return false;
  const hay = fold(raw, opts.matchCase);
  const needle = fold(query, opts.matchCase);
  return opts.wholeCell ? hay === needle : hay.includes(needle);
}

// findAll returns every matching cell in row-major order — the order Excel
// searches in, and the order Find Next steps through.
export function findAll(cells: RawCell[], query: string, opts: FindOptions = {}): CellRef[] {
  return cells
    .filter((c) => matches(c.raw, query, opts))
    .sort((a, b) => a.row - b.row || a.col - b.col)
    .map(({ row, col }) => ({ row, col }));
}

// findNext returns the first match strictly after `from` in row-major order,
// wrapping around to the beginning. null when nothing matches at all.
export function findNext(cells: RawCell[], query: string, from: CellRef, opts: FindOptions = {}): CellRef | null {
  const hits = findAll(cells, query, opts);
  if (hits.length === 0) return null;
  return hits.find((h) => h.row > from.row || (h.row === from.row && h.col > from.col)) ?? hits[0];
}

// replaceInRaw rewrites every occurrence in one cell. In whole-cell mode the
// content is swapped outright, matching Excel.
export function replaceInRaw(raw: string, query: string, replacement: string, opts: FindOptions = {}): string {
  if (!matches(raw, query, opts)) return raw;
  if (opts.wholeCell) return replacement;
  if (opts.matchCase) return raw.split(query).join(replacement);
  // Case-insensitive: walk the folded string so the untouched parts keep their
  // original casing (a plain regex would need the query escaped anyway).
  const hay = raw.toLowerCase();
  const needle = query.toLowerCase();
  let out = '';
  let i = 0;
  for (let at = hay.indexOf(needle); at !== -1; at = hay.indexOf(needle, i)) {
    out += raw.slice(i, at) + replacement;
    i = at + needle.length;
  }
  return out + raw.slice(i);
}

// replaceAll produces the cells whose content actually changes, so the caller
// can emit one op per changed cell and nothing for the rest.
export function replaceAll(cells: RawCell[], query: string, replacement: string, opts: FindOptions = {}): RawCell[] {
  const out: RawCell[] = [];
  for (const c of cells) {
    const next = replaceInRaw(c.raw, query, replacement, opts);
    if (next !== c.raw) out.push({ row: c.row, col: c.col, raw: next });
  }
  return out.sort((a, b) => a.row - b.row || a.col - b.col);
}

// How filter values compare (mirrors Go sheet.FilterKey / FilterKeeps):
// without case and surrounding space, numbers by value, so Excel's
// "1,234.5" in an imported filter matches a cell showing 1234.5.

const THOUSANDS = /^-?\d{1,3}(,\d{3})+(\.\d+)?$/;
const NUMBER = /^-?(\d+\.?\d*|\.\d+)(e[-+]?\d+)?$/;

export function filterKey(v: string): string {
  const t = v.trim().toLowerCase();
  const n = THOUSANDS.test(t) ? t.split(',').join('') : t;
  if (NUMBER.test(n)) {
    const f = Number(n);
    if (Number.isFinite(f)) return `#${f}`;
  }
  return t;
}

// filterKeeps reports whether a column filter keeps a row showing `value`.
// Empty cells never hide a row.
export function filterKeeps(values: string[] | undefined, hide: boolean, value: string): boolean {
  if (!values || value.trim() === '') return true;
  const k = filterKey(value);
  return values.some((v) => filterKey(v) === k) !== hide;
}

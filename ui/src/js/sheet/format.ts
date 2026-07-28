// Pure display formatting. raw stays the source of truth; this only affects how
// the computed value is shown. Uses the built-in Intl API (no dependency).

function parseFmt(numFmt: string): { kind: string; decimals: number | undefined } {
  const [kind, d] = numFmt.split(':');
  const n = d === undefined ? undefined : Number(d);
  return { kind, decimals: n === undefined || Number.isNaN(n) ? undefined : n };
}

// stepDecimals implements Excel's "increase/decrease decimal" buttons: it keeps
// the format kind and moves the digit count by `delta`, clamped to 0..9. A cell
// still on General becomes a plain number format, exactly like Excel.
export function stepDecimals(numFmt: string | undefined, delta: number): string {
  const current = !numFmt || numFmt === 'general' || numFmt === 'text' || numFmt === 'date' ? 'number:2' : numFmt;
  const { kind, decimals } = parseFmt(current);
  const next = Math.min(9, Math.max(0, (decimals ?? 2) + delta));
  return `${kind}:${next}`;
}

export function formatValue(value: string, _valueType: string, numFmt: string | undefined): string {
  if (!numFmt || numFmt === 'general' || numFmt === 'text') return value;
  const { kind, decimals } = parseFmt(numFmt);

  if (kind === 'date') {
    const d = /^\d+(\.\d+)?$/.test(value)
      ? new Date(Date.UTC(1899, 11, 30) + Number(value) * 86400000) // spreadsheet serial
      : new Date(value);
    // timeZone: 'UTC' so the calendar day is the same for every collaborator
    // regardless of their local offset (values are date-only, stored as UTC).
    return isNaN(d.getTime()) ? value : d.toLocaleDateString('en-US', { timeZone: 'UTC' });
  }

  const n = Number(value);
  if (value === '' || Number.isNaN(n)) return value; // non-numeric: leave as-is

  const opts: Intl.NumberFormatOptions =
    decimals === undefined ? {} : { minimumFractionDigits: decimals, maximumFractionDigits: decimals };
  switch (kind) {
    case 'number':
      return new Intl.NumberFormat('en-US', { useGrouping: true, ...opts }).format(n);
    case 'currency':
      return new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD', ...opts }).format(n);
    case 'percent':
      return new Intl.NumberFormat('en-US', { style: 'percent', ...opts }).format(n);
    default:
      return value;
  }
}

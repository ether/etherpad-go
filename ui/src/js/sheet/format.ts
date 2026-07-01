// Pure display formatting. raw stays the source of truth; this only affects how
// the computed value is shown. Uses the built-in Intl API (no dependency).

function parseFmt(numFmt: string): { kind: string; decimals: number | undefined } {
  const [kind, d] = numFmt.split(':');
  return { kind, decimals: d === undefined ? undefined : Number(d) };
}

export function formatValue(value: string, _valueType: string, numFmt: string | undefined): string {
  if (!numFmt || numFmt === 'general' || numFmt === 'text') return value;
  const { kind, decimals } = parseFmt(numFmt);

  if (kind === 'date') {
    const d = /^\d+(\.\d+)?$/.test(value)
      ? new Date(Date.UTC(1899, 11, 30) + Number(value) * 86400000) // spreadsheet serial
      : new Date(value);
    return isNaN(d.getTime()) ? value : d.toLocaleDateString();
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

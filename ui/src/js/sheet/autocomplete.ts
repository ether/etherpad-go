// Pure helpers for formula function-name autocomplete. No DOM.

// functionPrefix returns the uppercased partial function name immediately left
// of `caret` in a formula, or null. A function head is a run of letters that is
// NOT immediately followed by a digit (which would make it a cell ref like A1)
// and not already closed by '('. The gates are unconditional so the dropdown
// never pops while typing arguments or cell refs (e.g. "=SUM(A1", "=A1").
export function functionPrefix(raw: string, caret: number): string | null {
  if (!raw.startsWith('=')) return null;
  const left = raw.slice(0, caret);
  const m = /([A-Za-z]+)$/.exec(left);
  if (!m) return null;
  const after = raw.slice(caret);
  if (after.startsWith('(')) return null; // already a completed call head
  if (/^[0-9]/.test(after)) return null; // cell ref being typed, not a function
  return m[1].toUpperCase();
}

export function filterFunctions(names: string[], prefix: string): string[] {
  const p = prefix.toUpperCase();
  return names
    .filter((n) => n.toUpperCase().startsWith(p))
    .sort()
    .slice(0, 8);
}

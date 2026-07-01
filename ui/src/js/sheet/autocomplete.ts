// Pure helpers for formula function-name autocomplete. No DOM.

// functionPrefix returns the uppercased partial function name immediately left
// of `caret` in a formula, or null. A function head is a run of letters that is
// NOT immediately followed by a digit (which would make it a cell ref like A1)
// and not already closed by '('.
export function functionPrefix(raw: string, caret: number): string | null {
  if (!raw.startsWith('=')) return null;
  const left = raw.slice(0, caret);
  // Match the rightmost run of letters, even if followed by non-letters
  const m = /([A-Za-z]+)[^A-Za-z]*$/.exec(left);
  if (!m) return null;
  // Find where this letter run ends in the original string
  const letterEndPos = left.indexOf(m[1]) + m[1].length;
  // Check what comes after the letters
  const afterLetters = raw.slice(letterEndPos);
  if (afterLetters.startsWith('(')) return null; // already a completed call head
  // If the letter run is immediately followed by a digit in the original, skip
  // EXCEPT if the caret is not right after the letters (meaning there are chars in between)
  if (/^[0-9]/.test(afterLetters) && letterEndPos === caret) return null;
  return m[1].toUpperCase();
}

export function filterFunctions(names: string[], prefix: string): string[] {
  const p = prefix.toUpperCase();
  return names
    .filter((n) => n.toUpperCase().startsWith(p))
    .sort()
    .slice(0, 8);
}

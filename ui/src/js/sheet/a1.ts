// A1-notation helpers shared by the grid view and the formula bar's name box.

export function colName(col: number): string {
  let s = '';
  let n = col + 1;
  while (n > 0) {
    const rem = (n - 1) % 26;
    s = String.fromCharCode(65 + rem) + s;
    n = Math.floor((n - 1) / 26);
  }
  return s;
}

export function cellRefA1(row: number, col: number): string {
  return `${colName(col)}${row + 1}`;
}

export function rangeRefA1(r0: number, c0: number, r1: number, c1: number): string {
  const top = { r: Math.min(r0, r1), c: Math.min(c0, c1) };
  const bot = { r: Math.max(r0, r1), c: Math.max(c0, c1) };
  const a = cellRefA1(top.r, top.c);
  if (top.r === bot.r && top.c === bot.c) return a;
  return `${a}:${cellRefA1(bot.r, bot.c)}`;
}

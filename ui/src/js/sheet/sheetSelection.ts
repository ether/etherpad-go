// Pure range model for grid selection. Zero-based (row, col). anchor is the
// fixed corner (where a drag/extend started); focus is the moving corner.

export interface CellPos {
  row: number;
  col: number;
}

export interface Selection {
  anchor: CellPos;
  focus: CellPos;
}

export function selFromSingle(row: number, col: number): Selection {
  return { anchor: { row, col }, focus: { row, col } };
}

export function normalize(s: Selection): { r0: number; c0: number; r1: number; c1: number } {
  return {
    r0: Math.min(s.anchor.row, s.focus.row),
    c0: Math.min(s.anchor.col, s.focus.col),
    r1: Math.max(s.anchor.row, s.focus.row),
    c1: Math.max(s.anchor.col, s.focus.col),
  };
}

export function selContains(s: Selection, row: number, col: number): boolean {
  const { r0, c0, r1, c1 } = normalize(s);
  return row >= r0 && row <= r1 && col >= c0 && col <= c1;
}

export function selCells(s: Selection): CellPos[] {
  const { r0, c0, r1, c1 } = normalize(s);
  const out: CellPos[] = [];
  for (let r = r0; r <= r1; r++) {
    for (let c = c0; c <= c1; c++) out.push({ row: r, col: c });
  }
  return out;
}

export function selIsSingle(s: Selection): boolean {
  return s.anchor.row === s.focus.row && s.anchor.col === s.focus.col;
}

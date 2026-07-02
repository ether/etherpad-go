import { isStructural, type Op } from './op';

// transform ports lib/sheet/transform.go exactly. It adjusts `inOp` so it applies
// cleanly after `applied`, where both were composed against the same base
// revision and `applied` was ordered first. MUST match the Go implementation
// bit-for-bit: the server transforms on Submit while the client transforms its
// pending ops against incoming NEW_SHEET_OPs.
export function transform(inOp: Op, applied: Op): Op {
  if (inOp.sheet !== applied.sheet || !isStructural(applied)) return inOp;
  const index = applied.index ?? 0;
  const count = applied.count ?? 0;
  switch (applied.type) {
    case 'insertRows':
      return shiftRows(inOp, index, count);
    case 'deleteRows':
      return shiftRows(inOp, index, -count);
    case 'insertCols':
      return shiftCols(inOp, index, count);
    case 'deleteCols':
      return shiftCols(inOp, index, -count);
    default:
      return inOp;
  }
}

function shiftRows(inOp: Op, index: number, delta: number): Op {
  const out: Op = { ...inOp };
  out.row = shiftCoord(out.row ?? 0, index, delta);
  if (out.type === 'clearRange') {
    out.endRow = shiftCoord(out.endRow ?? 0, index, delta);
  }
  if (out.type === 'insertRows' || out.type === 'deleteRows') {
    out.index = shiftCoord(out.index ?? 0, index, delta);
  }
  if (out.type === 'setDimension' && out.axis === 'row') {
    out.index = shiftCoord(out.index ?? 0, index, delta);
  }
  return out;
}

function shiftCols(inOp: Op, index: number, delta: number): Op {
  const out: Op = { ...inOp };
  out.col = shiftCoord(out.col ?? 0, index, delta);
  if (out.type === 'clearRange') {
    out.endCol = shiftCoord(out.endCol ?? 0, index, delta);
  }
  if (out.type === 'insertCols' || out.type === 'deleteCols') {
    out.index = shiftCoord(out.index ?? 0, index, delta);
  }
  if (out.type === 'setDimension' && out.axis === 'col') {
    out.index = shiftCoord(out.index ?? 0, index, delta);
  }
  return out;
}

// shiftCoord: for inserts (delta>0) coords at/after index move; for deletes
// (delta<0) coords after the band move back, coords inside the band clamp to index.
function shiftCoord(coord: number, index: number, delta: number): number {
  if (delta >= 0) {
    return coord >= index ? coord + delta : coord;
  }
  const band = -delta;
  if (coord < index) return coord;
  if (coord < index + band) return index;
  return coord - band;
}

// Op mirrors the Go lib/sheet.Op type exactly (same JSON field names) so the
// same wire payload is understood by both the TS client and the Go server.
// All payload fields are optional per op type; type/sheet/baseRev are always set.

export type OpType =
  | 'setCell'
  | 'setStyle'
  | 'clearRange'
  | 'insertRows'
  | 'deleteRows'
  | 'insertCols'
  | 'deleteCols'
  | 'addSheet'
  | 'renameSheet'
  | 'deleteSheet'
  | 'moveSheet'
  | 'setDimension'
  | 'setFreeze';

export interface Op {
  type: OpType;
  sheet: string;
  baseRev: number;
  // cell / range top-left
  row?: number;
  col?: number;
  // range end (inclusive) for clearRange
  endRow?: number;
  endCol?: number;
  // setCell / setStyle payload
  raw?: string;
  value?: string;
  valueType?: string;
  styleId?: number;
  // setCell / setStyle style properties, interned into the workbook style pool.
  props?: Record<string, string>;
  // structural ops; index doubles as the insertion position for addSheet
  index?: number;
  count?: number;
  // sheet-list ops
  name?: string; // addSheet, renameSheet
  toIndex?: number; // moveSheet
  // setDimension
  axis?: 'col' | 'row';
  size?: number; // px
  // setFreeze (0 or 1 each)
  frozenRows?: number;
  frozenCols?: number;
}

// serializeOp produces the JSON the Go server unmarshals into sheet.Op.
// JSON.stringify omits undefined fields, matching Go's `omitempty` tags.
export function serializeOp(op: Op): string {
  return JSON.stringify(op);
}

export const isStructural = (op: Op): boolean =>
  op.type === 'insertRows' ||
  op.type === 'deleteRows' ||
  op.type === 'insertCols' ||
  op.type === 'deleteCols';

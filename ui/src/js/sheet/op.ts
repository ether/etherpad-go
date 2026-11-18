import type { Chart } from './chart';

// The sheet op version this code understands (Go ws.SheetOpsVersion); sent in
// CLIENT_READY so the server reloads an older client instead of sending it
// ops it cannot apply.
export const SHEET_OPS_VERSION = 2;
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
  | 'setFreeze'
  | 'mergeCells'
  | 'unmergeCells'
  | 'setFilter'
  | 'setChart'
  | 'deleteChart';

export interface Op {
  type: OpType;
  sheet: string;
  baseRev: number;
  // cell / range top-left
  row?: number;
  col?: number;
  // range end (inclusive) for clearRange / mergeCells / unmergeCells
  endRow?: number;
  endCol?: number;
  // clearRange: empty the cells but keep their style props (Excel's Delete)
  keepStyle?: boolean;
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
  // addSheet, renameSheet; the server also fills it on row/column inserts and
  // deletes and on deleteSheet, and oldName on renameSheet (see Go Op).
  name?: string;
  oldName?: string;
  toIndex?: number; // moveSheet
  // setDimension
  axis?: 'col' | 'row';
  size?: number; // px
  // setFreeze: number of frozen top rows / left columns
  frozenRows?: number;
  frozenCols?: number;
  // setFilter: show the rows whose value in `col` is one of `values` (rows up
  // to the header `row` always show); no values removes the column's filter,
  // clear removes all filters of the sheet.
  values?: string[];
  // setChart (anchored at row/col) / deleteChart (only chart.id)
  chart?: Chart;
  // setChart of a chart not there yet: its 1-based drawing position (undo
  // of a delete); 0 appends.
  order?: number;
  // Client-only, never sent: an undo restore of a chart only writes while
  // the chart is still as the undone op left it.
  expectChart?: Chart;
  // values are the hidden ones (mirrors Go Op.Exclude)
  exclude?: boolean;
  clear?: boolean;
  // Transform found nothing left to do (a filter on a column deleted
  // meanwhile); applyOp skips the op (mirrors Go Op.Noop).
  noop?: boolean;
  // The sending client's id for the op ("<client>:<seq>"). The server stores
  // it, so a client that lost the acknowledgement in a reconnect finds its op
  // again, and a resent op is not applied twice.
  cid?: string;
  // Client-only, never sent: an undo restore of a formula only writes when
  // the cell still holds this (what the undone op left there), so undo
  // does not overwrite a collaborator's later edit.
  expect?: string;
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

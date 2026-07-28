// invertOp builds the ops that undo `op`, read from the state *before* the op is
// applied. Undo is therefore just more ops through the normal collaborative
// pipeline: they get transformed against remote work and sent like any edit, so
// undoing never rolls back what somebody else did in the meantime.
//
// Inverses carry `props`, never `styleId`: style ids are a client-local pool
// index, only props travel on the wire.
import type { Op } from './op';
import type { SheetState, WorkbookState } from './workbookState';

// Defaults the grid falls back to when a row/column has no explicit size. The
// op vocabulary has no "unset dimension", so undoing the very first resize
// restores the default instead of removing the entry.
const DEFAULT_COL_WIDTH = 80;
const DEFAULT_ROW_HEIGHT = 22;

const parseKey = (k: string): [number, number] => {
  const i = k.indexOf(':');
  return [Number(k.slice(0, i)), Number(k.slice(i + 1))];
};

const base = (op: Op): Pick<Op, 'sheet' | 'baseRev'> => ({ sheet: op.sheet, baseRev: 0 });

// cellRestore reproduces one cell exactly: raw content plus its style props.
const cellRestore = (wb: WorkbookState, op: Op, sheet: SheetState, row: number, col: number): Op => {
  const cell = sheet.cells.get(`${row}:${col}`);
  return {
    ...base(op),
    type: 'setCell',
    row,
    col,
    raw: cell?.raw ?? '',
    props: { ...(wb.styles.get(cell?.styleId ?? 0) ?? {}) },
  };
};

const mergeRestore = (op: Op, row: number, col: number, rows: number, cols: number): Op => ({
  ...base(op),
  type: 'mergeCells',
  row,
  col,
  endRow: row + rows - 1,
  endCol: col + cols - 1,
});

// bandRestore rebuilds everything a row/column deletion destroys: the cells in
// the band, the sizes of the affected rows/columns, and the merges that the
// shift dropped or resized.
const bandRestore = (wb: WorkbookState, op: Op, sheet: SheetState, axis: 'row' | 'col', index: number, count: number): Op[] => {
  const out: Op[] = [];
  for (const [k] of sheet.cells) {
    const [r, c] = parseKey(k);
    const at = axis === 'row' ? r : c;
    if (at >= index && at < index + count) out.push(cellRestore(wb, op, sheet, r, c));
  }
  const dims = axis === 'row' ? sheet.rowHeights : sheet.colWidths;
  for (const [i, size] of dims) {
    if (i >= index) out.push({ ...base(op), type: 'setDimension', axis, index: i, size });
  }
  for (const [k, sp] of sheet.merges) {
    const [r, c] = parseKey(k);
    const lo = axis === 'row' ? r : c;
    const span = axis === 'row' ? sp.rows : sp.cols;
    if (lo + span > index) out.push(mergeRestore(op, r, c, sp.rows, sp.cols));
  }
  return out;
};

export function invertOp(wb: WorkbookState, op: Op): Op[] {
  switch (op.type) {
    case 'addSheet':
      return [{ ...base(op), type: 'deleteSheet' }];
    case 'deleteSheet': {
      const sheet = wb.sheetById(op.sheet);
      // The last sheet is never actually deleted (applyOp refuses), so undoing
      // it must not re-add anything either.
      if (!sheet || wb.sheets.length <= 1) return [];
      const out: Op[] = [
        { ...base(op), type: 'addSheet', name: sheet.name, index: wb.sheets.indexOf(sheet) },
      ];
      for (const [k] of sheet.cells) {
        const [r, c] = parseKey(k);
        out.push(cellRestore(wb, op, sheet, r, c));
      }
      for (const [i, size] of sheet.colWidths) out.push({ ...base(op), type: 'setDimension', axis: 'col', index: i, size });
      for (const [i, size] of sheet.rowHeights) out.push({ ...base(op), type: 'setDimension', axis: 'row', index: i, size });
      for (const [k, sp] of sheet.merges) {
        const [r, c] = parseKey(k);
        out.push(mergeRestore(op, r, c, sp.rows, sp.cols));
      }
      if (sheet.frozenRows || sheet.frozenCols) {
        out.push({ ...base(op), type: 'setFreeze', frozenRows: sheet.frozenRows, frozenCols: sheet.frozenCols });
      }
      return out;
    }
    case 'renameSheet': {
      const sheet = wb.sheetById(op.sheet);
      return sheet ? [{ ...base(op), type: 'renameSheet', name: sheet.name }] : [];
    }
    case 'moveSheet': {
      const i = wb.sheets.findIndex((s) => s.id === op.sheet);
      return i < 0 ? [] : [{ ...base(op), type: 'moveSheet', toIndex: i }];
    }
  }

  const sheet = wb.sheetById(op.sheet);
  if (!sheet) return [];
  const row = op.row ?? 0;
  const col = op.col ?? 0;
  const index = op.index ?? 0;
  const count = op.count ?? 0;

  switch (op.type) {
    case 'setCell':
    case 'setStyle':
      return [cellRestore(wb, op, sheet, row, col)];
    case 'clearRange': {
      const endRow = op.endRow ?? 0;
      const endCol = op.endCol ?? 0;
      const out: Op[] = [];
      for (const [k] of sheet.cells) {
        const [r, c] = parseKey(k);
        if (r >= row && r <= endRow && c >= col && c <= endCol) out.push(cellRestore(wb, op, sheet, r, c));
      }
      return out;
    }
    case 'setDimension': {
      const dims = op.axis === 'col' ? sheet.colWidths : sheet.rowHeights;
      const previous = dims.get(index) ?? (op.axis === 'col' ? DEFAULT_COL_WIDTH : DEFAULT_ROW_HEIGHT);
      return [{ ...base(op), type: 'setDimension', axis: op.axis, index, size: previous }];
    }
    case 'setFreeze':
      return [{ ...base(op), type: 'setFreeze', frozenRows: sheet.frozenRows, frozenCols: sheet.frozenCols }];
    case 'mergeCells': {
      const endRow = op.endRow ?? 0;
      const endCol = op.endCol ?? 0;
      if (endRow === row && endCol === col) return []; // degenerate: applyOp ignores it
      // Drop the new merge, then put back every merge it absorbed.
      const out: Op[] = [{ ...base(op), type: 'unmergeCells', row, col, endRow, endCol }];
      for (const [k, sp] of sheet.merges) {
        const [r, c] = parseKey(k);
        if (r <= endRow && r + sp.rows - 1 >= row && c <= endCol && c + sp.cols - 1 >= col) {
          out.push(mergeRestore(op, r, c, sp.rows, sp.cols));
        }
      }
      return out;
    }
    case 'unmergeCells': {
      const endRow = op.endRow ?? 0;
      const endCol = op.endCol ?? 0;
      const out: Op[] = [];
      for (const [k, sp] of sheet.merges) {
        const [r, c] = parseKey(k);
        if (r <= endRow && r + sp.rows - 1 >= row && c <= endCol && c + sp.cols - 1 >= col) {
          out.push(mergeRestore(op, r, c, sp.rows, sp.cols));
        }
      }
      return out;
    }
    case 'insertRows':
      return [{ ...base(op), type: 'deleteRows', index, count }];
    case 'insertCols':
      return [{ ...base(op), type: 'deleteCols', index, count }];
    case 'deleteRows':
      return [{ ...base(op), type: 'insertRows', index, count }, ...bandRestore(wb, op, sheet, 'row', index, count)];
    case 'deleteCols':
      return [{ ...base(op), type: 'insertCols', index, count }, ...bandRestore(wb, op, sheet, 'col', index, count)];
    default:
      return [];
  }
}

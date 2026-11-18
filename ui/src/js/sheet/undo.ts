// invertOp builds the ops that undo `op`, read from the state *before* the op is
// applied. Undo is therefore just more ops through the normal collaborative
// pipeline: they get transformed against remote work and sent like any edit, so
// undoing never rolls back what somebody else did in the meantime.
//
// Inverses carry `props`, never `styleId`: style ids are a client-local pool
// index, only props travel on the wire.
import type { Op } from './op';
import { rewriteFormula, structuralEdit, type RefEdit } from './formulaRefs';
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

// formulaRestores puts back every formula, on any sheet, whose references the
// op rewrites (moved, renamed or broken to #REF!). The inverse op alone cannot:
// a reference turned into #REF! does not come back by itself. Positions are
// those before the op, which the inverse structural op restores first.
// Each restore carries `expect`: the formula as the op and then the inverse
// op leave it. Undo only writes when the cell still says that, so a
// collaborator's later edit of the cell is kept.
const formulaRestores = (
  wb: WorkbookState, op: Op, edit: Omit<RefEdit, 'onTarget'>,
  inverse: Omit<RefEdit, 'onTarget'> | null, skip?: (r: number, c: number) => boolean,
): Op[] => {
  const out: Op[] = [];
  for (const sheet of wb.sheets) {
    const onTarget = sheet.id === op.sheet;
    for (const [k, cell] of sheet.cells) {
      if (!cell.raw.startsWith('=')) continue;
      const [r, c] = parseKey(k);
      if (onTarget && skip?.(r, c)) continue;
      const after = rewriteFormula(cell.raw, { ...edit, onTarget });
      if (after === cell.raw) continue;
      const expect = inverse ? rewriteFormula(after, { ...inverse, onTarget }) : after;
      if (expect === cell.raw) continue; // the inverse op alone puts it back
      out.push({ ...cellRestore(wb, op, sheet, r, c), sheet: sheet.id, expect });
    }
  }
  return out;
};

// filterRestore rebuilds a sheet's column filters (all, or one column).
const filterRestore = (op: Op, sheet: SheetState, only?: number): Op[] => {
  const out: Op[] = [];
  for (const [c, values] of sheet.filters) {
    if (only !== undefined && c !== only) continue;
    out.push({
      ...base(op), type: 'setFilter', row: sheet.filterRow, col: c, values: [...values],
      ...(sheet.filterHide.has(c) ? { exclude: true } : {}),
    });
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
      out.push(...filterRestore(op, sheet));
      // References from the other sheets became #REF!.
      out.push(...formulaRestores(wb, op, { target: sheet.name, deleted: true }, null, () => true));
      return out;
    }
    case 'renameSheet': {
      const sheet = wb.sheetById(op.sheet);
      if (!sheet) return [];
      const out: Op[] = [{ ...base(op), type: 'renameSheet', name: sheet.name }];
      if (op.name && op.name !== sheet.name) out.push(...formulaRestores(wb, op, { target: sheet.name, rename: op.name }, { target: op.name, rename: sheet.name }));
      return out;
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
    case 'setFilter': {
      if (op.clear) {
        // Back to exactly the filters before: clear, then put each one back.
        return [{ ...base(op), type: 'setFilter', row: 0, col: 0, clear: true }, ...filterRestore(op, sheet)];
      }
      // Only this column: filters others set meanwhile stay.
      const before = filterRestore(op, sheet, col);
      return before.length > 0 ? before : [{ ...base(op), type: 'setFilter', row: 0, col }];
    }
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
    case 'insertCols':
    case 'deleteRows':
    case 'deleteCols': {
      const edit = structuralEdit(op.type, index, count);
      if (!edit) return [];
      const back = { target: sheet.name, ...edit, delta: -(edit.delta ?? 0) };
      const refs = formulaRestores(wb, op, { target: sheet.name, ...edit }, back, (r, c) => {
        const at = edit.axis === 'row' ? r : c;
        return (edit.delta ?? 0) < 0 && at >= index && at < index + count; // bandRestore has them
      });
      switch (op.type) {
        case 'insertRows':
          return [{ ...base(op), type: 'deleteRows', index, count }, ...refs];
        case 'insertCols':
          return [{ ...base(op), type: 'deleteCols', index, count }, ...refs];
        case 'deleteRows': {
          // A filter header inside the band was moved to its edge: put it back.
          const header: Op[] = [];
          const fr = sheet.filterRow;
          if (sheet.filters.size > 0 && fr >= index && fr < index + count) {
            header.push(...filterRestore(op, sheet, sheet.filters.keys().next().value as number));
          }
          return [{ ...base(op), type: 'insertRows', index, count }, ...bandRestore(wb, op, sheet, 'row', index, count), ...refs, ...header];
        }
        default: {
          // Filters of the deleted columns come back too.
          const filters: Op[] = [];
          for (let c = index; c < index + count; c++) filters.push(...filterRestore(op, sheet, c));
          return [{ ...base(op), type: 'insertCols', index, count }, ...bandRestore(wb, op, sheet, 'col', index, count), ...refs, ...filters];
        }
      }
    }
    default:
      return [];
  }
}

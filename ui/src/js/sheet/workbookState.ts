import type { Op } from './op';
import { StylePoolMirror, type StyleProps } from './stylePool';

export interface Cell {
  raw: string;
  value?: string;
  valueType?: string;
  styleId?: number;
}

export interface SheetState {
  id: string;
  name: string;
  cells: Map<string, Cell>; // key "row:col"
  colWidths: Map<number, number>; // sparse px overrides
  rowHeights: Map<number, number>;
  frozenRows: number; // 0 or 1
  frozenCols: number;
}

const key = (row: number, col: number): string => `${row}:${col}`;
const parseKey = (k: string): [number, number] => {
  const i = k.indexOf(':');
  return [Number(k.slice(0, i)), Number(k.slice(i + 1))];
};

const cellIsEmpty = (c: Cell): boolean =>
  c.raw === '' && (c.styleId === undefined || c.styleId === 0) && (c.value === undefined || c.value === '');

const emptySheet = (id: string, name: string): SheetState => ({
  id, name, cells: new Map(), colWidths: new Map(), rowHeights: new Map(), frozenRows: 0, frozenCols: 0,
});

// shiftDims mirrors Go shiftDims: rebuild a sparse dimension map after an
// insert (delta>0) / delete of -delta indices at index (in-band entries drop).
const shiftDims = (m: Map<number, number>, index: number, delta: number): Map<number, number> => {
  if (m.size === 0) return m;
  const next = new Map<number, number>();
  for (const [i, v] of m) {
    if (delta < 0 && i >= index && i < index - delta) continue;
    next.set(shiftIdx(i, index, delta), v);
  }
  return next;
};

const shiftIdx = (coord: number, index: number, delta: number): number => {
  if (delta >= 0) return coord >= index ? coord + delta : coord;
  return coord < index ? coord : coord + delta;
};

// Snapshot shapes mirror the Go sheet.WorkbookSnapshot JSON.
export interface CellSnapshot {
  row: number;
  col: number;
  raw: string;
  value?: string;
  valueType?: string;
  styleId?: number;
}
export interface SheetSnapshot {
  id: string;
  name: string;
  cells: CellSnapshot[];
  colWidths?: Record<string, number>; // JSON object keys are stringified indices
  rowHeights?: Record<string, number>;
  frozenRows?: number;
  frozenCols?: number;
}
export interface WorkbookSnapshot {
  sheets: SheetSnapshot[];
  styles?: unknown;
}

// WorkbookState is the client mirror of the Go Workbook. applyOp ports
// lib/sheet/apply.go exactly so client optimistic state matches the server.
export class WorkbookState {
  sheets: SheetState[] = [];
  styles = new StylePoolMirror();

  sheetById(id: string): SheetState | undefined {
    return this.sheets.find((s) => s.id === id);
  }

  addSheet(id: string, name: string): SheetState {
    const s = emptySheet(id, name);
    this.sheets.push(s);
    return s;
  }

  getCell(sheetId: string, row: number, col: number): Cell | undefined {
    return this.sheetById(sheetId)?.cells.get(key(row, col));
  }

  clone(): WorkbookState {
    const cp = new WorkbookState();
    cp.sheets = this.sheets.map((s) => ({
      id: s.id, name: s.name, cells: new Map(s.cells),
      colWidths: new Map(s.colWidths), rowHeights: new Map(s.rowHeights),
      frozenRows: s.frozenRows, frozenCols: s.frozenCols,
    }));
    cp.styles = this.styles; // shared pool: interning is monotonic + content-deduped
    return cp;
  }

  loadSnapshot(snap: WorkbookSnapshot): void {
    this.sheets = (snap.sheets ?? []).map((ss) => {
      const s = emptySheet(ss.id, ss.name);
      for (const c of ss.cells ?? []) {
        s.cells.set(key(c.row, c.col), {
          raw: c.raw,
          value: c.value,
          valueType: c.valueType,
          styleId: c.styleId,
        });
      }
      for (const [i, v] of Object.entries(ss.colWidths ?? {})) s.colWidths.set(Number(i), v);
      for (const [i, v] of Object.entries(ss.rowHeights ?? {})) s.rowHeights.set(Number(i), v);
      s.frozenRows = ss.frozenRows ?? 0;
      s.frozenCols = ss.frozenCols ?? 0;
      return s;
    });
    this.styles.seed(snap.styles);
  }

  // Returns the LIVE pool object for the cell's style — never mutate it in
  // place; spread it into a new object (e.g. via mergeProps) before changing.
  getStyleProps(sheetId: string, row: number, col: number): StyleProps {
    const id = this.getCell(sheetId, row, col)?.styleId ?? 0;
    return this.styles.get(id) ?? {};
  }

  private setCell(sheet: SheetState, row: number, col: number, cell: Cell): void {
    if (cellIsEmpty(cell)) {
      sheet.cells.delete(key(row, col));
      return;
    }
    sheet.cells.set(key(row, col), cell);
  }

  private remap(sheet: SheetState, fn: (row: number, col: number) => [number, number, boolean]): void {
    const next = new Map<string, Cell>();
    for (const [k, cell] of sheet.cells) {
      const [r, c] = parseKey(k);
      const [nr, nc, keep] = fn(r, c);
      if (keep) next.set(key(nr, nc), cell);
    }
    sheet.cells = next;
  }

  // applyOp mirrors Go Workbook.Apply. The op is assumed already rebased to the
  // current revision. Cell ops are last-writer-wins.
  applyOp(op: Op): void {
    // Sheet-list ops manage the sheet list itself (mirrors Go Apply).
    switch (op.type) {
      case 'addSheet': {
        if (this.sheetById(op.sheet)) return; // concurrent duplicate add: first wins
        const s = emptySheet(op.sheet, op.name ?? '');
        this.sheets.splice(Math.min(op.index ?? 0, this.sheets.length), 0, s);
        return;
      }
      case 'deleteSheet': {
        if (this.sheets.length <= 1) return; // never delete the last sheet
        const i = this.sheets.findIndex((s) => s.id === op.sheet);
        if (i >= 0) this.sheets.splice(i, 1);
        return;
      }
      case 'renameSheet': {
        const s = this.sheetById(op.sheet);
        if (s) s.name = op.name ?? s.name;
        return;
      }
      case 'moveSheet': {
        const i = this.sheets.findIndex((s) => s.id === op.sheet);
        if (i < 0) return;
        const [s] = this.sheets.splice(i, 1);
        this.sheets.splice(Math.min(op.toIndex ?? 0, this.sheets.length), 0, s);
        return;
      }
    }

    const sheet = this.sheetById(op.sheet);
    // Deleted by an earlier-ordered op: converge as a no-op (mirrors Go Apply).
    if (!sheet) return;

    const row = op.row ?? 0;
    const col = op.col ?? 0;
    const index = op.index ?? 0;
    const count = op.count ?? 0;

    switch (op.type) {
      case 'setCell': {
        const cur: Cell = { ...(sheet.cells.get(key(row, col)) ?? { raw: '' }) };
        if (op.raw !== undefined) {
          cur.raw = op.raw;
          cur.value = undefined;
          cur.valueType = undefined;
        }
        if (op.value !== undefined) cur.value = op.value;
        if (op.valueType !== undefined) cur.valueType = op.valueType;
        if (op.props !== undefined) {
          cur.styleId = this.styles.put(op.props);
        } else if (op.styleId !== undefined) {
          cur.styleId = op.styleId;
        }
        this.setCell(sheet, row, col, cur);
        break;
      }
      case 'setStyle': {
        const cur: Cell = { ...(sheet.cells.get(key(row, col)) ?? { raw: '' }) };
        cur.styleId = op.props !== undefined ? this.styles.put(op.props) : op.styleId;
        this.setCell(sheet, row, col, cur);
        break;
      }
      case 'clearRange': {
        const endRow = op.endRow ?? 0;
        const endCol = op.endCol ?? 0;
        for (const k of [...sheet.cells.keys()]) {
          const [r, c] = parseKey(k);
          if (r >= row && r <= endRow && c >= col && c <= endCol) sheet.cells.delete(k);
        }
        break;
      }
      case 'setDimension': {
        // Mirror the Go server validation: axis col/row, size 1..4096.
        const size = op.size ?? 0;
        if ((op.axis !== 'col' && op.axis !== 'row') || !Number.isInteger(size) || size <= 0 || size > 4096) break;
        if (op.axis === 'col') sheet.colWidths.set(index, size);
        else sheet.rowHeights.set(index, size);
        break;
      }
      case 'setFreeze':
        sheet.frozenRows = op.frozenRows ?? 0;
        sheet.frozenCols = op.frozenCols ?? 0;
        break;
      case 'insertRows':
        this.remap(sheet, (r, c) => (r >= index ? [r + count, c, true] : [r, c, true]));
        sheet.rowHeights = shiftDims(sheet.rowHeights, index, count);
        break;
      case 'deleteRows':
        this.remap(sheet, (r, c) => {
          if (r >= index && r < index + count) return [r, c, false];
          if (r >= index + count) return [r - count, c, true];
          return [r, c, true];
        });
        sheet.rowHeights = shiftDims(sheet.rowHeights, index, -count);
        break;
      case 'insertCols':
        this.remap(sheet, (r, c) => (c >= index ? [r, c + count, true] : [r, c, true]));
        sheet.colWidths = shiftDims(sheet.colWidths, index, count);
        break;
      case 'deleteCols':
        this.remap(sheet, (r, c) => {
          if (c >= index && c < index + count) return [r, c, false];
          if (c >= index + count) return [r, c - count, true];
          return [r, c, true];
        });
        sheet.colWidths = shiftDims(sheet.colWidths, index, -count);
        break;
      default:
        throw new Error(`applyOp: unhandled op type ${(op as Op).type}`);
    }
  }
}

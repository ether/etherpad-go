import type { Op } from './op';

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
}

const key = (row: number, col: number): string => `${row}:${col}`;
const parseKey = (k: string): [number, number] => {
  const i = k.indexOf(':');
  return [Number(k.slice(0, i)), Number(k.slice(i + 1))];
};

const cellIsEmpty = (c: Cell): boolean =>
  c.raw === '' && (c.styleId === undefined || c.styleId === 0) && (c.value === undefined || c.value === '');

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
}
export interface WorkbookSnapshot {
  sheets: SheetSnapshot[];
  styles?: unknown;
}

// WorkbookState is the client mirror of the Go Workbook. applyOp ports
// lib/sheet/apply.go exactly so client optimistic state matches the server.
export class WorkbookState {
  sheets: SheetState[] = [];

  sheetById(id: string): SheetState | undefined {
    return this.sheets.find((s) => s.id === id);
  }

  addSheet(id: string, name: string): SheetState {
    const s: SheetState = { id, name, cells: new Map() };
    this.sheets.push(s);
    return s;
  }

  getCell(sheetId: string, row: number, col: number): Cell | undefined {
    return this.sheetById(sheetId)?.cells.get(key(row, col));
  }

  loadSnapshot(snap: WorkbookSnapshot): void {
    this.sheets = (snap.sheets ?? []).map((ss) => {
      const cells = new Map<string, Cell>();
      for (const c of ss.cells ?? []) {
        cells.set(key(c.row, c.col), {
          raw: c.raw,
          value: c.value,
          valueType: c.valueType,
          styleId: c.styleId,
        });
      }
      return { id: ss.id, name: ss.name, cells };
    });
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
    const sheet = this.sheetById(op.sheet);
    if (!sheet) throw new Error(`applyOp: unknown sheet ${op.sheet}`);

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
        if (op.styleId !== undefined) cur.styleId = op.styleId;
        this.setCell(sheet, row, col, cur);
        break;
      }
      case 'setStyle': {
        const cur: Cell = { ...(sheet.cells.get(key(row, col)) ?? { raw: '' }) };
        cur.styleId = op.styleId;
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
      case 'insertRows':
        this.remap(sheet, (r, c) => (r >= index ? [r + count, c, true] : [r, c, true]));
        break;
      case 'deleteRows':
        this.remap(sheet, (r, c) => {
          if (r >= index && r < index + count) return [r, c, false];
          if (r >= index + count) return [r - count, c, true];
          return [r, c, true];
        });
        break;
      case 'insertCols':
        this.remap(sheet, (r, c) => (c >= index ? [r, c + count, true] : [r, c, true]));
        break;
      case 'deleteCols':
        this.remap(sheet, (r, c) => {
          if (c >= index && c < index + count) return [r, c, false];
          if (c >= index + count) return [r, c - count, true];
          return [r, c, true];
        });
        break;
      default:
        throw new Error(`applyOp: unhandled op type ${(op as Op).type}`);
    }
  }
}

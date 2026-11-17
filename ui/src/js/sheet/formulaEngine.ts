import { HyperFormula, type ExportedChange, type ExportedCellChange } from 'hyperformula';
import { registerExcelFunctions } from './excelFunctions';

export interface CellResult {
  value: string;
  type: 'number' | 'text' | 'bool' | 'error' | 'empty';
}

export interface SetCellResult extends CellResult {
  // cells whose computed value changed as a result (dependents), incl. the set cell
  changed: Array<{ row: number; col: number }>;
}

// FormulaEngine wraps HyperFormula (GPLv3 — accepted in plan 1) behind a small
// interface so the engine stays swappable. raw is the source of truth; this
// derives the computed value/type, exactly the model in the design spec.
type EngineCells = Array<{ row: number; col: number; raw: string }>;

export interface EngineSheet {
  name: string;
  // A function is only called when the sheet has to be loaded.
  cells: EngineCells | (() => EngineCells);
  // Changes whenever the content does (e.g. a per-sheet stamp plus the live
  // edits); an unchanged key skips the sheet without reading its cells.
  key?: string;
}

export class FormulaEngine {
  private hf: HyperFormula;
  private sheetId: number;
  // Workbook mirror: sheet names in order and the content last loaded per
  // sheet, so an unchanged sheet is not loaded again.
  private names: string[] = ['Sheet1'];
  private loaded = new Map<string, string>();

  constructor() {
    registerExcelFunctions();
    this.hf = HyperFormula.buildEmpty({ licenseKey: 'gpl-v3' });
    const name = this.hf.addSheet('Sheet1');
    this.sheetId = this.hf.getSheetId(name) as number;
  }

  // setWorkbook loads every sheet, so formulas can reference other sheets
  // (=Sheet2!A1, ='My Data'!B2), and makes `active` the sheet getValue reads.
  setWorkbook(sheets: EngineSheet[], active: string): void {
    const names = sheets.map((s) => s.name);
    if (names.length === 0) return;
    if (names.join('\u0000') !== this.names.join('\u0000')) {
      // Sheets added, removed, renamed or moved: start over.
      this.hf.destroy();
      this.hf = HyperFormula.buildEmpty({ licenseKey: 'gpl-v3' });
      this.loaded.clear();
      for (const name of names) {
        try {
          this.hf.addSheet(name);
        } catch {
          // A name HyperFormula refuses (or a duplicate): the sheet is
          // still computed, it just cannot be referenced by name.
          this.hf.addSheet();
        }
      }
      this.names = names;
    }
    const ids = this.hf.getSheetNames().map((n) => this.hf.getSheetId(n) as number);
    this.hf.batch(() => {
      sheets.forEach((s, i) => {
        const slot = String(i);
        if (s.key !== undefined && this.loaded.get(slot) === `k:${s.key}`) return;
        const grid = toGrid(typeof s.cells === 'function' ? s.cells() : s.cells);
        const sig = s.key !== undefined ? `k:${s.key}` : JSON.stringify(grid);
        if (this.loaded.get(slot) === sig) return;
        this.loaded.set(slot, sig);
        this.hf.setSheetContent(ids[i], grid);
      });
    });
    const i = names.indexOf(active);
    this.sheetId = ids[i >= 0 ? i : 0];
  }

  private mapType(row: number, col: number): CellResult['type'] {
    switch (this.hf.getCellValueType({ sheet: this.sheetId, row, col })) {
      case 'NUMBER':
        return 'number';
      case 'BOOLEAN':
        return 'bool';
      case 'ERROR':
        return 'error';
      case 'EMPTY':
        return 'empty';
      default:
        return 'text';
    }
  }

  // height is the number of rows the active sheet uses, spilled arrays
  // included.
  height(): number {
    return this.hf.getSheetDimensions(this.sheetId).height;
  }

  getValue(row: number, col: number): CellResult {
    const v = this.hf.getCellValue({ sheet: this.sheetId, row, col });
    const type = this.mapType(row, col);
    return { value: v == null ? '' : String(v), type };
  }

  // setGrid replaces the whole sheet content from a list of populated cells,
  // keeping the engine in sync with the collaborative workbook state (cells
  // removed by structural ops are dropped because the grid is rebuilt).
  setGrid(cells: Array<{ row: number; col: number; raw: string }>): void {
    this.hf.setSheetContent(this.sheetId, toGrid(cells));
    this.loaded.clear();
  }

  // setCell sets the raw content and returns its computed result plus the list
  // of cells whose values changed (for targeted re-render).
  setCell(row: number, col: number, raw: string): SetCellResult {
    const changes: ExportedChange[] = this.hf.setCellContents({ sheet: this.sheetId, row, col }, raw);
    const changed = changes
      .filter((c): c is ExportedCellChange => 'address' in c && c.address.sheet === this.sheetId)
      .map((c) => ({ row: c.address.row, col: c.address.col }));
    return { ...this.getValue(row, col), changed };
  }

  // functionNames returns the HyperFormula-registered function names (for the
  // formula bar's autocomplete). Uses the default 'enGB' language pack that
  // buildEmpty registers.
  functionNames(): string[] {
    return HyperFormula.getRegisteredFunctionNames('enGB');
  }
}

function toGrid(cells: Array<{ row: number; col: number; raw: string }>): string[][] {
  let maxRow = -1;
  let maxCol = -1;
  for (const c of cells) {
    if (c.row > maxRow) maxRow = c.row;
    if (c.col > maxCol) maxCol = c.col;
  }
  const grid: string[][] = [];
  for (let r = 0; r <= maxRow; r++) {
    grid.push(new Array<string>(maxCol + 1).fill(''));
  }
  for (const c of cells) {
    grid[c.row][c.col] = c.raw;
  }
  return grid;
}

import { HyperFormula, type ExportedChange, type ExportedCellChange } from 'hyperformula';

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
export class FormulaEngine {
  private hf: HyperFormula;
  private sheetId: number;

  constructor() {
    this.hf = HyperFormula.buildEmpty({ licenseKey: 'gpl-v3' });
    const name = this.hf.addSheet('Sheet1');
    this.sheetId = this.hf.getSheetId(name) as number;
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

  getValue(row: number, col: number): CellResult {
    const v = this.hf.getCellValue({ sheet: this.sheetId, row, col });
    const type = this.mapType(row, col);
    return { value: v == null ? '' : String(v), type };
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
}

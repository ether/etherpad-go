// DomSheetView is a minimal, framework-agnostic spreadsheet grid rendered as an
// HTML table with contenteditable cells. It is intentionally simple (v1); the
// SheetView contract (rawValue/displayValue/onEdit) lets a virtualized canvas
// grid replace it later without touching the collaboration layer.

export interface SheetViewOptions {
  rows: number;
  cols: number;
  rawValue: (row: number, col: number) => string;
  displayValue: (row: number, col: number) => string;
  onEdit: (row: number, col: number, raw: string) => void;
}

function colName(c: number): string {
  let s = '';
  let n = c + 1;
  while (n > 0) {
    const rem = (n - 1) % 26;
    s = String.fromCharCode(65 + rem) + s;
    n = Math.floor((n - 1) / 26);
  }
  return s;
}

const STYLE_ID = 'sheet-grid-style';
const CSS = `
.sheet-grid { border-collapse: collapse; font: 13px/1.4 system-ui, sans-serif; }
.sheet-grid th, .sheet-grid td { border: 1px solid #d2d2d2; min-width: 80px; height: 22px; padding: 2px 6px; }
.sheet-grid th { background: #f2f3f4; color: #485365; font-weight: 600; text-align: center; }
.sheet-grid td { outline: none; }
.sheet-grid td:focus { box-shadow: inset 0 0 0 2px #64d29b; }
`;

export class DomSheetView {
  private opts: SheetViewOptions;
  private cells: HTMLTableCellElement[][] = [];
  private editing: { row: number; col: number } | null = null;

  constructor(root: HTMLElement, opts: SheetViewOptions) {
    this.opts = opts;
    this.ensureStyle();
    root.innerHTML = '';

    const table = document.createElement('table');
    table.className = 'sheet-grid';

    const thead = document.createElement('thead');
    const headRow = document.createElement('tr');
    headRow.appendChild(document.createElement('th')); // corner
    for (let c = 0; c < opts.cols; c++) {
      const th = document.createElement('th');
      th.textContent = colName(c);
      headRow.appendChild(th);
    }
    thead.appendChild(headRow);
    table.appendChild(thead);

    const tbody = document.createElement('tbody');
    for (let r = 0; r < opts.rows; r++) {
      const tr = document.createElement('tr');
      const rowHead = document.createElement('th');
      rowHead.textContent = String(r + 1);
      tr.appendChild(rowHead);
      const rowCells: HTMLTableCellElement[] = [];
      for (let c = 0; c < opts.cols; c++) {
        const td = document.createElement('td');
        td.contentEditable = 'true';
        this.attach(td, r, c);
        tr.appendChild(td);
        rowCells.push(td);
      }
      this.cells.push(rowCells);
      tbody.appendChild(tr);
    }
    table.appendChild(tbody);
    root.appendChild(table);
    this.render();
  }

  private ensureStyle(): void {
    if (document.getElementById(STYLE_ID)) return;
    const style = document.createElement('style');
    style.id = STYLE_ID;
    style.textContent = CSS;
    document.head.appendChild(style);
  }

  private attach(td: HTMLTableCellElement, r: number, c: number): void {
    td.addEventListener('focus', () => {
      this.editing = { row: r, col: c };
      td.textContent = this.opts.rawValue(r, c);
    });
    td.addEventListener('blur', () => {
      const raw = td.textContent ?? '';
      this.editing = null;
      if (raw !== this.opts.rawValue(r, c)) {
        this.opts.onEdit(r, c, raw);
      }
      td.textContent = this.opts.displayValue(r, c);
    });
    td.addEventListener('keydown', (e: KeyboardEvent) => {
      if (e.key === 'Enter') {
        e.preventDefault();
        td.blur();
      }
    });
  }

  // render refreshes every non-editing cell to its display value.
  render(): void {
    for (let r = 0; r < this.opts.rows; r++) {
      for (let c = 0; c < this.opts.cols; c++) {
        if (this.editing && this.editing.row === r && this.editing.col === c) continue;
        this.cells[r][c].textContent = this.opts.displayValue(r, c);
      }
    }
  }
}

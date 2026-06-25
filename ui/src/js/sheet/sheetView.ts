// DomSheetView is a minimal, framework-agnostic spreadsheet grid rendered as an
// HTML table with contenteditable cells. The SheetView contract lets a canvas
// grid replace it later without touching the collaboration layer.

export interface RemoteCursorDeco {
  userId: string;
  name: string;
  color: string;
  row: number;
  col: number;
}

export interface RemoteLiveEditDeco extends RemoteCursorDeco {
  raw: string;
}

export interface SheetViewOptions {
  rows: number;
  cols: number;
  rawValue: (row: number, col: number) => string;
  displayValue: (row: number, col: number) => string;
  onEdit: (row: number, col: number, raw: string) => void;
  onSelect?: (row: number, col: number) => void;
  onLiveEdit?: (row: number, col: number, raw: string) => void;
  onEditEnd?: (row: number, col: number, committed: boolean) => void;
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
.sheet-grid td { outline: none; position: relative; }
.sheet-grid td:focus { box-shadow: inset 0 0 0 2px #64d29b; }
.sheet-remote-tag { position: absolute; top: -15px; left: -1px; font: 10px/14px system-ui, sans-serif; padding: 0 4px; color: #fff; border-radius: 3px 3px 3px 0; white-space: nowrap; z-index: 5; pointer-events: none; }
.sheet-remote-tag::after { content: attr(data-label); }
`;

export class DomSheetView {
  private opts: SheetViewOptions;
  private cells: HTMLTableCellElement[][] = [];
  private editing: { row: number; col: number } | null = null;
  private escaped = false;
  private cursorByKey = new Map<string, RemoteCursorDeco>();
  private liveByKey = new Map<string, RemoteLiveEditDeco>();
  private decorated = new Set<HTMLTableCellElement>();

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
      this.escaped = false;
      td.style.boxShadow = '';
      td.querySelector('.sheet-remote-tag')?.remove();
      td.textContent = this.opts.rawValue(r, c);
      this.opts.onSelect?.(r, c);
    });
    td.addEventListener('input', () => {
      this.opts.onLiveEdit?.(r, c, td.textContent ?? '');
    });
    td.addEventListener('keydown', (e: KeyboardEvent) => {
      if (e.key === 'Enter') {
        e.preventDefault();
        td.blur();
      } else if (e.key === 'Escape') {
        e.preventDefault();
        this.escaped = true;
        td.blur();
      }
    });
    td.addEventListener('blur', () => {
      const raw = td.textContent ?? '';
      const prev = this.opts.rawValue(r, c);
      this.editing = null;
      if (this.escaped) {
        this.opts.onEditEnd?.(r, c, false);
      } else {
        const committed = raw !== prev;
        if (committed) this.opts.onEdit(r, c, raw);
        this.opts.onEditEnd?.(r, c, committed);
      }
      this.escaped = false;
      td.textContent = this.opts.displayValue(r, c);
      this.render();
    });
  }

  // setRemoteCursors / setRemoteLiveEdits replace the decoration sets. Call
  // render() afterwards (the editor batches both then renders once).
  setRemoteCursors(list: RemoteCursorDeco[]): void {
    this.cursorByKey = new Map(list.map((d) => [`${d.row}:${d.col}`, d]));
  }

  setRemoteLiveEdits(list: RemoteLiveEditDeco[]): void {
    this.liveByKey = new Map(list.map((d) => [`${d.row}:${d.col}`, d]));
  }

  // render refreshes every non-editing cell to its display value, then paints
  // remote live-edit text and cursor/live-edit decorations.
  render(): void {
    for (const td of this.decorated) {
      td.style.boxShadow = '';
      td.querySelector('.sheet-remote-tag')?.remove();
    }
    this.decorated.clear();

    for (let r = 0; r < this.opts.rows; r++) {
      for (let c = 0; c < this.opts.cols; c++) {
        if (this.editing && this.editing.row === r && this.editing.col === c) continue;
        const td = this.cells[r][c];
        const k = `${r}:${c}`;
        const live = this.liveByKey.get(k);
        td.textContent = live ? live.raw : this.opts.displayValue(r, c);
        const deco: RemoteCursorDeco | undefined = live ?? this.cursorByKey.get(k);
        if (deco) {
          td.style.boxShadow = `inset 0 0 0 2px ${deco.color}`;
          const tag = document.createElement('span');
          tag.className = 'sheet-remote-tag';
          // Label via a CSS ::after pseudo-element (data-label), NOT a text node,
          // so the peer's name never leaks into the cell's textContent.
          tag.setAttribute('data-label', deco.name || 'anon');
          tag.style.background = deco.color;
          td.appendChild(tag);
          this.decorated.add(td);
        }
      }
    }
  }
}

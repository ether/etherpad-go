// DomSheetView is a minimal, framework-agnostic spreadsheet grid rendered as an
// HTML table with contenteditable cells. The SheetView contract lets a canvas
// grid replace it later without touching the collaboration layer.

import { type Selection, selFromSingle, normalize, selContains } from './sheetSelection';

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
  onSelectionChange?: (sel: Selection) => void;
  onLiveEdit?: (row: number, col: number, raw: string) => void;
  onEditEnd?: (row: number, col: number, committed: boolean) => void;
  onFill?: (src: Selection, target: Selection) => void;
  readOnly?: boolean;
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
.sheet-grid td.sheet-sel { background: rgba(100, 210, 155, 0.15); }
.sheet-grid td.sheet-sel-focus { box-shadow: inset 0 0 0 2px #2f9e6b; }
.sheet-grid td.sheet-remote-sel { box-shadow: inset 0 0 0 2px var(--rsel, #888); }
.sheet-fill-handle { position: absolute; right: -4px; bottom: -4px; width: 8px; height: 8px; background: #2f9e6b; border: 1px solid #fff; cursor: crosshair; z-index: 6; }
.sheet-grid td.sheet-fill-target { box-shadow: inset 0 0 0 1px #2f9e6b; }
`;

export class DomSheetView {
  private opts: SheetViewOptions;
  private cells: HTMLTableCellElement[][] = [];
  private editing: { row: number; col: number } | null = null;
  private escaped = false;
  private activeEdit = false;
  private cursorByKey = new Map<string, RemoteCursorDeco>();
  private liveByKey = new Map<string, RemoteLiveEditDeco>();
  private decorated = new Set<HTMLTableCellElement>();
  private selection: Selection = selFromSingle(0, 0);
  private dragging = false;
  private filling = false;
  private fillSrc: Selection | null = null;
  private fillTarget: Selection | null = null;
  private remoteSel: Array<{ userId: string; color: string; sel: Selection }> = [];

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
        // Read-only viewers get non-editable cells: with no typing, no live-edit
        // or commit frames originate from the client (the server strips them too).
        td.contentEditable = opts.readOnly ? 'false' : 'true';
        this.attach(td, r, c);
        tr.appendChild(td);
        rowCells.push(td);
      }
      this.cells.push(rowCells);
      tbody.appendChild(tr);
    }
    table.appendChild(tbody);
    root.appendChild(table);
    document.addEventListener('mouseup', () => {
      if (this.filling && this.fillSrc && this.fillTarget) {
        this.opts.onFill?.(this.fillSrc, this.fillTarget);
        this.selection = this.fillTarget;
        this.opts.onSelectionChange?.(this.selection);
      }
      this.dragging = false;
      this.filling = false;
      this.fillSrc = null;
      this.fillTarget = null;
      this.render();
    });
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
    td.addEventListener('mousedown', (e: MouseEvent) => {
      if (e.shiftKey) {
        this.selection = { anchor: this.selection.anchor, focus: { row: r, col: c } };
      } else {
        this.selection = selFromSingle(r, c);
        this.dragging = true;
      }
      this.opts.onSelectionChange?.(this.selection);
      this.render();
    });
    td.addEventListener('mouseover', () => {
      if (this.filling && this.fillSrc) {
        const { r0, c0 } = normalize(this.fillSrc);
        this.fillTarget = { anchor: { row: r0, col: c0 }, focus: { row: r, col: c } };
        this.render();
        return;
      }
      if (!this.dragging) return;
      this.selection = { anchor: this.selection.anchor, focus: { row: r, col: c } };
      this.opts.onSelectionChange?.(this.selection);
      this.render();
    });
    td.addEventListener('focus', () => {
      this.editing = { row: r, col: c };
      this.escaped = false;
      this.activeEdit = false;
      td.style.boxShadow = '';
      td.querySelector('.sheet-remote-tag')?.remove();
      td.textContent = this.opts.rawValue(r, c);
      this.opts.onSelect?.(r, c);
    });
    td.addEventListener('input', () => {
      this.activeEdit = true;
      this.opts.onLiveEdit?.(r, c, td.textContent ?? '');
    });
    td.addEventListener('keydown', (e: KeyboardEvent) => {
      const move = (dr: number, dc: number, extend: boolean) => {
        e.preventDefault();
        const f = this.selection.focus;
        const nr = Math.min(this.opts.rows - 1, Math.max(0, f.row + dr));
        const nc = Math.min(this.opts.cols - 1, Math.max(0, f.col + dc));
        this.selection = extend
          ? { anchor: this.selection.anchor, focus: { row: nr, col: nc } }
          : selFromSingle(nr, nc);
        if (!extend) this.opts.onSelect?.(nr, nc);
        this.opts.onSelectionChange?.(this.selection);
        this.render();
      };
      if (e.key === 'ArrowUp') return move(-1, 0, e.shiftKey);
      if (e.key === 'ArrowDown') return move(1, 0, e.shiftKey);
      if (e.key === 'ArrowLeft') return move(0, -1, e.shiftKey);
      if (e.key === 'ArrowRight') return move(0, 1, e.shiftKey);
      if ((e.ctrlKey || e.metaKey) && (e.key === 'a' || e.key === 'A')) {
        e.preventDefault();
        this.selection = { anchor: { row: 0, col: 0 }, focus: { row: this.opts.rows - 1, col: this.opts.cols - 1 } };
        this.opts.onSelectionChange?.(this.selection);
        return this.render();
      }
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
      this.activeEdit = false;
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

  getSelection(): Selection {
    return this.selection;
  }

  // isEditing reports whether the user is actively typing into a cell (as
  // opposed to merely having a cell selected/focused). Clipboard and
  // range-delete shortcuts must NOT fire while actively editing.
  isEditing(): boolean {
    return this.activeEdit;
  }

  setRemoteSelections(list: Array<{ userId: string; color: string; sel: Selection }>): void {
    this.remoteSel = list;
  }

  // render refreshes every non-editing cell to its display value, then paints
  // remote live-edit text and cursor/live-edit decorations.
  render(): void {
    for (const td of this.decorated) {
      td.style.boxShadow = '';
      td.classList.remove('sheet-remote-sel', 'sheet-fill-target');
      td.style.removeProperty('--rsel');
      td.querySelector('.sheet-remote-tag')?.remove();
      td.querySelector('.sheet-fill-handle')?.remove();
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

    // local selection
    for (let r = 0; r < this.opts.rows; r++) {
      for (let c = 0; c < this.opts.cols; c++) {
        const td = this.cells[r][c];
        td.classList.toggle('sheet-sel', selContains(this.selection, r, c));
        td.classList.toggle(
          'sheet-sel-focus',
          r === this.selection.focus.row && c === this.selection.focus.col,
        );
      }
    }
    // remote selections (outline only, multi-cell)
    for (const rs of this.remoteSel) {
      const { r0, c0, r1, c1 } = normalize(rs.sel);
      if (r1 - r0 === 0 && c1 - c0 === 0) continue; // single cell handled by cursor deco
      for (let r = r0; r <= r1; r++) {
        for (let c = c0; c <= c1; c++) {
          const td = this.cells[r]?.[c];
          if (!td) continue;
          td.style.setProperty('--rsel', rs.color);
          td.classList.add('sheet-remote-sel');
          this.decorated.add(td);
        }
      }
    }

    // fill handle at the bottom-right corner of the current selection
    const { r1, c1 } = normalize(this.selection);
    const brCell = this.cells[r1]?.[c1];
    if (brCell && !this.opts.readOnly) {
      const h = document.createElement('span');
      h.className = 'sheet-fill-handle';
      h.addEventListener('mousedown', (e: MouseEvent) => {
        e.stopPropagation();
        e.preventDefault();
        this.filling = true;
        this.fillSrc = this.selection;
        const { r0, c0, r1, c1 } = normalize(this.selection);
        this.fillTarget = { anchor: { row: r0, col: c0 }, focus: { row: r1, col: c1 } };
      });
      brCell.appendChild(h);
      this.decorated.add(brCell);
    }
    if (this.fillTarget) {
      const t = normalize(this.fillTarget);
      for (let r = t.r0; r <= t.r1; r++) {
        for (let c = t.c0; c <= t.c1; c++) {
          const td = this.cells[r]?.[c];
          if (td) { td.classList.add('sheet-fill-target'); this.decorated.add(td); }
        }
      }
    }
  }
}

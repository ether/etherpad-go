// DomSheetView is a minimal, framework-agnostic spreadsheet grid rendered as an
// HTML table with contenteditable cells. The SheetView contract lets a canvas
// grid replace it later without touching the collaboration layer.

import { type Selection, selFromSingle, normalize, selContains } from './sheetSelection';
import { styleToCss } from './styleCss';
import { colName } from './a1';

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
  styleOf?: (row: number, col: number) => Record<string, string>;
  errorOf?: (row: number, col: number) => string | undefined;
  // Alignment of a cell without its own: Excel puts numbers on the right,
  // booleans and errors in the middle.
  defaultAlignOf?: (row: number, col: number) => 'right' | 'center' | undefined;
  // Hyperlink, note and list validation of a cell (cellMeta.ts).
  metaOf?: (row: number, col: number) => { link?: string; note?: string; list?: string[] };
  // Formatting from matching conditional-formatting rules; wins over the cell's own.
  condStyleOf?: (row: number, col: number) => { bg?: string; color?: string; bold?: boolean };
  // M4: sparse dimension overrides (px), freeze state, and resize commits.
  colWidth?: (col: number) => number | undefined;
  rowHeight?: (row: number) => number | undefined;
  frozen?: () => { rows: number; cols: number };
  onResize?: (axis: 'col' | 'row', index: number, sizePx: number) => void;
  // Rows hidden by the sheet's filters collapse via display:none.
  rowHidden?: (row: number) => boolean;
  // Columns with a filter get a marker in their header.
  colFiltered?: (col: number) => boolean;
  // Merged ranges (top-left anchor + span). The anchor td gets rowSpan/colSpan,
  // covered tds are display:none.
  merges?: () => Array<{ row: number; col: number; rows: number; cols: number }>;
}

interface RowEl {
  tr: HTMLTableRowElement;
  th: HTMLTableCellElement;
  tds: HTMLTableCellElement[];
}

const STYLE_ID = 'sheet-grid-style';
// border-collapse: separate (not collapse) because position: sticky drops
// collapsed borders while a frozen row/col sticks. Right/bottom-only borders
// keep the 1px grid look without doubling.
const CSS = `
.sheet-grid { border-collapse: separate; border-spacing: 0; border-top: 1px solid #d4d4d4; border-left: 1px solid #d4d4d4; font: 13px/1.4 system-ui, sans-serif; }
.sheet-grid th, .sheet-grid td { border-right: 1px solid #d4d4d4; border-bottom: 1px solid #d4d4d4; min-width: 80px; height: 22px; padding: 2px 6px; }
.sheet-grid th { background: #f5f6f7; color: #5f6b7a; font-weight: 600; text-align: center; position: relative; }
/* Excel keeps headers visible while the grid scrolls: column letters stick to
   the top, row numbers to the left of the scroll container (sheet-grid-host). */
.sheet-grid thead th { position: sticky; top: 0; z-index: 8; }
.sheet-grid thead th:first-child { left: 0; z-index: 9; }
.sheet-grid tbody th { position: sticky; left: 0; z-index: 4; }
.sheet-grid th.sheet-head-hl { background: #e6f2ec; color: #0f6b3a; }
.sheet-grid thead th.sheet-head-hl { box-shadow: inset 0 -2px 0 #107c41; }
.sheet-grid tbody th.sheet-head-hl { box-shadow: inset -2px 0 0 #107c41; }
.sheet-grid td { outline: none; position: relative; background: #fff; white-space: nowrap; overflow: hidden; }
.sheet-resizer-col { position: absolute; top: 0; right: -3px; width: 6px; height: 100%; cursor: col-resize; z-index: 9; }
.sheet-resizer-row { position: absolute; left: 0; bottom: -3px; height: 6px; width: 100%; cursor: row-resize; z-index: 9; }
.sheet-grid thead th.sheet-head-filter::after { content: ' ▾'; color: #107c41; }
.sheet-grid tr.sheet-spacer td { padding: 0; border: none; background: transparent; min-width: 0; height: auto; }
.sheet-grid td:focus { box-shadow: inset 0 0 0 2px #107c41; overflow: visible; z-index: 3; }
.sheet-remote-tag { position: absolute; top: -15px; left: -1px; font: 10px/14px system-ui, sans-serif; padding: 0 4px; color: #fff; border-radius: 3px 3px 3px 0; white-space: nowrap; z-index: 5; pointer-events: none; }
.sheet-remote-tag::after { content: attr(data-label); }
.sheet-grid td.sheet-sel { background: rgba(16, 124, 65, 0.10); }
.sheet-grid td.sheet-sel-focus { box-shadow: inset 0 0 0 2px #107c41; }
.sheet-grid td.sheet-remote-sel { box-shadow: inset 0 0 0 2px var(--rsel, #888); }
.sheet-fill-handle { position: absolute; width: 8px; height: 8px; background: #107c41; border: 1px solid #fff; cursor: crosshair; z-index: 6; }
.sheet-grid td.sheet-fill-target { box-shadow: inset 0 0 0 1px #107c41; }
.sheet-grid td.sheet-cell-error { color: #c0392b; }
.sheet-grid td.sheet-link { color: #0563c1; text-decoration: underline; }
.sheet-grid td.sheet-has-note::after { content: ''; position: absolute; top: 0; right: 0; border-left: 7px solid transparent; border-top: 7px solid #d9342b; pointer-events: none; }
.sheet-list-btn { position: absolute; z-index: 6; width: 18px; height: 18px; padding: 0; border: 1px solid #c8cdd3; background: #fff; border-radius: 2px; font: 10px/16px system-ui, sans-serif; color: #444; cursor: pointer; }
.sheet-list-btn:hover { background: #e6f2ec; }
.sheet-list-menu { position: absolute; z-index: 20; min-width: 120px; max-height: 220px; overflow-y: auto; background: #fff; border: 1px solid #c8cdd3; box-shadow: 0 4px 10px rgba(0,0,0,0.15); padding: 2px 0; font: 13px system-ui, sans-serif; }
.sheet-list-menu button { display: block; width: 100%; text-align: left; border: none; background: none; padding: 4px 10px; cursor: pointer; font: inherit; }
.sheet-list-menu button:hover, .sheet-list-menu button.on { background: #e6f2ec; }
`;

export class DomSheetView {
  private opts: SheetViewOptions;
  // Only the rows near the viewport exist in the DOM (plus frozen rows, the
  // row being edited and rows a merge needs); gaps are spacer rows.
  private rowEls = new Map<number, RowEl>();
  private tbody: HTMLTableSectionElement;
  private rowTops: number[] = [];
  private measured = new Map<number, number>();
  private defaultRowH = 27;
  private minRowH = 0;
  private renderedRange = { start: 0, end: -1 };
  private scrollQueued = false;
  private stickyEls = new Set<HTMLElement>();
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
  // Single fill-handle overlay, positioned over the selection's bottom-right
  // corner. It lives OUTSIDE the contenteditable tds: a decoration inside an
  // editable cell races the caret and re-renders can delete typed text.
  private fillHandle: HTMLSpanElement;
  private listBtn: HTMLButtonElement;
  private listMenu: HTMLDivElement | null = null;
  private listOutside: ((e: MouseEvent) => void) | null = null;
  private root: HTMLElement;
  private mergedTds = new Set<HTMLTableCellElement>();
  private table: HTMLTableElement;
  private thead: HTMLTableSectionElement;
  private colHeads: HTMLTableCellElement[] = [];


  constructor(root: HTMLElement, opts: SheetViewOptions) {
    this.opts = opts;
    this.ensureStyle();
    root.innerHTML = '';
    if (getComputedStyle(root).position === 'static') root.style.position = 'relative';

    const table = document.createElement('table');
    table.className = 'sheet-grid';
    this.table = table;

    this.fillHandle = document.createElement('span');
    this.fillHandle.className = 'sheet-fill-handle';
    this.fillHandle.style.display = 'none';
    this.fillHandle.addEventListener('mousedown', (e: MouseEvent) => {
      e.stopPropagation();
      e.preventDefault();
      this.filling = true;
      this.fillSrc = this.selection;
      const { r0, c0, r1, c1 } = normalize(this.selection);
      this.fillTarget = { anchor: { row: r0, col: c0 }, focus: { row: r1, col: c1 } };
    });
    root.appendChild(this.fillHandle);

    // Data-validation drop-down arrow beside the focused cell, outside the
    // contenteditable tds like the fill handle.
    this.listBtn = document.createElement('button');
    this.listBtn.type = 'button';
    this.listBtn.className = 'sheet-list-btn';
    this.listBtn.textContent = '▼';
    this.listBtn.title = 'Pick from list';
    this.listBtn.style.display = 'none';
    this.listBtn.addEventListener('mousedown', (e) => {
      e.preventDefault();
      e.stopPropagation();
    });
    this.listBtn.addEventListener('click', () => this.toggleListMenu());
    root.appendChild(this.listBtn);
    this.root = root;

    const thead = document.createElement('thead');
    this.thead = thead;
    const headRow = document.createElement('tr');
    headRow.appendChild(document.createElement('th')); // corner
    for (let c = 0; c < opts.cols; c++) {
      const th = document.createElement('th');
      th.textContent = colName(c);
      this.attachResizer(th, 'col', c);
      this.colHeads.push(th);
      headRow.appendChild(th);
    }
    thead.appendChild(headRow);
    table.appendChild(thead);

    const tbody = document.createElement('tbody');
    this.tbody = tbody;
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
    root.addEventListener('scroll', () => {
      if (this.scrollQueued) return;
      this.scrollQueued = true;
      requestAnimationFrame(() => {
        this.scrollQueued = false;
        const v = this.visibleRows();
        if (v.start < this.renderedRange.start || v.end > this.renderedRange.end) this.render();
      });
    });
    this.render();
  }

  private cellEl(r: number, c: number): HTMLTableCellElement | undefined {
    return this.rowEls.get(r)?.tds[c];
  }

  private buildRow(r: number): RowEl {
    const tr = document.createElement('tr');
    tr.dataset.row = String(r);
    const th = document.createElement('th');
    th.textContent = String(r + 1);
    this.attachResizer(th, 'row', r);
    tr.appendChild(th);
    const tds: HTMLTableCellElement[] = [];
    for (let c = 0; c < this.opts.cols; c++) {
      const td = document.createElement('td');
      // Read-only viewers get non-editable cells: with no typing, no live-edit
      // or commit frames originate from the client (the server strips them too).
      td.contentEditable = this.opts.readOnly ? 'false' : 'true';
      this.attach(td, r, c);
      const w = this.opts.colWidth?.(c);
      if (w !== undefined) {
        td.style.minWidth = `${w}px`;
        td.style.maxWidth = `${w}px`;
      }
      tr.appendChild(td);
      tds.push(td);
    }
    return { tr, th, tds };
  }

  private rowHeightOf(r: number): number {
    if (this.opts.rowHidden?.(r)) return 0;
    return this.opts.rowHeight?.(r) ?? this.measured.get(r) ?? this.defaultRowH;
  }

  // visibleRows is the range of rows inside the scrolled viewport.
  private visibleRows(): { start: number; end: number } {
    const head = this.thead.offsetHeight;
    const top = Math.max(0, this.root.scrollTop - head);
    const bottom = top + Math.max(this.root.clientHeight, 600);
    return { start: this.rowAt(top), end: this.rowAt(bottom) };
  }

  private rowAt(y: number): number {
    let lo = 0;
    let hi = this.opts.rows - 1;
    while (lo < hi) {
      const mid = (lo + hi + 1) >> 1;
      if (this.rowTops[mid] <= y) lo = mid;
      else hi = mid - 1;
    }
    return lo;
  }

  // layoutRows puts the rows that should exist into the tbody, in order, with
  // spacer rows standing in for the heights of the rows in between.
  private layoutRows(): void {
    const n = this.opts.rows;
    this.rowTops = new Array(n + 1);
    this.rowTops[0] = 0;
    for (let r = 0; r < n; r++) this.rowTops[r + 1] = this.rowTops[r] + this.rowHeightOf(r);

    const BUFFER = 20;
    const v = this.visibleRows();
    let start = Math.max(0, v.start - BUFFER);
    let end = Math.min(n - 1, v.end + BUFFER);
    // A merge crossing the window's edge needs all its rows (rowSpan counts
    // the following <tr>s).
    const merges = this.opts.merges?.() ?? [];
    for (let changed = true; changed;) {
      changed = false;
      for (const m of merges) {
        const last = m.row + m.rows - 1;
        if (m.row > end || last < start) continue;
        if (m.row < start) { start = m.row; changed = true; }
        if (last > end && last < n) { end = last; changed = true; }
      }
    }
    this.renderedRange = { start, end };
    const want = new Set<number>();
    // Rows a filter hides take no space; leave them out, or a filter hiding
    // most rows would build them all.
    for (let r = start; r <= end; r++) if (!this.opts.rowHidden?.(r)) want.add(r);
    const fz = this.opts.frozen?.() ?? { rows: 0, cols: 0 };
    for (let r = 0; r < Math.min(fz.rows, n); r++) want.add(r);
    if (this.editing) want.add(this.editing.row);
    // Every row a kept merge spans has to exist too (frozen or edited rows
    // can hold a merge reaching outside the window).
    for (let changed = true; changed;) {
      changed = false;
      for (const m of merges) {
        let touched = false;
        for (let r = m.row; r < Math.min(m.row + m.rows, n) && !touched; r++) touched = want.has(r);
        if (!touched) continue;
        for (let r = m.row; r < Math.min(m.row + m.rows, n); r++) {
          if (!want.has(r)) { want.add(r); changed = true; }
        }
      }
    }

    for (const [r, row] of this.rowEls) {
      if (!want.has(r)) {
        row.tr.remove();
        this.rowEls.delete(r);
      }
    }
    const order = [...want].sort((a, b) => a - b);
    const children: HTMLTableRowElement[] = [];
    let prev = -1;
    for (const r of order) {
      if (r > prev + 1) children.push(this.spacer(this.rowTops[r] - this.rowTops[prev + 1]));
      let row = this.rowEls.get(r);
      if (!row) {
        row = this.buildRow(r);
        this.rowEls.set(r, row);
      }
      children.push(row.tr);
      prev = r;
    }
    if (prev < n - 1) children.push(this.spacer(this.rowTops[n] - this.rowTops[prev + 1]));
    // Re-append in order; rows that are already in place are not moved.
    let cursor: ChildNode | null = this.tbody.firstChild;
    for (const el of children) {
      if (el === cursor) {
        cursor = cursor.nextSibling;
        continue;
      }
      this.tbody.insertBefore(el, cursor);
    }
    while (cursor) {
      const next: ChildNode | null = cursor.nextSibling;
      cursor.remove();
      cursor = next;
    }
  }

  private spacerPool: HTMLTableRowElement[] = [];
  private spacerUsed = 0;
  private spacer(height: number): HTMLTableRowElement {
    let tr = this.spacerPool[this.spacerUsed];
    if (!tr) {
      tr = document.createElement('tr');
      tr.className = 'sheet-spacer';
      tr.setAttribute('aria-hidden', 'true');
      const td = document.createElement('td');
      td.colSpan = this.opts.cols + 1;
      tr.appendChild(td);
      this.spacerPool.push(tr);
    }
    this.spacerUsed++;
    (tr.firstChild as HTMLTableCellElement).style.height = `${Math.max(0, height)}px`;
    return tr;
  }

  // ensureRowVisible scrolls a row into the viewport (rendering it first).
  private ensureVisible(r: number, c: number): void {
    const head = this.thead.offsetHeight;
    const fz = this.opts.frozen?.() ?? { rows: 0, cols: 0 };
    const frozenH = this.rowTops[Math.min(fz.rows, this.opts.rows)] ?? 0;
    const top = this.rowTops[r] ?? 0;
    const bottom = this.rowTops[r + 1] ?? top;
    const viewTop = this.root.scrollTop + frozenH;
    const viewBottom = this.root.scrollTop + this.root.clientHeight - head;
    if (r >= fz.rows) {
      if (top < viewTop) this.root.scrollTop = Math.max(0, top - frozenH);
      else if (bottom > viewBottom) this.root.scrollTop = bottom - this.root.clientHeight + head;
    }
    this.render();
    this.cellEl(r, c)?.scrollIntoView({ block: 'nearest', inline: 'nearest' });
  }

  // attachResizer adds a drag strip to a header cell edge. Dragging previews
  // the size live via inline style and commits once on mouseup via onResize.
  private attachResizer(th: HTMLTableCellElement, axis: 'col' | 'row', index: number): void {
    if (this.opts.readOnly) return;
    const grip = document.createElement('span');
    grip.className = axis === 'col' ? 'sheet-resizer-col' : 'sheet-resizer-row';
    grip.addEventListener('mousedown', (e: MouseEvent) => {
      e.stopPropagation();
      e.preventDefault();
      const startPos = axis === 'col' ? e.clientX : e.clientY;
      const startSize = axis === 'col' ? th.offsetWidth : th.offsetHeight;
      // ponytail: row floor matches the CSS td height (22px) — table cells
      // treat height as min-height, so rows cannot render below it anyway.
      const minSize = axis === 'col' ? 40 : 22;
      let size = startSize;
      const onMove = (me: MouseEvent) => {
        size = Math.max(minSize, startSize + ((axis === 'col' ? me.clientX : me.clientY) - startPos));
        this.applyDim(axis, index, size);
      };
      const onUp = () => {
        document.removeEventListener('mousemove', onMove);
        document.removeEventListener('mouseup', onUp);
        if (size !== startSize) this.opts.onResize?.(axis, index, size);
      };
      document.addEventListener('mousemove', onMove);
      document.addEventListener('mouseup', onUp);
    });
    th.appendChild(grip);
  }

  // applyDim paints one dimension override. Column widths also need min-width
  // relaxed on every cell in the column (the CSS default is min-width: 80px).
  private applyDim(axis: 'col' | 'row', index: number, size: number | undefined): void {
    const px = size === undefined ? '' : `${size}px`;
    if (axis === 'col') {
      const th = this.colHeads[index];
      if (!th) return;
      th.style.width = px;
      th.style.minWidth = px;
      th.style.maxWidth = px;
      for (const row of this.rowEls.values()) {
        const td = row.tds[index];
        td.style.minWidth = px;
        td.style.maxWidth = px;
      }
    } else {
      const th = this.rowEls.get(index)?.th;
      if (!th) return;
      th.style.height = px;
    }
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
      // Ctrl/Cmd+click follows a hyperlink, as in Excel.
      if ((e.ctrlKey || e.metaKey) && e.button === 0) {
        const link = this.opts.metaOf?.(r, c).link;
        if (link) {
          e.preventDefault();
          window.open(link, '_blank', 'noopener,noreferrer');
          return;
        }
      }
      // Right-click inside an existing selection keeps it (Excel behaviour), so
      // the context menu acts on the whole range instead of one cell.
      if (e.button === 2) {
        const { r0, c0, r1, c1 } = normalize(this.selection);
        if (r >= r0 && r <= r1 && c >= c0 && c <= c1) return;
      }
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
        let nr = Math.min(this.opts.rows - 1, Math.max(0, f.row + dr));
        let nc = Math.min(this.opts.cols - 1, Math.max(0, f.col + dc));
        // Merged ranges: a covered (hidden) cell can't take focus — snap to
        // the merge anchor, and step past the whole merge when leaving it.
        const m = this.mergeAt(nr, nc);
        if (m && !extend) {
          if (m.row === f.row && m.col === f.col) {
            // moving within our own merge: jump to the far side
            nr = Math.min(this.opts.rows - 1, Math.max(0, dr > 0 ? m.row + m.rows : dr < 0 ? m.row - 1 : nr));
            nc = Math.min(this.opts.cols - 1, Math.max(0, dc > 0 ? m.col + m.cols : dc < 0 ? m.col - 1 : nc));
            const m2 = this.mergeAt(nr, nc);
            if (m2) { nr = m2.row; nc = m2.col; }
          } else {
            nr = m.row;
            nc = m.col;
          }
        }
        this.selection = extend
          ? { anchor: this.selection.anchor, focus: { row: nr, col: nc } }
          : selFromSingle(nr, nc);
        if (!extend) this.opts.onSelect?.(nr, nc);
        this.opts.onSelectionChange?.(this.selection);
        this.ensureVisible(nr, nc);
        // The moved-to cell takes the keyboard, so typing goes into it.
        if (!extend) this.cellEl(nr, nc)?.focus();
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

  // setSelection moves the selection programmatically (Find Next, Go To).
  // It focuses the cell like a click would — DOM focus is what makes a cell the
  // active one for typing, so selecting without focusing would leave the
  // keyboard pointed at the previous cell.
  setSelection(sel: Selection): void {
    this.selection = sel;
    this.opts.onSelectionChange?.(sel);
    this.ensureVisible(sel.focus.row, sel.focus.col);
    this.cellEl(sel.focus.row, sel.focus.col)?.focus();
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

  // mergeAt returns the merge covering (r, c), or null.
  // ponytail: linear scan per lookup; merges per sheet are few. Index by cell
  // key if sheets ever carry hundreds of merges.
  private mergeAt(r: number, c: number): { row: number; col: number; rows: number; cols: number } | null {
    for (const m of this.opts.merges?.() ?? []) {
      if (r >= m.row && r < m.row + m.rows && c >= m.col && c < m.col + m.cols) return m;
    }
    return null;
  }

  private closeListMenu(): void {
    this.listMenu?.remove();
    this.listMenu = null;
    if (this.listOutside) document.removeEventListener('mousedown', this.listOutside, true);
    this.listOutside = null;
  }

  // toggleListMenu opens the list of allowed values under the focused cell;
  // picking one writes it into the cell.
  private toggleListMenu(): void {
    if (this.listMenu) return this.closeListMenu();
    const { row, col } = this.selection.focus;
    const td = this.cellEl(row, col);
    const list = this.opts.metaOf?.(row, col).list ?? [];
    if (!td || list.length === 0) return;
    const menu = document.createElement('div');
    menu.className = 'sheet-list-menu';
    menu.setAttribute('role', 'listbox');
    const current = this.opts.rawValue(row, col).trim().toLowerCase();
    for (const v of list) {
      const b = document.createElement('button');
      b.type = 'button';
      b.textContent = v;
      b.setAttribute('role', 'option');
      if (v.toLowerCase() === current) b.classList.add('on');
      b.addEventListener('mousedown', (e) => e.preventDefault());
      b.addEventListener('click', () => {
        this.closeListMenu();
        if (this.editing) (document.activeElement as HTMLElement | null)?.blur();
        if (this.opts.rawValue(row, col) !== v) this.opts.onEdit(row, col, v);
        this.render();
      });
      menu.appendChild(b);
    }
    menu.style.left = `${this.table.offsetLeft + td.offsetLeft}px`;
    menu.style.top = `${this.table.offsetTop + td.offsetTop + td.offsetHeight}px`;
    menu.style.minWidth = `${td.offsetWidth}px`;
    this.root.appendChild(menu);
    this.listMenu = menu;
    this.listOutside = (e: MouseEvent): void => {
      if (menu.contains(e.target as Node) || this.listBtn.contains(e.target as Node)) return;
      this.closeListMenu();
    };
    document.addEventListener('mousedown', this.listOutside, true);
  }

  // applyFreeze keeps the first rows/columns in place while the grid scrolls
  // (position: sticky, offset by the rows/columns before them).
  private applyFreeze(): void {
    for (const el of this.stickyEls) {
      el.style.position = '';
      el.style.top = '';
      el.style.left = '';
      el.style.zIndex = '';
    }
    this.stickyEls.clear();
    const fz = this.opts.frozen?.() ?? { rows: 0, cols: 0 };
    this.table.classList.toggle('sheet-frozen-r', fz.rows > 0);
    this.table.classList.toggle('sheet-frozen-c', fz.cols > 0);
    const stick = (el: HTMLElement, top: number | null, left: number | null, z: number): void => {
      el.style.position = 'sticky';
      if (top !== null) el.style.top = `${top}px`;
      if (left !== null) el.style.left = `${left}px`;
      el.style.zIndex = String(z);
      this.stickyEls.add(el);
    };
    const head = this.thead.offsetHeight;
    const rowHeadW = this.thead.rows[0]?.cells[0]?.offsetWidth ?? 40;
    const lefts: number[] = [];
    let x = rowHeadW;
    for (let c = 0; c < Math.min(fz.cols, this.opts.cols); c++) {
      lefts.push(x);
      x += this.colHeads[c].offsetWidth;
      stick(this.colHeads[c], null, lefts[c], 9);
    }
    let y = head;
    for (let r = 0; r < Math.min(fz.rows, this.opts.rows); r++) {
      const row = this.rowEls.get(r);
      if (!row) continue;
      stick(row.th, y, 0, 8);
      row.tds.forEach((td, c) => stick(td, y, c < lefts.length ? lefts[c] : null, c < lefts.length ? 8 : 7));
      y += row.tr.offsetHeight;
    }
    if (lefts.length > 0) {
      for (const [r, row] of this.rowEls) {
        if (r < fz.rows) continue;
        for (let c = 0; c < lefts.length; c++) stick(row.tds[c], null, lefts[c], 6);
      }
    }
  }

  // render refreshes every non-editing cell to its display value, then paints
  // remote live-edit text and cursor/live-edit decorations.
  render(): void {
    for (const td of this.decorated) {
      td.style.boxShadow = '';
      td.style.overflow = '';
      td.classList.remove('sheet-remote-sel', 'sheet-fill-target');
      td.style.removeProperty('--rsel');
      td.querySelector('.sheet-remote-tag')?.remove();
    }
    this.decorated.clear();

    // M4: dimension overrides, client-local row filter, freeze panes.
    this.spacerUsed = 0;
    this.layoutRows();
    for (let c = 0; c < this.opts.cols; c++) this.applyDim('col', c, this.opts.colWidth?.(c));
    for (const [r, row] of this.rowEls) {
      this.applyDim('row', r, this.opts.rowHeight?.(r));
      row.tr.style.display = this.opts.rowHidden?.(r) ? 'none' : '';
    }
    // Merged ranges: reset last render's spans, then apply the current set.
    for (const td of this.mergedTds) {
      td.rowSpan = 1;
      td.colSpan = 1;
      td.style.display = '';
    }
    this.mergedTds.clear();
    for (const m of this.opts.merges?.() ?? []) {
      const anchor = this.cellEl(m.row, m.col);
      if (!anchor) continue;
      anchor.rowSpan = Math.min(m.rows, this.opts.rows - m.row);
      anchor.colSpan = Math.min(m.cols, this.opts.cols - m.col);
      this.mergedTds.add(anchor);
      for (let r = m.row; r < Math.min(m.row + m.rows, this.opts.rows); r++) {
        for (let c = m.col; c < Math.min(m.col + m.cols, this.opts.cols); c++) {
          if (r === m.row && c === m.col) continue;
          const td = this.cellEl(r, c);
          if (td) {
            td.style.display = 'none';
            this.mergedTds.add(td);
          }
        }
      }
    }

    this.applyFreeze();

    for (const [r, row] of this.rowEls) {
      for (let c = 0; c < this.opts.cols; c++) {
        if (this.editing && this.editing.row === r && this.editing.col === c) continue;
        const td = row.tds[c];
        const k = `${r}:${c}`;
        const live = this.liveByKey.get(k);
        td.textContent = live ? live.raw : this.opts.displayValue(r, c);
        // Reset then apply cell formatting (props resolved by the editor).
        td.style.fontWeight = '';
        td.style.fontStyle = '';
        td.style.textDecoration = '';
        td.style.color = '';
        td.style.background = '';
        td.style.textAlign = '';
        td.style.border = '';
        td.style.fontFamily = '';
        td.style.fontSize = '';
        // whiteSpace: normal (wrap) overrides the CSS nowrap default via inline style.
        td.style.whiteSpace = '';
        if (this.opts.styleOf) {
          // NOTE: fontFamily/fontSize/whiteSpace are added to CellCss in a
          // parallel styleCss.ts change; until it lands tsc flags these three
          // property accesses as unknown (expected).
          const css = styleToCss(this.opts.styleOf(r, c));
          if (css.fontWeight) td.style.fontWeight = css.fontWeight;
          if (css.fontStyle) td.style.fontStyle = css.fontStyle;
          if (css.textDecoration) td.style.textDecoration = css.textDecoration;
          if (css.color) td.style.color = css.color;
          if (css.background) td.style.background = css.background;
          if (css.textAlign) td.style.textAlign = css.textAlign;
          if (css.border) td.style.border = css.border;
          if (css.fontFamily) td.style.fontFamily = css.fontFamily;
          if (css.fontSize) td.style.fontSize = css.fontSize;
          if (css.whiteSpace) td.style.whiteSpace = css.whiteSpace;
        }
        if (!td.style.textAlign && !live) td.style.textAlign = this.opts.defaultAlignOf?.(r, c) ?? '';
        const cond = live ? {} : this.opts.condStyleOf?.(r, c) ?? {};
        if (cond.bg) td.style.background = cond.bg;
        if (cond.color) td.style.color = cond.color;
        if (cond.bold) td.style.fontWeight = 'bold';
        const meta = this.opts.metaOf?.(r, c) ?? {};
        td.classList.toggle('sheet-link', !!meta.link);
        td.classList.toggle('sheet-has-note', !!meta.note);
        const err = this.opts.errorOf?.(r, c);
        td.classList.toggle('sheet-cell-error', !!err);
        const tip = [err, meta.note, meta.link ? `${meta.link}\nCtrl+click to follow the link` : ''].filter(Boolean).join('\n\n');
        if (tip) td.title = tip; else td.removeAttribute('title');
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
          // The tag sits above the cell (top: -15px) — lift the td's
          // overflow: hidden default so it is not clipped (reset on cleanup).
          td.style.overflow = 'visible';
          this.decorated.add(td);
        }
      }
    }

    // local selection
    for (const [r, row] of this.rowEls) {
      for (let c = 0; c < this.opts.cols; c++) {
        const td = row.tds[c];
        td.classList.toggle('sheet-sel', selContains(this.selection, r, c));
        td.classList.toggle(
          'sheet-sel-focus',
          r === this.selection.focus.row && c === this.selection.focus.col,
        );
      }
    }
    // Excel-style header highlight for the selected range.
    const selN = normalize(this.selection);
    this.colHeads.forEach((th, c) => {
      th.classList.toggle('sheet-head-hl', c >= selN.c0 && c <= selN.c1);
      th.classList.toggle('sheet-head-filter', this.opts.colFiltered?.(c) ?? false);
    });
    for (const [r, row] of this.rowEls) row.th.classList.toggle('sheet-head-hl', r >= selN.r0 && r <= selN.r1);
    // remote selections (outline only, multi-cell)
    for (const rs of this.remoteSel) {
      const { r0, c0, r1, c1 } = normalize(rs.sel);
      if (r1 - r0 === 0 && c1 - c0 === 0) continue; // single cell handled by cursor deco
      for (const r of this.rowEls.keys()) {
        if (r < r0 || r > r1) continue;
        for (let c = c0; c <= c1; c++) {
          const td = this.cellEl(r, c);
          if (!td) continue;
          td.style.setProperty('--rsel', rs.color);
          td.classList.add('sheet-remote-sel');
          this.decorated.add(td);
        }
      }
    }

    // Real heights (wrapped text, fonts) of the rows on screen; rows that
    // are not use the plain row height, the smallest one seen, so one tall
    // row does not stretch all others. Only rows on screen are kept: after
    // an insert or delete an index may mean another row.
    this.measured = new Map();
    for (const [r, row] of this.rowEls) {
      if (row.tr.style.display === 'none') continue;
      const h = row.tr.offsetHeight;
      if (h <= 0) continue;
      this.measured.set(r, h);
      if (this.opts.rowHeight?.(r) === undefined && (this.minRowH === 0 || h < this.minRowH)) {
        this.minRowH = h;
        this.defaultRowH = h;
      }
    }

    // fill handle overlay at the bottom-right corner of the current selection.
    // Hidden while typing (Excel does the same); positioned relative to the
    // root, so it never touches the contenteditable td's content.
    const { r1, c1 } = normalize(this.selection);
    const brCell = this.cellEl(r1, c1);
    const editingBr = this.activeEdit && this.editing?.row === r1 && this.editing?.col === c1;
    if (brCell && !this.opts.readOnly && !editingBr) {
      this.fillHandle.style.left = `${this.table.offsetLeft + brCell.offsetLeft + brCell.offsetWidth - 4}px`;
      this.fillHandle.style.top = `${this.table.offsetTop + brCell.offsetTop + brCell.offsetHeight - 4}px`;
      this.fillHandle.style.display = '';
    } else {
      this.fillHandle.style.display = 'none';
    }
    const f = this.selection.focus;
    const focusTd = this.cellEl(f.row, f.col);
    const list = this.opts.readOnly ? undefined : this.opts.metaOf?.(f.row, f.col).list;
    if (focusTd && list && list.length > 0 && focusTd.style.display !== 'none') {
      this.listBtn.style.left = `${this.table.offsetLeft + focusTd.offsetLeft + focusTd.offsetWidth + 1}px`;
      this.listBtn.style.top = `${this.table.offsetTop + focusTd.offsetTop + Math.max(0, focusTd.offsetHeight - 19)}px`;
      this.listBtn.style.display = '';
    } else {
      this.listBtn.style.display = 'none';
      this.closeListMenu();
    }
    if (this.fillTarget) {
      const t = normalize(this.fillTarget);
      for (const r of this.rowEls.keys()) {
        if (r < t.r0 || r > t.r1) continue;
        for (let c = t.c0; c <= t.c1; c++) {
          const td = this.cellEl(r, c);
          if (td) { td.classList.add('sheet-fill-target'); this.decorated.add(td); }
        }
      }
    }
  }
}

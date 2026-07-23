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
  // M4: sparse dimension overrides (px), freeze state, and resize commits.
  colWidth?: (col: number) => number | undefined;
  rowHeight?: (row: number) => number | undefined;
  frozen?: () => { rows: number; cols: number };
  onResize?: (axis: 'col' | 'row', index: number, sizePx: number) => void;
  // Client-local row filter: hidden rows collapse via display:none.
  rowHidden?: (row: number) => boolean;
  // Merged ranges (top-left anchor + span). The anchor td gets rowSpan/colSpan,
  // covered tds are display:none.
  merges?: () => Array<{ row: number; col: number; rows: number; cols: number }>;
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
.sheet-grid.sheet-frozen-r thead th { position: sticky; top: 0; z-index: 8; }
.sheet-grid.sheet-frozen-r tbody tr:first-child th, .sheet-grid.sheet-frozen-r tbody tr:first-child td { position: sticky; top: var(--fr-top, 24px); z-index: 7; }
.sheet-grid.sheet-frozen-c tbody th, .sheet-grid.sheet-frozen-c thead th:first-child { position: sticky; left: 0; z-index: 8; }
.sheet-grid.sheet-frozen-c thead th:nth-child(2), .sheet-grid.sheet-frozen-c tbody td:first-of-type { position: sticky; left: var(--fc-left, 40px); z-index: 6; }
.sheet-grid td:focus { box-shadow: inset 0 0 0 2px #107c41; overflow: visible; z-index: 3; }
.sheet-remote-tag { position: absolute; top: -15px; left: -1px; font: 10px/14px system-ui, sans-serif; padding: 0 4px; color: #fff; border-radius: 3px 3px 3px 0; white-space: nowrap; z-index: 5; pointer-events: none; }
.sheet-remote-tag::after { content: attr(data-label); }
.sheet-grid td.sheet-sel { background: rgba(16, 124, 65, 0.10); }
.sheet-grid td.sheet-sel-focus { box-shadow: inset 0 0 0 2px #107c41; }
.sheet-grid td.sheet-remote-sel { box-shadow: inset 0 0 0 2px var(--rsel, #888); }
.sheet-fill-handle { position: absolute; width: 8px; height: 8px; background: #107c41; border: 1px solid #fff; cursor: crosshair; z-index: 6; }
.sheet-grid td.sheet-fill-target { box-shadow: inset 0 0 0 1px #107c41; }
.sheet-grid td.sheet-cell-error { color: #c0392b; }
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
  // Single fill-handle overlay, positioned over the selection's bottom-right
  // corner. It lives OUTSIDE the contenteditable tds: a decoration inside an
  // editable cell races the caret and re-renders can delete typed text.
  private fillHandle: HTMLSpanElement;
  private mergedTds = new Set<HTMLTableCellElement>();
  private table: HTMLTableElement;
  private thead: HTMLTableSectionElement;
  private colHeads: HTMLTableCellElement[] = [];
  private rowHeads: HTMLTableCellElement[] = [];
  private rows: HTMLTableRowElement[] = [];

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

    // ponytail: every cell is a DOM node (~200x52 = 10k contenteditables).
    // Virtualization (render only the visible viewport) is the upgrade path
    // when larger grids hurt; not built here.
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
    for (let r = 0; r < opts.rows; r++) {
      const tr = document.createElement('tr');
      this.rows.push(tr);
      const rowHead = document.createElement('th');
      rowHead.textContent = String(r + 1);
      this.attachResizer(rowHead, 'row', r);
      this.rowHeads.push(rowHead);
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
      for (let r = 0; r < this.opts.rows; r++) {
        const td = this.cells[r][index];
        td.style.minWidth = px;
        td.style.maxWidth = px;
      }
    } else {
      const th = this.rowHeads[index];
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

  // mergeAt returns the merge covering (r, c), or null.
  // ponytail: linear scan per lookup; merges per sheet are few. Index by cell
  // key if sheets ever carry hundreds of merges.
  private mergeAt(r: number, c: number): { row: number; col: number; rows: number; cols: number } | null {
    for (const m of this.opts.merges?.() ?? []) {
      if (r >= m.row && r < m.row + m.rows && c >= m.col && c < m.col + m.cols) return m;
    }
    return null;
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
    for (let c = 0; c < this.opts.cols; c++) this.applyDim('col', c, this.opts.colWidth?.(c));
    for (let r = 0; r < this.opts.rows; r++) {
      this.applyDim('row', r, this.opts.rowHeight?.(r));
      this.rows[r].style.display = this.opts.rowHidden?.(r) ? 'none' : '';
    }
    // Merged ranges: reset last render's spans, then apply the current set.
    for (const td of this.mergedTds) {
      td.rowSpan = 1;
      td.colSpan = 1;
      td.style.display = '';
    }
    this.mergedTds.clear();
    for (const m of this.opts.merges?.() ?? []) {
      const anchor = this.cells[m.row]?.[m.col];
      if (!anchor) continue;
      anchor.rowSpan = Math.min(m.rows, this.opts.rows - m.row);
      anchor.colSpan = Math.min(m.cols, this.opts.cols - m.col);
      this.mergedTds.add(anchor);
      for (let r = m.row; r < Math.min(m.row + m.rows, this.opts.rows); r++) {
        for (let c = m.col; c < Math.min(m.col + m.cols, this.opts.cols); c++) {
          if (r === m.row && c === m.col) continue;
          const td = this.cells[r]?.[c];
          if (td) {
            td.style.display = 'none';
            this.mergedTds.add(td);
          }
        }
      }
    }

    const fz = this.opts.frozen?.() ?? { rows: 0, cols: 0 };
    this.table.classList.toggle('sheet-frozen-r', fz.rows > 0);
    this.table.classList.toggle('sheet-frozen-c', fz.cols > 0);
    if (fz.rows > 0) this.table.style.setProperty('--fr-top', `${this.thead.offsetHeight}px`);
    if (fz.cols > 0) this.table.style.setProperty('--fc-left', `${this.rowHeads[0]?.offsetWidth ?? 40}px`);

    for (let r = 0; r < this.opts.rows; r++) {
      for (let c = 0; c < this.opts.cols; c++) {
        if (this.editing && this.editing.row === r && this.editing.col === c) continue;
        const td = this.cells[r][c];
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
        const err = this.opts.errorOf?.(r, c);
        td.classList.toggle('sheet-cell-error', !!err);
        if (err) td.title = err; else td.removeAttribute('title');
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
    // Excel-style header highlight for the selected range.
    const selN = normalize(this.selection);
    this.colHeads.forEach((th, c) => th.classList.toggle('sheet-head-hl', c >= selN.c0 && c <= selN.c1));
    this.rowHeads.forEach((th, r) => th.classList.toggle('sheet-head-hl', r >= selN.r0 && r <= selN.r1));
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

    // fill handle overlay at the bottom-right corner of the current selection.
    // Hidden while typing (Excel does the same); positioned relative to the
    // root, so it never touches the contenteditable td's content.
    const { r1, c1 } = normalize(this.selection);
    const brCell = this.cells[r1]?.[c1];
    const editingBr = this.activeEdit && this.editing?.row === r1 && this.editing?.col === c1;
    if (brCell && !this.opts.readOnly && !editingBr) {
      this.fillHandle.style.left = `${this.table.offsetLeft + brCell.offsetLeft + brCell.offsetWidth - 4}px`;
      this.fillHandle.style.top = `${this.table.offsetTop + brCell.offsetTop + brCell.offsetHeight - 4}px`;
      this.fillHandle.style.display = '';
    } else {
      this.fillHandle.style.display = 'none';
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

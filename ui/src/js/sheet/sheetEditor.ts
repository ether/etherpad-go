import * as socketio from '../socketio';
import padutils, { Cookies } from '../pad_utils';
import { SheetCollabClient } from './sheetCollabClient';
import { FormulaEngine } from './formulaEngine';
import { DomSheetView } from './sheetView';
import { SheetPresence, effectiveCells, type PresenceFrame } from './sheetPresence';
import { rangeToTSV, rangeToCSV, parseTSV, parseCSV, pasteOps, fillOps } from './sheetClipboard';
import { normalize, selCells, selIsSingle, type Selection } from './sheetSelection';
import { createToolbar } from './sheetToolbar';
import { createSheetTabs } from './sheetTabs';
import { sortRangeOps, distinctValues, hiddenRowsForFilter } from './sheetSortFilter';
import { createFormulaBar, type FormulaBarHandle } from './sheetFormulaBar';
import { rangeRefA1 } from './a1';
import { mergeProps } from './styleCss';
import { formatValue } from './format';
import type { Op } from './op';
import type { WorkbookSnapshot } from './workbookState';

interface SheetVarsData {
  snapshot: WorkbookSnapshot;
  head: number;
  userId: string;
  userColor: string;
  readonly: boolean;
}

// Editor-level chrome (title bar above the ribbon, status bar below the tabs).
// Excel window layout: the chrome is pinned to the viewport and only the grid
// scrolls (sheet-grid-host is the scroll container; the freeze-pane sticky
// offsets resolve against it).
const CHROME_CSS = `
html, body { height: 100%; }
body { margin: 0; overflow: hidden; }
.sheet-app { display: flex; flex-direction: column; height: 100vh; }
.sheet-grid-host { flex: 1; overflow: auto; min-height: 0; position: relative; }
.sheet-titlebar { display: flex; align-items: center; gap: 8px; height: 36px; flex: none; padding: 0 12px; background: #107c41; color: #fff; font: 14px system-ui, sans-serif; }
.sheet-titlebar svg { flex: none; display: block; }
.sheet-title-name { font-weight: 600; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.sheet-statusbar { display: flex; align-items: center; justify-content: space-between; height: 22px; padding: 0 10px; background: #f5f6f7; border-top: 1px solid #d4d8dd; font: 12px system-ui, sans-serif; color: #444; }
.sheet-stats { display: flex; gap: 16px; }
`;
const TITLE_ICON_SVG =
  '<svg width="18" height="18" viewBox="0 0 16 16" aria-hidden="true">' +
  '<rect x="1" y="1" width="14" height="14" rx="1.5" fill="none" stroke="#fff" stroke-width="1.4"/>' +
  '<path d="M1 5.7h14M1 10.3h14M5.7 1v14M10.3 1v14" stroke="#fff" stroke-width="1" fill="none"/></svg>';

// startSheetEditor connects to the collaborative spreadsheet backend, performs
// the CLIENT_READY handshake (component "sheet"), and wires the collaboration
// client, formula engine, grid view and ephemeral presence.
export function startSheetEditor(root: HTMLElement): void {
  const padId = decodeURIComponent(
    location.pathname.substring(location.pathname.lastIndexOf('/') + 1),
  );
  const socket = socketio.connect('', '/', { query: { padId } });

  let collab: SheetCollabClient | null = null;
  let view: DomSheetView | null = null;
  let formulaBar: FormulaBarHandle | null = null;
  let presence: SheetPresence | null = null;
  let activeSheetId = 's1';
  const engine = new FormulaEngine();
  let selection: Selection = { anchor: { row: 0, col: 0 }, focus: { row: 0, col: 0 } };
  let readOnly = false;
  const GRID_ROWS = 200;
  const GRID_COLS = 52;
  // Client-local filter state (per active sheet, reset on switch — not collaborative).
  let hiddenRows = new Set<number>();
  let tabs: { el: HTMLElement; refresh: () => void } | null = null;

  const transport = {
    send: (op: Op) =>
      socket.emit('message', {
        type: 'COLLABROOM',
        component: 'sheet',
        data: { type: 'SHEET_OP', op, baseRev: op.baseRev },
      }),
  };

  const sendPresence = (
    row: number, col: number, editing: boolean, raw?: string, focusRow?: number, focusCol?: number,
  ): void =>
    socket.emit('message', {
      type: 'COLLABROOM',
      component: 'sheet',
      data: { type: 'SHEET_PRESENCE', sheet: activeSheetId, row, col, editing, raw, focusRow, focusCol },
    });

  // Live-edit throttle (trailing, ~60ms) so typing does not flood the socket.
  let liveTimer: ReturnType<typeof setTimeout> | null = null;
  let lastLive: { row: number; col: number; raw: string } | null = null;
  const sendLiveEdit = (row: number, col: number, raw: string): void => {
    lastLive = { row, col, raw };
    if (liveTimer) return;
    liveTimer = setTimeout(() => {
      liveTimer = null;
      if (lastLive) sendPresence(lastLive.row, lastLive.col, true, lastLive.raw);
    }, 60);
  };
  const cancelPendingLive = (): void => {
    if (liveTimer) {
      clearTimeout(liveTimer);
      liveTimer = null;
    }
    lastLive = null;
  };

  // Selection debounce (~50ms) against arrow-key spam.
  let selTimer: ReturnType<typeof setTimeout> | null = null;
  const sendSelect = (row: number, col: number): void => {
    if (selTimer) clearTimeout(selTimer);
    selTimer = setTimeout(() => sendPresence(row, col, false), 50);
  };

  const cellsOfActive = (): Array<{ row: number; col: number; raw: string }> => {
    const sheet = collab?.display.sheetById(activeSheetId);
    if (!sheet) return [];
    const out: Array<{ row: number; col: number; raw: string }> = [];
    for (const [k, cell] of sheet.cells) {
      const i = k.indexOf(':');
      out.push({ row: Number(k.slice(0, i)), col: Number(k.slice(i + 1)), raw: cell.raw });
    }
    return out;
  };

  const rawValue = (r: number, c: number): string =>
    collab?.display.getCell(activeSheetId, r, c)?.raw ?? '';

  // Status-bar stats (Excel wording): Average/Sum over numeric values —
  // formula cells count with their COMPUTED value, like Excel. Count =
  // non-empty cells. Shown only for multi-cell selections with at least one
  // numeric cell; empty otherwise.
  let statsEl: HTMLElement | null = null;
  const updateStats = (): void => {
    if (!statsEl) return;
    statsEl.textContent = '';
    if (selIsSingle(selection)) return;
    let sum = 0;
    let numCount = 0;
    let count = 0;
    let min = Infinity;
    let max = -Infinity;
    for (const { row, col } of selCells(selection)) {
      const raw = rawValue(row, col);
      if (raw === '') continue;
      count++;
      const value = raw.startsWith('=') ? engine.getValue(row, col).value : raw;
      const n = Number(value);
      if (value.trim() !== '' && Number.isFinite(n)) {
        sum += n;
        numCount++;
        if (n < min) min = n;
        if (n > max) max = n;
      }
    }
    if (numCount === 0) return;
    // toPrecision(12) strips float noise (0.1+0.2 -> 0.3) without truncating
    // typical spreadsheet magnitudes.
    const f = (n: number): string => String(parseFloat(n.toPrecision(12)));
    // Excel status-bar order: Average, Count, Min, Max, Sum.
    for (const part of [`Average: ${f(sum / numCount)}`, `Count: ${count}`, `Min: ${f(min)}`, `Max: ${f(max)}`, `Sum: ${f(sum)}`]) {
      const s = document.createElement('span');
      s.textContent = part;
      statsEl.appendChild(s);
    }
  };

  // ponytail: styleId is client-internal — only `props` travel on the wire and
  // are persisted, and each cell's styleId+pool entry are set together in one
  // applyOp. So an optimistic local put() assigning a different nextId than the
  // server (before the op is confirmed) is unobservable: rendering keys off
  // props, and reconnect re-seeds the pool. No cross-client render/persist drift.
  const propsOf = (r: number, c: number): Record<string, string> =>
    collab ? collab.display.getStyleProps(activeSheetId, r, c) : {};

  const displayValue = (r: number, c: number): string => {
    const cell = collab?.display.getCell(activeSheetId, r, c);
    if (!cell || cell.raw === '') return '';
    const raw = cell.raw.startsWith('=') ? engine.getValue(r, c).value : cell.raw;
    return formatValue(raw, '', propsOf(r, c).numFmt);
  };

  const applyStyleToSelection = (change: Record<string, string>): void => {
    if (readOnly || !collab) return;
    // Blur first: a setStyle op only ever changes styleId, never raw, so the
    // focused cell's DOM text still matches its stored raw at blur time and
    // the blur listener's commit check (raw !== prev) is false — nothing to
    // clobber. Clearing `editing` lets the render() below repaint it too
    // (render() otherwise skips repainting the currently-focused cell).
    blurActiveCell();
    for (const { row, col } of selCells(selection)) {
      const merged = mergeProps(propsOf(row, col), change);
      collab.applyLocal({ type: 'setStyle', sheet: activeSheetId, baseRev: collab.rev, row, col, props: merged });
    }
  };

  const onChange = (): void => {
    const live = presence ? presence.liveEditsForSheet(activeSheetId) : [];
    engine.setGrid(effectiveCells(cellsOfActive(), live));
    if (view && presence) {
      view.setRemoteCursors(
        presence.cursorsForSheet(activeSheetId).map((c) => ({
          userId: c.userId, name: c.name, color: c.color, row: c.row, col: c.col,
        })),
      );
      view.setRemoteLiveEdits(
        live.map((e) => ({
          userId: e.userId, name: e.name, color: e.color, row: e.row, col: e.col, raw: e.raw,
        })),
      );
      view.setRemoteSelections(
        presence
          .cursorsForSheet(activeSheetId)
          .filter((c) => c.focusRow !== undefined && c.focusCol !== undefined)
          .map((c) => ({
            userId: c.userId,
            color: c.color,
            sel: { anchor: { row: c.row, col: c.col }, focus: { row: c.focusRow as number, col: c.focusCol as number } },
          })),
      );
    }
    view?.render();
    tabs?.refresh();
    if (formulaBar) {
      const { r0, c0, r1, c1 } = normalize(selection);
      formulaBar.setActive(rangeRefA1(r0, c0, r1, c1), rawValue(selection.focus.row, selection.focus.col));
    }
    updateStats();
  };

  const editingNow = (): boolean => view?.isEditing() ?? false;

  const initSheet = (data: SheetVarsData): void => {
    activeSheetId = data.snapshot.sheets?.[0]?.id ?? 's1';
    readOnly = data.readonly;
    collab = new SheetCollabClient(data.snapshot, data.head, transport);
    collab.onChange = onChange;
    presence = new SheetPresence(data.userId);
    presence.onChange = onChange;

    // The view clears its container's innerHTML in its constructor, so the
    // toolbar gets its own sibling container (gridHost) rather than sharing
    // root with it.
    root.innerHTML = '';
    if (!document.getElementById('sheet-chrome-style')) {
      const s = document.createElement('style');
      s.id = 'sheet-chrome-style';
      s.textContent = CHROME_CSS;
      document.head.appendChild(s);
    }
    root.classList.add('sheet-app');
    const titlebar = document.createElement('div');
    titlebar.className = 'sheet-titlebar';
    titlebar.innerHTML = TITLE_ICON_SVG;
    const titleName = document.createElement('span');
    titleName.className = 'sheet-title-name';
    titleName.textContent = padId;
    titlebar.appendChild(titleName);
    root.appendChild(titlebar);
    const toolbar = createToolbar({
      getProps: (r, c) => propsOf(r, c),
      focusCell: () => selection.focus,
      applyToSelection: applyStyleToSelection,
      readOnly: data.readonly,
      sortSelection: (asc) => {
        if (readOnly || !collab || selIsSingle(selection)) return;
        blurActiveCell();
        for (const op of sortRangeOps(selection, selection.focus.col, asc, activeSheetId, collab.rev, rawValue)) {
          collab.applyLocal(op);
        }
      },
      toggleFreeze: (kind) => {
        if (readOnly || !collab) return;
        const s = collab.display.sheetById(activeSheetId);
        const rows = kind === 'row' ? ((s?.frozenRows ?? 0) > 0 ? 0 : 1) : (s?.frozenRows ?? 0);
        const cols = kind === 'col' ? ((s?.frozenCols ?? 0) > 0 ? 0 : 1) : (s?.frozenCols ?? 0);
        collab.applyLocal({ type: 'setFreeze', sheet: activeSheetId, baseRev: collab.rev, frozenRows: rows, frozenCols: cols });
      },
      frozenState: () => {
        const s = collab?.display.sheetById(activeSheetId);
        return { rows: s?.frozenRows ?? 0, cols: s?.frozenCols ?? 0 };
      },
      filterValues: () => distinctValues(selection.focus.col, GRID_ROWS, rawValue),
      applyFilter: (value) => {
        hiddenRows = value === null ? new Set() : hiddenRowsForFilter(selection.focus.col, value, GRID_ROWS, rawValue);
        view?.render();
      },
      // Explicit param types: until the ToolbarCallbacks interface gains these
      // optional fields (built in parallel), there is no contextual type.
      structural: (action: 'insRowAbove' | 'insRowBelow' | 'insColLeft' | 'insColRight' | 'delRows' | 'delCols') => {
        if (readOnly || !collab) return;
        blurActiveCell();
        const { r0, c0, r1, c1 } = normalize(selection);
        const ops: Record<typeof action, Op> = {
          insRowAbove: { type: 'insertRows', sheet: activeSheetId, baseRev: collab.rev, index: r0, count: 1 },
          insRowBelow: { type: 'insertRows', sheet: activeSheetId, baseRev: collab.rev, index: r1 + 1, count: 1 },
          insColLeft: { type: 'insertCols', sheet: activeSheetId, baseRev: collab.rev, index: c0, count: 1 },
          insColRight: { type: 'insertCols', sheet: activeSheetId, baseRev: collab.rev, index: c1 + 1, count: 1 },
          delRows: { type: 'deleteRows', sheet: activeSheetId, baseRev: collab.rev, index: r0, count: r1 - r0 + 1 },
          delCols: { type: 'deleteCols', sheet: activeSheetId, baseRev: collab.rev, index: c0, count: c1 - c0 + 1 },
        };
        collab.applyLocal(ops[action]);
      },
      exportXlsx: () => {
        // Anchor click, not location.href: Firefox treats the href navigation
        // as an unload and kills the websocket even though the response is a
        // download — the session would silently stop receiving broadcasts.
        const a = document.createElement('a');
        a.href = location.pathname + '/export.xlsx';
        a.download = '';
        document.body.appendChild(a);
        a.click();
        a.remove();
      },
      exportCsv: () => {
        // Client-side: serialize the active sheet's used range with computed
        // values (what the user sees), then download. No websocket-killing nav.
        const cells = cellsOfActive();
        let maxRow = 0;
        let maxCol = 0;
        for (const { row, col, raw } of cells) {
          if (raw === '') continue;
          if (row > maxRow) maxRow = row;
          if (col > maxCol) maxCol = col;
        }
        const sel = { anchor: { row: 0, col: 0 }, focus: { row: maxRow, col: maxCol } };
        const csv = rangeToCSV(sel, displayValue);
        const name = collab?.display.sheetById(activeSheetId)?.name || 'sheet';
        const a = document.createElement('a');
        a.href = URL.createObjectURL(new Blob([csv], { type: 'text/csv;charset=utf-8' }));
        a.download = `${name}.csv`;
        document.body.appendChild(a);
        a.click();
        a.remove();
        URL.revokeObjectURL(a.href);
      },
      importCsv: (file: File) => {
        if (readOnly || !collab) return;
        blurActiveCell();
        void file.text().then((text) => {
          if (!collab) return;
          const grid = parseCSV(text);
          // Replace semantics (like xlsx import): clear the current used range,
          // then lay the parsed grid down from A1.
          let maxRow = 0;
          let maxCol = 0;
          for (const { row, col, raw } of cellsOfActive()) {
            if (raw === '') continue;
            if (row > maxRow) maxRow = row;
            if (col > maxCol) maxCol = col;
          }
          collab.applyLocal({ type: 'clearRange', sheet: activeSheetId, baseRev: collab.rev, row: 0, col: 0, endRow: maxRow, endCol: maxCol });
          for (const op of pasteOps(grid, { row: 0, col: 0 }, activeSheetId, collab.rev)) collab.applyLocal(op);
        });
      },
      importXlsx: (file: File) => {
        const body = new FormData();
        body.append('file', file);
        void fetch(location.pathname + '/import', { method: 'POST', body, credentials: 'same-origin' })
          .then(async (res) => {
            // On success the server broadcasts SHEET_RELOAD; the message handler reloads.
            if (!res.ok) alert(await res.text());
          })
          .catch((err) => alert(String(err)));
      },
      // NOTE: clipboardAction/autoSum are added to ToolbarCallbacks in a
      // parallel sheetToolbar.ts change; until it lands tsc flags these two
      // object-literal properties as unknown here (expected).
      clipboardAction: (a: 'cut' | 'copy' | 'paste') => {
        if (a === 'copy') doCopy();
        else if (a === 'cut') doCut();
        else doPaste();
      },
      autoSum: () => {
        if (!readOnly) formulaBar?.beginFormula('=SUM(');
      },
      mergeToggle: () => {
        if (readOnly || !collab) return;
        blurActiveCell();
        const { r0, c0, r1, c1 } = normalize(selection);
        const s = collab.display.sheetById(activeSheetId);
        // Any merge intersecting the selection → unmerge; otherwise merge.
        let hit = false;
        for (const [k, sp] of s?.merges ?? []) {
          const i = k.indexOf(':');
          const mr = Number(k.slice(0, i));
          const mc = Number(k.slice(i + 1));
          if (mr <= r1 && mr + sp.rows - 1 >= r0 && mc <= c1 && mc + sp.cols - 1 >= c0) {
            hit = true;
            break;
          }
        }
        if (!hit && r0 === r1 && c0 === c1) return; // 1x1 merge is meaningless
        collab.applyLocal({
          type: hit ? 'unmergeCells' : 'mergeCells',
          sheet: activeSheetId, baseRev: collab.rev, row: r0, col: c0, endRow: r1, endCol: c1,
        });
      },
    });
    formulaBar = createFormulaBar({
      readOnly: data.readonly,
      getFunctionNames: () => engine.functionNames(),
      onCommit: (raw) => {
        if (readOnly || !collab) return;
        const { row, col } = selection.focus;
        collab.applyLocal({ type: 'setCell', sheet: activeSheetId, baseRev: collab.rev, row, col, raw });
      },
    });
    const gridHost = document.createElement('div');
    gridHost.className = 'sheet-grid-host';
    root.appendChild(toolbar);
    root.appendChild(formulaBar.el);
    root.appendChild(gridHost);

    const setActiveSheet = (id: string): void => {
      if (id === activeSheetId) return;
      activeSheetId = id;
      hiddenRows = new Set(); // the filter is per-sheet and client-local
      onChange();
    };
    tabs = createSheetTabs({
      sheets: () => (collab ? collab.display.sheets.map((s) => ({ id: s.id, name: s.name })) : []),
      activeId: () => activeSheetId,
      readOnly: data.readonly,
      onSwitch: setActiveSheet,
      onAdd: () => {
        if (!collab) return;
        const id = `s-${Math.random().toString(36).slice(2, 10)}`;
        collab.applyLocal({
          type: 'addSheet', sheet: id, baseRev: collab.rev,
          name: `Sheet${collab.display.sheets.length + 1}`, index: collab.display.sheets.length,
        });
        setActiveSheet(id);
      },
      onRename: (id, name) => {
        collab?.applyLocal({ type: 'renameSheet', sheet: id, baseRev: collab.rev, name });
      },
      onDelete: (id) => {
        if (!collab) return;
        collab.applyLocal({ type: 'deleteSheet', sheet: id, baseRev: collab.rev });
        if (id === activeSheetId) setActiveSheet(collab.display.sheets[0]?.id ?? 's1');
      },
      onMove: (id, toIndex) => {
        collab?.applyLocal({ type: 'moveSheet', sheet: id, baseRev: collab.rev, toIndex });
      },
    });
    root.appendChild(tabs.el);

    const statusbar = document.createElement('div');
    statusbar.className = 'sheet-statusbar';
    const readyEl = document.createElement('span');
    readyEl.textContent = 'Ready';
    statsEl = document.createElement('span');
    statsEl.className = 'sheet-stats';
    statusbar.append(readyEl, statsEl);
    root.appendChild(statusbar);

    view = new DomSheetView(gridHost, {
      rows: GRID_ROWS,
      cols: GRID_COLS,
      rawValue,
      displayValue,
      readOnly: data.readonly,
      styleOf: (r, c) => propsOf(r, c),
      colWidth: (c) => collab?.display.sheetById(activeSheetId)?.colWidths.get(c),
      rowHeight: (r) => collab?.display.sheetById(activeSheetId)?.rowHeights.get(r),
      frozen: () => {
        const s = collab?.display.sheetById(activeSheetId);
        return { rows: s?.frozenRows ?? 0, cols: s?.frozenCols ?? 0 };
      },
      onResize: (axis, index, size) => {
        if (readOnly || !collab) return;
        collab.applyLocal({ type: 'setDimension', sheet: activeSheetId, baseRev: collab.rev, axis, index, size });
      },
      rowHidden: (r) => hiddenRows.has(r),
      merges: () => {
        const s = collab?.display.sheetById(activeSheetId);
        if (!s) return [];
        return [...s.merges].map(([k, sp]) => {
          const i = k.indexOf(':');
          return { row: Number(k.slice(0, i)), col: Number(k.slice(i + 1)), rows: sp.rows, cols: sp.cols };
        });
      },
      // ponytail: second engine.getValue per formula cell per render (displayValue
      // already does one). Cheap: HyperFormula caches, and the raw.startsWith('=')
      // gate skips non-formula cells. Fold into displayValue if the grid grows.
      errorOf: (r, c) => {
        const cell = collab?.display.getCell(activeSheetId, r, c);
        if (!cell || !cell.raw.startsWith('=')) return undefined;
        const res = engine.getValue(r, c);
        return res.type === 'error' ? res.value : undefined;
      },
      onEdit: (r, c, raw) => {
        if (!collab) return;
        collab.applyLocal({ type: 'setCell', sheet: activeSheetId, baseRev: collab.rev, row: r, col: c, raw });
      },
      onSelect: (r, c) => sendSelect(r, c),
      onSelectionChange: (sel) => {
        selection = sel;
        sendPresence(sel.anchor.row, sel.anchor.col, false, undefined, sel.focus.row, sel.focus.col);
        const { r0, c0, r1, c1 } = normalize(sel);
        formulaBar?.setActive(rangeRefA1(r0, c0, r1, c1), rawValue(sel.focus.row, sel.focus.col));
        updateStats();
      },
      onLiveEdit: (r, c, raw) => sendLiveEdit(r, c, raw),
      onEditEnd: (r, c, committed) => {
        cancelPendingLive();
        // Commit path: the setCell op clears the overlay on receivers via
        // NEW_SHEET_OP.author — sending editing:false here would flicker.
        if (!committed) sendPresence(r, c, false);
      },
      onFill: (src, target) => {
        if (readOnly || !collab) return;
        for (const op of fillOps(src, target, activeSheetId, collab.rev, rawValue)) collab.applyLocal(op);
      },
    });
    onChange();
  };

  // Copy/Cut/Paste (TSV) and range delete. Skipped when a cell is mid-edit so
  // native in-cell text editing keeps its own clipboard/Delete behavior.
  //
  // These branches only run when !editingNow(), so the focused cell (if any)
  // has no unsaved edit: its DOM text matches its stored raw. We blur it
  // before applying the op so DomSheetView.render() (which otherwise skips
  // repainting `this.editing`) repaints the focused cell too. Blurring must
  // happen BEFORE the op, not after: after would make the blur listener's
  // commit check compare stale DOM text against the already-updated model
  // and clobber it.
  const blurActiveCell = (): void => {
    const el = document.activeElement as HTMLElement | null;
    if (el && el.tagName === 'TD' && el.isContentEditable) el.blur();
  };
  // doCopy/doCut/doPaste back both the Ctrl+C/X/V shortcuts and the ribbon's
  // clipboardAction buttons (same semantics: TSV, blur before ops, readOnly guards).
  const doCopy = (): void => {
    // ponytail: async Clipboard API only (requires secure context); a
    // hidden-textarea fallback is the upgrade path for plain-HTTP deploys.
    void navigator.clipboard.writeText(rangeToTSV(selection, rawValue));
  };
  const doCut = (): void => {
    void navigator.clipboard.writeText(rangeToTSV(selection, rawValue));
    if (readOnly || !collab) return;
    blurActiveCell();
    const { r0, c0, r1, c1 } = normalize(selection);
    collab.applyLocal({ type: 'clearRange', sheet: activeSheetId, baseRev: collab.rev, row: r0, col: c0, endRow: r1, endCol: c1 });
  };
  const doPaste = (): void => {
    if (readOnly || !collab) return;
    blurActiveCell();
    void navigator.clipboard.readText().then((text) => {
      if (text === '') return;
      if (!collab) return;
      const grid = parseTSV(text);
      const { r0, c0 } = normalize(selection);
      for (const op of pasteOps(grid, { row: r0, col: c0 }, activeSheetId, collab.rev)) collab.applyLocal(op);
    });
  };
  document.addEventListener('keydown', (e) => {
    if (!collab) return;
    const mod = e.ctrlKey || e.metaKey;
    if (mod && (e.key === 'c' || e.key === 'C') && !editingNow()) {
      e.preventDefault();
      doCopy();
      return;
    }
    if (mod && (e.key === 'x' || e.key === 'X') && !editingNow()) {
      e.preventDefault();
      doCut();
      return;
    }
    if (mod && (e.key === 'v' || e.key === 'V') && !editingNow() && !readOnly) {
      e.preventDefault();
      doPaste();
      return;
    }
    if ((e.key === 'Delete' || e.key === 'Backspace') && !editingNow() && !readOnly && !selIsSingle(selection)) {
      e.preventDefault();
      blurActiveCell();
      const { r0, c0, r1, c1 } = normalize(selection);
      collab.applyLocal({ type: 'clearRange', sheet: activeSheetId, baseRev: collab.rev, row: r0, col: c0, endRow: r1, endCol: c1 });
    }
  });

  const sendClientReady = (): void => {
    let token = Cookies.get('token');
    if (token == null || !padutils.isValidAuthorToken(token)) {
      token = padutils.generateAuthorToken();
      Cookies.set('token', token, { expires: 60 });
    }
    socket.emit('message', {
      component: 'sheet',
      type: 'CLIENT_READY',
      padId,
      token,
      userInfo: { colorId: null, name: null },
    });
  };

  // on, not once: after a reconnect the server has a fresh connection that is
  // not joined to the pad until CLIENT_READY is sent again — with `once` the
  // session silently stops receiving broadcasts after any network blip. The
  // SHEET_VARS reply re-runs initSheet with a fresh snapshot, which is exactly
  // the SHEET_RELOAD semantic.
  socket.on('connect', () => sendClientReady());
  socket.on('message', (msg: { type?: string; data?: any }) => {
    if (!msg || typeof msg !== 'object') return;
    if (msg.type === 'SHEET_VARS') {
      initSheet(msg.data as SheetVarsData);
      return;
    }
    if (msg.type === 'COLLABROOM' && msg.data) {
      const d = msg.data;
      if (d.type === 'ACCEPT_SHEET_OP') collab?.onAccept(d.newRev);
      else if (d.type === 'NEW_SHEET_OP') {
        collab?.onRemote(d.op as Op, d.newRev);
        if (d.author) presence?.clearLiveEdit(d.author);
      } else if (d.type === 'SHEET_PRESENCE') presence?.applyPresence(d as PresenceFrame);
      else if (d.type === 'USER_LEAVE') presence?.drop(d.userInfo?.userId);
      else if (d.type === 'SHEET_RELOAD') location.reload();
    }
  });
}

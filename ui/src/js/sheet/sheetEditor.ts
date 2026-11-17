import * as socketio from '../socketio';
import padutils, { Cookies } from '../pad_utils';
import { SheetCollabClient } from './sheetCollabClient';
import { FormulaEngine } from './formulaEngine';
import { DomSheetView } from './sheetView';
import { SheetPresence, effectiveCells, type PresenceFrame } from './sheetPresence';
import { rangeToTSV, rangeToCSV, parseTSV, parseCSV, pasteOps, fillOps } from './sheetClipboard';
import { normalize, selCells, selFromSingle, selIsSingle, type Selection } from './sheetSelection';
import { createToolbar, type ToolbarCallbacks, type ToolbarElement } from './sheetToolbar';
import { createSheetTabs } from './sheetTabs';
import { sortRangeOps, distinctValues, hiddenRowsForFilter } from './sheetSortFilter';
import { createFormulaBar, type FormulaBarHandle } from './sheetFormulaBar';
import { createFindDialog } from './sheetFindDialog';
import { findAll, findNext, matches, replaceAll, replaceInRaw } from './findReplace';
import { rangeRefA1 } from './a1';
import { mergeProps } from './styleCss';
import { createMetaActions } from './sheetMetaActions';
import { META_PROPS, PROP_COND_FMT, PROP_LINK, PROP_NOTE, PROP_VALID_LIST, condStyle, inList, listValues, parseCondRules } from './cellMeta';
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
/* View tab toggles (client-local, like Excel's Show checkboxes). */
.sheet-no-gridlines .sheet-grid, .sheet-no-gridlines .sheet-grid td { border-color: transparent; }
.sheet-no-headings .sheet-grid thead, .sheet-no-headings .sheet-grid tbody th { display: none; }
.sheet-ctx-menu { position: fixed; z-index: 40; min-width: 170px; background: #fff; border: 1px solid #d4d8dd; box-shadow: 0 4px 10px rgba(0,0,0,0.15); padding: 4px 0; font: 13px system-ui, sans-serif; }
.sheet-ctx-menu button { display: block; width: 100%; text-align: left; border: none; background: none; padding: 7px 14px; font: inherit; color: #333; cursor: pointer; }
.sheet-ctx-menu button:hover { background: #e6f2ec; }
.sheet-ctx-menu hr { border: none; border-top: 1px solid #e3e6e9; margin: 4px 0; }
.sheet-titlebar { display: flex; align-items: center; gap: 8px; height: 36px; flex: none; padding: 0 12px; background: #107c41; color: #fff; font: 14px system-ui, sans-serif; }
.sheet-titlebar svg { flex: none; display: block; }
.sheet-title-name { font-weight: 600; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.sheet-connection[data-state="reconnecting"], .sheet-connection[data-state="offline"] { color: #a4262c; font-weight: 600; }
.sheet-reconnect { font: inherit; padding: 0 6px; border: 1px solid #a4262c; border-radius: 3px; background: #fff; color: #a4262c; cursor: pointer; }
.sheet-statusbar { display: flex; align-items: center; justify-content: space-between; height: 22px; padding: 0 10px; background: #f5f6f7; border-top: 1px solid #d4d8dd; font: 12px system-ui, sans-serif; color: #444; }
.sheet-stats { display: flex; gap: 16px; }
.sheet-flash { color: #a4262c; font-weight: 600; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
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
  // Connection state, shown in the status bar. Edits made while the
  // connection is down are kept and sent after the reconnect.
  let connState: 'online' | 'reconnecting' | 'offline' = 'online';
  let connEl: HTMLElement | null = null;
  const renderConn = (): void => {
    if (!connEl) return;
    connEl.textContent = '';
    connEl.dataset.state = connState;
    if (connState === 'online') {
      connEl.textContent = 'Ready';
      return;
    }
    const kept = collab?.hasPending() ? ' Your changes are kept and will be saved.' : '';
    connEl.textContent = (connState === 'reconnecting' ? 'Reconnecting…' : 'Offline.') + kept;
    if (connState === 'offline') {
      const retry = document.createElement('button');
      retry.type = 'button';
      retry.className = 'sheet-reconnect';
      retry.textContent = 'Reconnect';
      retry.addEventListener('click', () => {
        connState = 'reconnecting';
        renderConn();
        // Still connected (the server only stopped listening, e.g. rate
        // limited): join again over this connection. Otherwise connect;
        // the 'reconnect' event joins.
        if (socket.getConnectionState().status === 'connected') rejoin();
        else socket.connect();
      });
      connEl.append(' ', retry);
    }
  };
  let toolbarEl: ToolbarElement | null = null;
  let metaActions: ReturnType<typeof createMetaActions> | null = null;

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

  const cellsOfActive = (): Array<{ row: number; col: number; raw: string }> => cellsOf(activeSheetId);

  const cellsOf = (sheetId: string): Array<{ row: number; col: number; raw: string }> => {
    const sheet = collab?.display.sheetById(sheetId);
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
    if (!cell || cell.raw === '') {
      // Cell has no content of its own, but an array formula elsewhere may
      // spill into it (=UNIQUE(..), =SORT(..), =SEQUENCE(..)). Excel shows
      // those spilled values, so ask the engine.
      const spilled = engine.getValue(r, c);
      return spilled.type === 'empty' ? '' : formatValue(spilled.value, '', propsOf(r, c).numFmt);
    }
    const raw = cell.raw.startsWith('=') ? engine.getValue(r, c).value : cell.raw;
    return formatValue(raw, '', propsOf(r, c).numFmt);
  };

  // clearContents empties the cells of a range but keeps their formatting,
  // notes, links and validation, like Excel's Delete key: one op.
  const clearContents = (r0: number, c0: number, r1: number, c1: number): void => {
    if (!collab) return;
    let any = false;
    for (const [k, cell] of collab.display.sheetById(activeSheetId)?.cells ?? []) {
      const i = k.indexOf(':');
      const r = Number(k.slice(0, i));
      const c = Number(k.slice(i + 1));
      if (cell.raw !== '' && r >= r0 && r <= r1 && c >= c0 && c <= c1) {
        any = true;
        break;
      }
    }
    if (!any) return;
    collab.applyLocal({
      type: 'clearRange', sheet: activeSheetId, baseRev: collab.rev,
      row: r0, col: c0, endRow: r1, endCol: c1, keepStyle: true,
    });
  };

  // commitRaw writes typed text into a cell. A cell with a list validation
  // only takes a value from its list, as in Excel ("Stop" style); the
  // editor's own writes (e.g. a link's display text) skip the check. Paste,
  // fill and Replace do not check either: Excel lets them through too.
  const commitRaw = (row: number, col: number, raw: string, trusted = false): void => {
    if (readOnly || !collab) return;
    const list = listValues(propsOf(row, col)[PROP_VALID_LIST]);
    if (!trusted && list.length > 0 && !inList(list, raw)) {
      flash(`"${raw}" is not in the list of allowed values: ${list.join(', ')}`);
      onChange();
      return;
    }
    // Use the list's spelling for a value typed in another case.
    const exact = list.find((v) => v.toLowerCase() === raw.trim().toLowerCase());
    collab.applyLocal({ type: 'setCell', sheet: activeSheetId, baseRev: collab.rev, row, col, raw: exact ?? raw });
  };

  // flash shows a short message in the status bar.
  let flashTimer: ReturnType<typeof setTimeout> | null = null;
  const flash = (msg: string): void => {
    const el = document.querySelector<HTMLElement>('.sheet-flash');
    if (!el) return;
    el.textContent = msg;
    el.hidden = false;
    if (flashTimer) clearTimeout(flashTimer);
    flashTimer = setTimeout(() => { el.hidden = true; }, 5000);
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
    // Every sheet goes to the engine, so formulas can reference other sheets,
    // with what others are typing on it. A sheet whose cells and live edits
    // did not change is skipped without being read (cursor moves).
    const sheets = collab?.display.sheets ?? [];
    if (sheets.length === 0) {
      engine.setGrid(effectiveCells(cellsOfActive(), live));
    } else {
      engine.setWorkbook(
        sheets.map((sh) => {
          const sheetLive = sh.id === activeSheetId ? live : presence?.liveEditsForSheet(sh.id) ?? [];
          return {
            name: sh.name,
            key: `${sh.stamp}|${sheetLive.length === 0 ? '' : JSON.stringify(sheetLive.map((e) => [e.row, e.col, e.raw]))}`,
            cells: () => effectiveCells(cellsOf(sh.id), sheetLive),
          };
        }),
        collab?.display.sheetById(activeSheetId)?.name ?? '',
      );
    }
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
    toolbarEl?.refreshHistory();
    if (formulaBar) {
      const { r0, c0, r1, c1 } = normalize(selection);
      formulaBar.setActive(rangeRefA1(r0, c0, r1, c1), rawValue(selection.focus.row, selection.focus.col));
    }
    updateStats();
    if (connState !== 'online') renderConn();
  };

  const editingNow = (): boolean => view?.isEditing() ?? false;

  const initSheet = (data: SheetVarsData): void => {
    activeSheetId = data.snapshot.sheets?.[0]?.id ?? 's1';
    readOnly = data.readonly;
    // A client that could not be caught up gets a fresh snapshot; its
    // unconfirmed edits are carried over instead of being dropped.
    const carried = collab?.pendingOps() ?? [];
    collab = new SheetCollabClient(data.snapshot, data.head, transport);
    collab.onChange = onChange;
    connState = 'online';
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
    // Held in a named object so the cell context menu can trigger the same
    // actions as the ribbon instead of duplicating them.
    const actions: ToolbarCallbacks = {
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
      autoSum: (fn = 'SUM') => {
        if (!readOnly) formulaBar?.beginFormula(`=${fn}(`);
      },
      fill: doFill,
      undo: () => doHistory('undo'),
      redo: () => doHistory('redo'),
      history: () => ({ canUndo: collab?.canUndo() ?? false, canRedo: collab?.canRedo() ?? false }),
      openFind: (mode: 'find' | 'replace') => findDialog.open(readOnly ? 'find' : mode),
      clear: (what: 'all' | 'formats' | 'contents') => {
        if (readOnly || !collab) return;
        blurActiveCell();
        const { r0, c0, r1, c1 } = normalize(selection);
        if (what === 'all') {
          collab.applyLocal({ type: 'clearRange', sheet: activeSheetId, baseRev: collab.rev, row: r0, col: c0, endRow: r1, endCol: c1 });
        } else if (what === 'contents') {
          clearContents(r0, c0, r1, c1);
        }
        if (what === 'formats') {
          // Excel keeps notes, links and validation on Clear Formats.
          for (const { row, col } of selCells(selection)) {
            const props = propsOf(row, col);
            if (Object.keys(props).length === 0) continue;
            const kept: Record<string, string> = {};
            for (const k of META_PROPS) if (props[k]) kept[k] = props[k];
            if (Object.keys(kept).length === Object.keys(props).length) continue;
            collab.applyLocal({ type: 'setStyle', sheet: activeSheetId, baseRev: collab.rev, row, col, props: kept });
          }
        }
      },
      viewOption: (opt: 'gridlines' | 'headings' | 'zoom', value: boolean | number) => {
        if (opt === 'zoom') {
          // font-size scaling, not a CSS transform: the grid keeps laying out
          // normally, so sticky headers and hit testing stay correct.
          gridHost.style.fontSize = `${(Number(value) * 13) / 100}px`;
          return;
        }
        gridHost.classList.toggle(`sheet-no-${opt}`, value === false);
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
    };
    const meta = createMetaActions({
      readOnly: () => readOnly || !collab,
      focus: () => selection.focus,
      cells: () => selCells(selection),
      propsOf,
      rawValue,
      setProps: (row, col, props) => {
        collab?.applyLocal({ type: 'setStyle', sheet: activeSheetId, baseRev: collab.rev, row, col, props });
      },
      setRaw: (row, col, raw) => commitRaw(row, col, raw, true),
      blur: () => blurActiveCell(),
    });
    Object.assign(actions, meta);
    metaActions = meta;
    const toolbar = createToolbar(actions);
    toolbarEl = toolbar;
    formulaBar = createFormulaBar({
      readOnly: data.readonly,
      getFunctionNames: () => engine.functionNames(),
      onCommit: (raw) => {
        const { row, col } = selection.focus;
        commitRaw(row, col, raw);
      },
    });
    const gridHost = document.createElement('div');
    gridHost.className = 'sheet-grid-host';

    // Right-click menu on the grid, wired to the same actions as the ribbon.
    gridHost.addEventListener('contextmenu', (e) => {
      if (!(e.target as HTMLElement).closest('td')) return;
      e.preventDefault();
      document.querySelector('.sheet-ctx-menu')?.remove();
      const menu = document.createElement('div');
      menu.className = 'sheet-ctx-menu';
      const items: Array<[string, () => void] | null> = readOnly
        ? [['Copy', () => actions.clipboardAction?.('copy')]]
        : [
            ['Cut', () => actions.clipboardAction?.('cut')],
            ['Copy', () => actions.clipboardAction?.('copy')],
            ['Paste', () => actions.clipboardAction?.('paste')],
            null,
            ['Insert row above', () => actions.structural?.('insRowAbove')],
            ['Insert column left', () => actions.structural?.('insColLeft')],
            ['Delete rows', () => actions.structural?.('delRows')],
            ['Delete columns', () => actions.structural?.('delCols')],
            null,
            ['Clear contents', () => actions.clear?.('contents')],
            ['Merge / unmerge', () => actions.mergeToggle?.()],
            null,
            [propsOf(selection.focus.row, selection.focus.col)[PROP_LINK] ? 'Edit link…' : 'Link…', () => actions.insertLink?.()],
            ...(propsOf(selection.focus.row, selection.focus.col)[PROP_LINK] ? [['Remove link', () => actions.removeLink?.()] as [string, () => void]] : []),
            [propsOf(selection.focus.row, selection.focus.col)[PROP_NOTE] ? 'Edit note…' : 'New note…', () => actions.editNote?.()],
            ...(propsOf(selection.focus.row, selection.focus.col)[PROP_NOTE] ? [['Delete note', () => actions.deleteNote?.()] as [string, () => void]] : []),
          ];
      const close = () => {
        menu.remove();
        document.removeEventListener('mousedown', outside, true);
      };
      const outside = (ev: MouseEvent) => {
        if (!menu.contains(ev.target as Node)) close();
      };
      for (const item of items) {
        if (!item) {
          menu.appendChild(document.createElement('hr'));
          continue;
        }
        const [label, run] = item;
        const b = document.createElement('button');
        b.textContent = label;
        b.addEventListener('click', () => { close(); run(); });
        menu.appendChild(b);
      }
      // Positioned inside the viewport (fixed coordinates, so no scroll math).
      menu.style.left = `${Math.min(e.clientX, window.innerWidth - 180)}px`;
      menu.style.top = `${Math.min(e.clientY, window.innerHeight - menu.childElementCount * 30 - 16)}px`;
      document.body.appendChild(menu);
      document.addEventListener('mousedown', outside, true);
    });

    root.appendChild(toolbar);
    root.appendChild(formulaBar.el);
    root.appendChild(gridHost);
    root.appendChild(findDialog.el);

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
    readyEl.className = 'sheet-connection';
    connEl = readyEl;
    renderConn();
    statsEl = document.createElement('span');
    statsEl.className = 'sheet-stats';
    const flashEl = document.createElement('span');
    flashEl.className = 'sheet-flash';
    flashEl.setAttribute('role', 'alert');
    flashEl.hidden = true;
    statusbar.append(readyEl, flashEl, statsEl);
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
        // '' included: an array formula can spill an error into a blank cell.
        if (cell && cell.raw !== '' && !cell.raw.startsWith('=')) return undefined;
        const res = engine.getValue(r, c);
        return res.type === 'error' ? res.value : undefined;
      },
      defaultAlignOf: (r, c) => {
        if (propsOf(r, c).numFmt === 'text') return undefined;
        // The engine holds the same raws, so it types literals the way it
        // computes with them (5%, 1,000 and dates are numbers).
        const type = engine.getValue(r, c).type;
        if (type === 'number') return 'right';
        if (type === 'bool' || type === 'error') return 'center';
        return undefined;
      },
      onEdit: (r, c, raw) => commitRaw(r, c, raw),
      metaOf: (r, c) => {
        const p = propsOf(r, c);
        return { link: p[PROP_LINK], note: p[PROP_NOTE], list: p[PROP_VALID_LIST] ? listValues(p[PROP_VALID_LIST]) : undefined };
      },
      condStyleOf: (r, c) => {
        const cf = propsOf(r, c)[PROP_COND_FMT];
        if (!cf) return {};
        const raw = rawValue(r, c);
        const value = raw.startsWith('=') ? engine.getValue(r, c).value : raw;
        return condStyle(parseCondRules(cf), value);
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
    if (carried.length > 0 && !readOnly) collab.requeue(carried);
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
  // --- Find & Replace ---------------------------------------------------
  // Searches the raw cell content (Excel's default "Look in: Formulas"), so a
  // formula is found by its text and a replacement rewrites the formula.
  const findDialog = createFindDialog({
    readOnly: false, // re-checked per action: `readOnly` is only known after the handshake
    findNext: (query, opts) => {
      const cells = cellsOfActive();
      const hit = findNext(cells, query, selection.focus, opts);
      if (hit) view?.setSelection(selFromSingle(hit.row, hit.col));
      return { found: hit !== null, total: findAll(cells, query, opts).length };
    },
    replace: (query, replacement, opts) => {
      const here = selection.focus;
      const raw = rawValue(here.row, here.col);
      let replaced = false;
      if (!readOnly && collab && matches(raw, query, opts)) {
        collab.applyLocal({
          type: 'setCell', sheet: activeSheetId, baseRev: collab.rev,
          row: here.row, col: here.col, raw: replaceInRaw(raw, query, replacement, opts),
        });
        replaced = true;
      }
      const hit = findNext(cellsOfActive(), query, here, opts);
      if (hit) view?.setSelection(selFromSingle(hit.row, hit.col));
      return { replaced, total: findAll(cellsOfActive(), query, opts).length };
    },
    replaceAll: (query, replacement, opts) => {
      if (readOnly || !collab) return 0;
      blurActiveCell();
      const changed = replaceAll(cellsOfActive(), query, replacement, opts);
      // One tick, so the whole sweep is a single undo step.
      for (const c of changed) {
        collab.applyLocal({ type: 'setCell', sheet: activeSheetId, baseRev: collab.rev, row: c.row, col: c.col, raw: c.raw });
      }
      return changed.length;
    },
  });

  // Undo/redo this client's own edits. Blur first so a half-typed cell does not
  // get committed over the restored value by the blur handler.
  const doHistory = (which: 'undo' | 'redo'): void => {
    if (readOnly || !collab) return;
    blurActiveCell();
    if (which === 'undo') collab.undo();
    else collab.redo();
  };

  // Fill the selection from its first row (down) or first column (right).
  // fillOps adjusts relative references, so formulas fill like in Excel.
  const doFill = (dir: 'down' | 'right'): void => {
    if (readOnly || !collab || selIsSingle(selection)) return;
    blurActiveCell();
    const { r0, c0, r1, c1 } = normalize(selection);
    const src = { anchor: { row: r0, col: c0 }, focus: dir === 'down' ? { row: r0, col: c1 } : { row: r1, col: c0 } };
    for (const op of fillOps(src, selection, activeSheetId, collab.rev, rawValue)) collab.applyLocal(op);
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
    // Ctrl+F / Ctrl+H open the dialog (Ctrl+H only when it can replace).
    if (mod && !editingNow() && (e.key === 'f' || e.key === 'F' || e.key === 'h' || e.key === 'H')) {
      e.preventDefault();
      findDialog.open(e.key.toLowerCase() === 'h' && !readOnly ? 'replace' : 'find');
      return;
    }
    // Ctrl+Z / Ctrl+Y (and Ctrl+Shift+Z) — only outside cell editing, where the
    // browser's own text undo still owns the keystroke.
    if (mod && !editingNow() && !readOnly) {
      const k = e.key.toLowerCase();
      if (k === 'z' && !e.shiftKey) {
        e.preventDefault();
        doHistory('undo');
        return;
      }
      if (k === 'y' || (k === 'z' && e.shiftKey)) {
        e.preventDefault();
        doHistory('redo');
        return;
      }
    }
    // Ctrl+K inserts a link, Shift+F2 edits the note (Excel's shortcuts).
    if (!readOnly && !editingNow() && ((mod && (e.key === 'k' || e.key === 'K')) || (e.shiftKey && e.key === 'F2'))) {
      e.preventDefault();
      if (e.key === 'F2') metaActions?.editNote();
      else metaActions?.insertLink();
      return;
    }
    // Ctrl+D / Ctrl+R fill the selection from its first row / column, like Excel.
    if (mod && !editingNow() && !readOnly && (e.key === 'd' || e.key === 'D' || e.key === 'r' || e.key === 'R')) {
      e.preventDefault();
      doFill(e.key.toLowerCase() === 'd' ? 'down' : 'right');
      return;
    }
    // Ctrl/Cmd+B/I/U toggle the style on the selection, mirroring the ribbon's
    // toggle buttons. Applying a style blurs the active cell, so the next
    // shortcut arrives with focus on <body> — we must NOT require grid focus
    // (that would break chaining B then I). Instead just skip real form fields
    // so the formula bar keeps these keys. preventDefault stops the browser's
    // contenteditable rich-text default on the focused cell.
    const tag = (e.target as HTMLElement | null)?.tagName;
    const inField = tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT';
    if (mod && !editingNow() && !readOnly && !inField) {
      const k = e.key.toLowerCase();
      const styleKey = k === 'b' ? 'bold' : k === 'i' ? 'italic' : k === 'u' ? 'underline' : null;
      if (styleKey) {
        e.preventDefault();
        const on = propsOf(selection.focus.row, selection.focus.col)[styleKey] === '1';
        applyStyleToSelection({ [styleKey]: on ? '' : '1' });
        return;
      }
    }
    // Clear the selection (single cell or range), like Excel. The grid-focus
    // guard replaces the old single-cell exclusion: it lets Delete clear one
    // cell while still keeping Backspace working in the formula bar and any
    // other input outside the grid (selection is single there too).
    if (
      (e.key === 'Delete' || e.key === 'Backspace') &&
      !editingNow() && !readOnly &&
      (e.target as HTMLElement | null)?.closest?.('.sheet-grid')
    ) {
      e.preventDefault();
      blurActiveCell();
      const { r0, c0, r1, c1 } = normalize(selection);
      clearContents(r0, c0, r1, c1);
    }
  });

  const sendClientReady = (): void => {
    // The author token is an HttpOnly cookie (see pad.ts); only an older
    // version's readable cookie is passed on.
    const legacyToken = Cookies.get('token');
    const token = legacyToken != null && padutils.isValidAuthorToken(legacyToken) ? legacyToken : '';
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
  // After an outage the server has a fresh connection that has to join again.
  // With a state to continue from, the client asks for the ops it missed
  // (ending with SHEET_CAUGHT_UP); otherwise it starts over.
  const rejoin = (): void => {
    if (!collab) {
      sendClientReady();
      return;
    }
    const legacyToken = Cookies.get('token');
    socket.emit('message', {
      component: 'sheet',
      type: 'CLIENT_READY',
      padId,
      token: legacyToken != null && padutils.isValidAuthorToken(legacyToken) ? legacyToken : '',
      userInfo: { colorId: null, name: null },
      reconnect: true,
      client_rev: collab.rev,
    });
  };
  socket.on('reconnect', rejoin);
  socket.on('disconnect', () => {
    collab?.disconnected();
    connState = 'reconnecting';
    renderConn();
  });
  socket.on('reconnect_failed', () => {
    connState = 'offline';
    renderConn();
  });
  // Leaving the page would lose edits the server has not confirmed.
  window.addEventListener('beforeunload', (e) => {
    if (collab?.hasPending()) {
      e.preventDefault();
      e.returnValue = '';
    }
  });
  socket.on('message', (msg: { type?: string; data?: any }) => {
    if (!msg || typeof msg !== 'object') return;
    // The server stopped taking messages from this connection (e.g.
    // rateLimited, as in etherpad-lite): an op it dropped is still
    // pending here and goes out again after "Reconnect".
    const disconnect = (msg as { disconnect?: unknown }).disconnect;
    if (typeof disconnect === 'string') {
      collab?.disconnected();
      connState = 'offline';
      renderConn();
      return;
    }
    if (msg.type === 'SHEET_VARS') {
      initSheet(msg.data as SheetVarsData);
      return;
    }
    if (msg.type === 'COLLABROOM' && msg.data) {
      const d = msg.data;
      if (d.type === 'ACCEPT_SHEET_OP') collab?.onAccept(d.newRev, d.cid);
      else if (d.type === 'SHEET_CAUGHT_UP') {
        connState = 'online';
        collab?.resume();
        renderConn();
      }
      else if (d.type === 'NEW_SHEET_OP') {
        collab?.onRemote(d.op as Op, d.newRev);
        if (d.author) presence?.clearLiveEdit(d.author);
      } else if (d.type === 'SHEET_PRESENCE') presence?.applyPresence(d as PresenceFrame);
      else if (d.type === 'USER_LEAVE') presence?.drop(d.userInfo?.userId);
      else if (d.type === 'SHEET_RELOAD') location.reload();
    }
  });
}

import * as socketio from '../socketio';
import padutils, { Cookies } from '../pad_utils';
import { SheetCollabClient } from './sheetCollabClient';
import { FormulaEngine } from './formulaEngine';
import { DomSheetView } from './sheetView';
import { SheetPresence, effectiveCells, type PresenceFrame } from './sheetPresence';
import type { Op } from './op';
import type { WorkbookSnapshot } from './workbookState';

interface SheetVarsData {
  snapshot: WorkbookSnapshot;
  head: number;
  userId: string;
  userColor: string;
  readonly: boolean;
}

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
  let presence: SheetPresence | null = null;
  let activeSheetId = 's1';
  const engine = new FormulaEngine();

  const transport = {
    send: (op: Op) =>
      socket.emit('message', {
        type: 'COLLABROOM',
        component: 'sheet',
        data: { type: 'SHEET_OP', op, baseRev: op.baseRev },
      }),
  };

  const sendPresence = (row: number, col: number, editing: boolean, raw?: string): void =>
    socket.emit('message', {
      type: 'COLLABROOM',
      component: 'sheet',
      data: { type: 'SHEET_PRESENCE', sheet: activeSheetId, row, col, editing, raw },
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

  const displayValue = (r: number, c: number): string => {
    const cell = collab?.display.getCell(activeSheetId, r, c);
    if (!cell || cell.raw === '') return '';
    if (cell.raw.startsWith('=')) return engine.getValue(r, c).value;
    return cell.raw;
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
    }
    view?.render();
  };

  const initSheet = (data: SheetVarsData): void => {
    activeSheetId = data.snapshot.sheets?.[0]?.id ?? 's1';
    collab = new SheetCollabClient(data.snapshot, data.head, transport);
    collab.onChange = onChange;
    presence = new SheetPresence(data.userId);
    presence.onChange = onChange;
    view = new DomSheetView(root, {
      rows: 50,
      cols: 20,
      rawValue,
      displayValue,
      readOnly: data.readonly,
      onEdit: (r, c, raw) => {
        if (!collab) return;
        collab.applyLocal({ type: 'setCell', sheet: activeSheetId, baseRev: collab.rev, row: r, col: c, raw });
      },
      onSelect: (r, c) => sendSelect(r, c),
      onLiveEdit: (r, c, raw) => sendLiveEdit(r, c, raw),
      onEditEnd: (r, c, committed) => {
        cancelPendingLive();
        // Commit path: the setCell op clears the overlay on receivers via
        // NEW_SHEET_OP.author — sending editing:false here would flicker.
        if (!committed) sendPresence(r, c, false);
      },
    });
    onChange();
  };

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

  socket.once('connect', () => sendClientReady());
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

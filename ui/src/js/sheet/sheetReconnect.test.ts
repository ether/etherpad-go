import { describe, expect, it } from 'vitest';
import { SheetCollabClient } from './sheetCollabClient';
import { WorkbookState, type WorkbookSnapshot } from './workbookState';
import { transform } from './transform';
import type { Op } from './op';

// A server like lib/sheetdoc.Manager.SubmitOnce: it rebases, stores the op with
// its cid, drops a second copy of a stored cid, and replays missed ops on a
// reconnect. The test controls which messages reach the client, to model a
// connection that drops before the op or before its acknowledgement.
class Server {
  wb = new WorkbookState();
  log: Op[] = [];

  constructor() {
    this.wb.addSheet('s1', 'Sheet1');
  }

  // submit returns the revision and whether the op was a duplicate.
  submit(op: Op): { rev: number; duplicate: boolean } {
    if (op.cid) {
      for (let i = Math.max(op.baseRev, 0); i < this.log.length; i++) {
        if (this.log[i].cid === op.cid) return { rev: i + 1, duplicate: true };
      }
    }
    let rebased: Op = { ...op };
    for (let i = op.baseRev; i < this.log.length; i++) rebased = transform(rebased, this.log[i]);
    rebased.baseRev = this.log.length;
    this.wb.applyOp(rebased);
    this.log.push(rebased);
    return { rev: this.log.length, duplicate: false };
  }

  // reconnect replays the ops after clientRev, then SHEET_CAUGHT_UP.
  reconnect(c: SheetCollabClient, clientRev: number): void {
    for (let i = clientRev; i < this.log.length; i++) c.onRemote(this.log[i], i + 1);
    c.resume();
  }
}

const snap: WorkbookSnapshot = { sheets: [{ id: 's1', name: 'Sheet1', cells: [] }] };

const setCell = (row: number, raw: string): Op => ({ type: 'setCell', sheet: 's1', baseRev: 0, row, col: 0, raw });
const insertRows = (index: number): Op => ({ type: 'insertRows', sheet: 's1', baseRev: 0, index, count: 1 });

const cells = (wb: WorkbookState): string[] => {
  const out: string[] = [];
  for (let r = 0; r < 6; r++) out.push(wb.getCell('s1', r, 0)?.raw ?? '');
  return out;
};

const setup = () => {
  const server = new Server();
  const sent: Op[] = [];
  const c = new SheetCollabClient(snap, 0, { send: (op) => sent.push(op) });
  return { server, sent, c };
};

describe('sheet reconnect', () => {
  it('resends an op the server never got, and edits made while offline', () => {
    const { server, sent, c } = setup();
    c.applyLocal(setCell(0, 'a'));
    expect(sent).toHaveLength(1); // in flight, lost with the connection
    c.disconnected();
    c.applyLocal(setCell(1, 'b'));
    expect(sent).toHaveLength(1); // nothing is sent while offline

    // Someone else inserted a row at the top meanwhile.
    server.submit(insertRows(0));
    server.reconnect(c, c.rev);
    // The lost op goes out again, same cid, moved below the new row.
    expect(sent).toHaveLength(2);
    expect(sent[1].cid).toBe(sent[0].cid);
    expect(sent[1].row).toBe(1);
    expect(sent[1].baseRev).toBe(1);

    const ack = (op: Op) => { const r = server.submit(op); c.onAccept(r.rev, op.cid); };
    ack(sent[1]);
    ack(sent[2]);
    expect(c.hasPending()).toBe(false);
    expect(cells(server.wb)).toEqual(['', 'a', 'b', '', '', '']);
    expect(cells(c.display)).toEqual(cells(server.wb));
  });

  it('takes its own op from the replay when only the acknowledgement was lost', () => {
    const { server, sent, c } = setup();
    c.applyLocal(insertRows(0)); // not idempotent: applying it twice would show
    c.applyLocal(setCell(0, 'x'));
    server.submit(sent[0]); // stored, but the ack never arrives
    c.disconnected();

    server.reconnect(c, c.rev);
    // The replayed op counted as the ack; the next op went out, not the first again.
    expect(sent).toHaveLength(2);
    expect(sent[1].type).toBe('setCell');
    const r = server.submit(sent[1]);
    c.onAccept(r.rev, sent[1].cid);

    expect(server.log.filter((o) => o.type === 'insertRows')).toHaveLength(1);
    expect(cells(c.display)).toEqual(cells(server.wb));
    expect(c.rev).toBe(2);
  });

  it('a resent copy of a stored op is not applied twice', () => {
    const { server, sent, c } = setup();
    c.applyLocal(insertRows(0));
    server.submit(sent[0]);
    // The server stored it, but the replay raced past it: the client resends.
    c.disconnected();
    c.resume();
    expect(sent).toHaveLength(2);
    const r = server.submit(sent[1]);
    expect(r.duplicate).toBe(true);
    expect(server.log).toHaveLength(1);
    // The client took the op from the broadcast, so the duplicate ack is ignored.
    c.onRemote(server.log[0], 1);
    c.onAccept(r.rev, sent[1].cid);
    expect(c.rev).toBe(1);
    expect(c.hasPending()).toBe(false);
  });

  it('ignores an acknowledgement for another op', () => {
    const { sent, c } = setup();
    c.applyLocal(setCell(0, 'a'));
    c.onAccept(1, 'someone:1');
    expect(c.hasPending()).toBe(true);
    c.onAccept(1, sent[0].cid);
    expect(c.hasPending()).toBe(false);
  });

  it('carries unconfirmed edits over to a fresh snapshot', () => {
    const { sent, c } = setup();
    c.disconnected();
    c.applyLocal(setCell(2, 'kept'));
    const fresh = new SheetCollabClient({ sheets: [{ id: 's1', name: 'Sheet1', cells: [] }] }, 7, { send: (op) => sent.push(op) });
    fresh.requeue(c.pendingOps());
    expect(cells(fresh.display)[2]).toBe('kept');
    expect(sent).toHaveLength(1);
    expect(sent[0].baseRev).toBe(7);
  });
});

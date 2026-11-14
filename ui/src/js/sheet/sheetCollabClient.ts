import type { Op } from './op';
import { transform } from './transform';
import { invertOp } from './undo';
import { WorkbookState, type WorkbookSnapshot } from './workbookState';

// Bounded so a long session cannot grow the history without limit; Excel caps
// its own undo list too.
const MAX_HISTORY = 100;

// CollabTransport is the outbound channel for ops (wraps the socket emit).
export interface CollabTransport {
  send(op: Op): void;
}

// SheetCollabClient mirrors the text collab_client.ts reconcile model for the
// op-based sheet protocol.
//
// It keeps `serverWb` as a faithful replay of the server's confirmed op log and
// `display` as the optimistic view (serverWb + locally-pending ops). Convergence
// argument: remote ops arrive already rebased by the server; the in-flight op is
// transformed against every remote op received before its ACCEPT, which equals
// the server's own rebase — so applying it to serverWb on ACCEPT reproduces the
// server state. Hence serverWb is always a replay of the server log and
// converges, exactly like lib/sheet's Document.
//
// Reconnects: while the connection is down nothing is sent and edits queue up
// in `pending`. After the reconnect the server replays the ops this client
// missed; the in-flight op is among them if the server stored it (found by its
// cid), and then counts as accepted. SHEET_CAUGHT_UP ends the replay: an op
// still waiting was not stored and is sent again (the server would also drop a
// second copy of a stored op by its cid).
export class SheetCollabClient {
  rev: number;
  display: WorkbookState;
  onChange: () => void = () => {};

  private serverWb: WorkbookState;
  private pending: Op[] = [];
  private committing = false;
  private connected = true;
  private transport: CollabTransport;
  private readonly clientId: string = newClientId();
  private seq = 0;

  // Undo history. Each entry is the op list that reverts one user action; the
  // ops are recorded per applyLocal but grouped per tick, so a multi-op action
  // (paste, fill, styling a range) undoes in one step. Only local ops are ever
  // recorded, so undo never touches another collaborator's work.
  private undoStack: Op[][] = [];
  private redoStack: Op[][] = [];
  private group: Op[] | null = null;
  private mode: 'edit' | 'undo' | 'redo' = 'edit';

  constructor(snap: WorkbookSnapshot, head: number, transport: CollabTransport) {
    this.rev = head;
    this.serverWb = new WorkbookState();
    this.serverWb.loadSnapshot(snap);
    this.transport = transport;
    this.display = this.serverWb.clone();
  }

  // hasPending reports edits the server has not confirmed yet.
  hasPending(): boolean {
    return this.pending.length > 0;
  }

  // pendingOps returns the unconfirmed edits, oldest first.
  pendingOps(): Op[] {
    return this.pending.map((p) => ({ ...p }));
  }

  // disconnected stops sending until resume(); edits keep queueing.
  disconnected(): void {
    this.connected = false;
  }

  // resume is called once the server has replayed the ops this client missed
  // (SHEET_CAUGHT_UP). An op still in flight was not stored, so it is sent
  // again, with its cid, against the current revision.
  resume(): void {
    this.connected = true;
    this.committing = false;
    this.flush();
  }

  // requeue takes over the unconfirmed edits of a client whose state could not
  // be caught up (the server started it over from a snapshot). They are sent
  // as new edits against the current state; best effort, as their base is gone.
  requeue(ops: Op[]): void {
    for (const op of ops) {
      const { cid: _cid, ...rest } = op;
      this.pending.push({ ...rest, baseRev: this.rev });
      try {
        this.display.applyOp(this.pending[this.pending.length - 1]);
      } catch {
        this.pending.pop(); // no longer applicable (e.g. its sheet is gone)
      }
    }
    this.onChange();
    this.flush();
  }

  // confirmedState exposes the server-confirmed workbook (for tests/convergence).
  confirmedState(): WorkbookState {
    return this.serverWb;
  }

  // applyLocal applies a local edit optimistically and schedules it for sending.
  applyLocal(op: Op): void {
    // Computed against the pre-op display state — that is what the inverse has
    // to restore.
    const inverse = invertOp(this.display, op);
    this.pending.push(op);
    this.display.applyOp(op);
    this.record(inverse);
    this.onChange();
    this.flush();
  }

  canUndo(): boolean {
    return this.undoStack.length > 0;
  }

  canRedo(): boolean {
    return this.redoStack.length > 0;
  }

  // undo/redo replay a recorded entry as ordinary local ops, which is what makes
  // them collaboration-safe: they are transformed, sent and acked like any edit.
  undo(): void {
    this.replay(this.undoStack.pop(), 'undo');
  }

  redo(): void {
    this.replay(this.redoStack.pop(), 'redo');
  }

  private replay(entry: Op[] | undefined, mode: 'undo' | 'redo'): void {
    if (!entry) return;
    this.mode = mode;
    for (const op of entry) this.applyLocal({ ...op, baseRev: this.rev });
    this.closeGroup(); // the entry is complete now; do not wait for the tick
  }

  // record collects the inverse ops of one tick into a single history entry.
  // Prepending keeps them in reverse application order, so undoing a group
  // reverts its last op first.
  private record(inverse: Op[]): void {
    if (inverse.length === 0) return;
    if (this.group === null) {
      this.group = [];
      queueMicrotask(() => this.closeGroup());
    }
    this.group.unshift(...inverse);
  }

  private closeGroup(): void {
    const entry = this.group;
    this.group = null;
    const mode = this.mode;
    this.mode = 'edit';
    if (!entry || entry.length === 0) return;
    if (mode === 'undo') {
      this.redoStack.push(entry);
      return;
    }
    this.undoStack.push(entry);
    if (this.undoStack.length > MAX_HISTORY) this.undoStack.shift();
    // A fresh edit invalidates the redo branch; a redo keeps it.
    if (mode === 'edit') this.redoStack.length = 0;
  }

  private flush(): void {
    if (!this.connected || this.committing || this.pending.length === 0) return;
    this.committing = true;
    // A resent op keeps its cid: that is how the server recognises it.
    const inflight: Op = { ...this.pending[0], baseRev: this.rev, cid: this.pending[0].cid ?? `${this.clientId}:${++this.seq}` };
    this.pending[0] = inflight;
    this.transport.send(inflight);
  }

  // onAccept confirms the in-flight op. Its current (transformed) form equals the
  // server's rebased op, so applying it to serverWb keeps serverWb == server.
  // An acknowledgement naming another op (a resent op the client already took
  // from the replayed ops) or no newer revision changes nothing.
  onAccept(newRev: number, cid?: string): void {
    if (!this.committing || this.pending.length === 0) return;
    if (cid !== undefined && cid !== this.pending[0].cid) return;
    if (newRev <= this.rev) return;
    const confirmed = this.pending.shift() as Op;
    this.serverWb.applyOp(confirmed);
    this.rev = newRev;
    this.committing = false;
    this.rebuildDisplay();
    this.onChange();
    this.flush();
  }

  // onRemote applies a remote (already server-rebased) op and re-bases the local
  // pending ops on top of it.
  onRemote(remoteOp: Op, newRev: number): void {
    if (newRev <= this.rev) return; // already applied
    // Our own in-flight op, replayed after a reconnect: its ack was lost.
    if (this.committing && remoteOp.cid !== undefined && remoteOp.cid === this.pending[0]?.cid) {
      this.onAccept(newRev, remoteOp.cid);
      return;
    }
    this.serverWb.applyOp(remoteOp);
    this.rev = newRev;
    this.pending = this.pending.map((p) => transform(p, remoteOp));
    // The history holds ops against the old coordinate space too: a remote row
    // insert must move a recorded undo the same way it moves a pending op.
    const rebase = (stack: Op[][]): Op[][] => stack.map((entry) => entry.map((o) => transform(o, remoteOp)));
    this.undoStack = rebase(this.undoStack);
    this.redoStack = rebase(this.redoStack);
    this.rebuildDisplay();
    this.onChange();
  }

  private rebuildDisplay(): void {
    this.display = this.serverWb.clone();
    for (const p of this.pending) this.display.applyOp(p);
  }
}

function newClientId(): string {
  const bytes = new Uint8Array(8);
  crypto.getRandomValues(bytes);
  return Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('');
}

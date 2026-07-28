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
export class SheetCollabClient {
  rev: number;
  display: WorkbookState;
  onChange: () => void = () => {};

  private serverWb: WorkbookState;
  private pending: Op[] = [];
  private committing = false;
  private transport: CollabTransport;

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
    if (this.committing || this.pending.length === 0) return;
    this.committing = true;
    const inflight: Op = { ...this.pending[0], baseRev: this.rev };
    this.pending[0] = inflight;
    this.transport.send(inflight);
  }

  // onAccept confirms the in-flight op. Its current (transformed) form equals the
  // server's rebased op, so applying it to serverWb keeps serverWb == server.
  onAccept(newRev: number): void {
    if (this.pending.length === 0) return;
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

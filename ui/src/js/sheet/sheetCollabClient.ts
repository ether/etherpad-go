import type { Op } from './op';
import { transform } from './transform';
import { WorkbookState, type WorkbookSnapshot } from './workbookState';

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
    this.pending.push(op);
    this.display.applyOp(op);
    this.onChange();
    this.flush();
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
    this.rebuildDisplay();
    this.onChange();
  }

  private rebuildDisplay(): void {
    this.display = this.serverWb.clone();
    for (const p of this.pending) this.display.applyOp(p);
  }
}

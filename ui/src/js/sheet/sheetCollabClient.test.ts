import { describe, it, expect } from 'vitest';
import { SheetCollabClient } from './sheetCollabClient';
import { WorkbookState, type WorkbookSnapshot } from './workbookState';
import { transform } from './transform';
import type { Op, OpType } from './op';

const emptySnap: WorkbookSnapshot = { sheets: [{ id: 's1', name: 'Sheet1', cells: [] }] };

// FakeServer mirrors lib/sheetdoc.Manager: it linearizes incoming ops, rebasing
// each against the ops applied since its baseRev, then acks the sender and
// broadcasts the rebased op to the others — with explicit stepping so the test
// can interleave concurrent submissions.
class FakeServer {
  wb = new WorkbookState();
  log: Op[] = [];
  private inbox: { sender: string; op: Op }[] = [];
  private clients = new Map<string, SheetCollabClient>();

  constructor() {
    this.wb.addSheet('s1', 'Sheet1');
  }
  register(id: string, c: SheetCollabClient) {
    this.clients.set(id, c);
  }
  enqueue(sender: string, op: Op) {
    this.inbox.push({ sender, op });
  }
  step(): boolean {
    const item = this.inbox.shift();
    if (!item) return false;
    let rebased: Op = { ...item.op };
    for (let i = item.op.baseRev; i < this.log.length; i++) rebased = transform(rebased, this.log[i]);
    rebased.baseRev = this.log.length;
    // Go Workbook.FillSheetNames.
    const sheet = this.wb.sheetById(rebased.sheet);
    if (['insertRows', 'deleteRows', 'insertCols', 'deleteCols', 'deleteSheet'].includes(rebased.type)) rebased.name = sheet?.name;
    delete rebased.oldName;
    if (rebased.type === 'renameSheet') rebased.oldName = sheet?.name;
    this.wb.applyOp(rebased);
    this.log.push(rebased);
    const rev = this.log.length;
    for (const [id, c] of this.clients) {
      if (id === item.sender) c.onAccept(rev);
      else c.onRemote(rebased, rev);
    }
    return true;
  }
  drain() {
    let guard = 0;
    while (this.step()) {
      if (++guard > 1_000_000) throw new Error('drain did not terminate');
    }
  }
}

function makeClient(server: FakeServer, id: string): SheetCollabClient {
  const c = new SheetCollabClient(emptySnap, 0, { send: (op) => server.enqueue(id, op) });
  server.register(id, c);
  return c;
}

function wbEqual(a: WorkbookState, b: WorkbookState): boolean {
  if (a.sheets.length !== b.sheets.length) return false;
  for (let i = 0; i < a.sheets.length; i++) {
    const sa = a.sheets[i];
    const sb = b.sheets[i];
    if (sa.cells.size !== sb.cells.size) return false;
    for (const [k, cell] of sa.cells) {
      const o = sb.cells.get(k);
      if (!o || o.raw !== cell.raw || (o.styleId ?? 0) !== (cell.styleId ?? 0)) return false;
    }
  }
  return true;
}

describe('SheetCollabClient convergence', () => {
  it('two concurrent setCells on different cells converge', () => {
    const s = new FakeServer();
    const a = makeClient(s, 'A');
    const b = makeClient(s, 'B');
    a.applyLocal({ type: 'setCell', sheet: 's1', baseRev: 0, row: 0, col: 0, raw: 'a' });
    b.applyLocal({ type: 'setCell', sheet: 's1', baseRev: 0, row: 1, col: 1, raw: 'b' });
    s.drain();
    expect(wbEqual(a.confirmedState(), s.wb)).toBe(true);
    expect(wbEqual(b.confirmedState(), s.wb)).toBe(true);
    expect(s.wb.getCell('s1', 0, 0)?.raw).toBe('a');
    expect(s.wb.getCell('s1', 1, 1)?.raw).toBe('b');
  });

  it('concurrent insertRows + setCell rebases and converges', () => {
    const s = new FakeServer();
    const a = makeClient(s, 'A');
    const b = makeClient(s, 'B');
    // A inserts a row at 0; B sets a cell at row 0 — concurrently from rev 0.
    a.applyLocal({ type: 'insertRows', sheet: 's1', baseRev: 0, index: 0, count: 1 });
    b.applyLocal({ type: 'setCell', sheet: 's1', baseRev: 0, row: 0, col: 0, raw: 'x' });
    s.drain();
    expect(wbEqual(a.confirmedState(), s.wb)).toBe(true);
    expect(wbEqual(b.confirmedState(), s.wb)).toBe(true);
  });

  it('contested same-cell write converges to the server order (LWW)', () => {
    const s = new FakeServer();
    const a = makeClient(s, 'A');
    const b = makeClient(s, 'B');
    a.applyLocal({ type: 'setCell', sheet: 's1', baseRev: 0, row: 2, col: 2, raw: 'a' });
    b.applyLocal({ type: 'setCell', sheet: 's1', baseRev: 0, row: 2, col: 2, raw: 'b' });
    s.drain();
    expect(wbEqual(a.confirmedState(), s.wb)).toBe(true);
    expect(wbEqual(b.confirmedState(), s.wb)).toBe(true);
    // server applied A then B (FIFO inbox) -> last writer "b" wins
    expect(s.wb.getCell('s1', 2, 2)?.raw).toBe('b');
  });
});

// Deterministic RNG for reproducible randomized trials (mirrors the Go lcg).
class Lcg {
  private state: bigint;
  constructor(seed: number) {
    this.state = BigInt(seed) * 2654435761n + 1n;
  }
  next(): number {
    this.state = (this.state * 6364136223846793005n + 1442695040888963407n) & 0xffffffffffffffffn;
    return Number((this.state >> 16n) & 0xffffffffn);
  }
  intn(n: number): number {
    return n <= 0 ? 0 : this.next() % n;
  }
}

function randomOp(r: Lcg, rev: number): Op {
  const t: OpType = (['setCell', 'insertRows', 'deleteRows', 'insertCols', 'deleteCols'] as OpType[])[r.intn(5)];
  if (t === 'setCell') {
    return { type: 'setCell', sheet: 's1', baseRev: rev, row: r.intn(6), col: r.intn(6), raw: 'v' };
  }
  return { type: t, sheet: 's1', baseRev: rev, index: r.intn(6), count: 1 + r.intn(2) };
}

describe('SheetCollabClient randomized convergence', () => {
  it('many interleaved trials converge to the server replay', () => {
    for (let trial = 0; trial < 100; trial++) {
      const r = new Lcg(trial + 1);
      const s = new FakeServer();
      const a = makeClient(s, 'A');
      const b = makeClient(s, 'B');
      const clients = [a, b];

      for (let round = 0; round < 20; round++) {
        const c = clients[r.intn(2)];
        c.applyLocal(randomOp(r, c.rev));
        // sometimes process some server steps mid-stream to interleave
        if (r.intn(2) === 0) s.step();
      }
      s.drain();

      expect(wbEqual(a.confirmedState(), s.wb)).toBe(true);
      expect(wbEqual(b.confirmedState(), s.wb)).toBe(true);
    }
  });
});

describe('formula references under concurrency', () => {
  const formulaOf = (c: SheetCollabClient, sheet: string, r: number, col: number) => c.display.getCell(sheet, r, col)?.raw;

  it('a formula naming the sheet, typed during a row insert on that sheet, moves', () => {
    const s = new FakeServer();
    const a = makeClient(s, 'A');
    const b = makeClient(s, 'B');
    a.applyLocal({ type: 'addSheet', sheet: 's2', baseRev: 0, name: 'Sheet2', index: 1 });
    s.drain();
    a.applyLocal({ type: 'insertRows', sheet: 's1', baseRev: a.rev, index: 0, count: 1 });
    b.applyLocal({ type: 'setCell', sheet: 's2', baseRev: b.rev, row: 0, col: 0, raw: '=Sheet1!A1' });
    b.applyLocal({ type: 'renameSheet', sheet: 's1', baseRev: b.rev, name: 'Neu' });
    a.applyLocal({ type: 'setCell', sheet: 's2', baseRev: a.rev, row: 1, col: 0, raw: '=Sheet1!B1' });
    s.drain();
    expect(s.wb.getCell('s2', 0, 0)?.raw).toBe('=Neu!A2');
    expect(s.wb.getCell('s2', 1, 0)?.raw).toBe('=Neu!B1');
    expect(wbEqual(a.confirmedState(), s.wb)).toBe(true);
    expect(wbEqual(b.confirmedState(), s.wb)).toBe(true);
    expect(wbEqual(a.display, s.wb)).toBe(true);
  });

  it('undo of a row delete restores #REF! formulas but keeps a later edit by someone else', async () => {
    const s = new FakeServer();
    const a = makeClient(s, 'A');
    const b = makeClient(s, 'B');
    a.applyLocal({ type: 'setCell', sheet: 's1', baseRev: 0, row: 0, col: 0, raw: '5' });
    a.applyLocal({ type: 'setCell', sheet: 's1', baseRev: 0, row: 2, col: 1, raw: '=A1*2' });
    a.applyLocal({ type: 'setCell', sheet: 's1', baseRev: 0, row: 3, col: 1, raw: '=A1*3' });
    s.drain();
    await Promise.resolve(); // undo groups close on a microtask
    a.applyLocal({ type: 'deleteRows', sheet: 's1', baseRev: a.rev, index: 0, count: 1 });
    s.drain();
    await Promise.resolve();
    expect(formulaOf(b, 's1', 1, 1)).toBe('=#REF!*2');
    // B fixes one of the broken formulas by hand.
    b.applyLocal({ type: 'setCell', sheet: 's1', baseRev: b.rev, row: 2, col: 1, raw: '=7' });
    s.drain();
    a.undo();
    s.drain();
    expect(s.wb.getCell('s1', 0, 0)?.raw).toBe('5');
    expect(s.wb.getCell('s1', 2, 1)?.raw).toBe('=A1*2');
    expect(s.wb.getCell('s1', 3, 1)?.raw).toBe('=7'); // B's edit stays
    expect(wbEqual(a.confirmedState(), s.wb)).toBe(true);
  });
});

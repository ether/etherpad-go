import { describe, it, expect } from 'vitest';
import { SheetPresence, effectiveCells, type PresenceFrame } from './sheetPresence';
import { FormulaEngine } from './formulaEngine';

const frame = (over: Partial<PresenceFrame>): PresenceFrame => ({
  userId: 'a', name: 'A', color: '#f00', sheet: 's1', row: 1, col: 1, editing: false, ...over,
});

describe('SheetPresence reducer', () => {
  it('sets a remote cursor and ignores own frames', () => {
    const p = new SheetPresence('me');
    p.applyPresence(frame({ userId: 'other' }));
    expect(p.cursorsForSheet('s1')).toHaveLength(1);
    p.applyPresence(frame({ userId: 'me' }));
    expect(p.cursorsForSheet('s1')).toHaveLength(1); // self ignored
  });

  it('editing:true adds a live edit, editing:false clears it', () => {
    const p = new SheetPresence('me');
    p.applyPresence(frame({ userId: 'a', editing: true, raw: '=A1*3' }));
    expect(p.liveEditsForSheet('s1')).toHaveLength(1);
    expect(p.liveEditsForSheet('s1')[0].raw).toBe('=A1*3');
    p.applyPresence(frame({ userId: 'a', editing: false }));
    expect(p.liveEditsForSheet('s1')).toHaveLength(0);
    expect(p.cursorsForSheet('s1')).toHaveLength(1); // cursor remains
  });

  it('drop removes cursor + live edit; clearLiveEdit removes only the live edit', () => {
    const p = new SheetPresence('me');
    p.applyPresence(frame({ userId: 'a', editing: true, raw: 'x' }));
    p.clearLiveEdit('a');
    expect(p.liveEditsForSheet('s1')).toHaveLength(0);
    expect(p.cursorsForSheet('s1')).toHaveLength(1);
    p.drop('a');
    expect(p.cursorsForSheet('s1')).toHaveLength(0);
  });

  it('filters by active sheet', () => {
    const p = new SheetPresence('me');
    p.applyPresence(frame({ userId: 'a', sheet: 's1' }));
    p.applyPresence(frame({ userId: 'b', sheet: 's2' }));
    expect(p.cursorsForSheet('s1')).toHaveLength(1);
    expect(p.cursorsForSheet('s2')).toHaveLength(1);
  });
});

describe('effectiveCells overlay drives live recompute', () => {
  it('overlays a remote in-progress raw so dependent cells recompute live', () => {
    // A1=10 (r0c0), C2==B2+1 (r1c2) committed; B2 (r1c1) being typed remotely.
    const base = [
      { row: 0, col: 0, raw: '10' },
      { row: 1, col: 2, raw: '=B2+1' },
    ];
    const live = [{ userId: 'a', name: 'A', color: '#f00', sheet: 's1', row: 1, col: 1, raw: '=A1*3' }];
    const cells = effectiveCells(base, live);

    const engine = new FormulaEngine();
    engine.setGrid(cells);
    expect(engine.getValue(1, 1).value).toBe('30'); // B2 = A1*3
    expect(engine.getValue(1, 2).value).toBe('31'); // C2 = B2+1
    expect(cells.find((c) => c.row === 1 && c.col === 1)?.raw).toBe('=A1*3');
  });
});

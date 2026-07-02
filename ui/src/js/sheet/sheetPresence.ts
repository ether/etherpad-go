// Ephemeral remote presence for the collaborative sheet: who is in which cell
// (cursors) and what they are currently typing (liveEdits). Never persisted.

export interface RemoteCursor {
  userId: string;
  name: string;
  color: string;
  sheet: string;
  row: number;
  col: number;
  focusRow?: number;
  focusCol?: number;
}

export interface RemoteLiveEdit extends RemoteCursor {
  raw: string;
}

// PresenceFrame is the server SHEET_PRESENCE payload (data.* fields).
export interface PresenceFrame {
  userId: string;
  name: string;
  color: string;
  sheet: string;
  row: number;
  col: number;
  editing: boolean;
  raw?: string;
  focusRow?: number;
  focusCol?: number;
}

export class SheetPresence {
  cursors = new Map<string, RemoteCursor>();
  liveEdits = new Map<string, RemoteLiveEdit>();
  onChange: () => void = () => {};
  private ownUserId: string;

  constructor(ownUserId: string) {
    this.ownUserId = ownUserId;
  }

  applyPresence(f: PresenceFrame): void {
    if (!f.userId) return; // not a server-authored frame (e.g. a relayed echo)
    if (f.userId === this.ownUserId) return; // never render our own cursor
    this.cursors.set(f.userId, {
      userId: f.userId, name: f.name, color: f.color, sheet: f.sheet,
      row: f.row, col: f.col, focusRow: f.focusRow, focusCol: f.focusCol,
    });
    if (f.editing) {
      this.liveEdits.set(f.userId, {
        userId: f.userId, name: f.name, color: f.color, sheet: f.sheet, row: f.row, col: f.col, raw: f.raw ?? '',
      });
    } else {
      this.liveEdits.delete(f.userId);
    }
    this.onChange();
  }

  // drop removes a user entirely (reused USER_LEAVE on disconnect).
  drop(userId: string): void {
    const had = this.cursors.delete(userId);
    const hadLive = this.liveEdits.delete(userId);
    if (had || hadLive) this.onChange();
  }

  // clearLiveEdit removes only the live overlay of an author whose op just
  // committed (reused NEW_SHEET_OP.author) — flicker-free formula->result swap.
  clearLiveEdit(userId: string): void {
    if (this.liveEdits.delete(userId)) this.onChange();
  }

  cursorsForSheet(sheetId: string): RemoteCursor[] {
    return [...this.cursors.values()].filter((c) => c.sheet === sheetId);
  }

  liveEditsForSheet(sheetId: string): RemoteLiveEdit[] {
    return [...this.liveEdits.values()].filter((e) => e.sheet === sheetId);
  }
}

// effectiveCells layers remote in-progress raws on top of the committed/optimistic
// cells, so the formula engine recomputes dependents from what others are typing.
export function effectiveCells(
  base: Array<{ row: number; col: number; raw: string }>,
  liveEdits: RemoteLiveEdit[],
): Array<{ row: number; col: number; raw: string }> {
  const byKey = new Map<string, { row: number; col: number; raw: string }>();
  for (const c of base) byKey.set(`${c.row}:${c.col}`, { row: c.row, col: c.col, raw: c.raw });
  for (const e of liveEdits) byKey.set(`${e.row}:${e.col}`, { row: e.row, col: e.col, raw: e.raw });
  return [...byKey.values()];
}

# Sheet Live-Presence & Live-Calculation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Google-Docs-style collaboration to the existing collaborative spreadsheet: per-user active-cell cursor presence and live broadcast of in-progress cell input with live recompute of dependent cells before commit.

**Architecture:** One new ephemeral `SHEET_PRESENCE` frame each direction (never persisted, never routed through the per-doc OT goroutine). The server relays it to the other room sockets and stamps identity from the session author. Cleanup reuses the existing `USER_LEAVE` broadcast (disconnect) and the existing `NEW_SHEET_OP.author` field (commit). The client keeps remote presence in a small reducer, overlays remote in-progress raws onto the formula engine's grid so dependents recompute live, and renders colored cell borders + name tags.

**Tech Stack:** Go (lib/ws), TypeScript (ui/src/js/sheet, vitest), HyperFormula, Playwright.

## Global Constraints

- Presence/live-edit is **ephemeral**: never written to the OT op log, never persisted, never replayed on reconnect.
- Identity (`userId`/`name`/`color`) is **always stamped server-side** from `session.Author` — client-supplied identity is ignored (no spoofing).
- **Read-only** sessions may move a cursor but never broadcast a live edit (`editing` forced false, `raw` dropped) — enforced on both client and server.
- **No new dependencies.** Throttle/debounce are hand-written (`setTimeout`).
- Live-edit throttle ≈ 60 ms (trailing); selection debounce ≈ 50 ms.
- Cleanup reuses existing frames: `USER_LEAVE` (drop cursor + live-edit of that user) and `NEW_SHEET_OP.author` (clear that author's live-edit on commit). No new leave/snapshot/heartbeat frames in v1.
- Wire envelope mirrors the existing `SHEET_OP`: client `socket.emit('message', {type:'COLLABROOM', component:'sheet', data:{…}})`; server `json.Marshal([]any{"message", out})`.

---

## File Structure

- `lib/models/ws/sheetMessages.go` (modify) — add `SheetPresenceIncoming` (client→server) and `SheetPresence`/`SheetPresenceData` (server→client).
- `lib/ws/SheetHandler.go` (modify) — add `HandlePresence` relay method.
- `lib/ws/client.go` (modify) — add the `SHEET_PRESENCE` routing branch.
- `lib/ws/sheet_handler_test.go` (modify) — give the test harness an `authorManager`; add presence relay tests.
- `ui/src/js/sheet/sheetPresence.ts` (create) — `SheetPresence` reducer + `effectiveCells` overlay helper.
- `ui/src/js/sheet/sheetPresence.test.ts` (create) — reducer + overlay-recompute unit tests.
- `ui/src/js/sheet/sheetView.ts` (modify) — selection/input/escape callbacks + remote-cursor / live-edit decorations.
- `ui/src/js/sheet/sheetEditor.ts` (modify) — wire presence client, transport, throttling, overlay recompute, incoming frame handling.
- `playwright/specs/sheet_presence.spec.ts` (create) — two-session E2E.

No change to `PadMessageHandler.go`: `USER_LEAVE` is already broadcast on disconnect.

---

## Task 1: Server presence relay (Go)

**Files:**
- Modify: `lib/models/ws/sheetMessages.go` (append new types)
- Modify: `lib/ws/SheetHandler.go` (append `HandlePresence`)
- Modify: `lib/ws/client.go:176` (add routing branch next to `SHEET_OP`)
- Test: `lib/ws/sheet_handler_test.go` (modify harness + add tests)

**Interfaces:**
- Consumes: `PadMessageHandler{SessionStore, hub, authorManager, Logger}`, `Client{SessionId, Send, SafeSend}`, `(*PadMessageHandler).GetRoomSockets(padID) []Client`, `author.Manager.GetAuthor(id) (*author.Author, error)` where `Author{Name *string, ColorId string}`, `(*SessionStore)` test helpers `InitSessionForTest/SetPadIdForTest/SetAuthorForTest/SetReadOnlyForTest`.
- Produces: `ws.SheetPresenceIncoming`, `ws.SheetPresence`/`ws.SheetPresenceData`, `(*PadMessageHandler).HandlePresence(client *Client, msg ws.SheetPresenceIncoming)`.

- [ ] **Step 1: Add the message types**

Append to `lib/models/ws/sheetMessages.go`:

```go
// SheetPresenceIncoming is the client->server SHEET_PRESENCE frame. Ephemeral:
// never persisted, never ordered through the per-doc goroutine. Wire shape
// mirrors SheetOpIncoming: {"event":"message","data":{"component":"sheet",
// "type":"COLLABROOM","data":{"type":"SHEET_PRESENCE","sheet":..,"row":..,
// "col":..,"editing":bool,"raw":".."}}}.
type SheetPresenceIncoming struct {
	Event string `json:"event"`
	Data  struct {
		Component string `json:"component"` // "sheet"
		Type      string `json:"type"`      // "COLLABROOM"
		Data      struct {
			Type    string `json:"type"` // "SHEET_PRESENCE"
			Sheet   string `json:"sheet"`
			Row     int    `json:"row"`
			Col     int    `json:"col"`
			Editing bool   `json:"editing"`
			Raw     string `json:"raw"`
		} `json:"data"`
	} `json:"data"`
}

// SheetPresence is the server->clients relay of a cursor / live-edit frame.
// Sent as ["message", SheetPresence]. Identity is stamped server-side.
type SheetPresence struct {
	Type string            `json:"type"` // "COLLABROOM"
	Data SheetPresenceData `json:"data"`
}

type SheetPresenceData struct {
	Type    string `json:"type"` // "SHEET_PRESENCE"
	UserId  string `json:"userId"`
	Name    string `json:"name"`
	Color   string `json:"color"`
	Sheet   string `json:"sheet"`
	Row     int    `json:"row"`
	Col     int    `json:"col"`
	Editing bool   `json:"editing"`
	Raw     string `json:"raw,omitempty"`
}
```

- [ ] **Step 2: Give the test harness an authorManager and add the test helper**

In `lib/ws/sheet_handler_test.go`, add the import and set `authorManager` in `newSheetTestHandler`, then add a presence-message builder.

Add to the import block:
```go
	"github.com/ether/etherpad-go/lib/author"
```

Change `newSheetTestHandler` to construct the handler with an author manager backed by the same store:
```go
func newSheetTestHandler(t *testing.T) (*PadMessageHandler, *SessionStore, *Hub) {
	t.Helper()
	store := db2.NewMemoryDataStore()
	ss := NewSessionStore()
	hub := NewHub()
	h := &PadMessageHandler{
		SessionStore:  &ss,
		hub:           hub,
		Logger:        zap.NewNop().Sugar(),
		sheetManager:  sheetdoc.NewManager(store),
		authorManager: author.NewManager(store),
	}
	return h, &ss, hub
}
```

Add a builder helper:
```go
func buildPresenceMsg(sheet string, row, col int, editing bool, raw string) modelws.SheetPresenceIncoming {
	var m modelws.SheetPresenceIncoming
	m.Event = "message"
	m.Data.Component = "sheet"
	m.Data.Type = "COLLABROOM"
	m.Data.Data.Type = "SHEET_PRESENCE"
	m.Data.Data.Sheet = sheet
	m.Data.Data.Row = row
	m.Data.Data.Col = col
	m.Data.Data.Editing = editing
	m.Data.Data.Raw = raw
	return m
}
```

- [ ] **Step 3: Write the failing tests**

Add to `lib/ws/sheet_handler_test.go`:

```go
func decodePresence(t *testing.T, frame []byte) modelws.SheetPresence {
	t.Helper()
	var arr []json.RawMessage
	if err := json.Unmarshal(frame, &arr); err != nil || len(arr) != 2 {
		t.Fatalf("frame not [\"message\", payload]: %v / %s", err, string(frame))
	}
	var sp modelws.SheetPresence
	if err := json.Unmarshal(arr[1], &sp); err != nil {
		t.Fatalf("unmarshal SheetPresence: %v", err)
	}
	return sp
}

func TestHandlePresenceRelaysWithStampedIdentity(t *testing.T) {
	h, ss, hub := newSheetTestHandler(t)
	const sidA, sidB = "sess-a", "sess-b"

	authorA, _ := h.authorManager.CreateAuthor(nil)
	_ = h.authorManager.SetAuthorName(authorA.Id, "Anna")
	_ = h.authorManager.SetAuthorColor(authorA.Id, "#ff0000")
	authorB, _ := h.authorManager.CreateAuthor(nil)

	for sid, aid := range map[string]string{sidA: authorA.Id, sidB: authorB.Id} {
		ss.InitSessionForTest(sid)
		ss.SetPadIdForTest(sid, "p1")
		ss.SetAuthorForTest(sid, aid)
	}
	a := &Client{SessionId: sidA, Send: make(chan []byte, 256), Hub: hub}
	b := &Client{SessionId: sidB, Send: make(chan []byte, 256), Hub: hub}
	hub.Clients[a] = true
	hub.Clients[b] = true

	h.HandlePresence(a, buildPresenceMsg(sheetdoc.DefaultSheetID, 1, 1, true, "=A1*3"))

	// Sender A must NOT receive its own frame.
	select {
	case f := <-a.Send:
		t.Fatalf("sender received own presence frame: %s", string(f))
	default:
	}

	// B receives the relayed frame with server-stamped identity.
	select {
	case f := <-b.Send:
		sp := decodePresence(t, f)
		if sp.Data.Type != "SHEET_PRESENCE" {
			t.Fatalf("type = %q", sp.Data.Type)
		}
		if sp.Data.UserId != authorA.Id || sp.Data.Name != "Anna" || sp.Data.Color != "#ff0000" {
			t.Fatalf("identity not stamped: %+v", sp.Data)
		}
		if !sp.Data.Editing || sp.Data.Raw != "=A1*3" || sp.Data.Row != 1 || sp.Data.Col != 1 {
			t.Fatalf("payload wrong: %+v", sp.Data)
		}
	default:
		t.Fatal("other client did not receive SHEET_PRESENCE")
	}
}

func TestHandlePresenceReadOnlyStripsLiveEdit(t *testing.T) {
	h, ss, hub := newSheetTestHandler(t)
	const sidRO, sidB = "sess-ro", "sess-b"

	authorRO, _ := h.authorManager.CreateAuthor(nil)
	authorB, _ := h.authorManager.CreateAuthor(nil)
	ss.InitSessionForTest(sidRO)
	ss.SetPadIdForTest(sidRO, "p1")
	ss.SetAuthorForTest(sidRO, authorRO.Id)
	ss.SetReadOnlyForTest(sidRO, true)
	ss.InitSessionForTest(sidB)
	ss.SetPadIdForTest(sidB, "p1")
	ss.SetAuthorForTest(sidB, authorB.Id)

	ro := &Client{SessionId: sidRO, Send: make(chan []byte, 256), Hub: hub}
	b := &Client{SessionId: sidB, Send: make(chan []byte, 256), Hub: hub}
	hub.Clients[ro] = true
	hub.Clients[b] = true

	h.HandlePresence(ro, buildPresenceMsg(sheetdoc.DefaultSheetID, 2, 2, true, "=SUM(A1:A9)"))

	select {
	case f := <-b.Send:
		sp := decodePresence(t, f)
		if sp.Data.Editing || sp.Data.Raw != "" {
			t.Fatalf("read-only live edit not stripped: %+v", sp.Data)
		}
		if sp.Data.Row != 2 || sp.Data.Col != 2 {
			t.Fatalf("read-only cursor lost: %+v", sp.Data)
		}
	default:
		t.Fatal("read-only cursor was not relayed")
	}
}
```

- [ ] **Step 4: Run the tests to verify they fail**

Run: `go test ./lib/ws/ -run 'TestHandlePresence' -v`
Expected: compile error / FAIL — `HandlePresence` undefined.

- [ ] **Step 5: Implement `HandlePresence`**

Append to `lib/ws/SheetHandler.go`:

```go
// HandlePresence relays an ephemeral cursor / live-edit frame to the other
// clients of the sheet. It is NOT persisted and NOT ordered through the per-doc
// goroutine. Identity is stamped server-side from the session author (no client
// spoofing); read-only sessions may move a cursor but never broadcast a live
// edit.
func (p *PadMessageHandler) HandlePresence(client *Client, msg ws.SheetPresenceIncoming) {
	session := p.SessionStore.getSession(client.SessionId)
	if session == nil || session.PadId == "" {
		return
	}

	editing := msg.Data.Data.Editing
	raw := msg.Data.Data.Raw
	if session.ReadOnly {
		editing = false
		raw = ""
	}

	var name, color string
	if a, err := p.authorManager.GetAuthor(session.Author); err == nil && a != nil {
		if a.Name != nil {
			name = *a.Name
		}
		color = a.ColorId
	}

	out := ws.SheetPresence{Type: "COLLABROOM"}
	out.Data = ws.SheetPresenceData{
		Type:    "SHEET_PRESENCE",
		UserId:  session.Author,
		Name:    name,
		Color:   color,
		Sheet:   msg.Data.Data.Sheet,
		Row:     msg.Data.Data.Row,
		Col:     msg.Data.Data.Col,
		Editing: editing,
		Raw:     raw,
	}
	encoded, err := json.Marshal([]any{"message", out})
	if err != nil {
		p.Logger.Warn("marshal SHEET_PRESENCE: ", err)
		return
	}
	for _, socket := range p.GetRoomSockets(session.PadId) {
		if socket.SessionId == client.SessionId {
			continue
		}
		socket.SafeSend(encoded)
	}
}
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go test ./lib/ws/ -run 'TestHandlePresence' -v`
Expected: PASS (both tests).

- [ ] **Step 7: Wire the routing branch**

In `lib/ws/client.go`, immediately after the `SHEET_OP` branch (ends at line ~183 with `c.Handler.EnqueueSheetOp(c, sheetOp)`), add:

```go
		} else if strings.Contains(decodedMessage, "SHEET_PRESENCE") {
			var presence ws.SheetPresenceIncoming
			if err := json.Unmarshal(message, &presence); err != nil {
				logger.Error("Error unmarshalling SHEET_PRESENCE: ", err)
				continue
			}
			c.Handler.HandlePresence(c, presence)
```

(`SHEET_PRESENCE` does not contain the substring `SHEET_OP`, so branch order is irrelevant.)

- [ ] **Step 8: Build the whole package and commit**

Run: `go build ./... && go test ./lib/ws/ -run 'TestHandleSheetOp|TestHandlePresence' -v`
Expected: build OK; all listed tests PASS.

```bash
git add lib/models/ws/sheetMessages.go lib/ws/SheetHandler.go lib/ws/client.go lib/ws/sheet_handler_test.go
git commit -m "feat(sheet): server relay for ephemeral SHEET_PRESENCE frames"
```

---

## Task 2: Frontend presence reducer + overlay helper

**Files:**
- Create: `ui/src/js/sheet/sheetPresence.ts`
- Test: `ui/src/js/sheet/sheetPresence.test.ts`

**Interfaces:**
- Consumes: `FormulaEngine` from `./formulaEngine` (`setGrid(cells)`, `getValue(row,col).value`).
- Produces:
  - `interface RemoteCursor { userId; name; color; sheet; row; col }`
  - `interface RemoteLiveEdit extends RemoteCursor { raw }`
  - `interface PresenceFrame { userId; name; color; sheet; row; col; editing; raw? }`
  - `class SheetPresence(ownUserId)` with `cursors`, `liveEdits`, `onChange`, `applyPresence(frame)`, `drop(userId)`, `clearLiveEdit(userId)`, `cursorsForSheet(sheetId)`, `liveEditsForSheet(sheetId)`.
  - `function effectiveCells(base: {row,col,raw}[], liveEdits: RemoteLiveEdit[]): {row,col,raw}[]`.

- [ ] **Step 1: Write the failing tests**

Create `ui/src/js/sheet/sheetPresence.test.ts`:

```ts
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd ui && npx vitest run src/js/sheet/sheetPresence.test.ts`
Expected: FAIL — cannot resolve `./sheetPresence`.

- [ ] **Step 3: Implement the module**

Create `ui/src/js/sheet/sheetPresence.ts`:

```ts
// Ephemeral remote presence for the collaborative sheet: who is in which cell
// (cursors) and what they are currently typing (liveEdits). Never persisted.

export interface RemoteCursor {
  userId: string;
  name: string;
  color: string;
  sheet: string;
  row: number;
  col: number;
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
    if (f.userId === this.ownUserId) return; // never render our own cursor
    this.cursors.set(f.userId, {
      userId: f.userId, name: f.name, color: f.color, sheet: f.sheet, row: f.row, col: f.col,
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd ui && npx vitest run src/js/sheet/sheetPresence.test.ts`
Expected: PASS (all cases).

- [ ] **Step 5: Commit**

```bash
git add ui/src/js/sheet/sheetPresence.ts ui/src/js/sheet/sheetPresence.test.ts
git commit -m "feat(sheet): presence reducer + live-edit overlay helper"
```

---

## Task 3: Grid view — selection/input callbacks + remote decorations

**Files:**
- Modify: `ui/src/js/sheet/sheetView.ts` (full replacement below)

**Interfaces:**
- Consumes: nothing new (DOM only).
- Produces (used by Task 4):
  - `SheetViewOptions` gains optional `onSelect(row,col)`, `onLiveEdit(row,col,raw)`, `onEditEnd(row,col,committed)`.
  - `interface RemoteCursorDeco { userId; name; color; row; col }`, `interface RemoteLiveEditDeco extends RemoteCursorDeco { raw }`.
  - `DomSheetView.setRemoteCursors(list: RemoteCursorDeco[])`, `DomSheetView.setRemoteLiveEdits(list: RemoteLiveEditDeco[])`.

- [ ] **Step 1: Replace `sheetView.ts`**

Replace the entire contents of `ui/src/js/sheet/sheetView.ts` with:

```ts
// DomSheetView is a minimal, framework-agnostic spreadsheet grid rendered as an
// HTML table with contenteditable cells. The SheetView contract lets a canvas
// grid replace it later without touching the collaboration layer.

export interface RemoteCursorDeco {
  userId: string;
  name: string;
  color: string;
  row: number;
  col: number;
}

export interface RemoteLiveEditDeco extends RemoteCursorDeco {
  raw: string;
}

export interface SheetViewOptions {
  rows: number;
  cols: number;
  rawValue: (row: number, col: number) => string;
  displayValue: (row: number, col: number) => string;
  onEdit: (row: number, col: number, raw: string) => void;
  onSelect?: (row: number, col: number) => void;
  onLiveEdit?: (row: number, col: number, raw: string) => void;
  onEditEnd?: (row: number, col: number, committed: boolean) => void;
}

function colName(c: number): string {
  let s = '';
  let n = c + 1;
  while (n > 0) {
    const rem = (n - 1) % 26;
    s = String.fromCharCode(65 + rem) + s;
    n = Math.floor((n - 1) / 26);
  }
  return s;
}

const STYLE_ID = 'sheet-grid-style';
const CSS = `
.sheet-grid { border-collapse: collapse; font: 13px/1.4 system-ui, sans-serif; }
.sheet-grid th, .sheet-grid td { border: 1px solid #d2d2d2; min-width: 80px; height: 22px; padding: 2px 6px; }
.sheet-grid th { background: #f2f3f4; color: #485365; font-weight: 600; text-align: center; }
.sheet-grid td { outline: none; position: relative; }
.sheet-grid td:focus { box-shadow: inset 0 0 0 2px #64d29b; }
.sheet-remote-tag { position: absolute; top: -15px; left: -1px; font: 10px/14px system-ui, sans-serif; padding: 0 4px; color: #fff; border-radius: 3px 3px 3px 0; white-space: nowrap; z-index: 5; pointer-events: none; }
`;

export class DomSheetView {
  private opts: SheetViewOptions;
  private cells: HTMLTableCellElement[][] = [];
  private editing: { row: number; col: number } | null = null;
  private escaped = false;
  private cursorByKey = new Map<string, RemoteCursorDeco>();
  private liveByKey = new Map<string, RemoteLiveEditDeco>();
  private decorated = new Set<HTMLTableCellElement>();

  constructor(root: HTMLElement, opts: SheetViewOptions) {
    this.opts = opts;
    this.ensureStyle();
    root.innerHTML = '';

    const table = document.createElement('table');
    table.className = 'sheet-grid';

    const thead = document.createElement('thead');
    const headRow = document.createElement('tr');
    headRow.appendChild(document.createElement('th')); // corner
    for (let c = 0; c < opts.cols; c++) {
      const th = document.createElement('th');
      th.textContent = colName(c);
      headRow.appendChild(th);
    }
    thead.appendChild(headRow);
    table.appendChild(thead);

    const tbody = document.createElement('tbody');
    for (let r = 0; r < opts.rows; r++) {
      const tr = document.createElement('tr');
      const rowHead = document.createElement('th');
      rowHead.textContent = String(r + 1);
      tr.appendChild(rowHead);
      const rowCells: HTMLTableCellElement[] = [];
      for (let c = 0; c < opts.cols; c++) {
        const td = document.createElement('td');
        td.contentEditable = 'true';
        this.attach(td, r, c);
        tr.appendChild(td);
        rowCells.push(td);
      }
      this.cells.push(rowCells);
      tbody.appendChild(tr);
    }
    table.appendChild(tbody);
    root.appendChild(table);
    this.render();
  }

  private ensureStyle(): void {
    if (document.getElementById(STYLE_ID)) return;
    const style = document.createElement('style');
    style.id = STYLE_ID;
    style.textContent = CSS;
    document.head.appendChild(style);
  }

  private attach(td: HTMLTableCellElement, r: number, c: number): void {
    td.addEventListener('focus', () => {
      this.editing = { row: r, col: c };
      this.escaped = false;
      td.style.boxShadow = '';
      td.querySelector('.sheet-remote-tag')?.remove();
      td.textContent = this.opts.rawValue(r, c);
      this.opts.onSelect?.(r, c);
    });
    td.addEventListener('input', () => {
      this.opts.onLiveEdit?.(r, c, td.textContent ?? '');
    });
    td.addEventListener('keydown', (e: KeyboardEvent) => {
      if (e.key === 'Enter') {
        e.preventDefault();
        td.blur();
      } else if (e.key === 'Escape') {
        e.preventDefault();
        this.escaped = true;
        td.blur();
      }
    });
    td.addEventListener('blur', () => {
      const raw = td.textContent ?? '';
      const prev = this.opts.rawValue(r, c);
      this.editing = null;
      if (this.escaped) {
        this.opts.onEditEnd?.(r, c, false);
      } else {
        const committed = raw !== prev;
        if (committed) this.opts.onEdit(r, c, raw);
        this.opts.onEditEnd?.(r, c, committed);
      }
      this.escaped = false;
      td.textContent = this.opts.displayValue(r, c);
      this.render();
    });
  }

  // setRemoteCursors / setRemoteLiveEdits replace the decoration sets. Call
  // render() afterwards (the editor batches both then renders once).
  setRemoteCursors(list: RemoteCursorDeco[]): void {
    this.cursorByKey = new Map(list.map((d) => [`${d.row}:${d.col}`, d]));
  }

  setRemoteLiveEdits(list: RemoteLiveEditDeco[]): void {
    this.liveByKey = new Map(list.map((d) => [`${d.row}:${d.col}`, d]));
  }

  // render refreshes every non-editing cell to its display value, then paints
  // remote live-edit text and cursor/live-edit decorations.
  render(): void {
    for (const td of this.decorated) {
      td.style.boxShadow = '';
      td.querySelector('.sheet-remote-tag')?.remove();
    }
    this.decorated.clear();

    for (let r = 0; r < this.opts.rows; r++) {
      for (let c = 0; c < this.opts.cols; c++) {
        if (this.editing && this.editing.row === r && this.editing.col === c) continue;
        const td = this.cells[r][c];
        const k = `${r}:${c}`;
        const live = this.liveByKey.get(k);
        td.textContent = live ? live.raw : this.opts.displayValue(r, c);
        const deco: RemoteCursorDeco | undefined = live ?? this.cursorByKey.get(k);
        if (deco) {
          td.style.boxShadow = `inset 0 0 0 2px ${deco.color}`;
          const tag = document.createElement('span');
          tag.className = 'sheet-remote-tag';
          tag.textContent = deco.name || 'anon';
          tag.style.background = deco.color;
          td.appendChild(tag);
          this.decorated.add(td);
        }
      }
    }
  }
}
```

- [ ] **Step 2: Type-check**

Run: `cd ui && npx tsc --noEmit`
Expected: no errors. (Behavior is verified end-to-end in Task 5; this view is DOM glue with no pure logic to unit-test.)

- [ ] **Step 3: Commit**

```bash
git add ui/src/js/sheet/sheetView.ts
git commit -m "feat(sheet): grid view selection/input callbacks + remote decorations"
```

---

## Task 4: Editor wiring — presence client, transport, throttling, overlay

**Files:**
- Modify: `ui/src/js/sheet/sheetEditor.ts` (full replacement below)

**Interfaces:**
- Consumes: `SheetCollabClient`, `FormulaEngine`, `DomSheetView` (+ new callbacks/methods from Task 3), `SheetPresence`/`effectiveCells` (Task 2), the server `SHEET_PRESENCE` relay + existing `USER_LEAVE`/`NEW_SHEET_OP` (Task 1).
- Produces: the fully wired `startSheetEditor(root)`.

- [ ] **Step 1: Replace `sheetEditor.ts`**

Replace the entire contents of `ui/src/js/sheet/sheetEditor.ts` with:

```ts
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
```

- [ ] **Step 2: Type-check**

Run: `cd ui && npx tsc --noEmit`
Expected: no errors.

- [ ] **Step 3: Run the full frontend unit suite (no regressions)**

Run: `cd ui && npm test`
Expected: PASS (existing sheet tests + new `sheetPresence.test.ts`).

- [ ] **Step 4: Commit**

```bash
git add ui/src/js/sheet/sheetEditor.ts
git commit -m "feat(sheet): wire presence client, live-edit throttle and overlay recompute"
```

---

## Task 5: End-to-end two-session test

**Files:**
- Create: `playwright/specs/sheet_presence.spec.ts`

**Interfaces:**
- Consumes: the running server (Playwright `webServer` builds and starts it), `/s/:pad` route, `.sheet-grid` table from `DomSheetView`, `.sheet-remote-tag` decoration.

- [ ] **Step 1: Write the E2E spec**

Create `playwright/specs/sheet_presence.spec.ts`:

```ts
import { test, expect, type Page } from '@playwright/test';

// 0-based cell locator. Row header is th (child 1), so data column c is
// td:nth-child(c + 2); data row r is tbody tr:nth-child(r + 1).
const cell = (page: Page, r: number, c: number) =>
  page.locator(`.sheet-grid tbody tr:nth-child(${r + 1}) td:nth-child(${c + 2})`);

async function openSheet(page: Page, padId: string): Promise<void> {
  await page.goto(`/s/${padId}`);
  await page.locator('.sheet-grid').waitFor({ state: 'visible', timeout: 20000 });
}

async function commitCell(page: Page, r: number, c: number, text: string): Promise<void> {
  await cell(page, r, c).click();
  await page.keyboard.type(text, { delay: 30 });
  await page.keyboard.press('Enter');
}

test.describe('Sheet live presence & live calculation', () => {
  test('cursor presence and live calculation across two sessions', async ({ browser }) => {
    test.setTimeout(120000);
    const padId = `sheet-presence-${Date.now()}`;

    // Session A sets up A1=10 and C2==B2+1.
    const ctxA = await browser.newContext();
    const pageA = await ctxA.newPage();
    await openSheet(pageA, padId);
    await commitCell(pageA, 0, 0, '10');     // A1
    await commitCell(pageA, 1, 2, '=B2+1');  // C2

    // Session B joins and sees the committed value.
    const ctxB = await browser.newContext();
    const pageB = await ctxB.newPage();
    await openSheet(pageB, padId);
    await expect(cell(pageB, 0, 0)).toHaveText('10', { timeout: 20000 });

    // A focuses B2 -> B sees a remote cursor tag on B2.
    await cell(pageA, 1, 1).click();
    await expect(cell(pageB, 1, 1).locator('.sheet-remote-tag')).toBeVisible({ timeout: 20000 });

    // A types a formula in B2 WITHOUT committing -> B sees the live formula text
    // in B2 and C2 recomputed to 31, before Enter.
    await pageA.keyboard.type('=A1*3', { delay: 50 });
    await expect(cell(pageB, 1, 1)).toHaveText('=A1*3', { timeout: 20000 });
    await expect(cell(pageB, 1, 2)).toHaveText('31', { timeout: 20000 });

    // A commits -> B2 shows the computed result 30 (overlay replaced).
    await pageA.keyboard.press('Enter');
    await expect(cell(pageB, 1, 1)).toHaveText('30', { timeout: 20000 });

    // A disconnects -> A's cursor tag disappears on B (reused USER_LEAVE).
    await ctxA.close();
    await expect(cell(pageB, 1, 1).locator('.sheet-remote-tag')).toHaveCount(0, { timeout: 20000 });

    await ctxB.close();
  });
});
```

- [ ] **Step 2: Run the E2E test**

Run: `cd playwright && npx playwright test specs/sheet_presence.spec.ts --project=chromium`
Expected: PASS. (The `webServer` config builds and starts `etherpad-go` on :9001 automatically; first run includes a Go build.)

- [ ] **Step 3: Commit**

```bash
git add playwright/specs/sheet_presence.spec.ts
git commit -m "test(sheet): e2e two-session live presence & live calculation"
```

---

## Self-Review

**Spec coverage:**
- §3 Wire-Protokoll (one `SHEET_PRESENCE` each way, reuse `USER_LEAVE`/`NEW_SHEET_OP.author`) → Task 1 (types/relay/routing) + Task 4 (client emit/handle).
- §4 Server (relay, identity stamp, read-only strip, no server state) → Task 1 (`HandlePresence`, tests).
- §5 Client & Rendering (reducer, overlay recompute, decorations, throttle, callbacks) → Task 2 (reducer + `effectiveCells`), Task 3 (view), Task 4 (wiring).
- §6 Lifecycle (commit flicker-free, cancel/Escape, read-only, self-healing) → Task 3 (`onEditEnd(committed)`, Escape), Task 4 (`clearLiveEdit` on op, `cancelPendingLive`), Task 1 (read-only strip).
- §7 Tests → Task 1 (Go), Task 2 (vitest reducer + overlay), Task 5 (Playwright 4-in-1).

**Placeholder scan:** none — every step has complete code or an exact command.

**Type consistency:** `PresenceFrame` fields (`userId/name/color/sheet/row/col/editing/raw?`) match the server `SheetPresenceData` JSON and the client `applyPresence`. `RemoteLiveEdit` (Task 2) → mapped to `RemoteLiveEditDeco` (Task 3) in `onChange` (Task 4). `effectiveCells` signature identical across Task 2 definition and Task 4 use. `clearLiveEdit`/`drop`/`applyPresence`/`cursorsForSheet`/`liveEditsForSheet` names identical across tasks.

**Deliberate simplifications (ponytail):** no join-snapshot (idle cursors appear on first move), no heartbeat/TTL, full grid re-render per frame — all documented in the design spec §8 with upgrade paths; not implemented here.

# Kollaborative Tabelle — Plan 3: Frontend-Editor + WebSocket-Wire

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:executing-plans / superpowers:subagent-driven-development. Steps use `- [ ]`.

**Goal:** Die nutzersichtbare kollaborative Tabelle: server-seitige WebSocket-Verdrahtung (Sheet-Wire-Protokoll auf Basis des fertigen `sheetdoc.Manager`), ein Frontend-Sheet-Client (client-seitige Op-Logik + Konvergenz + HyperFormula) und ein Grid-View, plus Playwright-E2E als Konvergenz-/Reconnect-/Formel-Nachweis.

**Architecture:** Wir spiegeln exakt das Text-Pad-Muster. Server: neue Wire-Messages (`SHEET_VARS`, `SHEET_OP`, `ACCEPT_SHEET_OP`, `NEW_SHEET_OP`) werden in `lib/ws` dispatcht und an den `sheetdoc.Manager` (Plan 2c) delegiert; Präsenz/Sessions/Hub werden wiederverwendet. Client: `SheetCollabClient` spiegelt `collab_client.ts` (`rev`/`committing`, optimistisches Anwenden, Server-Ack-Reconcile); `WorkbookState` + eine **TS-Portierung von `Apply`/`Transform`** geben client-seitige Konvergenz; `FormulaEngine` kapselt HyperFormula; `SheetView` kapselt ein austauschbares Canvas-Grid.

**Tech Stack:** Go (Fiber WS, `lib/ws`), TypeScript/Vite, **Vitest** (neu), **HyperFormula** (neu, GPLv3 — in Plan 1 akzeptiert), ein MIT-Canvas-Grid (Auswahl per Spike), Playwright (vorhanden).

**Bezug:** Spec §3–§6; baut auf Plan 1/2/2b/2c auf. Der `sheetdoc.Manager` bietet bereits `Submit`/`Snapshot`/`OpsSince` — Plan 3 verdrahtet sie nur.

---

## Decomposition & Empfehlung

Plan 3 ist zu groß für eine Ausführungseinheit und zerfällt in vier unabhängig lieferbare Phasen. **Empfohlene Reihenfolge** (jede für sich lauffähig/testbar):

| Phase | Inhalt | Verifizierbarkeit |
|---|---|---|
| **3a** | Server-Wire (`lib/ws` + models/ws) | Go-Unit-Tests (Manager-Delegation, Frame-(De)Serialisierung) |
| **3b** | Frontend-Op-Logik (`WorkbookState`, TS-`apply`/`transform`, `SheetCollabClient`, `FormulaEngine`) | **Vitest** (inkl. Konvergenz-Property, gespiegelt aus Go) |
| **3c** | View/Bootstrap (Grid-Spike, `SheetView`, `sheet.entry.ts`, Toolbar, Präsenz) | manuell + Build; Grid hinter schmaler Schnittstelle |
| **3d** | Playwright-E2E | zwei Browser konvergieren, Reconnect, Formel-Recompute |

3a und 3b sind die korrektheitskritischen, gut testbaren Teile und sollten zuerst kommen. 3c/3d sind UI-/Browser-lastig und am Ende.

---

# Phase 3a — Server-seitiges Sheet-Wire-Protokoll

**Wire-Design** (Client↔Server, Hülle wie beim Pad: `emit('message', {type:'COLLABROOM', component:'sheet', data:{...}})`):

- **CLIENT_READY** (wiederverwendet): kommt von der `/s/:pad`-Seite. Der Server lädt den Pad via `GetTypedPad(padId,"sheet",author)`; ist `document_type == "sheet"`, antwortet er mit **SHEET_VARS** statt CLIENT_VARS.
- **SHEET_VARS** (Server→Client): `{type:"SHEET_VARS", data:{snapshot, head, userId, userColor, readonly}}` — Initialzustand (Workbook-Snapshot + Head).
- **SHEET_OP** (Client→Server): `{type:"COLLABROOM", component:"sheet", data:{type:"SHEET_OP", op:<sheet.Op JSON>, baseRev:N}}`.
- **ACCEPT_SHEET_OP** (Server→Client, an den Absender): `{type:"COLLABROOM", data:{type:"ACCEPT_SHEET_OP", newRev}}`.
- **NEW_SHEET_OP** (Server→andere): `{type:"COLLABROOM", data:{type:"NEW_SHEET_OP", op:<rebased>, newRev, author}}`.
- Präsenz: bestehende `USER_NEWINFO`/`USER_LEAVE` unverändert wiederverwenden.

**Files:** create `lib/models/ws/sheetMessages.go`, `lib/ws/SheetHandler.go`; modify `lib/ws/client.go` (dispatch), `lib/ws/PadMessageHandler.go` (CLIENT_READY-Branch + handler field + init), `lib/api/static/init.go` (none — same `/socket.io`).

### Task 3a.1: Wire-Message-Structs + (De)Serialisierungstest

- [ ] `lib/models/ws/sheetMessages.go`:
```go
package ws

import "encoding/json"

// SheetOpIncoming is the client->server SHEET_OP payload (inside COLLABROOM).
type SheetOpIncoming struct {
	Event string `json:"event"`
	Data  struct {
		Component string `json:"component"` // "sheet"
		Type      string `json:"type"`      // "COLLABROOM"
		Data      struct {
			Type    string          `json:"type"`    // "SHEET_OP"
			Op      json.RawMessage `json:"op"`       // a sheet.Op
			BaseRev int             `json:"baseRev"`
		} `json:"data"`
	} `json:"data"`
}

// SheetVars is the server->client initial state message.
type SheetVars struct {
	Type string        `json:"type"` // "SHEET_VARS"
	Data SheetVarsData `json:"data"`
}
type SheetVarsData struct {
	Snapshot  json.RawMessage `json:"snapshot"` // sheet.WorkbookSnapshot
	Head      int             `json:"head"`
	UserId    string          `json:"userId"`
	UserColor string          `json:"userColor"`
	ReadOnly  bool            `json:"readonly"`
}

// AcceptSheetOp / NewSheetOp are COLLABROOM payloads (server->client).
type AcceptSheetOp struct {
	Type string `json:"type"` // "COLLABROOM"
	Data struct {
		Type   string `json:"type"` // "ACCEPT_SHEET_OP"
		NewRev int    `json:"newRev"`
	} `json:"data"`
}
type NewSheetOp struct {
	Type string `json:"type"` // "COLLABROOM"
	Data struct {
		Type   string          `json:"type"` // "NEW_SHEET_OP"
		Op     json.RawMessage `json:"op"`
		NewRev int             `json:"newRev"`
		Author string          `json:"author"`
	} `json:"data"`
}
```
- [ ] Test `lib/models/ws/sheetMessages_test.go`: marshal each, unmarshal, assert types/fields round-trip (mirror existing ws message tests if any).
- [ ] `go test ./lib/models/ws/` → pass. Commit.

### Task 3a.2: `SheetMessageHandler` delegating to `sheetdoc.Manager`

Decision: extend the existing `PadMessageHandler` (it already holds `hub`, `SessionStore`, `padManager`) with a `sheetManager *sheetdoc.Manager` and sheet methods, plus a per-document `ChannelOperator` (mirror `PadMessageHandler.go:53-87`) so SHEET_OPs serialize per pad. This reuses sessions/presence/hub with minimal surface.

- [ ] In `PadMessageHandler` struct: add `sheetManager *sheetdoc.Manager` and `sheetChannels SheetChannelOperator`. In its constructor init both (`sheetdoc.NewManager(store)`, `NewSheetChannelOperator(&h)`).
- [ ] `lib/ws/SheetHandler.go`: a `SheetChannelOperator` (copy of `ChannelOperator` with a `SheetTask{socket *Client; msg ws.SheetOpIncoming}`), and:
```go
func (p *PadMessageHandler) handleSheetOp(task SheetTask) {
	session := p.SessionStore.getSession(task.socket.SessionId)
	if session == nil { return }
	// readonly guard (mirror UserChange readonly check in HandleMessage)
	var op sheet.Op
	if err := json.Unmarshal(task.msg.Data.Data.Op, &op); err != nil { p.Logger.Warn("bad sheet op"); return }
	op.BaseRev = task.msg.Data.Data.BaseRev
	author := session.Author
	rebased, newRev, err := p.sheetManager.Submit(session.PadId, op, &author, time.Now().UnixMilli())
	if err != nil { p.Logger.Warn("sheet submit: ", err); return }
	// ACCEPT to sender
	p.sendAcceptSheetOp(task.socket, newRev)
	// broadcast NEW_SHEET_OP to others (mirror UpdatePadClients room iteration + SafeSend)
	p.broadcastNewSheetOp(session.PadId, task.socket.SessionId, rebased, newRev, author)
}
```
plus `sendAcceptSheetOp`, `broadcastNewSheetOp` (build the structs from 3a.1, `json.Marshal` an `["message", msg]` array, `SafeSend` — mirror `UpdatePadClients` at `PadMessageHandler.go:1660-1681`).
- [ ] Go test `lib/ws/sheet_handler_test.go`: construct a handler with a `db.NewMemoryDataStore()` + a fake/minimal session, call `handleSheetOp` with a marshaled op, assert the manager advanced head and the sender socket received an ACCEPT frame. (Use a test double for `*Client.SafeSend` capture — verify the existing Client allows this, else extract a tiny `sender` interface.)
- [ ] pass. Commit.

### Task 3a.3: Dispatch SHEET_OP in `client.go` + SHEET_VARS branch in CLIENT_READY

- [ ] `lib/ws/client.go` readPump (near the `USER_CHANGES` string-match at ~line 166): add
```go
} else if strings.Contains(decodedMessage, "SHEET_OP") {
	var sop ws.SheetOpIncoming
	if err := json.Unmarshal(message, &sop); err != nil { logger.Error("bad SHEET_OP: ", err); continue }
	c.Handler.EnqueueSheetOp(c, sop)
}
```
where `EnqueueSheetOp` calls `p.sheetChannels.AddToQueue(client.Room, SheetTask{...})`.
- [ ] `PadMessageHandler.HandleClientReadyMessage` (`PadMessageHandler.go:1252+`): after loading the pad via `GetTypedPad(thisSession.PadId, "sheet"?, ...)` — actually load with `GetPad` first to read `DocumentType`; if `== "sheet"`, call a new `p.SendSheetVars(client, thisSession, retrievedPad)` and **return** before the text CLIENT_VARS path. `SendSheetVars`: `snap, head, _ := p.sheetManager.Snapshot(pad.Id)`; marshal `ws.SheetVars`; `SafeSend`. Then do the same presence broadcast block the text path does (reuse).
  - Note: `/s/:pad` first HTTP load does NOT create the pad (Plan 1 handler is a stub). So on CLIENT_READY for a sheet, create it: call `p.padManager.GetTypedPad(padId, "sheet", &author)` to materialize the pad row with type sheet, then `sheetManager.Snapshot` (which creates the sheet doc + row on first access).
- [ ] Build `go build ./...`; existing pad flow unaffected (branch only when DocumentType=="sheet"). Manual: not yet (needs client). Commit.

### Task 3a.4: Reconnect support

- [ ] In the CLIENT_READY sheet branch, if `ready.Data.Reconnect` is set with `client_rev`, send the missed ops: `ops,_ := p.sheetManager.OpsSince(padId, *ready.Data.ClientRev)`; for each, send a `NEW_SHEET_OP`. Mirror the text `CLIENT_RECONNECT` loop (`PadMessageHandler.go:1391-1435`).
- [ ] Build + commit.

---

# Phase 3b — Frontend Op-Logik (Vitest-getestet)

**Files:** add Vitest config + `hyperformula` dep; create `ui/src/js/sheet/op.ts`, `workbookState.ts`, `transform.ts`, `sheetCollabClient.ts`, `formulaEngine.ts` + `*.test.ts`.

### Task 3b.1: Vitest-Setup + HyperFormula

- [ ] `cd ui && npm i -D vitest && npm i hyperformula`
- [ ] `ui/vitest.config.ts`:
```ts
import { defineConfig } from 'vitest/config';
export default defineConfig({ test: { environment: 'node', include: ['src/**/*.test.ts'] } });
```
- [ ] `package.json` scripts: add `"test": "vitest run"`.
- [ ] Sanity test `ui/src/js/sheet/smoke.test.ts` (`expect(1+1).toBe(2)`), `npm test` → pass. Commit.

### Task 3b.2: TS Op type + JSON parity with Go

The TS `Op` must serialize to the **same JSON** the Go `sheet.Op` consumes (Plan 2 `op.go` json tags). Mirror field names exactly (`type, sheet, baseRev, row, col, endRow, endCol, raw, value, valueType, styleId, index, count`).
- [ ] `ui/src/js/sheet/op.ts`: `export type OpType = 'setCell'|'setStyle'|'clearRange'|'insertRows'|'deleteRows'|'insertCols'|'deleteCols';` and `export interface Op { type: OpType; sheet: string; baseRev: number; row?: number; col?: number; endRow?: number; endCol?: number; raw?: string; value?: string; valueType?: string; styleId?: number; index?: number; count?: number; }`
- [ ] Test: JSON.stringify a setCell/insertRows op and assert the exact shape (golden string) that matches Go's omitempty output (e.g. `{"type":"setCell","sheet":"s1","baseRev":0,"row":1,"col":2,"raw":"x"}`). Commit.

### Task 3b.3: WorkbookState + TS `applyOp` (port of Go Apply)

- [ ] `ui/src/js/sheet/workbookState.ts`: a `WorkbookState` holding `Map<string /*"r:c"*/, Cell>` per sheet + sheet order + a style pool mirror; `applyOp(op: Op)` ports `lib/sheet/apply.go` exactly (setCell LWW, clearRange, insert/delete rows/cols with the same shift semantics). `loadSnapshot(snap)` builds from the SHEET_VARS snapshot.
- [ ] Test `workbookState.test.ts`: port the Go `apply_test.go` cases (insert shifts cells, delete removes+shifts, clearRange bounds). Commit.

### Task 3b.4: TS `transform` (port of Go Transform)

- [ ] `ui/src/js/sheet/transform.ts`: port `lib/sheet/transform.go` (`transform(inOp, applied)` with the same `shiftCoord` clamp/shift rules).
- [ ] Test `transform.test.ts`: port `transform_test.go` cases. **Critical parity:** these must match Go bit-for-bit, since the client transforms its pending ops against incoming `NEW_SHEET_OP`s while the server transforms on `Submit`. Commit.

### Task 3b.5: SheetCollabClient (mirror collab_client.ts)

- [ ] `ui/src/js/sheet/sheetCollabClient.ts`: state `rev`, `committing`, `pending: Op[]` (local optimistic queue). API:
  - `applyLocal(op)`: assign `op.baseRev = rev`, apply to `WorkbookState` immediately (optimistic), push to `pending`, `flush()`.
  - `flush()`: if `!committing && pending.length`, set `committing=true`, emit `SHEET_OP` for `pending[0]` (the in-flight op), via the socket `emit('message', {type:'COLLABROOM', component:'sheet', data:{type:'SHEET_OP', op, baseRev: op.baseRev}})`.
  - on `ACCEPT_SHEET_OP{newRev}`: `rev = newRev`; shift `pending` (drop the acked op); `committing=false`; rebase remaining `pending` baseRevs to `rev`; `flush()`.
  - on `NEW_SHEET_OP{op, newRev}`: apply remote `op` to `WorkbookState`; `rev = newRev`; **transform every queued/in-flight `pending` op against the remote op** (client-side OT) so local optimistic state stays consistent; re-render.
- [ ] Test `sheetCollabClient.test.ts`: a fake socket; simulate the server echoing ACCEPT for own ops and NEW_SHEET_OP for a second client; assert the local WorkbookState converges to the server-replayed state. **Port the convergence property test** from `lib/sheet/convergence_test.go` against two SheetCollabClients sharing a fake ordering server. Commit.

### Task 3b.6: FormulaEngine (HyperFormula wrapper)

- [ ] `ui/src/js/sheet/formulaEngine.ts`: wrap a HyperFormula instance per sheet; `setCell(ref, raw)` → returns computed `{value, type}` + changed dependent refs; `recomputeAll()`. Keep behind this interface so the engine is swappable (license note: GPLv3).
- [ ] Test `formulaEngine.test.ts`: set A1=2, A2=3, B1="=SUM(A1:A2)" → value 5; change A1=10 → B1 recomputes to 13; dependents reported. Commit.

---

# Phase 3c — View, Bootstrap, Toolbar, Präsenz

### Task 3c.1: Grid-Library-Spike + `SheetView`-Schnittstelle

- [ ] **Spike (timeboxed):** evaluate 2–3 MIT-licensed canvas grids (e.g. a lightweight canvas table/`@glideapps/glide-data-grid`-style or a minimal custom canvas) for: virtualized render of 100k cells, inline edit, selection, column/row resize, frozen header. Record the pick + why in this file.
- [ ] Define `ui/src/js/sheet/sheetView.ts` interface (stable, grid-agnostic):
```ts
export interface SheetView {
  render(state: WorkbookState): void;
  onEdit(cb: (ref: {row:number;col:number}, raw: string) => void): void;
  onSelectionChange(cb: (sel: {row:number;col:number}) => void): void;
  showRemoteCursor(userId: string, color: string, ref: {row:number;col:number}): void;
  destroy(): void;
}
```
- [ ] Implement `CanvasSheetView implements SheetView` against the chosen lib. (Literal code TBD after the spike — bounded by this interface; do not invent the lib API before the spike picks it.)
- [ ] Commit (spike notes + interface, even if the impl lands next task).

### Task 3c.2: `sheet.entry.ts` bootstrap (replace the Plan 1 stub)

- [ ] Replace `ui/src/sheet.entry.ts`: connect socket (`socketio.connect(baseURL,'/',{query:{padId}})`, reuse `pad_connectionstatus`), send `CLIENT_READY` (component just identifies; server keys off document_type), on `SHEET_VARS` build `WorkbookState.loadSnapshot`, init `FormulaEngine`, `SheetView`, and `SheetCollabClient(rev=head)`. Wire `SheetView.onEdit` → `collabClient.applyLocal(setCell op)`; route `NEW_SHEET_OP`/`ACCEPT_SHEET_OP` from the socket into the collab client; `USER_NEWINFO`/`USER_LEAVE` → remote cursors.
- [ ] `npm run build` (mode sheet) succeeds. Manual smoke: load `/s/<id>` → empty grid renders. Commit.

### Task 3c.3: Toolbar + presence

- [ ] Toolbar using existing webcomponents (`<ep-button>`, `<ep-toolbar-select>`, `<ep-color-picker>`, `<ep-dropdown>`): number format, bold/italic/color, alignment, insert/delete row/col, sheet tabs. Each control emits the corresponding `Op` via `collabClient.applyLocal`.
- [ ] Formula bar (cell ref + raw input). Remote cursors colored by author (reuse author colors from `USER_NEWINFO`).
- [ ] Build + manual. Commit.

---

# Phase 3d — Playwright E2E

**Files:** add specs under `playwright/` (mirror existing pad specs).

### Task 3d.1: Two-client convergence
- [ ] Spec: open `/s/<rand>` in two browser contexts; client A sets A1="x", client B sets B2="y" concurrently; assert both see both cells; client A inserts a row at 0; assert B's "y" shifts to B3. Assert both DOM/grid states equal.

### Task 3d.2: Formula recompute across clients
- [ ] A sets A1=2, A2=3, B1="=SUM(A1:A2)"; both clients show 5; A changes A1=10; both show 13.

### Task 3d.3: Reconnect
- [ ] A edits while B's socket is dropped; on B reconnect (CLIENT_READY with client_rev), B receives missed NEW_SHEET_OPs and converges.

- [ ] Wire into the existing Playwright run; commit.

---

## Self-Review (Planner)

- **Coverage vs spec:** §4 formulas (3b.6 HyperFormula), §5 frontend modules + grid-behind-interface (3b/3c), §3 collaboration over the wire (3a + 3b.5), presence/author-colors (3a/3c), reconnect (3a.4/3d.3). xlsx is Plan 4.
- **No-placeholder where verifiable:** 3a (Go, full code/signatures), 3b (TS logic ports with golden-JSON parity to Go + ported tests) are concrete. **Honest exception:** 3c.1 grid rendering literal code is deferred behind the `SheetView` interface pending the spike — inventing a third-party lib's API before selecting it would be guessing, not planning.
- **Parity risk called out:** TS `transform`/`applyOp` MUST match Go bit-for-bit (3b.3/3b.4 port the Go tests; 3b.2 pins JSON shape). This is the top correctness risk and is explicitly tested on both sides.
- **Reuse:** sessions, hub, presence, connection-status, socket wrapper all reused; only sheet-specific frames + handler are new.

## Roadmap next: Plan 4 — xlsx import/export (excelize), per Plan 1 sketch.

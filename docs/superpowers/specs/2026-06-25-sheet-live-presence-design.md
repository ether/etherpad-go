# Design: Live-Präsenz & Live-Kalkulation für die kollaborative Tabelle

**Datum:** 2026-06-25
**Status:** Entwurf zur Umsetzung freigegeben
**Baut auf:** `2026-06-21-collaborative-spreadsheet-design.md` (v1-Fundament: `lib/sheet` OT, `sheetdoc.Manager`, `SheetHandler.go`, Frontend `ui/src/js/sheet/`).
**Stil-Hinweis:** ponytail — schlankste Variante, die funktioniert. Bewusste Vereinfachungen sind in §8 mit Upgrade-Pfad notiert.

---

## 1. Ziel & Kontext

Die kollaborative Tabelle (PR #306) synchronisiert heute Zell-Edits über einen OT-Op-Strom, zeigt aber **keine** Präsenz der anderen Nutzer und überträgt Eingaben erst beim Bestätigen (`setCell`-Op bei Zell-`blur`). Das ursprüngliche Design (`design.md:204`) sah „farbige Zell-Cursor/Selektionen anderer Nutzer" bereits vor — in v1 wurde das zurückgestellt.

Ziel: Die Tabelle soll sich kollaborativ wie Google Docs anfühlen:

1. **Präsenz** — jeder sieht, in welcher Zelle die anderen gerade sind (farbiger Rahmen um die aktive Zelle + Namensschild, Google-Sheets-Stil).
2. **Live-Kalkulation** — während ein Nutzer eine Formel/Zahl tippt, sehen die anderen **live** den entstehenden Formeltext in dessen Zelle, und abhängige Zellen rechnen bei jedem Tastendruck mit — **bevor** Enter gedrückt wird.

Beides ist **ephemerer** Zustand: nicht persistiert, nicht im OT-Op-Log, kein Reconnect-Replay. Der OT-Pfad und das Persistenzmodell bleiben unangetastet.

---

## 2. Scope

**In Scope (v1):**
- Cursor-Präsenz pro Nutzer (aktive Zelle) in Autorfarbe + Namensschild.
- Live-Übertragung der laufenden (unbestätigten) Zell-Eingabe inkl. Live-Neuberechnung abhängiger Zellen bei den Betrachtern.
- Aufräumen bei Disconnect (über das bestehende `USER_LEAVE`) und bei Commit (über das bestehende `NEW_SHEET_OP.author`).

**Bewusst NICHT in v1** (siehe §8 für Upgrade-Pfade):
- Join-Snapshot idle-Cursor (Server-Präsenz-Cache) — Cursor erscheinen, sobald sich der Nutzer bewegt/tippt.
- Heartbeat/TTL gegen halb-offene Verbindungen.
- Mehrere Sheet-Tabs im Frontend (das Protokoll trägt `sheet` bereits, das Rendering filtert korrekt).
- Bereichs-Selektionen (nur Einzelzelle); Remote-Auswahl-Rechtecke.

---

## 3. Wire-Protokoll

Hülle wie beim bestehenden `SHEET_OP`: Client
`socket.emit('message', {type:'COLLABROOM', component:'sheet', data:{…}})`,
Server `["message", {type:'COLLABROOM', data:{…}}]`.

**Ein** neues Message-Type je Richtung; Cleanup nutzt vorhandene Frames.

### Client → Server (nur Position/Eingabe, KEINE Identität)

| `data.type` | Felder | Wann |
|---|---|---|
| `SHEET_PRESENCE` | `sheet, row, col, editing(bool), raw?` | Selektionswechsel (`editing:false`) **und** beim Tippen (`editing:true` + `raw`); throttled |

- `editing:false` ⇒ reiner Cursor (Selektion gewechselt, Eingabe beendet oder abgebrochen).
- `editing:true` + `raw` ⇒ laufende, unbestätigte Eingabe.

### Server → Clients (Server stempelt Identität aus `session.Author`)

| `data.type` | Felder | Zweck |
|---|---|---|
| `SHEET_PRESENCE` | `userId, name, color, sheet, row, col, editing, raw?` | Cursor setzen/bewegen + ggf. Live-Overlay |

### Wiederverwendete vorhandene Frames (kein Neubau)

| Frame | Nutzung hier |
|---|---|
| `USER_LEAVE` (bereits bei Disconnect gebroadcastet, `PadMessageHandler.go:1216`) | Sheet-Client droppt Cursor **und** Live-Edit des Users |
| `NEW_SHEET_OP` mit `author` (bereits vorhanden) | Beim Commit löscht der Reducer das Live-Overlay genau dieses Autors → flackerfreier Wechsel Formeltext→Ergebnis |

**Routing** (`client.go`, neben `SHEET_OP`): ein Branch für `SHEET_PRESENCE`. `strings.Contains`-frei von Kollisionen (`SHEET_PRESENCE` ⊄ `SHEET_OP`).

**Identitäts-Schlüssel:** Cursor/Live-Edit werden über `userId` (Author) identifiziert — wie die bestehenden `USER_NEWINFO`/`USER_LEAVE`. Mehrere Tabs desselben Authors teilen sich einen Cursor (Last-Wins); akzeptierter v1-Kompromiss.

---

## 4. Server

- **Nachrichten-Typen** in `lib/models/ws/sheetMessages.go`: ein `SheetPresenceIncoming` (Client→Server) und ein `SheetPresence` (Server→Client), analog zu den vorhandenen `SheetOpIncoming`/`NewSheetOp`.
- **Ein Relay-Handler** `HandlePresence(client, msg)` in `SheetHandler.go` — **ohne** die per-Pad-Serialisierungs-Goroutine (ephemer, kein Ordering; direkt im Read-Loop):
  1. Session holen; Identität (`userId`/`name`/`color`) aus `session.Author` via `authorManager` stempeln (Client-Werte werden ignoriert → kein Spoofing).
  2. Bei `session.ReadOnly`: `editing=false` setzen und `raw` verwerfen (Read-only sieht/setzt Cursor, sendet aber nie Live-Edits).
  3. `SHEET_PRESENCE` an alle **anderen** Room-Sockets broadcasten (Sender ausgenommen).
- **Kein** Server-State: keine Registry, kein Snapshot, keine neue Disconnect-Logik. `USER_LEAVE` wird bereits gesendet.
- **Routing** in `client.go`: ein zusätzlicher Branch ruft `HandlePresence` auf.

---

## 5. Client & Rendering

### 5.1 Präsenz-Reducer (`ui/src/js/sheet/sheetPresence.ts`)
Minimaler Zustand, zwei Maps:
```
cursors:   Map<userId, {name,color,sheet,row,col}>
liveEdits: Map<userId, {name,color,sheet,row,col,raw}>
```
Reduktion:
- `SHEET_PRESENCE` mit `userId === ownUserId` → ignorieren (kein Self-Cursor).
- sonst `cursors.set(userId, …)`; wenn `editing` → `liveEdits.set(userId, …{raw})`, sonst `liveEdits.delete(userId)`.
- `USER_LEAVE(userId)` → `cursors.delete` **und** `liveEdits.delete`.
- `NEW_SHEET_OP` (Autor `a`, Zelle `r,c`) → `liveEdits.delete(a)` (Commit beendet dessen Live-Overlay flackerfrei).
Jede Änderung ruft `onChange()`.

### 5.2 Live-Neuberechnung mit Overlay (`sheetEditor.ts`)
Die Formel-Engine bekommt das **effektive** Grid: committed/optimistische Raws (`collab.display`) **überlagert** mit allen Remote-Live-Raws des aktiven Sheets.
```
recompute():
  engine.setGrid(effectiveCells())      // committed + remote-live raws
  view.setRemoteCursors(cursorsForActiveSheet())
  view.setRemoteLiveEdits(liveEditsForActiveSheet())
  view.render()
```
Dadurch zeigen abhängige Zellen (`C2 = B2+1`) live das mitgerechnete Ergebnis (`31`), während die bearbeitete Zelle den getippten Formeltext zeigt.

### 5.3 `DomSheetView`-Render, pro Zelle
- lokal in Bearbeitung → überspringen (contenteditable unberührt);
- Remote-Live-Edit auf der Zelle → `textContent = liveEdit.raw` + farbiger Rahmen + Namensschild (Editor-Farbe);
- sonst → `textContent = displayValue(r,c)` (Engine-Ergebnis aus Overlay-Grid); liegt ein Remote-Cursor darauf → farbiger Rahmen + Namensschild.

Dekoration: `box-shadow: inset 0 0 0 2px <farbe>`; ein absolut positioniertes `<span>`-Fähnchen mit Name (Autorfarbe). Nur gerendert, wenn `entry.sheet === activeSheetId`.

### 5.4 Sende-Seite (neue `DomSheetView`-Callbacks)
- `onSelect(r,c)` (Focus) → `SHEET_PRESENCE{editing:false}`, **debounced ~50 ms** (gegen Pfeiltasten-Spam).
- `onLiveEdit(r,c,raw)` (neuer `input`-Listener) → `SHEET_PRESENCE{editing:true,raw}`, **throttled ~60 ms mit Trailing-Edge**.
- `onEditEnd(r,c,committed)` (Blur / Enter / **neu: Escape**):
  - `committed` (Inhalt geändert) → **nur** der bestehende `setCell`-Op; das `NEW_SHEET_OP` räumt das Overlay bei den Empfängern auf (§5.1). Kein `editing:false`-Frame → kein Flackern.
  - `!committed` (Escape / Blur ohne Änderung) → `SHEET_PRESENCE{editing:false}` (Overlay weg, Cursor bleibt).

### 5.5 Transport
Neue `presence`-Sendemethode neben dem bestehenden `transport.send(op)`, gleicher `socket.emit('message', …)`-Pfad. Sheet-Client-Handler lauscht zusätzlich auf `SHEET_PRESENCE` und `USER_LEAVE`.

---

## 6. Lifecycle & Edge-Cases

- **Commit (flackerfrei):** §5.4/§5.1 — Op statt `editing:false`, Empfänger räumen via `NEW_SHEET_OP.author` auf; Zelle wechselt in einem Frame von Formeltext (`=A1*3`) zum Ergebnis (`30`).
- **Abbruch (Escape) / Blur ohne Änderung:** `SHEET_PRESENCE{editing:false}` entfernt das Overlay.
- **Read-only:** Cursor ja, Live-Edit nie — Client unterdrückt, Server strippt (§4.2).
- **Sheet-Wechsel:** Frames tragen `sheet`; gerendert wird nur das aktive Sheet; eigener Cursor wird nach Wechsel neu gesendet.
- **Multi-Tab gleicher Author:** Last-Wins pro Author; bei Disconnect eines Tabs verschwindet der Cursor kurz, der andere Tab setzt ihn bei der nächsten Bewegung neu.
- **Selbstheilung verwaister Overlays:** jeder spätere `SHEET_PRESENCE`-Frame eines Users setzt dessen Live-Edit anhand des `editing`-Flags neu — ein hängendes Overlay verschwindet spätestens bei der nächsten Bewegung; `USER_LEAVE` deckt den Disconnect ab.

---

## 7. Test-Strategie (TDD)

**Go-Unit (`lib/ws`), neben `sheet_handler_test.go`:**
- `HandlePresence`: Frame von A geht an B, **nicht** zurück an A; `userId`/`name`/`color` **serverseitig** gestempelt; Client kann `userId` **nicht spoofen**.
- Read-only-Sender: `editing/raw` werden gestrippt (kein Live-Edit relayed; Cursor schon).

**Frontend-Unit (vitest, `ui/src/js/sheet`):**
- Reducer: `SHEET_PRESENCE` setzt Cursor; `editing:true` setzt Live-Edit, `editing:false` löscht; `USER_LEAVE` löscht beides; `NEW_SHEET_OP(author)` löscht dessen Live-Edit; Self-Frames ignoriert.
- **Kern-Korrektheit (Overlay-Recompute):** committed `A1=10`, remote-live `B2="=A1*3"`, committed `C2="=B2+1"` ⇒ `effectiveCells()` liefert der Engine `C2 = 31`, während `B2` als Formeltext angezeigt wird.

**Playwright-E2E (`/playwright`), zwei Sessions:**
1. A wählt B2 → B sieht A's farbigen Cursor + Namen auf B2.
2. A tippt `=A1*3` in B2 (`A1=10`, `C2=B2+1`) → B sieht **live** den Formeltext in B2 und `C2 = 31` **vor** Enter; nach Enter zeigt B2 `30`.
3. A trennt die Verbindung mitten im Edit → Cursor und Live-Overlay verschwinden bei B (via `USER_LEAVE`).
4. Read-only-Betrachter: sieht fremde Cursor; sein eigener Live-Edit erscheint nie bei anderen.

---

## 8. Bewusste Vereinfachungen (ponytail) & Upgrade-Pfade

| Vereinfachung | Ceiling | Upgrade, wenn … |
|---|---|---|
| Ein `SHEET_PRESENCE`-Frame statt drei (`editing`-Flag + optional `raw`) | — | (kein Upgrade nötig; bewusst kompakt) |
| Cleanup via vorhandenes `USER_LEAVE` statt eigenem `SHEET_PRESENCE_LEAVE` | — | (kein Upgrade nötig) |
| Commit-Cleanup via vorhandenes `NEW_SHEET_OP.author` | — | (kein Upgrade nötig) |
| **Kein** Join-Snapshot (kein Server-Präsenz-Cache) | idle Cursor erscheinen erst bei Bewegung/Tippen | sofortige idle-Cursor beim Join gewünscht → kleiner Präsenz-Cache (`map[pad]map[author]entry` + Mutex) im Handler, Snapshot in `HandleSheetClientReady` |
| **Kein** Heartbeat/TTL | hängendes Overlay bei halb-offener Verbindung bis `USER_LEAVE`/nächste Bewegung | halb-offene Verbindungen real problematisch → ~3 s Heartbeat im Edit-Modus + ~8 s Client-TTL |
| Voll-Render des Grids pro Live-Frame | unkritisch bei 50×20 | großes Grid → gezieltes Re-Render via `engine.setCell().changed[]` + Dekorations-Diff |

---

## 9. Leitprinzipien

- **Ephemer bleibt ephemer:** Präsenz/Live-Edit gehen nie ins OT-Op-Log, werden nie persistiert; der Kollaborations-Kern aus `design.md` bleibt unberührt.
- **Wiederverwenden vor Neubau:** Cleanup über `USER_LEAVE`, Commit-Clear über `NEW_SHEET_OP.author`, Identität über `authorManager` — kein neuer Server-State.
- **Sicherheit bleibt:** Identität immer serverseitig gestempelt; Read-only sendet keine Edits.
- **`raw` ist die Wahrheit, `value` wird lokal abgeleitet** — auch für Live-Edits: Betrachter rechnen den fremden `raw` deterministisch in ihrer eigenen Engine nach.

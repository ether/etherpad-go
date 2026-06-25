package ws

import (
	"encoding/json"
	"strings"
	"testing"

	db2 "github.com/ether/etherpad-go/lib/db"
	modelws "github.com/ether/etherpad-go/lib/models/ws"
	"github.com/ether/etherpad-go/lib/author"
	"github.com/ether/etherpad-go/lib/sheet"
	"github.com/ether/etherpad-go/lib/sheetdoc"
	"go.uber.org/zap"
)

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

func buildSheetOpMsg(t *testing.T, op sheet.Op, baseRev int) modelws.SheetOpIncoming {
	t.Helper()
	opBytes, err := json.Marshal(op)
	if err != nil {
		t.Fatalf("marshal op: %v", err)
	}
	var m modelws.SheetOpIncoming
	m.Event = "message"
	m.Data.Component = "sheet"
	m.Data.Type = "COLLABROOM"
	m.Data.Data.Type = "SHEET_OP"
	m.Data.Data.Op = opBytes
	m.Data.Data.BaseRev = baseRev
	return m
}

func TestHandleSheetOpAdvancesManagerAndAcksSender(t *testing.T) {
	h, ss, hub := newSheetTestHandler(t)
	const sid = "sess-1"
	ss.InitSessionForTest(sid)
	ss.SetPadIdForTest(sid, "p1")
	ss.SetAuthorForTest(sid, "a.1")

	client := &Client{SessionId: sid, Send: make(chan []byte, 256), Hub: hub}
	hub.Clients[client] = true

	raw := "hi"
	msg := buildSheetOpMsg(t, sheet.Op{Type: sheet.OpSetCell, Sheet: sheetdoc.DefaultSheetID, Row: 0, Col: 0, Raw: &raw}, 0)
	h.handleSheetOp(SheetTask{socket: client, message: msg})

	// Manager advanced to head 1 with the cell persisted.
	snap, head, err := h.sheetManager.Snapshot("p1")
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if head != 1 {
		t.Fatalf("expected head 1, got %d", head)
	}
	wb := sheet.WorkbookFromSnapshot(snap)
	if wb.SheetByID(sheetdoc.DefaultSheetID).GetCell(sheet.CellRef{Row: 0, Col: 0}).Raw != "hi" {
		t.Fatal("cell not applied via handler")
	}

	// Sender received an ACCEPT_SHEET_OP frame.
	select {
	case frame := <-client.Send:
		if !strings.Contains(string(frame), "ACCEPT_SHEET_OP") {
			t.Fatalf("expected ACCEPT_SHEET_OP, got %s", string(frame))
		}
	default:
		t.Fatal("sender did not receive an ACCEPT frame")
	}
}

func TestHandleSheetOpBroadcastsToOtherClients(t *testing.T) {
	h, ss, hub := newSheetTestHandler(t)
	const sidA, sidB = "sess-a", "sess-b"
	for _, sid := range []string{sidA, sidB} {
		ss.InitSessionForTest(sid)
		ss.SetPadIdForTest(sid, "p1")
		ss.SetAuthorForTest(sid, "a."+sid)
	}
	a := &Client{SessionId: sidA, Send: make(chan []byte, 256), Hub: hub}
	b := &Client{SessionId: sidB, Send: make(chan []byte, 256), Hub: hub}
	hub.Clients[a] = true
	hub.Clients[b] = true

	raw := "x"
	msg := buildSheetOpMsg(t, sheet.Op{Type: sheet.OpSetCell, Sheet: sheetdoc.DefaultSheetID, Row: 1, Col: 1, Raw: &raw}, 0)
	h.handleSheetOp(SheetTask{socket: a, message: msg})

	// b (the other client) must receive a NEW_SHEET_OP broadcast.
	select {
	case frame := <-b.Send:
		if !strings.Contains(string(frame), "NEW_SHEET_OP") {
			t.Fatalf("expected NEW_SHEET_OP, got %s", string(frame))
		}
	default:
		t.Fatal("other client did not receive NEW_SHEET_OP")
	}
}

func TestHandleSheetOpReadOnlyRejected(t *testing.T) {
	h, ss, hub := newSheetTestHandler(t)
	const sid = "sess-ro"
	ss.InitSessionForTest(sid)
	ss.SetPadIdForTest(sid, "p1")
	ss.SetAuthorForTest(sid, "a.ro")
	ss.SetReadOnlyForTest(sid, true)

	client := &Client{SessionId: sid, Send: make(chan []byte, 256), Hub: hub}
	hub.Clients[client] = true

	raw := "no"
	msg := buildSheetOpMsg(t, sheet.Op{Type: sheet.OpSetCell, Sheet: sheetdoc.DefaultSheetID, Row: 0, Col: 0, Raw: &raw}, 0)
	h.handleSheetOp(SheetTask{socket: client, message: msg})

	_, head, _ := h.sheetManager.Snapshot("p1")
	if head != 0 {
		t.Fatalf("read-only op must not advance head, got %d", head)
	}
}

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

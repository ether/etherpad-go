package ws

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSheetOpIncomingUnmarshal(t *testing.T) {
	raw := `{"event":"message","data":{"component":"sheet","type":"COLLABROOM","data":{"type":"SHEET_OP","op":{"type":"setCell","sheet":"s1","row":1,"col":2,"raw":"x"},"baseRev":3}}}`
	var m SheetOpIncoming
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m.Data.Component != "sheet" || m.Data.Data.Type != "SHEET_OP" || m.Data.Data.BaseRev != 3 {
		t.Fatalf("unexpected: %+v", m)
	}
	var op map[string]any
	if err := json.Unmarshal(m.Data.Data.Op, &op); err != nil {
		t.Fatalf("op unmarshal: %v", err)
	}
	if op["type"] != "setCell" || op["sheet"] != "s1" {
		t.Fatalf("unexpected op: %+v", op)
	}
}

func TestSheetVarsMarshal(t *testing.T) {
	sv := SheetVars{Type: "SHEET_VARS", Data: SheetVarsData{
		Snapshot: json.RawMessage(`{"sheets":[]}`), Head: 7, UserId: "a.1", UserColor: "#ff0000",
	}}
	b, err := json.Marshal([]any{"message", sv})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back []json.RawMessage
	if err := json.Unmarshal(b, &back); err != nil || len(back) != 2 {
		t.Fatalf("envelope: %v len=%d", err, len(back))
	}
	var got SheetVars
	if err := json.Unmarshal(back[1], &got); err != nil {
		t.Fatalf("inner: %v", err)
	}
	if got.Type != "SHEET_VARS" || got.Data.Head != 7 {
		t.Fatalf("unexpected: %+v", got)
	}
}

func TestAcceptAndNewSheetOpMarshal(t *testing.T) {
	a := AcceptSheetOp{Type: "COLLABROOM", Data: AcceptSheetOpData{Type: "ACCEPT_SHEET_OP", NewRev: 4}}
	if b, err := json.Marshal(a); err != nil || len(b) == 0 {
		t.Fatalf("accept marshal: %v", err)
	}
	n := NewSheetOp{Type: "COLLABROOM", Data: NewSheetOpData{Type: "NEW_SHEET_OP", Op: json.RawMessage(`{"type":"setCell"}`), NewRev: 5, Author: "a.1"}}
	b, err := json.Marshal(n)
	if err != nil {
		t.Fatalf("new marshal: %v", err)
	}
	var got NewSheetOp
	if err := json.Unmarshal(b, &got); err != nil || got.Data.NewRev != 5 {
		t.Fatalf("roundtrip: %v %+v", err, got)
	}
}

func TestSheetPresenceFocusZeroNotOmitted(t *testing.T) {
	d := SheetPresenceData{Type: "SHEET_PRESENCE", Row: 0, Col: 0, FocusRow: 0, FocusCol: 0}
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, `"focusRow"`) || !strings.Contains(s, `"focusCol"`) {
		t.Fatalf("focusRow/focusCol must not be omitted at zero: %s", s)
	}
}

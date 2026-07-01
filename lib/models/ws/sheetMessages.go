package ws

import "encoding/json"

// SheetOpIncoming is the client->server SHEET_OP message. Wire shape mirrors
// UserChange: {"event":"message","data":{"component":"sheet","type":"COLLABROOM",
// "data":{"type":"SHEET_OP","op":<sheet.Op>,"baseRev":N}}}.
type SheetOpIncoming struct {
	Event string `json:"event"`
	Data  struct {
		Component string `json:"component"` // "sheet"
		Type      string `json:"type"`      // "COLLABROOM"
		Data      struct {
			Type    string          `json:"type"` // "SHEET_OP"
			Op      json.RawMessage `json:"op"`   // a sheet.Op
			BaseRev int             `json:"baseRev"`
		} `json:"data"`
	} `json:"data"`
}

// SheetVars is the server->client initial state message (the sheet analogue of
// CLIENT_VARS). Sent as ["message", SheetVars].
type SheetVars struct {
	Type string        `json:"type"` // "SHEET_VARS"
	Data SheetVarsData `json:"data"`
}

type SheetVarsData struct {
	Snapshot  json.RawMessage `json:"snapshot"` // a sheet.WorkbookSnapshot
	Head      int             `json:"head"`
	UserId    string          `json:"userId"`
	UserColor string          `json:"userColor"`
	ReadOnly  bool            `json:"readonly"`
}

// AcceptSheetOp acknowledges the sender's own op. Sent as ["message", AcceptSheetOp].
type AcceptSheetOp struct {
	Type string            `json:"type"` // "COLLABROOM"
	Data AcceptSheetOpData `json:"data"`
}

type AcceptSheetOpData struct {
	Type   string `json:"type"` // "ACCEPT_SHEET_OP"
	NewRev int    `json:"newRev"`
}

// NewSheetOp broadcasts a rebased op to the other clients of a sheet.
// Sent as ["message", NewSheetOp].
type NewSheetOp struct {
	Type string         `json:"type"` // "COLLABROOM"
	Data NewSheetOpData `json:"data"`
}

type NewSheetOpData struct {
	Type   string          `json:"type"` // "NEW_SHEET_OP"
	Op     json.RawMessage `json:"op"`
	NewRev int             `json:"newRev"`
	Author string          `json:"author"`
}

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
			Type     string `json:"type"` // "SHEET_PRESENCE"
			Sheet    string `json:"sheet"`
			Row      int    `json:"row"`
			Col      int    `json:"col"`
			Editing  bool   `json:"editing"`
			Raw      string `json:"raw"`
			FocusRow int    `json:"focusRow,omitempty"`
			FocusCol int    `json:"focusCol,omitempty"`
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
	Type     string `json:"type"` // "SHEET_PRESENCE"
	UserId   string `json:"userId"`
	Name     string `json:"name"`
	Color    string `json:"color"`
	Sheet    string `json:"sheet"`
	Row      int    `json:"row"`
	Col      int    `json:"col"`
	Editing  bool   `json:"editing"`
	Raw      string `json:"raw,omitempty"`
	FocusRow int    `json:"focusRow,omitempty"`
	FocusCol int    `json:"focusCol,omitempty"`
}

package ws

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/ether/etherpad-go/lib/models/ws"
	"github.com/ether/etherpad-go/lib/pad"
	"github.com/ether/etherpad-go/lib/sheet"
	"github.com/ether/etherpad-go/lib/sheetdoc"
	"github.com/ether/etherpad-go/lib/test/testutils"
	libws "github.com/ether/etherpad-go/lib/ws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Sheets on two instances: ops, acknowledgements, presence and a workbook
// import reach the users on the other instance, and every client gets the
// ops in revision order.
func TestClusterSheets(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(testutils.TestRunConfig{Name: "sheet on two instances", Test: testClusterSheets})
	testDb.StartTestDBHandler()
}

// sheetFrame is a message a sheet client received.
type sheetFrame struct {
	Type string // SHEET_VARS, or the COLLABROOM data type
	Rev  int
}

func sheetFrames(client *libws.Client) []sheetFrame {
	var out []sheetFrame
	for _, raw := range drainClientMessages(client) {
		var arr []json.RawMessage
		if json.Unmarshal([]byte(raw), &arr) != nil || len(arr) != 2 {
			continue
		}
		var msg struct {
			Type string `json:"type"`
			Data struct {
				Type   string `json:"type"`
				NewRev int    `json:"newRev"`
			} `json:"data"`
		}
		if json.Unmarshal(arr[1], &msg) != nil {
			continue
		}
		if msg.Type == "COLLABROOM" {
			out = append(out, sheetFrame{Type: msg.Data.Type, Rev: msg.Data.NewRev})
		} else {
			out = append(out, sheetFrame{Type: msg.Type})
		}
	}
	return out
}

func sheetOpMsg(t *testing.T, row int, raw string, baseRev int) ws.SheetOpIncoming {
	t.Helper()
	opBytes, err := json.Marshal(sheet.Op{Type: sheet.OpSetCell, Sheet: sheetdoc.DefaultSheetID, Row: row, Col: 0, Raw: &raw})
	require.NoError(t, err)
	var m ws.SheetOpIncoming
	m.Event = "message"
	m.Data.Component = "sheet"
	m.Data.Type = "COLLABROOM"
	m.Data.Data.Type = "SHEET_OP"
	m.Data.Data.Op = opBytes
	m.Data.Data.BaseRev = baseRev
	return m
}

func testClusterSheets(t *testing.T, ds testutils.TestDataStore) {
	busA, busB := newBusPair()
	hubB := libws.NewHub()
	go hubB.Run()
	sessB := libws.NewSessionStore()
	managerB := pad.NewManager(ds.DS, ds.Hooks)
	handlerA := ds.PadMessageHandler
	handlerB := libws.NewPadMessageHandler(ds.DS, ds.Hooks, managerB, &sessB, hubB, ds.Logger, testutils.GetTestAssets(), ds.Settings)
	handlerA.SetCluster(busA)
	handlerB.SetCluster(busB)
	t.Cleanup(func() { handlerA.StopCluster(); handlerB.StopCluster() })

	const padID = "clusterSheet"
	join := func(h *libws.PadMessageHandler, hub *libws.Hub, sessionID string) *libws.Client {
		client := createTestClient(hub, sessionID, padID, libws.NewActualMockWebSocketconn())
		client.Send = make(chan []byte, 1024)
		t.Cleanup(func() { removeClient(hub, client) })
		h.SessionStore.InitSessionForTest(sessionID)
		h.HandleMessage(ws.ClientReady{
			Event: "message",
			Data:  ws.ClientReadyData{Component: "sheet", Type: "CLIENT_READY", PadID: padID, Token: "t.sheet" + sessionID},
		}, client, ds.Settings, ds.Logger)
		frames := sheetFrames(client)
		require.NotEmpty(t, frames, "joining client gets its state")
		require.Equal(t, "SHEET_VARS", frames[0].Type)
		return client
	}
	onA := join(handlerA, ds.Hub, "sheetA")
	onB := join(handlerB, hubB, "sheetB")
	sheetFrames(onA) // B's join announcement

	// collect gathers client's sheet op frames until it has n of them.
	collect := func(client *libws.Client, got *[]sheetFrame, n int) {
		t.Helper()
		require.Eventually(t, func() bool {
			for _, f := range sheetFrames(client) {
				if f.Type == "NEW_SHEET_OP" || f.Type == "ACCEPT_SHEET_OP" || f.Type == "SHEET_RELOAD" {
					*got = append(*got, f)
				}
			}
			return len(*got) >= n
		}, 10*time.Second, 20*time.Millisecond, "have %v", *got)
	}

	// An op through A: A's user gets the acknowledgement, B's user the op.
	handlerA.EnqueueSheetOp(onA, sheetOpMsg(t, 0, "from A", 0))
	var atA, atB []sheetFrame
	collect(onA, &atA, 1)
	collect(onB, &atB, 1)
	assert.Equal(t, []sheetFrame{{"ACCEPT_SHEET_OP", 1}}, atA)
	assert.Equal(t, []sheetFrame{{"NEW_SHEET_OP", 1}}, atB)

	// Two more through A, then at once one through B that is based on an
	// older revision. B's user must get A's ops before the acknowledgement
	// of its own, each exactly once and in order.
	handlerA.EnqueueSheetOp(onA, sheetOpMsg(t, 1, "A2", 1))
	handlerA.EnqueueSheetOp(onA, sheetOpMsg(t, 2, "A3", 2))
	handlerB.EnqueueSheetOp(onB, sheetOpMsg(t, 3, "B4", 1))
	collect(onB, &atB, 4)
	collect(onA, &atA, 4)
	time.Sleep(200 * time.Millisecond) // anything sent twice would arrive now
	collect(onB, &atB, 4)
	collect(onA, &atA, 4)
	assert.Equal(t, []sheetFrame{{"NEW_SHEET_OP", 1}, {"NEW_SHEET_OP", 2}, {"NEW_SHEET_OP", 3}, {"ACCEPT_SHEET_OP", 4}}, atB)
	assert.Equal(t, []sheetFrame{{"ACCEPT_SHEET_OP", 1}, {"ACCEPT_SHEET_OP", 2}, {"ACCEPT_SHEET_OP", 3}, {"NEW_SHEET_OP", 4}}, atA)

	// Both instances hold the same workbook.
	for _, h := range []*libws.PadMessageHandler{handlerA, handlerB} {
		snap, head, err := h.SheetManager().Snapshot(padID)
		require.NoError(t, err)
		require.Equal(t, 4, head)
		s := sheet.WorkbookFromSnapshot(snap).SheetByID(sheetdoc.DefaultSheetID)
		assert.Equal(t, "from A", s.GetCell(sheet.CellRef{Row: 0, Col: 0}).Raw)
		assert.Equal(t, "B4", s.GetCell(sheet.CellRef{Row: 3, Col: 0}).Raw)
	}

	// Presence (cursor) of A's user reaches B's user.
	var presence ws.SheetPresenceIncoming
	presence.Data.Data.Sheet = sheetdoc.DefaultSheetID
	presence.Data.Data.Row = 5
	handlerA.HandlePresence(onA, presence)
	require.Eventually(t, func() bool {
		for _, f := range sheetFrames(onB) {
			if f.Type == "SHEET_PRESENCE" {
				return true
			}
		}
		return false
	}, 5*time.Second, 20*time.Millisecond, "presence must cross instances")

	// An import through A replaces the workbook: users on both instances
	// are told to reload.
	func() {
		unlock, err := ds.PadManager.LockPad(padID)
		require.NoError(t, err)
		defer unlock()
		wb := sheet.NewWorkbook()
		wb.AddSheet(sheetdoc.DefaultSheetID, "Imported")
		require.NoError(t, handlerA.SheetManager().SetWorkbook(padID, wb))
		handlerA.BroadcastSheetReload(padID)
	}()
	atA, atB = nil, nil
	collect(onA, &atA, 1)
	collect(onB, &atB, 1)
	assert.Equal(t, "SHEET_RELOAD", atA[0].Type)
	assert.Equal(t, "SHEET_RELOAD", atB[0].Type)
}

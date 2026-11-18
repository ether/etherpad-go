package ws

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/ether/etherpad-go/lib/models/ws"
	"github.com/ether/etherpad-go/lib/sheet"
	"github.com/ether/etherpad-go/lib/sheetdoc"
	"github.com/ether/etherpad-go/lib/test/testutils"
	libws "github.com/ether/etherpad-go/lib/ws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A sheet client that reconnects gets the ops it missed (its own lost op
// among them, by cid) and SHEET_CAUGHT_UP; an op it sends again is not
// applied twice.
func TestSheetReconnect(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(testutils.TestRunConfig{Name: "sheet reconnect", Test: testSheetReconnect})
	testDb.AddTests(testutils.TestRunConfig{Name: "sheet old client reloads", Test: testSheetOldClientReloads})
	testDb.StartTestDBHandler()
}

type reconnectFrame struct {
	Type string
	Rev  int
	Head int
	Cid  string
}

func reconnectFrames(client *libws.Client) []reconnectFrame {
	var out []reconnectFrame
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
				Head   int    `json:"head"`
				Cid    string `json:"cid"`
				Op     struct {
					Cid string `json:"cid"`
				} `json:"op"`
			} `json:"data"`
		}
		if json.Unmarshal(arr[1], &msg) != nil {
			continue
		}
		switch {
		case msg.Type != "COLLABROOM":
			out = append(out, reconnectFrame{Type: msg.Type})
		case msg.Data.Type == "NEW_SHEET_OP":
			out = append(out, reconnectFrame{Type: msg.Data.Type, Rev: msg.Data.NewRev, Cid: msg.Data.Op.Cid})
		case msg.Data.Type == "SHEET_RELOAD":
			out = append(out, reconnectFrame{Type: msg.Data.Type})
		case msg.Data.Type == "ACCEPT_SHEET_OP" || msg.Data.Type == "SHEET_CAUGHT_UP":
			out = append(out, reconnectFrame{Type: msg.Data.Type, Rev: msg.Data.NewRev, Head: msg.Data.Head, Cid: msg.Data.Cid})
		}
	}
	return out
}

func insertRowMsg(t *testing.T, cid string, baseRev int) ws.SheetOpIncoming {
	t.Helper()
	opBytes, err := json.Marshal(sheet.Op{Type: sheet.OpInsertRows, Sheet: sheetdoc.DefaultSheetID, Index: 0, Count: 1, Cid: cid})
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

func testSheetReconnect(t *testing.T, ds testutils.TestDataStore) {
	h := ds.PadMessageHandler
	const padID = "reconnectSheet"
	join := func(sessionID string, clientRev *int) (*libws.Client, []reconnectFrame) {
		client := createTestClient(ds.Hub, sessionID, padID, libws.NewActualMockWebSocketconn())
		client.Send = make(chan []byte, 1024)
		t.Cleanup(func() { removeClient(ds.Hub, client) })
		h.SessionStore.InitSessionForTest(sessionID)
		data := ws.ClientReadyData{Component: "sheet", Type: "CLIENT_READY", PadID: padID, Token: "t.reconnect"}
		if clientRev != nil {
			yes := true
			data.Reconnect = &yes
			data.ClientRev = clientRev
		}
		h.HandleMessage(ws.ClientReady{Event: "message", Data: data}, client, ds.Settings, ds.Logger)
		return client, reconnectFrames(client)
	}
	// await collects the client's op frames until there are n.
	await := func(client *libws.Client, n int) []reconnectFrame {
		t.Helper()
		var got []reconnectFrame
		require.Eventually(t, func() bool {
			for _, f := range reconnectFrames(client) {
				if f.Type != "SHEET_PRESENCE" && f.Type != "USER_NEWINFO" {
					got = append(got, f)
				}
			}
			return len(got) >= n
		}, 5*time.Second, 10*time.Millisecond, "have %v", got)
		return got
	}

	first, frames := join("rc1", nil)
	require.NotEmpty(t, frames)
	require.Equal(t, "SHEET_VARS", frames[0].Type)

	// The op is stored, but the connection drops before the acknowledgement
	// is read.
	h.EnqueueSheetOp(first, insertRowMsg(t, "client:1", 0))
	assert.Equal(t, []reconnectFrame{{Type: "ACCEPT_SHEET_OP", Rev: 1, Cid: "client:1"}}, await(first, 1))
	removeClient(ds.Hub, first)

	// The new connection gets the op it missed, with its cid, then the end.
	rev0 := 0
	second, frames := join("rc2", &rev0)
	assert.Equal(t, []reconnectFrame{
		{Type: "NEW_SHEET_OP", Rev: 1, Cid: "client:1"},
		{Type: "SHEET_CAUGHT_UP", Head: 1},
	}, frames)

	// Sent again anyway: acknowledged, not applied a second time.
	h.EnqueueSheetOp(second, insertRowMsg(t, "client:1", 0))
	assert.Equal(t, []reconnectFrame{{Type: "ACCEPT_SHEET_OP", Rev: 1, Cid: "client:1"}}, await(second, 1))
	_, head, err := h.SheetManager().State(padID)
	require.NoError(t, err)
	assert.Equal(t, 1, head)

	// A client at the head gets just the end of the (empty) replay.
	rev1 := 1
	_, frames = join("rc3", &rev1)
	assert.Equal(t, []reconnectFrame{{Type: "SHEET_CAUGHT_UP", Head: 1}}, frames)

	// A client ahead of the server (the sheet was restored) starts over.
	rev9 := 9
	_, frames = join("rc4", &rev9)
	require.NotEmpty(t, frames)
	assert.Equal(t, "SHEET_VARS", frames[0].Type)
}

// A browser loaded before charts does not know the ops; it gets a reload
// (and with it the new code) instead of an op it would fail on.
func testSheetOldClientReloads(t *testing.T, ds testutils.TestDataStore) {
	h := ds.PadMessageHandler
	const padID = "oldClientSheet"
	join := func(sessionID string, version int) *libws.Client {
		client := createTestClient(ds.Hub, sessionID, padID, libws.NewActualMockWebSocketconn())
		client.Send = make(chan []byte, 1024)
		t.Cleanup(func() { removeClient(ds.Hub, client) })
		h.SessionStore.InitSessionForTest(sessionID)
		data := ws.ClientReadyData{Component: "sheet", Type: "CLIENT_READY", PadID: padID, Token: "t." + sessionID, SheetOps: version}
		h.HandleMessage(ws.ClientReady{Event: "message", Data: data}, client, ds.Settings, ds.Logger)
		reconnectFrames(client) // SHEET_VARS
		return client
	}
	author, current, old := join("new1", 2), join("new2", 2), join("old1", 0)
	frames := func(c *libws.Client) []reconnectFrame {
		var got []reconnectFrame
		for _, f := range reconnectFrames(c) {
			if f.Type == "NEW_SHEET_OP" || f.Type == "SHEET_RELOAD" || f.Type == "ACCEPT_SHEET_OP" {
				got = append(got, f)
			}
		}
		return got
	}
	send := func(op sheet.Op) {
		b, err := json.Marshal(op)
		require.NoError(t, err)
		var m ws.SheetOpIncoming
		m.Event, m.Data.Component, m.Data.Type = "message", "sheet", "COLLABROOM"
		m.Data.Data.Type, m.Data.Data.Op = "SHEET_OP", b
		h.EnqueueSheetOp(author, m)
	}
	// An op every version knows reaches the old client too.
	send(sheet.Op{Type: sheet.OpInsertRows, Sheet: sheetdoc.DefaultSheetID, Count: 1, Cid: "c:1"})
	require.Eventually(t, func() bool { return len(frames(author)) == 1 }, 5*time.Second, 10*time.Millisecond)
	assert.Equal(t, []reconnectFrame{{Type: "NEW_SHEET_OP", Rev: 1, Cid: "c:1"}}, frames(old))
	frames(current)

	send(sheet.Op{Type: sheet.OpSetChart, Sheet: sheetdoc.DefaultSheetID, BaseRev: 1, Cid: "c:2",
		Chart: &sheet.Chart{Id: "ch1", Type: "pie", Range: "A1:B3", Width: 300, Height: 200}})
	require.Eventually(t, func() bool { return len(frames(author)) == 1 }, 5*time.Second, 10*time.Millisecond)
	assert.Equal(t, []reconnectFrame{{Type: "NEW_SHEET_OP", Rev: 2, Cid: "c:2"}}, frames(current))
	assert.Equal(t, []reconnectFrame{{Type: "SHEET_RELOAD"}}, frames(old))
}

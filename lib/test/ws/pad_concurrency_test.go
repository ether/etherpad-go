package ws

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"testing"

	apipad "github.com/ether/etherpad-go/lib/api/pad"
	"github.com/ether/etherpad-go/lib/models/ws"
	"github.com/ether/etherpad-go/lib/test/testutils"
	libws "github.com/ether/etherpad-go/lib/ws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPadConcurrentWriters(t *testing.T) {
	// Datastores run one after another: ToInitStore writes the global
	// settings, which a parallel run's goroutines would be reading, and that
	// test-only race would drown out real reports under -race.
	testutils.RunSequentialMemorySQLite(t, testutils.TestRunConfig{
		Name: "USER_CHANGES and HTTP API writes to one pad are serialised",
		Test: testConcurrentUserChangesAndAPIWrites,
	})
}

// testConcurrentUserChangesAndAPIWrites fires websocket commits and HTTP API
// appendText calls at the same pad in parallel. Before the shared per-pad
// lock, API writes bypassed the USER_CHANGES queue: both sides read the same
// head and appended "their" next revision, corrupting the history (and the
// unsynchronised pool maps could crash the process). Run with -race.
func testConcurrentUserChangesAndAPIWrites(t *testing.T, ds testutils.TestDataStore) {
	const n = 40
	padId := "test-pad-concurrent-writers"
	token := "t.concurrentwriters"

	tokenAuthor, err := ds.AuthorManager.GetAuthor4Token(token)
	require.NoError(t, err)
	authorId := tokenAuthor.Id

	startPad, err := ds.PadManager.GetPad(padId, nil, &authorId)
	require.NoError(t, err)
	headBefore := startPad.Head
	textBefore := startPad.Text()

	mockConn := libws.NewActualMockWebSocketconn()
	sessionId := "test-session-concurrent-writers"
	client := createTestClient(ds.Hub, sessionId, padId, mockConn)
	client.Send = make(chan []byte, 4096)
	defer func() { delete(ds.Hub.Clients, client) }()

	ds.PadMessageHandler.SessionStore.InitSessionForTest(sessionId)
	ds.PadMessageHandler.SessionStore.AddHandleClientInformationForTest(sessionId, padId, token)
	ds.PadMessageHandler.SessionStore.SetAuthorForTest(sessionId, authorId)
	ds.PadMessageHandler.SessionStore.SetPadIdForTest(sessionId, padId)
	ds.PadMessageHandler.SessionStore.AddPadReadOnlyIdsForTest(sessionId, padId, "readonly-concurrent", false)

	initStore := ds.ToInitStore()
	apipad.Init(initStore)

	var wg sync.WaitGroup
	errs := make(chan error, 2*n)
	wg.Add(2)
	// Websocket side behaves like a real client: each commit is based on the
	// revision the server accepted for the previous one, while the API keeps
	// appending underneath, so the server has to rebase every commit.
	go func() {
		defer wg.Done()
		base, baseLen := headBefore, len(textBefore)
		for i := 0; i < n; i++ {
			userChange := ws.UserChange{
				Event: "message",
				Data: ws.UserChangeData{
					Component: "pad",
					Type:      "USER_CHANGES",
					Data: ws.UserChangeDataData{
						// Inserted text must carry the committing author.
						Apool: ws.UserChangeDataDataApool{
							NumToAttrib: map[int][]string{0: {"author", authorId}},
							NextNum:     1,
						},
						BaseRev:   base,
						Changeset: "Z:" + toBase36(baseLen) + ">1*0+1$x",
					},
				},
			}
			ds.PadMessageHandler.HandleMessage(userChange, client, initStore.RetrievedSettings, ds.Logger)
			newRev := acceptedRev(drainClientMessages(client))
			if newRev < 0 {
				errs <- fmt.Errorf("commit %d was not accepted", i)
				return
			}
			// Only key revisions store the full text; rebuild it.
			unlock := ds.PadManager.RLockPad(padId)
			p, err := ds.PadManager.GetPad(padId, nil, nil)
			if err != nil {
				unlock()
				errs <- err
				return
			}
			revText := p.GetInternalRevisionAText(newRev)
			unlock()
			if revText == nil {
				errs <- fmt.Errorf("revision %d not found", newRev)
				return
			}
			base, baseLen = newRev, len(revText.Text)
		}
	}()
	go func() {
		defer wg.Done()
		body, _ := json.Marshal(apipad.AppendTextRequest{Text: "y", AuthorId: authorId})
		for i := 0; i < n; i++ {
			req := httptest.NewRequest("POST", "/admin/api/pads/"+padId+"/appendText", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			resp, err := initStore.C.Test(req)
			if err != nil {
				errs <- err
				continue
			}
			if resp.StatusCode != 200 {
				errs <- assert.AnError
			}
		}
	}()
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}

	live, err := ds.PadManager.GetPad(padId, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, headBefore+2*n, live.Head, "every write must get its own revision")
	assert.Equal(t, n, strings.Count(live.Text(), "x"))
	assert.Equal(t, n, strings.Count(live.Text(), "y"))

	// History must be contiguous and replaying it must give the live text.
	for rev := 0; rev <= live.Head; rev++ {
		_, err := live.GetRevision(rev)
		require.NoError(t, err, "revision %d missing", rev)
	}
	liveText := live.Text()
	ds.PadManager.UnloadPad(padId)
	reloaded, err := ds.PadManager.GetPad(padId, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, live.Head, reloaded.Head)
	assert.Equal(t, liveText, reloaded.Text(), "stored head revision must match the live pad")
}

// TestUserChangesLeaveNoGoroutines: USER_CHANGES used to start a goroutine
// per pad that ran forever, so every pad ever edited leaked one.
func TestUserChangesLeaveNoGoroutines(t *testing.T) {
	testutils.RunSequentialMemorySQLite(t, testutils.TestRunConfig{
		Name: "editing many pads does not grow the goroutine count",
		Test: testUserChangesLeaveNoGoroutines,
	})
}

func testUserChangesLeaveNoGoroutines(t *testing.T, ds testutils.TestDataStore) {
	const pads = 100
	token := "t.goroutineleak"
	tokenAuthor, err := ds.AuthorManager.GetAuthor4Token(token)
	require.NoError(t, err)
	authorId := tokenAuthor.Id
	initStore := ds.ToInitStore()

	edit := func(i int) {
		padId := fmt.Sprintf("leak-pad-%d", i)
		p, err := ds.PadManager.GetPad(padId, nil, &authorId)
		require.NoError(t, err)
		sessionId := "leak-session-" + padId
		client := createTestClient(ds.Hub, sessionId, padId, libws.NewActualMockWebSocketconn())
		defer delete(ds.Hub.Clients, client)
		ss := ds.PadMessageHandler.SessionStore
		ss.InitSessionForTest(sessionId)
		ss.AddHandleClientInformationForTest(sessionId, padId, token)
		ss.SetAuthorForTest(sessionId, authorId)
		ss.SetPadIdForTest(sessionId, padId)
		ss.AddPadReadOnlyIdsForTest(sessionId, padId, "ro-"+padId, false)
		ds.PadMessageHandler.HandleMessage(ws.UserChange{
			Event: "message",
			Data: ws.UserChangeData{
				Component: "pad",
				Type:      "USER_CHANGES",
				Data: ws.UserChangeDataData{
					Apool:     ws.UserChangeDataDataApool{NumToAttrib: map[int][]string{0: {"author", authorId}}, NextNum: 1},
					BaseRev:   p.Head,
					Changeset: "Z:" + toBase36(len(p.Text())) + ">1*0+1$x",
				},
			},
		}, client, initStore.RetrievedSettings, ds.Logger)
		require.GreaterOrEqual(t, acceptedRev(drainClientMessages(client)), 1, "commit on %s not accepted", padId)
	}

	edit(0) // warm up lazily started background goroutines
	before := runtime.NumGoroutine()
	for i := 1; i <= pads; i++ {
		edit(i)
	}
	after := runtime.NumGoroutine()
	assert.Less(t, after-before, 10, "goroutines grew from %d to %d after editing %d pads", before, after, pads)
}

// acceptedRev returns newRev of the last ACCEPT_COMMIT in msgs, or -1.
func acceptedRev(msgs []string) int {
	rev := -1
	for _, m := range msgs {
		var frame []json.RawMessage
		if json.Unmarshal([]byte(m), &frame) != nil || len(frame) != 2 {
			continue
		}
		var body struct {
			Data struct {
				Type   string `json:"type"`
				NewRev int    `json:"newRev"`
			} `json:"data"`
		}
		if json.Unmarshal(frame[1], &body) == nil && body.Data.Type == "ACCEPT_COMMIT" {
			rev = body.Data.NewRev
		}
	}
	return rev
}

func toBase36(n int) string {
	const digits = "0123456789abcdefghijklmnopqrstuvwxyz"
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{digits[n%36]}, b...)
		n /= 36
	}
	return string(b)
}

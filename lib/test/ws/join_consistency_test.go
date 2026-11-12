package ws

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

	"github.com/ether/etherpad-go/lib/models/ws"
	"github.com/ether/etherpad-go/lib/test/testutils"
	libws "github.com/ether/etherpad-go/lib/ws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Go counterparts of etherpad-lite's clientvar_rev_consistency and
// joinCatchUpRace specs. Node tests them by pausing inside an async hook;
// here the same situations arise from real concurrency: clients join while
// other users' changes are being stored.
func TestJoinConsistency(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(
		testutils.TestRunConfig{Name: "CLIENT_VARS rev matches its text", Test: testClientVarsRevMatchesText},
		testutils.TestRunConfig{Name: "CLIENT_VARS stays consistent while the pad is edited", Test: testClientVarsDuringEdits},
		testutils.TestRunConfig{Name: "a join does not reject another user's change", Test: testJoinDuringUserChanges},
	)
	testDb.StartTestDBHandler()
}

// frame is a decoded message a client received.
type frame struct {
	Type       string
	Disconnect string
	Data       map[string]any
}

func frames(client *libws.Client) []frame {
	var out []frame
	for _, raw := range drainClientMessages(client) {
		var arr []json.RawMessage
		if json.Unmarshal([]byte(raw), &arr) != nil || len(arr) != 2 {
			continue
		}
		var msg struct {
			Type       string         `json:"type"`
			Disconnect string         `json:"disconnect"`
			Data       map[string]any `json:"data"`
		}
		if json.Unmarshal(arr[1], &msg) != nil {
			continue
		}
		f := frame{Type: msg.Type, Disconnect: msg.Disconnect, Data: msg.Data}
		if msg.Type == "COLLABROOM" && msg.Data != nil {
			f.Type, _ = msg.Data["type"].(string)
		}
		out = append(out, f)
	}
	return out
}

// joined is what a client learned from its CLIENT_VARS.
type joined struct {
	client  *libws.Client
	author  string
	rev     int
	text    string
	attribs string
	// later holds the frames received after CLIENT_VARS in the same batch.
	later []frame
	// early holds the revisions of NEW_CHANGES received before CLIENT_VARS.
	early []int
}

func joinPad(t *testing.T, ds testutils.TestDataStore, padID, sessionID string) joined {
	t.Helper()
	// Registered under the hub's lock: other clients are active meanwhile.
	client := &libws.Client{
		Hub:       ds.Hub,
		Conn:      libws.NewActualMockWebSocketconn(),
		Send:      make(chan []byte, 4096),
		Room:      padID,
		SessionId: sessionID,
	}
	ds.Hub.ClientsRWMutex.Lock()
	ds.Hub.Clients[client] = true
	ds.Hub.ClientsRWMutex.Unlock()
	t.Cleanup(func() { removeClient(ds.Hub, client) })
	ds.PadMessageHandler.SessionStore.InitSessionForTest(sessionID)
	ds.PadMessageHandler.HandleMessage(ws.ClientReady{
		Event: "message",
		Data:  ws.ClientReadyData{Component: "pad", Type: "CLIENT_READY", PadID: padID, Token: "t.join" + sessionID},
	}, client, ds.Settings, ds.Logger)

	j := joined{client: client, rev: -1}
	for _, f := range frames(client) {
		switch {
		case f.Type == "CLIENT_VARS":
			collab, ok := f.Data["collab_client_vars"].(map[string]any)
			require.True(t, ok, "CLIENT_VARS without collab_client_vars")
			rev, ok := collab["rev"].(float64)
			require.True(t, ok, "CLIENT_VARS rev must be a number")
			atext := collab["initialAttributedText"].(map[string]any)
			j.rev, j.text, j.attribs = int(rev), atext["text"].(string), atext["attribs"].(string)
			j.author, _ = f.Data["userId"].(string)
		case j.rev >= 0:
			// Already after CLIENT_VARS in the same batch.
			j.later = append(j.later, f)
		case f.Type == "NEW_CHANGES":
			j.early = append(j.early, int(f.Data["newRev"].(float64)))
		}
	}
	require.GreaterOrEqual(t, j.rev, 0, "session %s got no CLIENT_VARS", sessionID)
	return j
}

// edit replaces the pad's text through the pad manager, as an API call does.
func edit(t *testing.T, ds testutils.TestDataStore, padID, text string) {
	t.Helper()
	unlock, err := ds.PadManager.LockPad(padID)
	require.NoError(t, err)
	defer unlock()
	p, err := ds.PadManager.GetPad(padID, nil, nil)
	require.NoError(t, err)
	require.NoError(t, p.SpliceText(0, utf8.RuneCountInString(p.Text())-1, text, nil))
	ds.PadMessageHandler.UpdatePadClients(p)
}

func revisionText(t *testing.T, ds testutils.TestDataStore, padID string, rev int) (text, attribs string, head int) {
	t.Helper()
	unlock, err := ds.PadManager.RLockPad(padID)
	require.NoError(t, err)
	defer unlock()
	p, err := ds.PadManager.GetPad(padID, nil, nil)
	require.NoError(t, err)
	atext := p.GetInternalRevisionAText(rev)
	require.NotNil(t, atext, "revision %d", rev)
	return atext.Text, atext.Attribs, p.Head
}

func testClientVarsRevMatchesText(t *testing.T, ds testutils.TestDataStore) {
	const padID = "clientVarsRev"
	for _, text := range []string{"initial text", "edit one", "edit two", "edit three"} {
		edit(t, ds, padID, text)
	}
	j := joinPad(t, ds, padID, "cvr")
	text, attribs, _ := revisionText(t, ds, padID, j.rev)
	assert.Equal(t, text, j.text, "initialAttributedText.text at rev %d", j.rev)
	assert.Equal(t, attribs, j.attribs, "initialAttributedText.attribs at rev %d", j.rev)
	assert.Equal(t, "edit three\n", j.text)
}

func testClientVarsDuringEdits(t *testing.T, ds testutils.TestDataStore) {
	const padID = "clientVarsRace"
	for i := 0; i < 20; i++ {
		edit(t, ds, padID, fmt.Sprint("pre-load-", i))
	}
	_, _, preConnectHead := revisionText(t, ds, padID, 0)

	// Edits keep landing while clients join.
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
				edit(t, ds, padID, fmt.Sprint("mid-handshake-edit-", i))
			}
		}
	}()
	var joins []joined
	for i := 0; i < 15; i++ {
		joins = append(joins, joinPad(t, ds, padID, fmt.Sprint("race", i)))
	}
	close(stop)
	wg.Wait()
	_, _, finalHead := revisionText(t, ds, padID, 0)

	for i, j := range joins {
		text, attribs, _ := revisionText(t, ds, padID, j.rev)
		assert.Equal(t, text, j.text, "client %d: text at advertised rev %d", i, j.rev)
		assert.Equal(t, attribs, j.attribs, "client %d: attribs at advertised rev %d", i, j.rev)
		assert.GreaterOrEqual(t, j.rev, preConnectHead, "client %d: rev older than the head before it connected", i)
		assert.LessOrEqual(t, j.rev, finalHead)
		assert.Empty(t, j.early, "client %d: no revisions before its initial state", i)

		// Everything stored after the advertised rev reached the client as
		// NEW_CHANGES, without a gap.
		next := j.rev + 1
		for _, f := range append(j.later, frames(j.client)...) {
			if f.Type != "NEW_CHANGES" {
				continue
			}
			require.Equal(t, float64(next), f.Data["newRev"], "client %d: revisions must arrive in order (joined at %d, got before CLIENT_VARS: %v)", i, j.rev, j.early)
			next++
		}
		assert.Equal(t, finalHead+1, next, "client %d joined at %d and must be brought to head %d", i, j.rev, finalHead)
	}
}

func testJoinDuringUserChanges(t *testing.T, ds testutils.TestDataStore) {
	const padID = "joinCatchUpRace"
	edit(t, ds, padID, "")
	writer := joinPad(t, ds, padID, "writer")

	// Other users keep joining while the writer types.
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
				joinPad(t, ds, padID, fmt.Sprint("joiner", i))
			}
		}
	}()

	const n = 40
	base, length := writer.rev, utf8.RuneCountInString(writer.text)
	var got []string
	for i := 0; i < n; i++ {
		ds.PadMessageHandler.HandleMessage(ws.UserChange{
			Event: "message",
			Data: ws.UserChangeData{
				Component: "pad",
				Type:      "USER_CHANGES",
				Data: ws.UserChangeDataData{
					Apool:     ws.UserChangeDataDataApool{NumToAttrib: map[int][]string{0: {"author", writer.author}}, NextNum: 1},
					BaseRev:   base,
					Changeset: "Z:" + toBase36(length) + ">1*0+1$h",
				},
			},
		}, writer.client, ds.Settings, ds.Logger)
		accepted := -1
		for _, f := range frames(writer.client) {
			switch {
			case f.Disconnect != "":
				got = append(got, "disconnect:"+f.Disconnect)
			case f.Type == "ACCEPT_COMMIT":
				accepted = int(f.Data["newRev"].(float64))
				got = append(got, f.Type)
			case f.Type == "USER_NEWINFO" || f.Type == "USER_LEAVE" || f.Type == "CUSTOM":
				// other users joining
			default:
				got = append(got, f.Type)
			}
		}
		require.GreaterOrEqual(t, accepted, 0, "change %d was not accepted; writer got %v", i, got)
		base, length = accepted, length+1
	}
	close(stop)
	wg.Wait()

	assert.Equal(t, strings.Repeat("ACCEPT_COMMIT,", n), strings.Join(got, ",")+",", "the writer only gets acknowledgements")
	text, _, _ := revisionText(t, ds, padID, base)
	assert.Equal(t, strings.Repeat("h", n)+"\n", text)
}

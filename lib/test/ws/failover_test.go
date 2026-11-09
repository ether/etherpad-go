package ws

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/ether/etherpad-go/lib/models/ws"
	"github.com/ether/etherpad-go/lib/pad"
	"github.com/ether/etherpad-go/lib/test/testutils"
	libws "github.com/ether/etherpad-go/lib/ws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An instance dies while a user types. The killbench covers this with real
// processes; these tests pin down the two cases around the moment of death
// that timing alone rarely hits:
//
//   - the change was stored, but the instance died before acknowledging it;
//   - the change was not stored.
//
// Either way the client reconnects to another instance with the revision it
// has, and the change must end up in the pad exactly once.
func TestFailoverToAnotherInstance(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(
		testutils.TestRunConfig{Name: "stored but not acknowledged", Test: testFailoverStoredNotAcknowledged},
		testutils.TestRunConfig{Name: "not stored", Test: testFailoverNotStored},
	)
	testDb.StartTestDBHandler()
}

type failoverSetup struct {
	ds      testutils.TestDataStore
	b       instance
	padID   string
	token   string
	author  string
	client  *libws.Client
	baseRev int
	baseLen int
}

func newFailoverSetup(t *testing.T, ds testutils.TestDataStore, padID string) *failoverSetup {
	initStore := ds.ToInitStore()
	hubB := libws.NewHub()
	go hubB.Run()
	sessB := libws.NewSessionStore()
	managerB := pad.NewManager(ds.DS, ds.Hooks)
	s := &failoverSetup{
		ds:    ds,
		padID: padID,
		token: "t.failover" + padID,
		b: instance{
			handler: libws.NewPadMessageHandler(ds.DS, ds.Hooks, managerB, &sessB, hubB, ds.Logger, testutils.GetTestAssets(), ds.Settings),
			hub:     hubB,
			manager: managerB,
		},
	}
	// The user joins through instance A.
	s.client = createTestClient(ds.Hub, "failover-a-"+padID, padID, libws.NewActualMockWebSocketconn())
	t.Cleanup(func() { delete(ds.Hub.Clients, s.client) })
	ds.PadMessageHandler.SessionStore.InitSessionForTest(s.client.SessionId)
	ds.PadMessageHandler.HandleMessage(ws.ClientReady{
		Event: "message",
		Data:  ws.ClientReadyData{Component: "pad", Type: "CLIENT_READY", PadID: padID, Token: s.token},
	}, s.client, initStore.RetrievedSettings, ds.Logger)
	sess := ds.PadMessageHandler.SessionStore.GetSessionForTest(s.client.SessionId)
	require.NotNil(t, sess)
	s.author = sess.Author
	received(s.client)

	unlock := mustLock(ds.PadManager.RLockPad(padID))
	p, err := ds.PadManager.GetPad(padID, nil, nil)
	require.NoError(t, err)
	s.baseRev, s.baseLen = p.Head, utf8.RuneCountInString(p.Text())
	unlock()
	return s
}

// change inserts "Q" at the start, as the user would type it.
func (s *failoverSetup) change(baseRev, baseLen int) ws.UserChange {
	return ws.UserChange{
		Event: "message",
		Data: ws.UserChangeData{
			Component: "pad",
			Type:      "USER_CHANGES",
			Data: ws.UserChangeDataData{
				Apool:     ws.UserChangeDataDataApool{NumToAttrib: map[int][]string{0: {"author", s.author}}, NextNum: 1},
				BaseRev:   baseRev,
				Changeset: "Z:" + toBase36(baseLen) + ">1*0+1$Q",
			},
		},
	}
}

// reconnectToB joins instance B as the browser does after a broken
// connection: same token, reconnect flag, the last revision it has.
func (s *failoverSetup) reconnectToB(t *testing.T) (*libws.Client, []map[string]any) {
	client := createTestClient(s.b.hub, "failover-b-"+s.padID, s.padID, libws.NewActualMockWebSocketconn())
	t.Cleanup(func() { removeClient(s.b.hub, client) })
	s.b.handler.SessionStore.InitSessionForTest(client.SessionId)
	yes, rev := true, s.baseRev
	s.b.handler.HandleMessage(ws.ClientReady{
		Event: "message",
		Data: ws.ClientReadyData{Component: "pad", Type: "CLIENT_READY", PadID: s.padID, Token: s.token,
			Reconnect: &yes, ClientRev: &rev},
	}, client, s.ds.Settings, s.ds.Logger)
	return client, ofType(received(client), "CLIENT_RECONNECT")
}

func (s *failoverSetup) textOnB(t *testing.T) string {
	unlock := mustLock(s.b.manager.RLockPad(s.padID))
	defer unlock()
	p, err := s.b.manager.GetPad(s.padID, nil, nil)
	require.NoError(t, err)
	return p.Text()
}

func testFailoverStoredNotAcknowledged(t *testing.T, ds testutils.TestDataStore) {
	s := newFailoverSetup(t, ds, "failoverStored")
	// A stores the change; its acknowledgement never reaches the client.
	ds.PadMessageHandler.HandleMessage(s.change(s.baseRev, s.baseLen), s.client, ds.Settings, ds.Logger)
	drainClientMessages(s.client)

	_, msgs := s.reconnectToB(t)
	require.Len(t, msgs, 1, "B sends the one revision the client missed")
	assert.Equal(t, float64(s.baseRev+1), msgs[0]["newRev"])
	assert.Equal(t, float64(s.baseRev+1), msgs[0]["headRev"])
	assert.Equal(t, s.author, msgs[0]["author"],
		"it is the client's own change, so the client takes it as acknowledged and does not send it again")

	text := s.textOnB(t)
	assert.True(t, strings.HasPrefix(text, "Q") && !strings.HasPrefix(text, "QQ"), "the change is in the pad once: %q", text[:min(10, len(text))])
}

func testFailoverNotStored(t *testing.T, ds testutils.TestDataStore) {
	s := newFailoverSetup(t, ds, "failoverLost")
	// A dies before storing the change: nothing reaches the database.

	clientB, msgs := s.reconnectToB(t)
	require.Len(t, msgs, 1)
	assert.Equal(t, true, msgs[0]["noChanges"], "nothing was stored")

	// The client sends its change again, now to B.
	s.b.handler.HandleMessage(s.change(s.baseRev, s.baseLen), clientB, ds.Settings, ds.Logger)
	assert.Equal(t, s.baseRev+1, acceptedRev(drainClientMessages(clientB)), "B accepts the resent change")

	text := s.textOnB(t)
	assert.True(t, strings.HasPrefix(text, "Q") && !strings.HasPrefix(text, "QQ"), "the change is in the pad once: %q", text[:min(10, len(text))])
}

// mustLock returns the unlock function of a successful LockPad/RLockPad and
// panics (failing the test) otherwise.
func mustLock(unlock func(), err error) func() {
	if err != nil {
		panic(err)
	}
	return unlock
}

// removeClient takes a test client out of the hub under the hub's lock; the
// hub's run loop may be handling the client concurrently.
func removeClient(hub *libws.Hub, client *libws.Client) {
	hub.ClientsRWMutex.Lock()
	delete(hub.Clients, client)
	hub.ClientsRWMutex.Unlock()
}

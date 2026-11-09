package ws

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/ether/etherpad-go/lib/cluster"
	"github.com/ether/etherpad-go/lib/models/ws"
	"github.com/ether/etherpad-go/lib/pad"
	"github.com/ether/etherpad-go/lib/test/testutils"
	libws "github.com/ether/etherpad-go/lib/ws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pairBus connects two handlers in one process the way memberlist connects
// two instances: whatever one publishes, the other receives.
type pairBus struct {
	name string
	mu   sync.Mutex
	peer *pairBus
	subs []func(cluster.Event)
}

func newBusPair() (*pairBus, *pairBus) {
	a, b := &pairBus{name: "a"}, &pairBus{name: "b"}
	a.peer, b.peer = b, a
	return a, b
}

func (b *pairBus) deliver(e cluster.Event) {
	e.Origin = b.name
	b.peer.mu.Lock()
	subs := b.peer.subs
	b.peer.mu.Unlock()
	for _, fn := range subs {
		fn(e)
	}
}

func (b *pairBus) Publish(e cluster.Event) { b.deliver(e) }
func (b *pairBus) SendTo(node string, e cluster.Event) {
	if node == b.peer.name {
		b.deliver(e)
	}
}
func (b *pairBus) Subscribe(fn func(cluster.Event)) {
	b.mu.Lock()
	b.subs = append(b.subs, fn)
	b.mu.Unlock()
}
func (b *pairBus) Enabled() bool { return true }
func (b *pairBus) Members() int  { return 2 }
func (b *pairBus) Close() error  { return nil }

type instance struct {
	handler *libws.PadMessageHandler
	hub     *libws.Hub
	manager *pad.Manager
}

// received returns the COLLABROOM message types client got so far.
func received(client *libws.Client) []map[string]any {
	var out []map[string]any
	for {
		select {
		case raw := <-client.Send:
			var arr []json.RawMessage
			if json.Unmarshal(raw, &arr) != nil || len(arr) != 2 {
				continue
			}
			var msg struct {
				Type string         `json:"type"`
				Data map[string]any `json:"data"`
			}
			if json.Unmarshal(arr[1], &msg) == nil && msg.Data != nil {
				out = append(out, msg.Data)
			}
		default:
			return out
		}
	}
}

func ofType(msgs []map[string]any, typ string) []map[string]any {
	var out []map[string]any
	for _, m := range msgs {
		if m["type"] == typ {
			out = append(out, m)
		}
	}
	return out
}

func TestClusterPassesLiveUpdatesBetweenInstances(t *testing.T) {
	testutils.RunSequentialMemorySQLite(t, testutils.TestRunConfig{
		Name: "live updates between two instances",
		Test: testClusterLiveUpdates,
	})
}

func testClusterLiveUpdates(t *testing.T, ds testutils.TestDataStore) {
	initStore := ds.ToInitStore()
	busA, busB := newBusPair()

	a := instance{handler: ds.PadMessageHandler, hub: ds.Hub, manager: ds.PadManager}
	hubB := libws.NewHub()
	go hubB.Run()
	sessB := libws.NewSessionStore()
	managerB := pad.NewManager(ds.DS, ds.Hooks)
	b := instance{
		handler: libws.NewPadMessageHandler(ds.DS, ds.Hooks, managerB, &sessB, hubB, ds.Logger, testutils.GetTestAssets(), ds.Settings),
		hub:     hubB,
		manager: managerB,
	}
	a.handler.SetCluster(busA)
	b.handler.SetCluster(busB)
	t.Cleanup(func() { a.handler.StopCluster(); b.handler.StopCluster() })

	padID := "clusterLivePad"
	join := func(in instance, sessionID string) (*libws.Client, string) {
		client := createTestClient(in.hub, sessionID, padID, libws.NewActualMockWebSocketconn())
		t.Cleanup(func() { delete(in.hub.Clients, client) })
		in.handler.SessionStore.InitSessionForTest(sessionID)
		in.handler.HandleMessage(ws.ClientReady{
			Event: "message",
			Data:  ws.ClientReadyData{Component: "pad", Type: "CLIENT_READY", PadID: padID, Token: "t.cluster" + sessionID},
		}, client, initStore.RetrievedSettings, ds.Logger)
		sess := in.handler.SessionStore.GetSessionForTest(sessionID)
		require.NotNil(t, sess)
		require.NotEmpty(t, sess.Author)
		return client, sess.Author
	}

	clientA, authorA := join(a, "cluster-a")
	received(clientA)
	clientB, authorB := join(b, "cluster-b")

	// Presence: each side learns about the user on the other instance.
	newInfoAtA := ofType(received(clientA), "USER_NEWINFO")
	require.NotEmpty(t, newInfoAtA, "user on A must hear of the user joining on B")
	assert.Equal(t, authorB, newInfoAtA[0]["userInfo"].(map[string]any)["userId"])
	var usersSeenByB []string
	for _, m := range ofType(received(clientB), "USER_NEWINFO") {
		usersSeenByB = append(usersSeenByB, m["userInfo"].(map[string]any)["userId"].(string))
	}
	assert.Contains(t, usersSeenByB, authorA, "user joining on B must hear of the user on A")

	// Chat.
	sessA := a.handler.SessionStore.GetSessionForTest("cluster-a")
	now := time.Now().UnixMilli()
	a.handler.SendChatMessageToPadClients(sessA, ws.ChatMessageData{Text: "hello from A", Time: &now, AuthorId: &authorA})
	chatAtB := ofType(received(clientB), "CHAT_MESSAGE")
	require.Len(t, chatAtB, 1)
	assert.Equal(t, "hello from A", chatAtB[0]["message"].(map[string]any)["text"])

	// A revision written through A reaches the client on B.
	func() {
		unlock := mustLock(a.manager.LockPad(padID))
		defer unlock()
		p, err := a.manager.GetPad(padID, nil, nil)
		require.NoError(t, err)
		require.NoError(t, p.SpliceText(utf8.RuneCountInString(p.Text())-1, 0, "typed on A\n", &authorA))
		a.handler.UpdatePadClients(p)
	}()
	var newChanges []map[string]any
	require.Eventually(t, func() bool {
		newChanges = append(newChanges, ofType(received(clientB), "NEW_CHANGES")...)
		return len(newChanges) > 0
	}, 5*time.Second, 20*time.Millisecond, "client on B must get the revision made on A")
	assert.True(t, strings.Contains(newChanges[len(newChanges)-1]["changeset"].(string), "typed on A"))

	// Leaving on B is seen on A.
	received(clientA)
	b.handler.HandleDisconnectOfPadClient(clientB, initStore.RetrievedSettings, ds.Logger)
	leaves := ofType(received(clientA), "USER_LEAVE")
	require.Len(t, leaves, 1)
	assert.Equal(t, authorB, leaves[0]["userInfo"].(map[string]any)["userId"])
}

// The same author joining a pad through another instance takes over, as on
// one instance: the older session is disconnected with "userdup".
func TestClusterDisconnectsDuplicateSessionOnOtherInstance(t *testing.T) {
	testutils.RunSequentialMemorySQLite(t, testutils.TestRunConfig{
		Name: "duplicate author across instances",
		Test: testClusterUserDup,
	})
}

func testClusterUserDup(t *testing.T, ds testutils.TestDataStore) {
	busA, busB := newBusPair()
	hubB := libws.NewHub()
	go hubB.Run()
	sessB := libws.NewSessionStore()
	managerB := pad.NewManager(ds.DS, ds.Hooks)
	handlerB := libws.NewPadMessageHandler(ds.DS, ds.Hooks, managerB, &sessB, hubB, ds.Logger, testutils.GetTestAssets(), ds.Settings)
	ds.PadMessageHandler.SetCluster(busA)
	handlerB.SetCluster(busB)
	t.Cleanup(func() { ds.PadMessageHandler.StopCluster(); handlerB.StopCluster() })

	const padID = "clusterUserDup"
	const token = "t.sameAuthorEverywhere"
	join := func(h *libws.PadMessageHandler, hub *libws.Hub, sessionID string) *libws.Client {
		client := createTestClient(hub, sessionID, padID, libws.NewActualMockWebSocketconn())
		t.Cleanup(func() { delete(hub.Clients, client) })
		h.SessionStore.InitSessionForTest(sessionID)
		h.HandleMessage(ws.ClientReady{
			Event: "message",
			Data:  ws.ClientReadyData{Component: "pad", Type: "CLIENT_READY", PadID: padID, Token: token},
		}, client, ds.Settings, ds.Logger)
		return client
	}

	onA := join(ds.PadMessageHandler, ds.Hub, "dup-a")
	drainClientMessages(onA)
	sessA := ds.PadMessageHandler.SessionStore.GetSessionForTest("dup-a")
	require.NotEmpty(t, sessA.Author)
	roA := sessA.ReadOnlyPadId
	onB := join(handlerB, hubB, "dup-b")
	bSession := handlerB.SessionStore.GetSessionForTest("dup-b")
	require.Equal(t, sessA.Author, bSession.Author)
	assert.Equal(t, roA, bSession.ReadOnlyPadId, "both instances give out the same read-only link")

	var gotDup bool
	for _, m := range drainClientMessages(onA) {
		if strings.Contains(m, `"userdup"`) {
			gotDup = true
		}
	}
	assert.True(t, gotDup, "the older session on A is told it was taken over")
	for _, m := range drainClientMessages(onB) {
		assert.NotContains(t, m, `"userdup"`, "the new session on B stays")
	}
}

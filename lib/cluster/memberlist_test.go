package cluster

import (
	"net"
	"testing"
	"time"

	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func startNode(t *testing.T, store db.DataStore) *Memberlist {
	t.Helper()
	b, err := NewMemberlist(settings.Cluster{BindAddr: "127.0.0.1", BindPort: freePort(t), AdvertiseAddr: "127.0.0.1"}, store, zap.NewNop().Sugar())
	require.NoError(t, err)
	t.Cleanup(func() { b.Close() })
	return b
}

func TestNodesFindEachOtherThroughTheDatabaseAndExchangeEvents(t *testing.T) {
	store := db.NewMemoryDataStore()
	a := startNode(t, store)
	b := startNode(t, store) // joins a at start: a is in the registry

	require.Eventually(t, func() bool { return a.Members() == 2 && b.Members() == 2 }, 10*time.Second, 50*time.Millisecond)

	got := make(chan Event, 10)
	b.Subscribe(func(e Event) { got <- e })

	a.Publish(Event{Kind: KindRevision, Pad: "p1", Rev: 7})
	a.Publish(Event{Kind: KindRoom, Pad: "p1", Payload: []byte(`["message",{"type":"COLLABROOM"}]`)})

	// UDP (hints) and TCP (payloads) may overtake each other.
	kinds := map[Kind]bool{}
	for range 2 {
		select {
		case e := <-got:
			kinds[e.Kind] = true
			assert.Equal(t, "p1", e.Pad)
			assert.Equal(t, a.name, e.Origin)
		case <-time.After(5 * time.Second):
			t.Fatalf("got only %v", kinds)
		}
	}
	assert.Equal(t, map[Kind]bool{KindRevision: true, KindRoom: true}, kinds)

	// Answering the sender only.
	replies := make(chan Event, 1)
	a.Subscribe(func(e Event) { replies <- e })
	b.SendTo(a.name, Event{Kind: KindSession, Session: "s1", Payload: []byte(`{}`)})
	select {
	case e := <-replies:
		assert.Equal(t, KindSession, e.Kind)
	case <-time.After(5 * time.Second):
		t.Fatal("no reply")
	}
}

// Both nodes use the key the first one stored, or they could not decrypt
// each other's traffic.
func TestNodesShareTheGossipKey(t *testing.T) {
	store := db.NewMemoryDataStore()
	k1, err := gossipKey(store)
	require.NoError(t, err)
	k2, err := gossipKey(store)
	require.NoError(t, err)
	assert.Equal(t, k1, k2)
	assert.Len(t, k1, 32)
}

func TestClosedNodeLeavesTheRegistry(t *testing.T) {
	store := db.NewMemoryDataStore()
	a := startNode(t, store)
	nodes, err := store.ListClusterNodes(time.Now().Add(-time.Minute))
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	require.NoError(t, a.Close())
	nodes, err = store.ListClusterNodes(time.Now().Add(-time.Minute))
	require.NoError(t, err)
	assert.Empty(t, nodes)
}

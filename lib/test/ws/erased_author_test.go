package ws

import (
	"testing"
	"time"

	"github.com/ether/etherpad-go/lib/metrics"
	"github.com/ether/etherpad-go/lib/models/ws"
	"github.com/ether/etherpad-go/lib/test/testutils"
	libws "github.com/ether/etherpad-go/lib/ws"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Erasing an author closes the sessions still connected as that author;
// chat from such a session is not stored under the erased author again.
func TestErasedAuthorSessions(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(
		testutils.TestRunConfig{Name: "erasure disconnects the author", Test: testErasureDisconnects},
		testutils.TestRunConfig{Name: "chat of an erased author is not stored", Test: testErasedAuthorChat},
	)
	testDb.StartTestDBHandler()
}

func erasedTestSession(t *testing.T, ds testutils.TestDataStore, padID, sessionID string) (*libws.Client, *libws.MockWebSocketConn, string) {
	authorID, err := setupPadAndAuthor(t, ds, padID, "Erasable")
	require.NoError(t, err)
	conn := libws.NewActualMockWebSocketconn()
	client := createTestClient(ds.Hub, sessionID, padID, conn)
	t.Cleanup(func() { removeClient(ds.Hub, client) })
	ss := ds.PadMessageHandler.SessionStore
	ss.InitSessionForTest(sessionID)
	ss.AddHandleClientInformationForTest(sessionID, padID, "t.erased")
	ss.SetAuthorForTest(sessionID, authorID)
	ss.SetPadIdForTest(sessionID, padID)
	return client, conn, authorID
}

func testErasureDisconnects(t *testing.T, ds testutils.TestDataStore) {
	victim, victimConn, authorID := erasedTestSession(t, ds, "erasedPadA", "erased-a")
	other, otherConn, _ := erasedTestSession(t, ds, "erasedPadA", "erased-b")
	_ = drainClientMessages(victim)
	_ = drainClientMessages(other)

	ds.PadMessageHandler.DisconnectAuthor(authorID)

	assert.Contains(t, drainClientMessages(victim), `["message",{"disconnect":"kicked"}]`)
	assert.True(t, victimConn.IsClosed())
	assert.False(t, otherConn.IsClosed(), "other authors stay connected")
}

func testErasedAuthorChat(t *testing.T, ds testutils.TestDataStore) {
	const padID = "erasedPadChat"
	_, conn, authorID := erasedTestSession(t, ds, padID, "erased-chat")
	_, err := ds.AuthorManager.Anonymize(authorID, false)
	require.NoError(t, err)

	session := ds.PadMessageHandler.SessionStore.GetSessionForTest("erased-chat")
	now := time.Now().UnixMilli()
	ds.PadMessageHandler.SendChatMessageToPadClients(session, ws.ChatMessageData{Text: "still here", Time: &now, AuthorId: &authorID})

	chats, err := ds.DS.GetChatsOfPad(padID, 0, 100)
	require.NoError(t, err)
	if chats != nil {
		for _, c := range *chats {
			assert.NotEqual(t, "still here", c.Message, "chat of an erased author was stored")
		}
	}
	assert.True(t, conn.IsClosed(), "the erased author's session is closed")
}

// The join/leave system chat counts as a CHAT_MESSAGE emit like any chat.
func TestSystemChatCountsAsEmit(t *testing.T) {
	// Once for all database variants, which run in parallel; the parent's
	// cleanup runs after them.
	metrics.SetEnabled(true)
	t.Cleanup(func() { metrics.SetEnabled(false) })
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(testutils.TestRunConfig{Name: "system chat emit", Test: func(t *testing.T, ds testutils.TestDataStore) {
		before := testutil.ToFloat64(metrics.SocketEmitsTotal.WithLabelValues("CHAT_MESSAGE"))
		ds.PadMessageHandler.BroadcastSystemChatToRoom("systemChatPad", map[string]any{"text": "joined"})
		after := testutil.ToFloat64(metrics.SocketEmitsTotal.WithLabelValues("CHAT_MESSAGE"))
		assert.GreaterOrEqual(t, after-before, 1.0)
	}})
	testDb.StartTestDBHandler()
}

package ws

import (
	"testing"

	"github.com/ether/etherpad-go/lib/hooks/events"
	"github.com/ether/etherpad-go/lib/models/webaccess"
	"github.com/ether/etherpad-go/lib/models/ws"
	"github.com/ether/etherpad-go/lib/test/testutils"
	libws "github.com/ether/etherpad-go/lib/ws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// etherpad-lite #8458 and #8461: what a CLIENT_READY does to the session of
// a socket that has already joined a pad.
func TestClientReadyStaging(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(
		testutils.TestRunConfig{Name: "a refused CLIENT_READY leaves the joined pad's session", Test: testRefusedClientReadyKeepsSession},
		testutils.TestRunConfig{Name: "a later CLIENT_READY supersedes an earlier one", Test: testLaterClientReadySupersedes},
		testutils.TestRunConfig{Name: "a refused socket gets none of the pad's messages", Test: testRefusedSocketGetsNoBroadcasts},
	)
	defer testDb.StartTestDBHandler()
}

func clientReady(s *authnServer, client *libws.Client, padID, token string) {
	s.handler.HandleMessage(ws.ClientReady{
		Event: "message",
		Data:  ws.ClientReadyData{Component: "pad", Type: "CLIENT_READY", PadID: padID, Token: token},
	}, client, s.cfg, s.ds.Logger)
}

func inRoom(s *authnServer, padID, sessionID string) bool {
	for _, c := range s.handler.GetRoomSockets(padID) {
		if c.SessionId == sessionID {
			return true
		}
	}
	return false
}

// The socket joined padA; a CLIENT_READY for padB, which it may not open,
// is refused and must not touch its session: it stays on padA as the same
// author and can go on editing there (before, the session was replaced
// before the access check and pointed at padB).
func testRefusedClientReadyKeepsSession(t *testing.T, ds testutils.TestDataStore) {
	s := newAuthnServer(t, ds, true, func(c *events.AuthorizeContext) {
		if c.PadId == "stagingAllowed" {
			c.Grant("create")
		}
	})
	j := s.join(t, "stagingAllowed", "stagingSession", socketUser("user"))
	require.Empty(t, j.access)
	require.NotEmpty(t, j.author)
	before := *s.handler.SessionStore.GetSessionForTest("stagingSession")

	clientReady(s, j.client, "stagingForbidden", "t.authnstagingSession")
	drainClientMessages(j.client)
	after := s.handler.SessionStore.GetSessionForTest("stagingSession")
	require.NotNil(t, after)
	assert.Equal(t, "stagingAllowed", after.PadId, "the session stays on the joined pad")
	assert.Equal(t, "stagingAllowed", after.Auth.PadId)
	assert.Equal(t, before.Author, after.Author, "and keeps its author")
	assert.False(t, inRoom(s, "stagingForbidden", "stagingSession"), "the refused pad's room is not joined")

	assert.True(t, s.edit(t, j, "stagingAllowed"), "the socket can still edit its pad")
}

// Two CLIENT_READYs in a row: Go handles a socket's messages one after the
// other, so the first join completes before the second starts and the
// socket ends up in the second pad only (in Node the two could interleave).
func testLaterClientReadySupersedes(t *testing.T, ds testutils.TestDataStore) {
	s := newAuthnServer(t, ds, true, func(c *events.AuthorizeContext) { c.Grant("create") })
	j := s.join(t, "supersedeFirst", "supersedeSession", socketUser("user"))
	require.Empty(t, j.access)
	clientReady(s, j.client, "supersedeSecond", "t.authnsupersedeSession")
	drainClientMessages(j.client)

	sess := s.handler.SessionStore.GetSessionForTest("supersedeSession")
	require.NotNil(t, sess)
	assert.Equal(t, "supersedeSecond", sess.PadId)
	assert.True(t, inRoom(s, "supersedeSecond", "supersedeSession"))
	assert.False(t, inRoom(s, "supersedeFirst", "supersedeSession"), "the first pad's room is left")
	assert.True(t, s.edit(t, authnJoin{client: j.client, author: sess.Author}, "supersedeSecond"))
}

// A socket refused for a pad must get none of its live messages. Rooms
// follow the session's pad, which used to be set before the access check
// and kept after a refusal: anyone could follow the edits and chat of a
// pad they were refused by sending CLIENT_READY for it.
func testRefusedSocketGetsNoBroadcasts(t *testing.T, ds testutils.TestDataStore) {
	s := newAuthnServer(t, ds, true, func(c *events.AuthorizeContext) {
		if u, ok := c.User.(*webaccess.SocketClientRequest); ok && u.Username != nil && *u.Username == "owner" {
			c.Grant("create")
		}
	})
	owner := s.join(t, "privatePad", "privateOwner", socketUser("owner"))
	require.Empty(t, owner.access)
	intruder := s.join(t, "privatePad", "privateIntruder", socketUser("intruder"))
	require.NotEmpty(t, intruder.access, "the intruder is refused")

	require.True(t, s.edit(t, owner, "privatePad"))
	chat := ws.ChatMessage{Event: "message"}
	chat.Data.Type, chat.Data.Component = "COLLABROOM", "pad"
	chat.Data.Data.Type = "CHAT_MESSAGE"
	chat.Data.Data.Message = ws.ChatMessageData{Text: "secret"}
	s.handler.HandleMessage(chat, owner.client, s.cfg, s.ds.Logger)
	require.Contains(t, frameTypes(owner.client), "CHAT_MESSAGE", "the owner's chat went out")
	for _, typ := range frameTypes(intruder.client) {
		assert.NotContains(t, []string{"NEW_CHANGES", "CHAT_MESSAGE"}, typ, "the refused socket got the pad's %s", typ)
	}
	assert.False(t, inRoom(s, "privatePad", "privateIntruder"))
}

func frameTypes(client *libws.Client) []string {
	var out []string
	for _, f := range frames(client) {
		out = append(out, f.Type)
	}
	return out
}

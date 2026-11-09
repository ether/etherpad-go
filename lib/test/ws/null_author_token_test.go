package ws

import (
	"testing"

	"github.com/ether/etherpad-go/lib/models/ws"
	"github.com/ether/etherpad-go/lib/test/testutils"
	libws "github.com/ether/etherpad-go/lib/ws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Websocket half of etherpad-lite's nullAuthorToken.ts
// (GHSA-746x-9w89-7xv6).
func TestNullAuthorTokenWebsocket(t *testing.T) {
	for _, conf := range []testutils.TestRunConfig{
		{Name: "each token-less connection gets its own author", Test: testTokenlessConnectionsGetOwnAuthor},
		{Name: "unrelated token-less connection cannot delete the pad", Test: testTokenlessConnectionCannotDeleteOthersPad},
	} {
		testutils.RunSequentialMemorySQLite(t, conf)
	}
}

// tokenlessClientReady connects a client whose CLIENT_READY carries no token
// and returns it with the author the server assigned.
func tokenlessClientReady(t *testing.T, ds testutils.TestDataStore, padId, sessionId string) (*libws.Client, string) {
	t.Helper()
	initStore := ds.ToInitStore()
	ds.Settings.LoadTest = false
	client := createTestClient(ds.Hub, sessionId, padId, libws.NewActualMockWebSocketconn())
	t.Cleanup(func() { removeClient(ds.Hub, client) })
	ds.PadMessageHandler.SessionStore.InitSessionForTest(sessionId)
	ds.PadMessageHandler.HandleMessage(ws.ClientReady{
		Event: "message",
		Data:  ws.ClientReadyData{Component: "pad", Type: "CLIENT_READY", PadID: padId, Token: ""},
	}, client, initStore.RetrievedSettings, ds.Logger)
	sess := ds.PadMessageHandler.SessionStore.GetSessionForTest(sessionId)
	require.NotNil(t, sess)
	require.NotEmpty(t, sess.Author, "token-less connection must be granted access")
	return client, sess.Author
}

func testTokenlessConnectionsGetOwnAuthor(t *testing.T, ds testutils.TestDataStore) {
	padId := "nullTokenWsOwnAuthor"
	_, first := tokenlessClientReady(t, ds, padId, "null-ws-1")
	_, second := tokenlessClientReady(t, ds, padId, "null-ws-2")
	assert.NotEqual(t, first, second)
}

func testTokenlessConnectionCannotDeleteOthersPad(t *testing.T, ds testutils.TestDataStore) {
	padId := "nullTokenWsDelete"
	_, creator := tokenlessClientReady(t, ds, padId, "null-ws-creator")
	p, err := ds.PadManager.GetPad(padId, nil, nil)
	require.NoError(t, err)
	firstAuthor, err := p.GetRevisionAuthor(0)
	require.NoError(t, err)
	require.Equal(t, creator, *firstAuthor, "the creator's connection made revision 0")

	other, otherAuthor := tokenlessClientReady(t, ds, padId, "null-ws-other")
	require.NotEqual(t, creator, otherAuthor)
	ds.PadMessageHandler.HandlePadDelete(other, libws.PadDelete{
		Type: "PAD_DELETE",
		Data: struct {
			PadID string `json:"padId"`
		}{PadID: padId},
	})
	exists, err := ds.PadManager.DoesPadExist(padId)
	require.NoError(t, err)
	assert.True(t, *exists, "pad was deleted by an unrelated token-less connection")
}

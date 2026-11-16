package ws

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/ether/etherpad-go/lib/author"
	"github.com/ether/etherpad-go/lib/test/testutils"
	libws "github.com/ether/etherpad-go/lib/ws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// etherpad-lite's authorNameCap spec (GHSA-w4w9-j4c3-hjf6): display names
// come from unauthenticated messages and go to everyone opening a pad, so
// they are capped when stored and again when read.
func TestAuthorNameCap(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(
		testutils.TestRunConfig{Name: "an oversized name is cut before it is stored", Test: testNameCappedOnWrite},
		testutils.TestRunConfig{Name: "a name stored before the cap is cut when read", Test: testNameCappedOnRead},
		testutils.TestRunConfig{Name: "USERINFO_UPDATE stores and sends the cut name", Test: testUserInfoUpdateCapsName},
		testutils.TestRunConfig{Name: "USERINFO_UPDATE without colorId is ignored", Test: testUserInfoUpdateWithoutColor},
	)
	defer testDb.StartTestDBHandler()
}

var hugeName = strings.Repeat("A", 200_000)

func assertCapped(t *testing.T, name *string, what string) {
	t.Helper()
	require.NotNil(t, name, what)
	assert.Equal(t, author.MaxNameLength, utf8.RuneCountInString(*name), what)
}

func testNameCappedOnWrite(t *testing.T, ds testutils.TestDataStore) {
	a, err := ds.AuthorManager.CreateAuthor(&hugeName)
	require.NoError(t, err)
	stored, err := ds.DS.GetAuthor(a.Id)
	require.NoError(t, err)
	assertCapped(t, stored.Name, "createAuthor")

	normal := "  Alice  "
	require.NoError(t, ds.AuthorManager.SetAuthorName(a.Id, normal))
	stored, err = ds.DS.GetAuthor(a.Id)
	require.NoError(t, err)
	require.NotNil(t, stored.Name)
	assert.Equal(t, "Alice", *stored.Name, "a normal name, trimmed")

	require.NoError(t, ds.AuthorManager.SetAuthorName(a.Id, hugeName))
	stored, err = ds.DS.GetAuthor(a.Id)
	require.NoError(t, err)
	assertCapped(t, stored.Name, "setAuthorName")
}

func testNameCappedOnRead(t *testing.T, ds testutils.TestDataStore) {
	padId := "nameCapRead"
	authorId, err := setupPadAndAuthor(t, ds, padId, "x")
	require.NoError(t, err)
	// Stored as an older version did: in full.
	require.NoError(t, ds.DS.SaveAuthorName(authorId, hugeName))

	a, err := ds.AuthorManager.GetAuthor(authorId)
	require.NoError(t, err)
	assertCapped(t, a.Name, "getAuthor (historicalAuthorData, USER_NEWINFO, own clientVars)")
	name, err := ds.AuthorManager.GetAuthorName(authorId)
	require.NoError(t, err)
	assertCapped(t, name, "getAuthorName")
	list, err := ds.AuthorManager.GetAuthors([]string{authorId})
	require.NoError(t, err)
	require.Len(t, *list, 1)
	assertCapped(t, (*list)[0].Name, "getAuthors")

	p, err := ds.PadManager.GetPad(padId, nil, nil)
	require.NoError(t, err)
	_, err = p.AppendChatMessage(&authorId, 1, "hi")
	require.NoError(t, err)
	msgs, err := p.GetChatMessages(0, p.ChatHead)
	require.NoError(t, err)
	require.NotEmpty(t, *msgs)
	assertCapped(t, (*msgs)[len(*msgs)-1].DisplayName, "chat display name")
}

func userInfoUpdate(color, name *string) libws.UserInfoUpdate {
	u := libws.UserInfoUpdate{Type: "COLLABROOM"}
	u.Data.Type = "USERINFO_UPDATE"
	u.Data.UserInfo.ColorId = color
	u.Data.UserInfo.Name = name
	return u
}

func testUserInfoUpdateCapsName(t *testing.T, ds testutils.TestDataStore) {
	padId := "nameCapUserInfo"
	authorId, err := setupPadAndAuthor(t, ds, padId, "Old")
	require.NoError(t, err)
	mockConn := libws.NewActualMockWebSocketconn()
	sessionId := "nameCapUserInfoSession"
	client := createTestClient(ds.Hub, sessionId, padId, mockConn)
	defer delete(ds.Hub.Clients, client)
	wg := startMockWritePump(client, mockConn)
	ds.PadMessageHandler.SessionStore.InitSessionForTest(sessionId)
	ds.PadMessageHandler.SessionStore.AddHandleClientInformationForTest(sessionId, padId, "test-token")
	ds.PadMessageHandler.SessionStore.SetAuthorForTest(sessionId, authorId)
	ds.PadMessageHandler.SessionStore.SetPadIdForTest(sessionId, padId)

	color := "#00ff00"
	ds.PadMessageHandler.HandleUserInfoUpdate(userInfoUpdate(&color, &hugeName), client)
	wg.Wait()

	stored, err := ds.DS.GetAuthor(authorId)
	require.NoError(t, err)
	assertCapped(t, stored.Name, "stored")

	require.NotEmpty(t, mockConn.Data, "USER_NEWINFO")
	var msg []json.RawMessage
	require.NoError(t, json.Unmarshal(mockConn.Data[0].Data, &msg))
	var newInfo struct {
		Data struct {
			Type     string `json:"type"`
			UserInfo struct {
				Name *string `json:"name"`
			} `json:"userInfo"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(msg[1], &newInfo))
	assert.Equal(t, "USER_NEWINFO", newInfo.Data.Type)
	assertCapped(t, newInfo.Data.UserInfo.Name, "USER_NEWINFO to the pad")
}

func testUserInfoUpdateWithoutColor(t *testing.T, ds testutils.TestDataStore) {
	padId := "nameCapNoColor"
	authorId, err := setupPadAndAuthor(t, ds, padId, "Keep")
	require.NoError(t, err)
	mockConn := libws.NewActualMockWebSocketconn()
	sessionId := "nameCapNoColorSession"
	client := createTestClient(ds.Hub, sessionId, padId, mockConn)
	defer delete(ds.Hub.Clients, client)
	ds.PadMessageHandler.SessionStore.InitSessionForTest(sessionId)
	ds.PadMessageHandler.SessionStore.SetAuthorForTest(sessionId, authorId)
	ds.PadMessageHandler.SessionStore.SetPadIdForTest(sessionId, padId)

	name := "New"
	assert.NotPanics(t, func() {
		ds.PadMessageHandler.HandleUserInfoUpdate(userInfoUpdate(nil, &name), client)
	})
	stored, err := ds.DS.GetAuthor(authorId)
	require.NoError(t, err)
	require.NotNil(t, stored.Name)
	assert.Equal(t, "Keep", *stored.Name, "ignored like etherpad-lite's 'missing colorId'")
}

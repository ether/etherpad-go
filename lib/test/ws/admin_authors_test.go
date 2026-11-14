package ws

import (
	"encoding/json"
	"testing"

	"github.com/ether/etherpad-go/lib/models/ws/admin"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/ether/etherpad-go/lib/test/testutils"
	libws "github.com/ether/etherpad-go/lib/ws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Port of etherpad-lite's admin/anonymizeAuthorSocket spec.
func TestAdminAuthors(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(
		testutils.TestRunConfig{Name: "list, preview and erase", Test: testAdminAuthors},
		testutils.TestRunConfig{Name: "off by default", Test: testAdminAuthorsDisabled},
	)
	testDb.StartTestDBHandler()
}

// authorRequests sends admin events with gdprAuthorErasure set as given
// (its own settings copy: the tests run in parallel).
func authorRequests(erasure bool) func(t *testing.T, ds testutils.TestDataStore, event string, payload any) map[string]any {
	return func(t *testing.T, ds testutils.TestDataStore, event string, payload any) map[string]any {
		t.Helper()
		client := &libws.Client{Hub: libws.NewHub(), Conn: ds.MockWebSocket, Send: make(chan []byte, 256), SessionId: "admin"}
		data, err := json.Marshal(payload)
		require.NoError(t, err)
		before := len(ds.MockWebSocket.Data)
		pump := startMockWritePump(client, ds.MockWebSocket)
		s := settings.Displayed
		s.GdprAuthorErasure.Enabled = erasure
		ds.AdminMessageHandler.HandleMessage(admin.EventMessage{Event: event, Data: data}, &s, client)
		pump.Wait()
		require.Len(t, ds.MockWebSocket.Data, before+1, "%s must get exactly one reply", event)
		var resp []json.RawMessage
		require.NoError(t, json.Unmarshal(ds.MockWebSocket.Data[before].Data, &resp))
		var name string
		require.NoError(t, json.Unmarshal(resp[0], &name))
		require.Equal(t, "results:"+event, name)
		var m map[string]any
		require.NoError(t, json.Unmarshal(resp[1], &m))
		return m
	}
}

func testAdminAuthors(t *testing.T, ds testutils.TestDataStore) {
	authorReply := authorRequests(true)
	name := "Sock tag"
	a, err := ds.AuthorManager.CreateAuthorIfNotExistsFor("m-sock-tag", &name)
	require.NoError(t, err)

	res := authorReply(t, ds, "authorLoad", map[string]any{"pattern": "sock", "offset": 0, "limit": 12, "sortBy": "name", "ascending": true})
	assert.EqualValues(t, 1, res["total"])
	rows := res["results"].([]any)
	require.Len(t, rows, 1)
	row := rows[0].(map[string]any)
	assert.Equal(t, "Sock tag", row["name"])
	assert.Equal(t, a.Id, row["authorID"])
	assert.Equal(t, []any{"m-sock-tag"}, row["mapper"])

	preview := authorReply(t, ds, "anonymizeAuthorPreview", map[string]string{"authorID": a.Id})
	assert.Equal(t, a.Id, preview["authorID"])
	assert.EqualValues(t, 1, preview["removedExternalMappings"])
	r, _ := ds.DS.GetAuthorRow(a.Id)
	assert.False(t, r.Erased, "a preview erases nothing")

	erased := authorReply(t, ds, "anonymizeAuthor", map[string]string{"authorID": a.Id})
	assert.Equal(t, a.Id, erased["authorID"])
	assert.EqualValues(t, 1, erased["removedExternalMappings"])
	r, _ = ds.DS.GetAuthorRow(a.Id)
	assert.True(t, r.Erased)

	// Payload-less or empty requests are answered with an error.
	for _, ev := range []string{"anonymizeAuthorPreview", "anonymizeAuthor"} {
		assert.NotEmpty(t, authorReply(t, ds, ev, nil)["error"], ev)
	}
	assert.NotEmpty(t, authorReply(t, ds, "anonymizeAuthor", map[string]string{"authorID": "a.unknown"})["error"])
	// A list without a payload uses the defaults.
	assert.Empty(t, authorReply(t, ds, "authorLoad", nil)["error"])
}

func testAdminAuthorsDisabled(t *testing.T, ds testutils.TestDataStore) {
	authorReply := authorRequests(false)
	name := "Off"
	a, err := ds.AuthorManager.CreateAuthorIfNotExistsFor("m-off", &name)
	require.NoError(t, err)

	res := authorReply(t, ds, "anonymizeAuthor", map[string]string{"authorID": a.Id})
	assert.Equal(t, "disabled", res["error"])
	r, _ := ds.DS.GetAuthorRow(a.Id)
	assert.False(t, r.Erased)

	preview := authorReply(t, ds, "anonymizeAuthorPreview", map[string]string{"authorID": a.Id})
	assert.Equal(t, "disabled", preview["error"])
	assert.NotContains(t, preview, "removedExternalMappings", "no counters when off")

	list := authorReply(t, ds, "authorLoad", map[string]any{"pattern": ""})
	assert.Equal(t, "disabled", list["error"])
	assert.Equal(t, []any{}, list["results"])
}

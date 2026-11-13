package ws

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	db2 "github.com/ether/etherpad-go/lib/models/db"
	"github.com/ether/etherpad-go/lib/models/ws/admin"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/ether/etherpad-go/lib/test/testutils"
	libws "github.com/ether/etherpad-go/lib/ws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Port of etherpad-lite's padLoadResilience (#7935): a pad that can no
// longer be loaded must still be deletable from the admin page, together
// with its chat, without touching other pads.
func TestDeleteUnloadablePad(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(
		testutils.TestRunConfig{Name: "a pad with a missing revision can be deleted", Test: testDeletePadWithRevisionHole},
		testutils.TestRunConfig{Name: "a pad with an id that is no longer valid can be deleted", Test: testDeleteLegacyInvalidIdPad},
		testutils.TestRunConfig{Name: "admin page lists and deletes an unloadable pad", Test: testAdminListsAndDeletesUnloadablePad},
		testutils.TestRunConfig{Name: "padLoad always replies", Test: testAdminPadLoadAlwaysReplies},
	)
	defer testDb.StartTestDBHandler()
}

func testDeletePadWithRevisionHole(t *testing.T, ds testutils.TestDataStore) {
	padId := "unloadableHole"
	other := "unloadableHole-other"
	a, err := ds.AuthorManager.CreateAuthor(nil)
	require.NoError(t, err)
	text := "start\n"
	p, err := ds.PadManager.GetPad(padId, &text, &a.Id)
	require.NoError(t, err)
	for i := 1; i <= 5; i++ {
		require.NoError(t, p.SetText(p.Text()+fmt.Sprintf("line %d\n", i), &a.Id))
	}
	require.NoError(t, ds.DS.SaveChatMessage(padId, 0, &a.Id, time.Now().UnixMilli(), "hello"))
	_, err = ds.PadManager.GetPad(other, &text, &a.Id)
	require.NoError(t, err)
	require.NoError(t, ds.DS.SaveChatMessage(other, 0, &a.Id, time.Now().UnixMilli(), "keep me"))

	punchRevisionHole(t, ds, padId, 3)
	_, err = ds.PadManager.GetPad(padId, nil, nil)
	require.Error(t, err, "the damaged pad must not load")

	require.NoError(t, ds.PadMessageHandler.DeletePad(padId))

	exists, err := ds.DS.DoesPadExist(padId)
	require.NoError(t, err)
	assert.False(t, *exists)
	_, err = ds.DS.GetRevision(padId, 0)
	assert.Error(t, err, "revisions removed")
	chats, err := ds.DS.GetChatsOfPad(padId, 0, 10)
	if err == nil && chats != nil {
		assert.Empty(t, *chats, "chat removed")
	}

	exists, err = ds.DS.DoesPadExist(other)
	require.NoError(t, err)
	assert.True(t, *exists, "other pad untouched")
	chats, err = ds.DS.GetChatsOfPad(other, 0, 10)
	require.NoError(t, err)
	require.Len(t, *chats, 1, "chat of the other pad untouched")
}

func testDeleteLegacyInvalidIdPad(t *testing.T, ds testutils.TestDataStore) {
	// Ids like this were accepted by older versions; ":" is no longer valid.
	padId := "legacy:id"
	prefixPad := "legacy"
	require.False(t, ds.PadManager.IsValidPadId(padId))

	a, err := ds.AuthorManager.CreateAuthor(nil)
	require.NoError(t, err)
	text := "x\n"
	_, err = ds.PadManager.GetPad(prefixPad, &text, &a.Id)
	require.NoError(t, err)
	require.NoError(t, ds.DS.SaveChatMessage(prefixPad, 0, &a.Id, time.Now().UnixMilli(), "keep me"))

	require.NoError(t, ds.DS.CreatePad(padId, db2.PadDB{
		ID:        padId,
		Head:      0,
		ChatHead:  0,
		Pool:      db2.RevPool{NumToAttrib: map[string][]string{}, AttribToNum: map[string]int{}},
		ATextText: "old\n",
		CreatedAt: time.Now(),
	}))
	require.NoError(t, ds.DS.SaveChatMessage(padId, 0, &a.Id, time.Now().UnixMilli(), "old chat"))

	// Deleting the pad whose id is a prefix must not take the other's chat.
	other := "legacy2"
	_, err = ds.PadManager.GetPad(other, &text, &a.Id)
	require.NoError(t, err)
	require.NoError(t, ds.DS.CreatePad(other+":x", db2.PadDB{
		ID: other + ":x", Pool: db2.RevPool{NumToAttrib: map[string][]string{}, AttribToNum: map[string]int{}},
		ATextText: "old\n", CreatedAt: time.Now(),
	}))
	require.NoError(t, ds.DS.SaveChatMessage(other+":x", 0, &a.Id, time.Now().UnixMilli(), "legacy chat"))
	require.NoError(t, ds.PadMessageHandler.DeletePad(other))
	chats, err := ds.DS.GetChatsOfPad(other+":x", 0, 10)
	require.NoError(t, err)
	require.Len(t, *chats, 1, "chat of %q untouched by deleting %q", other+":x", other)
	require.NoError(t, ds.PadMessageHandler.DeletePad(other+":x"))

	require.NoError(t, ds.PadMessageHandler.DeletePad(padId))

	exists, err := ds.DS.DoesPadExist(padId)
	require.NoError(t, err)
	assert.False(t, *exists)

	exists, err = ds.DS.DoesPadExist(prefixPad)
	require.NoError(t, err)
	assert.True(t, *exists)
	chats, err = ds.DS.GetChatsOfPad(prefixPad, 0, 10)
	require.NoError(t, err)
	require.Len(t, *chats, 1, "chat of pad %q untouched", prefixPad)
}

// punchRevisionHole removes one stored revision, so the pad no longer loads.
func punchRevisionHole(t *testing.T, ds testutils.TestDataStore, padId string, hole int) {
	t.Helper()
	row, err := ds.DS.GetPad(padId)
	require.NoError(t, err)
	revs, err := ds.DS.GetRevisions(padId, 0, row.Head)
	require.NoError(t, err)
	require.NoError(t, ds.DS.RemoveRevisionsOfPad(padId))
	for _, r := range *revs {
		if r.RevNum == hole {
			continue
		}
		pool := db2.RevPool{}
		if r.Pool != nil {
			pool = *r.Pool
		}
		require.NoError(t, ds.DS.SaveRevision(padId, r.RevNum, r.Changeset, r.AText, pool, r.AuthorId, r.Timestamp))
	}
	ds.PadManager.UnloadPad(padId)
}

// adminRequest sends one admin event and returns the reply.
func adminRequest(t *testing.T, ds testutils.TestDataStore, event string, payload any) (string, json.RawMessage) {
	t.Helper()
	client := &libws.Client{Hub: libws.NewHub(), Conn: ds.MockWebSocket, Send: make(chan []byte, 256), SessionId: "admin"}
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	before := len(ds.MockWebSocket.Data)
	pump := startMockWritePump(client, ds.MockWebSocket)
	s := settings.Displayed
	ds.AdminMessageHandler.HandleMessage(admin.EventMessage{Event: event, Data: data}, &s, client)
	pump.Wait()
	require.Len(t, ds.MockWebSocket.Data, before+1, "%s must get exactly one reply", event)
	var resp []json.RawMessage
	require.NoError(t, json.Unmarshal(ds.MockWebSocket.Data[before].Data, &resp))
	require.Len(t, resp, 2)
	var name string
	require.NoError(t, json.Unmarshal(resp[0], &name))
	return name, resp[1]
}

func listedPads(t *testing.T, ds testutils.TestDataStore, pattern, sortBy string) []string {
	t.Helper()
	name, raw := adminRequest(t, ds, "padLoad", admin.PadLoadData{Limit: 12, Pattern: pattern, SortBy: sortBy})
	require.Equal(t, "results:padLoad", name)
	var res admin.PadDefinition
	require.NoError(t, json.Unmarshal(raw, &res))
	require.Empty(t, res.Error)
	var names []string
	for _, r := range res.Results {
		names = append(names, r.PadName)
	}
	return names
}

func testAdminListsAndDeletesUnloadablePad(t *testing.T, ds testutils.TestDataStore) {
	good := "resilience-good"
	corrupt := "resilience-corrupt"
	a, err := ds.AuthorManager.CreateAuthor(nil)
	require.NoError(t, err)
	text := "good content\n"
	_, err = ds.PadManager.GetPad(good, &text, &a.Id)
	require.NoError(t, err)
	p, err := ds.PadManager.GetPad(corrupt, &text, &a.Id)
	require.NoError(t, err)
	for i := 1; i <= 3; i++ {
		require.NoError(t, p.SetText(p.Text()+fmt.Sprintf("line %d\n", i), &a.Id))
	}
	punchRevisionHole(t, ds, corrupt, 2)
	_, err = ds.PadManager.GetPad(corrupt, nil, nil)
	require.Error(t, err, "the setup must produce a pad that does not load")

	for _, sortBy := range []string{"lastEdited", "padName"} {
		names := listedPads(t, ds, "resilience", sortBy)
		assert.Contains(t, names, good, "sort %s", sortBy)
		assert.Contains(t, names, corrupt, "the unloadable pad is listed so it can be deleted (sort %s)", sortBy)
	}

	name, raw := adminRequest(t, ds, "deletePad", corrupt)
	assert.Equal(t, "results:deletePad", name)
	var ack string
	require.NoError(t, json.Unmarshal(raw, &ack))
	assert.Equal(t, corrupt, ack)

	names := listedPads(t, ds, "resilience", "padName")
	assert.NotContains(t, names, corrupt)
	assert.Contains(t, names, good)
}

func testAdminPadLoadAlwaysReplies(t *testing.T, ds testutils.TestDataStore) {
	name, raw := adminRequest(t, ds, "padLoad", "not an object")
	assert.Equal(t, "results:padLoad", name)
	var res admin.PadDefinition
	require.NoError(t, json.Unmarshal(raw, &res))
	assert.NotEmpty(t, res.Error)
	assert.Empty(t, res.Results)
}

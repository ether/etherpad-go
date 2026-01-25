package db

import (
	"reflect"
	"sort"

	"testing"

	"github.com/ether/etherpad-go/lib/apool"
	author2 "github.com/ether/etherpad-go/lib/author"
	"github.com/ether/etherpad-go/lib/db"
	modeldb "github.com/ether/etherpad-go/lib/models/db"
	sessionmodel "github.com/ether/etherpad-go/lib/models/session"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/assert"
)

func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

func TestAllDataStores(t *testing.T) {
	testHandler := testutils.NewTestDBHandler(t)
	runAllDataStoreTests(testHandler)
}

func runAllDataStoreTests(testHandler *testutils.TestDBHandler) {

	defer testHandler.StartTestDBHandler()
	testHandler.AddTests(
		testutils.TestRunConfig{
			Name: "CreateGetRemovePadAndIds",
			Test: testCreateGetRemovePadAndIds,
		},
		testutils.TestRunConfig{
			Name: "GetAuthors",
			Test: testGetAuthors,
		},
		testutils.TestRunConfig{
			Name: "GetRevisionOnNonexistentPad",
			Test: testGetRevisionOnNonexistentPad,
		},
		testutils.TestRunConfig{
			Name: "GetRevisionsOnNonexistentPad",
			Test: testGetRevisionsOnNonexistentPad,
		},
		testutils.TestRunConfig{
			Name: "SaveRevisionsOnNonexistentPad",
			Test: testSaveRevisionsOnNonexistentPad,
		},
		testutils.TestRunConfig{
			Name: "RemoveChatOnNonExistingPad",
			Test: testRemoveChatOnNonExistingPad,
		},
		testutils.TestRunConfig{
			Name: "RemoveChatOnExistingPadWithNoChatMessage",
			Test: testRemoveChatOnExistingPadWithNoChatMessage,
		},
		testutils.TestRunConfig{
			Name: "RemoveChatOnExistingPadWithOneChatMessage",
			Test: testRemoveChatOnExistingPadWithOneChatMessage,
		},
		testutils.TestRunConfig{
			Name: "GetChatAuthorIdsChatOnExistingPadWithOneChatMessage",
			Test: testGetChatAuthorIdsChatOnExistingPadWithOneChatMessage,
		},
		testutils.TestRunConfig{
			Name: "RemoveNonExistingSession",
			Test: testRemoveNonExistingSession,
		},
		testutils.TestRunConfig{
			Name: "GetRevisionOnNonExistingRevision",
			Test: testGetRevisionOnNonExistingRevision,
		},
		testutils.TestRunConfig{
			Name: "SaveAndGetRevisionAndMetaData",
			Test: testSaveAndGetRevisionAndMetaData,
		},
		testutils.TestRunConfig{
			Name: "GetPadMetadataOnNonExistingPad",
			Test: testGetPadMetadataOnNonExistingPad,
		},
		testutils.TestRunConfig{
			Name: "GetPadMetadataOnNonExistingPadRevision",
			Test: testGetPadMetadataOnNonExistingPadRevision,
		},
		testutils.TestRunConfig{
			Name: "GetPadOnNonExistingPad",
			Test: testGetPadOnNonExistingPad,
		},
		testutils.TestRunConfig{
			Name: "testGetReadonlyPadOnNonExistingPad",
			Test: testGetReadonlyPadOnNonExistingPad,
		},
		testutils.TestRunConfig{
			Name: "testGetReadonlyPadWhenPadDoesNotHaveReadonlyIdOnNonExistingPad",
			Test: testGetReadonlyPadWhenPadDoesNotHaveReadonlyIdOnNonExistingPad,
		},
		testutils.TestRunConfig{
			Name: "GetAuthorOnNonExistingAuthor",
			Test: testGetAuthorOnNonExistingAuthor,
		},
		testutils.TestRunConfig{
			Name: "GetAuthorByTokenOnNonExistingToken",
			Test: testGetAuthorByTokenOnNonExistingToken,
		},
		testutils.TestRunConfig{
			Name: "SaveAuthorNameOnNonExistingAuthor",
			Test: testSaveAuthorNameOnNonExistingAuthor,
		},
		testutils.TestRunConfig{
			Name: "SaveAuthorColorOnNonExistingAuthor",
			Test: testSaveAuthorColorOnNonExistingAuthor,
		},
		testutils.TestRunConfig{
			Name: "QueryPadSortingAndPattern",
			Test: testQueryPadSortingAndPattern,
		},
		testutils.TestRunConfig{
			Name: "SaveChatHeadOfPadOnNonExistentPad",
			Test: testSaveChatHeadOfPadOnNonExistentPad,
		},
		testutils.TestRunConfig{
			Name: "RemovePad2ReadOnly",
			Test: testRemovePad2ReadOnly,
		},
		testutils.TestRunConfig{
			Name: "GetGroupNonExistingGroup",
			Test: testGetGroupNonExistingGroup,
		},
		testutils.TestRunConfig{
			Name: "GetGroupOnExistingGroup",
			Test: testGetGroupOnExistingGroup,
		},
		testutils.TestRunConfig{
			Name: "SaveAndRemoveGroup",
			Test: testSaveAndRemoveGroup,
		},
		testutils.TestRunConfig{
			Name: "ChatSaveGetAndHead",
			Test: testChatSaveGetAndHead,
		},
		testutils.TestRunConfig{
			Name: "SessionsTokensAuthors",
			Test: testSessionsTokensAuthors,
		},
		testutils.TestRunConfig{
			Name: "RemoveRevisionsOfPadNonExistingPad",
			Test: testRemoveRevisionsOfPadNonExistingPad,
		},
		testutils.TestRunConfig{
			Name: "ReadonlyMappingsAndRemoveRevisions",
			Test: testReadonlyMappingsAndRemoveRevisions,
		},
	)
}

func testGetAuthors(t *testing.T, ds testutils.TestDataStore) {
	author1 := author2.NewRandomAuthor()
	author3 := author2.NewRandomAuthor()
	assert.NoError(t, ds.DS.SaveAuthor(*author2.ToDBAuthor(author1)))

	assert.NoError(t, ds.DS.SaveAuthor(*author2.ToDBAuthor(author3)))
	authorsToFind := []string{author1.Id, author3.Id}
	authorList, err := ds.DS.GetAuthors(authorsToFind)
	if err != nil {
		t.Fatalf("GetAuthors returned error: %v", err)
	}
	if len(*authorList) != 2 {
		t.Fatalf("GetAuthors returned unexpected number of authors: %d", len(*authorList))
	}
	var foundAuthor1, foundAuthor3 = false, false

	for _, dbAuthor := range *authorList {
		if dbAuthor.ID == author1.Id {
			assert.Equal(t, *author1, author2.MapFromDB(dbAuthor))
			foundAuthor1 = true
		}
		if dbAuthor.ID == author3.Id {
			assert.Equal(t, *author3, author2.MapFromDB(dbAuthor))
			foundAuthor3 = true
		}
	}

	assert.True(t, foundAuthor1)
	assert.True(t, foundAuthor3)
}

func testCreateGetRemovePadAndIds(t *testing.T, testDSStore testutils.TestDataStore) {

	pad := db.CreateRandomPad()
	pad2 := db.CreateRandomPad()
	err := testDSStore.DS.CreatePad("padA", pad)
	if err != nil {
		t.Fatalf("CreatePad for padA returned error: %v", err)
	}
	padExists, err := testDSStore.DS.DoesPadExist("padA")
	if err != nil {
		t.Fatalf("DoesPadExist returned error: %v", err)
	}
	if !*padExists {
		t.Fatalf("padA should exist after CreatePad")
	}

	gotPad, err := testDSStore.DS.GetPad("padA")
	if err != nil {
		t.Fatalf("GetPad failed: %v", err)
	}
	if gotPad.Head != 0 {
		t.Fatalf("unexpected RevNum, got %d", gotPad.Head)
	}

	err = testDSStore.DS.CreatePad("padB", pad2)
	if err != nil {
		t.Fatalf("CreatePad for padB returned error: %v", err)
	}
	ids, err := testDSStore.DS.GetPadIds()
	if err != nil {
		t.Fatalf("GetPadIds returned error: %v", err)
	}
	if !containsString(*ids, "padA") || !containsString(*ids, "padB") {
		t.Fatalf("GetPadIds missing pads: %v", ids)
	}

	if err := testDSStore.DS.RemovePad("padA"); err != nil {
		t.Fatalf("RemovePad returned error: %v", err)
	}
	doesAPadExists, err := testDSStore.DS.DoesPadExist("padA")
	if err != nil {
		t.Fatalf("DoesPadExist returned error: %v", err)
	}
	if *doesAPadExists {
		t.Fatalf("padA should not exist after RemovePad")
	}
}

func testGetRevisionOnNonexistentPad(t *testing.T, testStore testutils.TestDataStore) {
	_, err := testStore.DS.GetRevision("nonexistentPad", 0)
	if err == nil {
		t.Fatalf("should return error for nonexistent pad")
	}
}

func testGetRevisionsOnNonexistentPad(t *testing.T, ds testutils.TestDataStore) {
	_, err := ds.DS.GetRevisions("nonexistentPad", 0, 100)
	if err == nil || err.Error() != "pad not found" {
		t.Fatalf("should return error for nonexistent pad")
	}
}

func testSaveRevisionsOnNonexistentPad(t *testing.T, ds testutils.TestDataStore) {
	err := ds.DS.SaveRevision("nonexistentPad", 0, "test", modeldb.AText{}, modeldb.RevPool{}, nil, 1234)
	if err == nil || err.Error() != "pad not found" {
		t.Fatalf("should return error for nonexistent pad")
	}
}

func testRemoveChatOnNonExistingPad(t *testing.T, ds testutils.TestDataStore) {
	err := ds.DS.RemoveChat("nonexistentPad")
	if err != nil {
		t.Fatalf("RemoveChat should not return error for nonexistent pad")
	}
}

func testRemoveChatOnExistingPadWithNoChatMessage(t *testing.T, ds testutils.TestDataStore) {
	randomPad := db.CreateRandomPad()
	err := ds.DS.CreatePad("existentPad", randomPad)
	if err != nil {
		t.Fatalf("CreatePad should not return error for nonexistent pad")
	}

	err = ds.DS.RemoveChat("existentPad")
	if err != nil {
		t.Fatalf("RemoveChat should not return error for nonexistent pad")
	}
}

func testRemoveChatOnExistingPadWithOneChatMessage(t *testing.T, ds testutils.TestDataStore) {
	randomPad := db.CreateRandomPad()
	err := ds.DS.CreatePad("existentPad", randomPad)
	if err != nil {
		t.Fatalf("RemoveChat should not return error for nonexistent pad")
	}
	if err := ds.DS.SaveChatMessage("existentPad", 0, nil, 1234, "hello"); err != nil {
		t.Fatalf("SaveChatMessage failed: %v", err)
	}

	err = ds.DS.RemoveChat("existentPad")
	if err != nil {
		t.Fatalf("RemoveChat should not return error for existent pad: %v", err)
	}
	res, err := ds.DS.GetChatsOfPad("existentPad", 0, 0)
	if err != nil {
		t.Fatalf("GetChatsOfPad failed: %v", err)
	}
	if len(*res) != 0 {
		t.Fatalf("expected 0 chat messages after RemoveChat, got %d", len(*res))
	}
}

func testGetChatAuthorIdsChatOnExistingPadWithOneChatMessage(t *testing.T, ds testutils.TestDataStore) {
	randomPad := db.CreateRandomPad()
	err := ds.DS.CreatePad("existentPad", randomPad)
	if err != nil {
		t.Fatalf("RemoveChat should not return error for nonexistent pad")
	}
	randomAuthor := author2.NewRandomAuthor()
	err = ds.DS.SaveAuthor(*author2.ToDBAuthor(randomAuthor))
	if err := ds.DS.SaveChatMessage("existentPad", 0, &randomAuthor.Id, 1234, "hello"); err != nil {
		t.Fatalf("SaveChatMessage failed: %v", err)
	}

	resultingAuthorIds, err := ds.DS.GetAuthorIdsOfPadChats("existentPad")
	if err != nil {
		t.Fatalf("GetAuthorIdsOfPadChats failed: %v", err)
	}
	if len(*resultingAuthorIds) != 1 || (*resultingAuthorIds)[0] != randomAuthor.Id {
		t.Fatalf("GetAuthorIdsOfPadChats returned unexpected result: %#v", resultingAuthorIds)
	}
}

func testRemoveNonExistingSession(t *testing.T, ds testutils.TestDataStore) {
	err := ds.DS.RemoveSessionById("nonexistentSession")
	if err == nil {
		t.Fatalf("RemoveSessionById should return error for nonexistent session")
	}
}

func testGetRevisionOnNonExistingRevision(t *testing.T, ds testutils.TestDataStore) {
	err := ds.DS.CreatePad("padA", db.CreateRandomPad())
	if err != nil {
		t.Fatalf("CreatePad failed: %v", err)
	}
	_, err = ds.DS.GetRevision("padA", 5)
	if err == nil || err.Error() != db.PadRevisionNotFoundError {
		t.Fatalf("should return error for nonexistent revision")
	}
}

func testSaveAndGetRevisionAndMetaData(t *testing.T, ds testutils.TestDataStore) {
	text := apool.AText{}
	numToAttribPool := make(map[int]apool.Attribute)
	numToAttribPool[0] = apool.Attribute{Key: "bold", Value: "true"}
	attribToNum := make(map[apool.Attribute]int)

	pool := apool.APool{
		NextNum:     1,
		NumToAttrib: numToAttribPool,
		AttribToNum: attribToNum,
	}
	randomAuthor := author2.NewRandomAuthor()

	err := ds.DS.SaveAuthor(*author2.ToDBAuthor(randomAuthor))
	if err != nil {
		t.Fatalf("SaveAuthor failed: %v", err)
	}

	pad := modeldb.PadDB{
		Head:           -1,
		SavedRevisions: make([]modeldb.SavedRevision, 0),
	}
	if err := ds.DS.CreatePad("pad1", pad); err != nil {
		t.Fatalf("CreatePad failed: %v", err)
	}

	if err := ds.DS.SaveRevision("pad1", 0, "changeset0", text.ToDBAText(), pool.ToRevDB(), &randomAuthor.Id, 12345); err != nil {
		t.Fatalf("SaveRevision failed: %v", err)
	}

	rev, err := ds.DS.GetRevision("pad1", 0)
	if err != nil {
		t.Fatalf("GetRevision failed: %v", err)
	}
	if rev.Changeset != "changeset0" || rev.Timestamp != 12345 {
		t.Fatalf("revision data mismatch: %#v", rev)
	}
	if !reflect.DeepEqual(rev.AText, text.ToDBAText()) {
		t.Fatalf("AText mismatch")
	}

	list, err := ds.DS.GetRevisions("pad1", 0, 0)
	if err != nil {
		t.Fatalf("GetRevisions failed: %v", err)
	}
	if len(*list) != 1 || (*list)[0].RevNum != 0 {
		t.Fatalf("GetRevisions returned unexpected: %#v", list)
	}

	meta, err := ds.DS.GetPadMetaData("pad1", 0)
	if err != nil {
		t.Fatalf("GetPadMetaData failed: %v", err)
	}
	if meta.Timestamp != 12345 {
		t.Fatalf("GetPadMetaData Timestamp mismatch")
	}

	if len(meta.PadPool.NumToAttrib) != 1 || meta.PadPool.NextNum != 1 {
		t.Fatalf("GetPadMetaData PadPool mismatch")
	}

	if meta.AuthorId == nil || *meta.AuthorId != randomAuthor.Id {
		t.Fatalf("GetPadMetaData SavedBy mismatch")
	}
}

func testGetPadMetadataOnNonExistingPad(t *testing.T, ds testutils.TestDataStore) {
	_, err := ds.DS.GetPadMetaData("nonexistentPad", 0)
	if err == nil || err.Error() != "pad not found" {
		t.Fatalf("should return error for nonexistent pad")
	}
}

func testGetPadMetadataOnNonExistingPadRevision(t *testing.T, ds testutils.TestDataStore) {
	err := ds.DS.CreatePad("nonexistentPad", db.CreateRandomPad())
	if err != nil {
		t.Fatalf("CreatePad failed: %v", err)
	}
	_, err = ds.DS.GetPadMetaData("nonexistentPad", 23)
	if err == nil || err.Error() != db.PadRevisionNotFoundError {
		t.Fatalf("should return error when pad revision does not exist")
	}
}

func testGetPadOnNonExistingPad(t *testing.T, ds testutils.TestDataStore) {
	pad, err := ds.DS.GetPad("nonexistentPad")
	if pad != nil && err == nil || err.Error() != db.PadDoesNotExistError {
		t.Fatalf("should return error for nonexistent pad")
	}
}

func testGetReadonlyPadOnNonExistingPad(t *testing.T, ds testutils.TestDataStore) {
	_, err := ds.DS.GetReadonlyPad("nonexistentPad")
	if err == nil || err.Error() != db.PadDoesNotExistError {
		t.Fatalf("should return error for nonexistent readonly pad mapping")
	}
}

func testGetReadonlyPadWhenPadDoesNotHaveReadonlyIdOnNonExistingPad(t *testing.T, ds testutils.TestDataStore) {
	randomPad := db.CreateRandomPad()
	randomPad.ReadOnlyId = nil
	assert.NoError(t, ds.DS.CreatePad(randomPad.ID, randomPad))
	_, err := ds.DS.GetReadonlyPad(randomPad.ID)
	if err == nil || err.Error() != db.PadReadOnlyIdNotFoundError {
		t.Fatalf("should return error for nonexistent readonly pad mapping")
	}
}

func testGetAuthorOnNonExistingAuthor(t *testing.T, ds testutils.TestDataStore) {
	_, err := ds.DS.GetAuthor("nonexistentAuthor")
	if err == nil || err.Error() != db.AuthorNotFoundError {
		t.Fatalf("should return error for nonexistent author")
	}
}

func testGetAuthorByTokenOnNonExistingToken(t *testing.T, ds testutils.TestDataStore) {
	_, err := ds.DS.GetAuthorByToken("nonexistentToken")
	if err == nil || err.Error() != db.AuthorNotFoundError {
		t.Fatalf("should return error for nonexistent token")
	}
}

func testSaveAuthorNameOnNonExistingAuthor(t *testing.T, ds testutils.TestDataStore) {
	err := ds.DS.SaveAuthorName("nonexistentAuthor", "NewName")
	if err == nil || err.Error() != "author not found" {
		t.Fatalf("should return error for nonexistent author")
	}
}

func testSaveAuthorColorOnNonExistingAuthor(t *testing.T, ds testutils.TestDataStore) {
	err := ds.DS.SaveAuthorColor("nonexistentAuthor", "NewName")
	if err == nil || err.Error() != "author not found" {
		t.Fatalf("should return error for nonexistent author")
	}
}

func testQueryPadSortingAndPattern(t *testing.T, ds testutils.TestDataStore) {

	makePad := func(name string, rev int, ts int64) {
		text := apool.AText{}
		pool := apool.APool{}
		randomAuthor := author2.NewRandomAuthor()
		dbAuthor := author2.ToDBAuthor(randomAuthor)
		dbAuthor.ID = name + "_a"

		savedAuthor, err := ds.AuthorManager.CreateAuthor(&dbAuthor.ID)

		if err != nil {
			t.Fatalf("CreateAuthor failed: %v", err)
		}
		p := modeldb.PadDB{
			Head: rev,
		}
		if err := ds.DS.CreatePad(name, p); err != nil {
			t.Fatalf("CreatePad failed: %v", err)
		}
		if err := ds.DS.SaveRevision(name, rev, "changeset", text.ToDBAText(), pool.ToRevDB(), &savedAuthor.Id, ts); err != nil {
			t.Fatalf("SaveRevision failed: %v", err)
		}
	}

	makePad("bPad", 1, 20)
	makePad("aPad", 2, 30)
	makePad("cPad", 3, 10)

	res, err := ds.DS.QueryPad(0, 2, "padName", true, "")
	if err != nil {
		t.Fatalf("QueryPad failed: %v", err)
	}
	if res.TotalPads < 3 || len(res.Pads) != 2 {
		t.Fatalf("QueryPad unexpected result: %#v", res)
	}
	names := []string{res.Pads[0].Padname, res.Pads[1].Padname}
	expected := []string{"aPad", "bPad"}
	if !reflect.DeepEqual(names, expected) {
		t.Fatalf("expected %v got %v", expected, names)
	}

	res2, err := ds.DS.QueryPad(0, 10, "", true, "bPad")
	if err != nil {
		t.Fatalf("QueryPad with pattern failed: %v", err)
	}
	if res2.TotalPads != 1 || res2.Pads[0].Padname != "bPad" {
		t.Fatalf("pattern filtering failed: %#v", res2)
	}

	res3, err := ds.DS.QueryPad(0, 3, "padName", false, "")
	if err != nil {
		t.Fatalf("QueryPad desc failed: %v", err)
	}

	var gotNames []string
	for _, p := range res3.Pads {
		gotNames = append(gotNames, p.Padname)
	}
	sorted := append([]string(nil), gotNames...)
	sort.Sort(sort.Reverse(sort.StringSlice(sorted)))
	if !reflect.DeepEqual(sorted, gotNames) {
		t.Fatalf("expected descending order, got %v", gotNames)
	}
}

func testSaveChatHeadOfPadOnNonExistentPad(t *testing.T, ds testutils.TestDataStore) {
	err := ds.DS.SaveChatHeadOfPad("nonexistentPad", 10)
	if err == nil || err.Error() != "pad not found" {
		t.Fatalf("should return error for nonexistent pad")
	}
}

func testRemovePad2ReadOnly(t *testing.T, ds testutils.TestDataStore) {
	if err := ds.DS.CreatePad("padROTest", db.CreateRandomPad()); err != nil {
		t.Fatalf("CreatePad failed: %v", err)
	}

	err := ds.DS.SetReadOnlyId("padROTest", "ro1")
	if err != nil {
		t.Fatalf("CreatePad2ReadOnly failed: %v", err)
	}
	err = ds.DS.RemovePad("padROTest")
	if err != nil {
		t.Fatalf("RemovePad2ReadOnly failed: %v", err)
	}
	_, err = ds.DS.GetReadonlyPad("padROTest")
	if err == nil {
		t.Fatalf("GetReadonlyPad failed: %v", err)
	}
}

func testGetGroupNonExistingGroup(t *testing.T, ds testutils.TestDataStore) {
	group, err := ds.DS.GetGroup("nonexistentGroup")
	if err == nil || group != nil {
		t.Fatalf("GetGroup should return error and nil for nonexistent group")
	}
}

func testGetGroupOnExistingGroup(t *testing.T, ds testutils.TestDataStore) {

	err := ds.DS.SaveGroup("group1")
	if err != nil {
		t.Fatalf("SaveGroup failed: %v", err)
	}
	group, err := ds.DS.GetGroup("group1")
	if err != nil || group == nil || *group != "group1" {
		t.Fatalf("GetGroup failed: %v %#v", err, group)
	}
}

func testSaveAndRemoveGroup(t *testing.T, ds testutils.TestDataStore) {

	err := ds.DS.SaveGroup("group1")
	if err != nil {
		t.Fatalf("SaveGroup failed: %v", err)
	}
	group, err := ds.DS.GetGroup("group1")
	if err != nil || group == nil || *group != "group1" {
		t.Fatalf("GetGroup failed: %v %#v", err, group)
	}
	err = ds.DS.RemoveGroup("group1")
	if err != nil {
		t.Fatalf("RemoveGroup failed: %v", err)
	}
	group2, err := ds.DS.GetGroup("group1")
	if err == nil || group2 != nil {
		t.Fatalf("GetGroup should return error and nil after removal")
	}
}

func testChatSaveGetAndHead(t *testing.T, ds testutils.TestDataStore) {
	err := ds.DS.CreatePad("padX", db.CreateRandomPad())
	if err != nil {
		t.Fatalf("CreatePad failed: %v", err)
	}
	author := "auth1"
	displayName := "Display"
	if err := ds.DS.SaveAuthor(modeldb.AuthorDB{ID: author, Name: &displayName}); err != nil {
		t.Fatalf("SaveAuthor failed: %v", err)
	}
	foundAuthor, err := ds.DS.GetAuthor(author)
	if err != nil || foundAuthor.ID != author {
		t.Fatalf("GetAuthor failed: %v %#v", err, foundAuthor)
	}

	if err := ds.DS.SaveChatMessage("padX", 0, &author, 1000, "hello"); err != nil {
		t.Fatalf("SaveChatMessage failed: %v", err)
	}
	if err := ds.DS.SaveChatMessage("padX", 1, &author, 1001, "anon"); err != nil {
		t.Fatalf("SaveChatMessage failed: %v", err)
	}

	chats, err := ds.DS.GetChatsOfPad("padX", 0, 1)
	if err != nil {
		t.Fatalf("GetChatsOfPad failed: %v", err)
	}
	if len(*chats) != 2 {
		t.Fatalf("expected 2 chat messages, got %d", len(*chats))
	}
	if (*chats)[0].ChatMessageDB.AuthorId == nil {
		t.Fatalf("expected author id on first chat")
	}
	if (*chats)[0].DisplayName == nil || *(*chats)[0].DisplayName != "Display" {
		t.Fatalf("expected display name 'Display', got %#v", (*chats)[0].DisplayName)
	}

	if err := ds.DS.SaveChatHeadOfPad("padX", 5); err != nil {
		t.Fatalf("SaveChatHeadOfPad failed: %v", err)
	}
	pad, _ := ds.DS.GetPad("padX")
	if pad.ChatHead != 5 {
		t.Fatalf("ChatHead not updated, got %d", pad.ChatHead)
	}
}

func testSessionsTokensAuthors(t *testing.T, ds testutils.TestDataStore) {

	s := sessionmodel.Session{}
	authorId := "authX"
	createdAuthor, err := ds.AuthorManager.CreateAuthor(&authorId)
	if err != nil {
		t.Fatalf("CreateAuthor failed: %v", err)
	}
	if err := ds.DS.SetSessionById("sess1", s); err != nil {
		t.Fatalf("SetSessionById failed: %v", err)
	}
	got, err := ds.DS.GetSessionById("sess1")
	if err != nil {
		t.Fatalf("GetSessionById failed: %v", err)
	}
	if got == nil {
		t.Fatalf("GetSessionById returned nil")
	}
	err = ds.DS.RemoveSessionById("sess1")
	if err != nil {
		t.Fatalf("RemoveSessionById failed: %v", err)
	}

	retrievedSess, err := ds.DS.GetSessionById("sess1")
	if err != nil || retrievedSess != nil {
		t.Fatalf("session should be removed")
	}

	if err := ds.DS.SetAuthorByToken("tok1", createdAuthor.Id); err != nil {
		t.Fatalf("SetAuthorByToken failed: %v", err)
	}
	authorPtr, err := ds.DS.GetAuthorByToken("tok1")
	if err != nil || authorPtr == nil || *authorPtr != createdAuthor.Id {
		t.Fatalf("GetAuthorByToken mismatch: %v %v", err, authorPtr)
	}

	a := modeldb.AuthorDB{ID: "authY", Name: nil, ColorId: ""}
	err = ds.DS.SaveAuthor(a)
	if err != nil {
		t.Fatalf("SaveAuthor failed: %v", err)
	}
	gotA, err := ds.DS.GetAuthor("authY")
	if err != nil || gotA.ID != "authY" {
		t.Fatalf("GetAuthor failed: %v %#v", err, gotA)
	}
	err = ds.DS.SaveAuthorName("authY", "NameY")
	if err != nil {
		t.Fatalf("SaveAuthorName failed: %v", err)
	}
	err = ds.DS.SaveAuthorColor("authY", "blue")
	if err != nil {
		t.Fatalf("SaveAuthorColor failed: %v", err)
	}
	gotA2, _ := ds.DS.GetAuthor("authY")
	if gotA2.Name == nil || *gotA2.Name != "NameY" || gotA2.ColorId != "blue" {
		t.Fatalf("author updates failed: %#v", gotA2)
	}
}

func testRemoveRevisionsOfPadNonExistingPad(t *testing.T, ds testutils.TestDataStore) {
	err := ds.DS.RemoveRevisionsOfPad("nonexistentPad")
	if err == nil || err.Error() != "pad not found" {
		t.Fatalf("should return error for nonexistent pad")
	}
}

func testReadonlyMappingsAndRemoveRevisions(t *testing.T, ds testutils.TestDataStore) {
	padDb := db.CreateRandomPad()

	if err := ds.DS.CreatePad("padR", padDb); err != nil {
		t.Fatalf("CreatePad failed: %v", err)
	}

	if err := ds.DS.SetReadOnlyId("padR", "r1"); err != nil {
		t.Fatalf("CreatePad2ReadOnly failed: %v", err)
	}
	gotRO, err := ds.DS.GetReadonlyPad("padR")
	if err != nil || gotRO == nil || *gotRO != "r1" {
		t.Fatalf("CreatePad2ReadOnly/GetReadonlyPad failed")
	}
	rev, err := ds.DS.GetPadByReadOnlyId("r1")
	if err != nil {
		t.Fatalf("GetReadOnly2Pad failed: %v", err)
	}
	if rev == nil || *rev != "padR" {
		t.Fatalf("CreateReadOnly2Pad/GetReadOnly2Pad failed")
	}

	pad := modeldb.PadDB{
		Head: 2,
	}
	err = ds.DS.CreatePad("padRem", pad)
	if err != nil {
		t.Fatalf("CreatePad failed: %v", err)
	}
	if err := ds.DS.RemoveRevisionsOfPad("padRem"); err != nil {
		t.Fatalf("RemoveRevisionsOfPad failed: %v", err)
	}
	// TODO fix this inconsistency. SQL uses separate revisions table, memory store keeps revisions in pad struct
	/*gotPad, _ := ds.GetPad("padRem")
	if len(gotPad.SavedRevisions) != 0 || gotPad.RevNum != -1 {
		t.Fatalf("RemoveRevisionsOfPad did not clear revisions: %#v", gotPad)
	}*/
}

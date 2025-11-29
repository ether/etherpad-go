package db

import (
	"reflect"
	"sort"
	"testing"

	"github.com/ether/etherpad-go/lib/apool"
	modeldb "github.com/ether/etherpad-go/lib/models/db"
	sessionmodel "github.com/ether/etherpad-go/lib/models/session"
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
	datastores := map[string]func() DataStore{
		"Memory": func() DataStore {
			return NewMemoryDataStore()
		},
		"SQLite": func() DataStore {
			sqliteDB, err := NewSQLiteDB(":memory:")
			if err != nil {
				t.Fatalf("Failed to create SQLite DataStore: %v", err)
			}
			t.Cleanup(func() {
				if err := sqliteDB.Close(); err != nil {
					t.Fatalf("Failed to close SQLite DataStore: %v", err)
				}
			})
			return sqliteDB
		},
		// "Postgres": func() DataStore {
		//     // Postgres-Setup mit Test-Container oder Mock
		//     return setupTestPostgres(t)
		// },
	}

	for name, newDS := range datastores {
		t.Run(name, func(t *testing.T) {
			runAllDataStoreTests(t, newDS())
		})
	}
}

func runAllDataStoreTests(t *testing.T, ds DataStore) {
	t.Run("CreateGetRemovePadAndIds", func(t *testing.T) {
		testCreateGetRemovePadAndIds(t, ds)
	})
	t.Run("GetRevisionOnNonexistentPad", func(t *testing.T) {
		testGetRevisionOnNonexistentPad(t, ds)
	})
	t.Run("testGetRevisionsOnNonexistentPad", func(t *testing.T) {
		testGetRevisionsOnNonexistentPad(t, ds)
	})

}

func testCreateGetRemovePadAndIds(t *testing.T, ds DataStore) {
	if ds == nil {
		t.Fatalf("NewMemoryDataStore returned nil")
	}

	pad := CreateRandomPad()
	ds.CreatePad("padA", pad)
	padExists, err := ds.DoesPadExist("padA")
	if err != nil {
		t.Fatalf("DoesPadExist returned error: %v", err)
	}
	if !*padExists {
		t.Fatalf("padA should exist after CreatePad")
	}

	gotPad, err := ds.GetPad("padA")
	if err != nil {
		t.Fatalf("GetPad failed: %v", err)
	}
	if gotPad.RevNum != 0 {
		t.Fatalf("unexpected RevNum, got %d", gotPad.RevNum)
	}

	ds.CreatePad("padB", pad)
	ids, err := ds.GetPadIds()
	if err != nil {
		t.Fatalf("GetPadIds returned error: %v", err)
	}
	if !containsString(*ids, "padA") || !containsString(*ids, "padB") {
		t.Fatalf("GetPadIds missing pads: %v", ids)
	}

	if err := ds.RemovePad("padA"); err != nil {
		t.Fatalf("RemovePad returned error: %v", err)
	}
	doesAPadExists, err := ds.DoesPadExist("padA")
	if err != nil {
		t.Fatalf("DoesPadExist returned error: %v", err)
	}
	if *doesAPadExists {
		t.Fatalf("padA should not exist after RemovePad")
	}
}

func testGetRevisionOnNonexistentPad(t *testing.T, ds DataStore) {
	_, err := ds.GetRevision("nonexistentPad", 0)
	if err == nil {
		t.Fatalf("should return error for nonexistent pad")
	}
}

func testGetRevisionsOnNonexistentPad(t *testing.T, ds DataStore) {
	_, err := ds.GetRevisions("nonexistentPad", 0, 100)
	if err == nil || err.Error() != "pad not found" {
		t.Fatalf("should return error for nonexistent pad")
	}
}

func TestSaveRevisionsOnNonexistentPad(t *testing.T) {
	ds := NewMemoryDataStore()
	err := ds.SaveRevision("nonexistentPad", 0, "test", apool.AText{}, apool.APool{}, nil, 1234)
	if err == nil || err.Error() != "pad not found" {
		t.Fatalf("should return error for nonexistent pad")
	}
}

func TestGetRevisionsOnExistentPadWithNonExistingRevision(t *testing.T) {
	ds := NewMemoryDataStore()
	ds.CreatePad("padA", CreateRandomPad())
	_, err := ds.GetRevisions("padA", 0, 100)
	if err == nil || err.Error() != "revision of pad not found" {
		t.Fatalf("should return error for nonexistent pad")
	}
}

func TestRemoveChatOnNonExistingPad(t *testing.T) {
	ds := NewMemoryDataStore()
	err := ds.RemoveChat("nonexistentPad")
	if err != nil {
		t.Fatalf("RemoveChat should not return error for nonexistent pad")
	}
}

func TestRemoveChatOnExistingPadWithNoChatMessage(t *testing.T) {
	ds := NewMemoryDataStore()
	randomPad := CreateRandomPad()
	ds.CreatePad("existentPad", randomPad)

	err := ds.RemoveChat("existentPad")
	if err != nil {
		t.Fatalf("RemoveChat should not return error for nonexistent pad")
	}
}

func TestRemoveChatOnExistingPadWithOneChatMessage(t *testing.T) {
	ds := NewMemoryDataStore()
	randomPad := CreateRandomPad()
	ds.CreatePad("existentPad", randomPad)
	if err := ds.SaveChatMessage("existentPad", 0, nil, 1234, "hello"); err != nil {
		t.Fatalf("SaveChatMessage failed: %v", err)
	}

	err := ds.RemoveChat("existentPad")
	if err != nil {
		t.Fatalf("RemoveChat should not return error for existent pad: %v", err)
	}
	res, err := ds.GetChatsOfPad("existentPad", 0, 0)
	if err != nil {
		t.Fatalf("GetChatsOfPad failed: %v", err)
	}
	if len(*res) != 0 {
		t.Fatalf("expected 0 chat messages after RemoveChat, got %d", len(*res))
	}
}

func TestRemoveNonExistingSession(t *testing.T) {
	ds := NewMemoryDataStore()
	removed, err := ds.RemoveSessionById("nonexistentSession")
	if err != nil {
		t.Fatalf("RemoveSessionById should not return error for nonexistent session")
	}
	if removed != nil {
		t.Fatalf("RemoveSessionById should return nil for nonexistent session")
	}
}

func TestGetRevisionOnNonExistingRevision(t *testing.T) {
	ds := NewMemoryDataStore()
	ds.CreatePad("padA", CreateRandomPad())
	_, err := ds.GetRevision("padA", 5)
	if err == nil || err.Error() != "revision of pad not found" {
		t.Fatalf("should return error for nonexistent revision")
	}
}

func TestSaveAndGetRevisionAndMetaData(t *testing.T) {
	ds := NewMemoryDataStore()
	text := apool.AText{}
	pool := apool.APool{}
	author := "author1"

	pad := modeldb.PadDB{
		RevNum:         -1,
		SavedRevisions: make(map[int]modeldb.PadRevision),
	}
	ds.CreatePad("pad1", pad)

	if err := ds.SaveRevision("pad1", 0, "changeset0", text, pool, &author, 12345); err != nil {
		t.Fatalf("SaveRevision failed: %v", err)
	}

	rev, err := ds.GetRevision("pad1", 0)
	if err != nil {
		t.Fatalf("GetRevision failed: %v", err)
	}
	if rev.Changeset != "changeset0" || rev.Timestamp != 12345 {
		t.Fatalf("revision data mismatch: %#v", rev)
	}
	if !reflect.DeepEqual(rev.AText, text) {
		t.Fatalf("AText mismatch")
	}

	list, err := ds.GetRevisions("pad1", 0, 0)
	if err != nil {
		t.Fatalf("GetRevisions failed: %v", err)
	}
	if len(*list) != 1 || (*list)[0].RevNum != 0 {
		t.Fatalf("GetRevisions returned unexpected: %#v", list)
	}

	meta, err := ds.GetPadMetaData("pad1", 0)
	if err != nil {
		t.Fatalf("GetPadMetaData failed: %v", err)
	}
	if meta.Timestamp != 12345 {
		t.Fatalf("GetPadMetaData Timestamp mismatch")
	}
	if meta.AuthorId == nil || *meta.AuthorId != author {
		t.Fatalf("GetPadMetaData AuthorId mismatch")
	}
}

func TestGetPadMetadataOnNonExistingPad(t *testing.T) {
	ds := NewMemoryDataStore()
	_, err := ds.GetPadMetaData("nonexistentPad", 0)
	if err == nil || err.Error() != "pad not found" {
		t.Fatalf("should return error for nonexistent pad")
	}
}

func TestGetPadMetadataOnNonExistingPadRevision(t *testing.T) {
	ds := NewMemoryDataStore()
	ds.CreatePad("nonexistentPad", CreateRandomPad())
	_, err := ds.GetPadMetaData("nonexistentPad", 23)
	if err == nil || err.Error() != "revision not found" {
		t.Fatalf("should return error when pad revision does not exist")
	}
}

func TestGetPadOnNonExistingPad(t *testing.T) {
	ds := NewMemoryDataStore()
	_, err := ds.GetPad("nonexistentPad")
	if err == nil || err.Error() != "pad not found" {
		t.Fatalf("should return error for nonexistent pad")
	}
}

func TestGetReadonlyPadOnNonExistingPad(t *testing.T) {
	ds := NewMemoryDataStore()
	_, err := ds.GetReadonlyPad("nonexistentPad")
	if err == nil || err.Error() != "read only id not found" {
		t.Fatalf("should return error for nonexistent readonly pad mapping")
	}
}

func TestGetAuthorOnNonExistingAuthor(t *testing.T) {
	ds := NewMemoryDataStore()
	_, err := ds.GetAuthor("nonexistentAuthor")
	if err == nil || err.Error() != "Author not found" {
		t.Fatalf("should return error for nonexistent author")
	}
}

func TestGetAuthorByTokenOnNonExistingToken(t *testing.T) {
	ds := NewMemoryDataStore()
	_, err := ds.GetAuthorByToken("nonexistentToken")
	if err == nil || err.Error() != "no author available for token" {
		t.Fatalf("should return error for nonexistent token")
	}
}

func TestSaveAuthorNameOnNonExistingAuthor(t *testing.T) {
	ds := NewMemoryDataStore()
	err := ds.SaveAuthorName("nonexistentAuthor", "NewName")
	if err == nil || err.Error() != "author not found" {
		t.Fatalf("should return error for nonexistent author")
	}
}

func TestSaveAuthorColorOnNonExistingAuthor(t *testing.T) {
	ds := NewMemoryDataStore()
	err := ds.SaveAuthorColor("nonexistentAuthor", "NewName")
	if err == nil || err.Error() != "author not found" {
		t.Fatalf("should return error for nonexistent author")
	}
}

func TestQueryPadSortingAndPattern(t *testing.T) {
	ds := NewMemoryDataStore()

	makePad := func(name string, rev int, ts int64) {
		text := apool.AText{}
		pool := apool.APool{}
		author := name + "_a"
		p := modeldb.PadDB{
			RevNum:         rev,
			SavedRevisions: map[int]modeldb.PadRevision{rev: {Content: "c", PadDBMeta: modeldb.PadDBMeta{AText: &text, Pool: &pool, Author: &author, Timestamp: ts}}},
		}
		ds.CreatePad(name, p)
	}

	makePad("bPad", 1, 20)
	makePad("aPad", 2, 30)
	makePad("cPad", 3, 10)

	res, err := ds.QueryPad(0, 2, "padName", true, "")
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

	res2, err := ds.QueryPad(0, 10, "", true, "bPad")
	if err != nil {
		t.Fatalf("QueryPad with pattern failed: %v", err)
	}
	if res2.TotalPads != 1 || res2.Pads[0].Padname != "bPad" {
		t.Fatalf("pattern filtering failed: %#v", res2)
	}

	res3, err := ds.QueryPad(0, 3, "padName", false, "")
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

func TestSaveChatHeadOfPadOnNonExistentPad(t *testing.T) {
	ds := NewMemoryDataStore()
	err := ds.SaveChatHeadOfPad("nonexistentPad", 10)
	if err == nil || err.Error() != "pad not found" {
		t.Fatalf("should return error for nonexistent pad")
	}
}

func TestRemovePad2ReadOnly(t *testing.T) {
	ds := NewMemoryDataStore()
	ds.CreatePad2ReadOnly("padROTest", "ro1")
	err := ds.RemovePad2ReadOnly("padROTest")
	if err != nil {
		t.Fatalf("RemovePad2ReadOnly failed: %v", err)
	}
	_, err = ds.GetReadonlyPad("padROTest")
	if err == nil {
		t.Fatalf("GetReadonlyPad failed: %v", err)
	}
}

func TestGetGroupNonExistingGroup(t *testing.T) {
	ds := NewMemoryDataStore()
	group, err := ds.GetGroup("nonexistentGroup")
	if err == nil || group != nil {
		t.Fatalf("GetGroup should return error and nil for nonexistent group")
	}
}

func TestGetGroupOnExistingGroup(t *testing.T) {
	ds := NewMemoryDataStore()

	ds.SaveGroup("group1")
	group, err := ds.GetGroup("group1")
	if err != nil || group == nil || *group != "group1" {
		t.Fatalf("GetGroup failed: %v %#v", err, group)
	}
}

func TestSaveAndRemoveGroup(t *testing.T) {
	ds := NewMemoryDataStore()

	ds.SaveGroup("group1")
	group, err := ds.GetGroup("group1")
	if err != nil || group == nil || *group != "group1" {
		t.Fatalf("GetGroup failed: %v %#v", err, group)
	}
	ds.RemoveGroup("group1")
	group2, err := ds.GetGroup("group1")
	if err == nil || group2 != nil {
		t.Fatalf("GetGroup should return error and nil after removal")
	}
}

func TestChatSaveGetAndHead(t *testing.T) {
	ds := NewMemoryDataStore()
	ds.CreatePad("padX", CreateRandomPad())
	author := "auth1"
	ds.SaveAuthor(modeldb.AuthorDB{ID: author})
	ds.authorMapper[author] = "Display"

	if err := ds.SaveChatMessage("padX", 0, &author, 1000, "hello"); err != nil {
		t.Fatalf("SaveChatMessage failed: %v", err)
	}
	if err := ds.SaveChatMessage("padX", 1, nil, 1001, "anon"); err != nil {
		t.Fatalf("SaveChatMessage failed: %v", err)
	}

	chats, err := ds.GetChatsOfPad("padX", 0, 1)
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

	if err := ds.SaveChatHeadOfPad("padX", 5); err != nil {
		t.Fatalf("SaveChatHeadOfPad failed: %v", err)
	}
	pad, _ := ds.GetPad("padX")
	if pad.ChatHead != 5 {
		t.Fatalf("ChatHead not updated, got %d", pad.ChatHead)
	}
}

func TestSessionsTokensAuthors(t *testing.T) {
	ds := NewMemoryDataStore()

	s := sessionmodel.Session{}
	ds.SetSessionById("sess1", s)
	got, err := ds.GetSessionById("sess1")
	if err != nil {
		t.Fatalf("GetSessionById failed: %v", err)
	}
	if got == nil {
		t.Fatalf("GetSessionById returned nil")
	}
	removed, err := ds.RemoveSessionById("sess1")
	if err != nil {
		t.Fatalf("RemoveSessionById failed: %v", err)
	}
	if removed == nil {
		t.Fatalf("RemoveSessionById did not return session")
	}

	retrievedSess, err := ds.GetSessionById("sess1")
	if err == nil || retrievedSess != nil {
		t.Fatalf("session should be removed")
	}

	if err := ds.SetAuthorByToken("tok1", "authX"); err != nil {
		t.Fatalf("SetAuthorByToken failed: %v", err)
	}
	authorPtr, err := ds.GetAuthorByToken("tok1")
	if err != nil || authorPtr == nil || *authorPtr != "authX" {
		t.Fatalf("GetAuthorByToken mismatch: %v %v", err, authorPtr)
	}

	a := modeldb.AuthorDB{ID: "authY", Name: nil, ColorId: ""}
	ds.SaveAuthor(a)
	gotA, err := ds.GetAuthor("authY")
	if err != nil || gotA.ID != "authY" {
		t.Fatalf("GetAuthor failed: %v %#v", err, gotA)
	}
	ds.SaveAuthorName("authY", "NameY")
	ds.SaveAuthorColor("authY", "blue")
	gotA2, _ := ds.GetAuthor("authY")
	if gotA2.Name == nil || *gotA2.Name != "NameY" || gotA2.ColorId != "blue" {
		t.Fatalf("author updates failed: %#v", gotA2)
	}
}

func TestRemoveRevisionsOfPadNonExistingPad(t *testing.T) {
	ds := NewMemoryDataStore()
	err := ds.RemoveRevisionsOfPad("nonexistentPad")
	if err == nil || err.Error() != "pad not found" {
		t.Fatalf("should return error for nonexistent pad")
	}
}

func TestReadonlyMappingsAndRemoveRevisions(t *testing.T) {
	ds := NewMemoryDataStore()
	ds.CreatePad2ReadOnly("padR", "r1")
	gotRO, err := ds.GetReadonlyPad("padR")
	if err != nil || gotRO == nil || *gotRO != "r1" {
		t.Fatalf("CreatePad2ReadOnly/GetReadonlyPad failed")
	}
	ds.CreateReadOnly2Pad("padR", "r1")
	rev, err := ds.GetReadOnly2Pad("r1")
	if err != nil {
		t.Fatalf("GetReadOnly2Pad failed: %v", err)
	}
	if rev == nil || *rev != "padR" {
		t.Fatalf("CreateReadOnly2Pad/GetReadOnly2Pad failed")
	}
	if err := ds.RemoveReadOnly2Pad("r1"); err != nil {
		t.Fatalf("RemoveReadOnly2Pad failed: %v", err)
	}
	readOnlyPad, err := ds.GetReadOnly2Pad("r1")
	if err != nil {
		t.Fatalf("GetReadOnly2Pad after removal failed: %v", err)
	}
	if readOnlyPad != nil {
		t.Fatalf("RemoveReadOnly2Pad did not remove mapping")
	}

	text := apool.AText{}
	pool := apool.APool{}
	author := "a"
	pad := modeldb.PadDB{
		RevNum:         2,
		SavedRevisions: map[int]modeldb.PadRevision{0: {Content: "c", PadDBMeta: modeldb.PadDBMeta{AText: &text, Pool: &pool, Author: &author, Timestamp: 1}}},
	}
	ds.CreatePad("padRem", pad)
	if err := ds.RemoveRevisionsOfPad("padRem"); err != nil {
		t.Fatalf("RemoveRevisionsOfPad failed: %v", err)
	}
	gotPad, _ := ds.GetPad("padRem")
	if len(gotPad.SavedRevisions) != 0 || gotPad.RevNum != -1 {
		t.Fatalf("RemoveRevisionsOfPad did not clear revisions: %#v", gotPad)
	}
}

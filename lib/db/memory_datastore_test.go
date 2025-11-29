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

func TestCreateGetRemovePadAndIds(t *testing.T) {
	m := NewMemoryDataStore()
	if m == nil {
		t.Fatalf("NewMemoryDataStore returned nil")
	}

	pad := CreateRandomPad()
	m.CreatePad("padA", pad)
	if !m.DoesPadExist("padA") {
		t.Fatalf("padA should exist after CreatePad")
	}

	gotPad, err := m.GetPad("padA")
	if err != nil {
		t.Fatalf("GetPad failed: %v", err)
	}
	if gotPad.RevNum != 0 {
		t.Fatalf("unexpected RevNum, got %d", gotPad.RevNum)
	}

	m.CreatePad("padB", pad)
	ids := m.GetPadIds()
	if !containsString(ids, "padA") || !containsString(ids, "padB") {
		t.Fatalf("GetPadIds missing pads: %v", ids)
	}

	if err := m.RemovePad("padA"); err != nil {
		t.Fatalf("RemovePad returned error: %v", err)
	}
	if m.DoesPadExist("padA") {
		t.Fatalf("padA should not exist after RemovePad")
	}
}

func TestGetRevisionOnNonexistentPad(t *testing.T) {
	m := NewMemoryDataStore()
	_, err := m.GetRevision("nonexistentPad", 0)
	if err == nil {
		t.Fatalf("should return error for nonexistent pad")
	}
}

func TestGetRevisionsOnNonexistentPad(t *testing.T) {
	m := NewMemoryDataStore()
	_, err := m.GetRevisions("nonexistentPad", 0, 100)
	if err == nil || err.Error() != "pad not found" {
		t.Fatalf("should return error for nonexistent pad")
	}
}

func TestSaveRevisionsOnNonexistentPad(t *testing.T) {
	m := NewMemoryDataStore()
	err := m.SaveRevision("nonexistentPad", 0, "test", apool.AText{}, apool.APool{}, nil, 1234)
	if err == nil || err.Error() != "pad not found" {
		t.Fatalf("should return error for nonexistent pad")
	}
}

func TestGetRevisionsOnExistentPadWithNonExistingRevision(t *testing.T) {
	m := NewMemoryDataStore()
	m.CreatePad("padA", CreateRandomPad())
	_, err := m.GetRevisions("padA", 0, 100)
	if err == nil || err.Error() != "revision of pad not found" {
		t.Fatalf("should return error for nonexistent pad")
	}
}

func TestRemoveChatOnNonExistingPad(t *testing.T) {
	m := NewMemoryDataStore()
	err := m.RemoveChat("nonexistentPad")
	if err != nil {
		t.Fatalf("RemoveChat should not return error for nonexistent pad")
	}
}

func TestRemoveChatOnExistingPadWithNoChatMessage(t *testing.T) {
	m := NewMemoryDataStore()
	randomPad := CreateRandomPad()
	m.CreatePad("existentPad", randomPad)

	err := m.RemoveChat("existentPad")
	if err != nil {
		t.Fatalf("RemoveChat should not return error for nonexistent pad")
	}
}

func TestRemoveChatOnExistingPadWithOneChatMessage(t *testing.T) {
	m := NewMemoryDataStore()
	randomPad := CreateRandomPad()
	m.CreatePad("existentPad", randomPad)
	if err := m.SaveChatMessage("existentPad", 0, nil, 1234, "hello"); err != nil {
		t.Fatalf("SaveChatMessage failed: %v", err)
	}

	err := m.RemoveChat("existentPad")
	if err != nil {
		t.Fatalf("RemoveChat should not return error for existent pad: %v", err)
	}
	res, err := m.GetChatsOfPad("existentPad", 0, 0)
	if err != nil {
		t.Fatalf("GetChatsOfPad failed: %v", err)
	}
	if len(*res) != 0 {
		t.Fatalf("expected 0 chat messages after RemoveChat, got %d", len(*res))
	}
}

func TestRemoveNonExistingSession(t *testing.T) {
	m := NewMemoryDataStore()
	removed := m.RemoveSessionById("nonexistentSession")
	if removed != nil {
		t.Fatalf("RemoveSessionById should return nil for nonexistent session")
	}
}

func TestGetRevisionOnNonExistingRevision(t *testing.T) {
	m := NewMemoryDataStore()
	m.CreatePad("padA", CreateRandomPad())
	_, err := m.GetRevision("padA", 5)
	if err == nil || err.Error() != "revision of pad not found" {
		t.Fatalf("should return error for nonexistent revision")
	}
}

func TestSaveAndGetRevisionAndMetaData(t *testing.T) {
	m := NewMemoryDataStore()
	text := apool.AText{}
	pool := apool.APool{}
	author := "author1"

	pad := modeldb.PadDB{
		RevNum:         -1,
		SavedRevisions: make(map[int]modeldb.PadRevision),
	}
	m.CreatePad("pad1", pad)

	if err := m.SaveRevision("pad1", 0, "changeset0", text, pool, &author, 12345); err != nil {
		t.Fatalf("SaveRevision failed: %v", err)
	}

	rev, err := m.GetRevision("pad1", 0)
	if err != nil {
		t.Fatalf("GetRevision failed: %v", err)
	}
	if rev.Changeset != "changeset0" || rev.Timestamp != 12345 {
		t.Fatalf("revision data mismatch: %#v", rev)
	}
	if !reflect.DeepEqual(rev.AText, text) {
		t.Fatalf("AText mismatch")
	}

	list, err := m.GetRevisions("pad1", 0, 0)
	if err != nil {
		t.Fatalf("GetRevisions failed: %v", err)
	}
	if len(*list) != 1 || (*list)[0].RevNum != 0 {
		t.Fatalf("GetRevisions returned unexpected: %#v", list)
	}

	meta, err := m.GetPadMetaData("pad1", 0)
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
	m := NewMemoryDataStore()
	_, err := m.GetPadMetaData("nonexistentPad", 0)
	if err == nil || err.Error() != "pad not found" {
		t.Fatalf("should return error for nonexistent pad")
	}
}

func TestGetPadMetadataOnNonExistingPadRevision(t *testing.T) {
	m := NewMemoryDataStore()
	m.CreatePad("nonexistentPad", CreateRandomPad())
	_, err := m.GetPadMetaData("nonexistentPad", 23)
	if err == nil || err.Error() != "revision not found" {
		t.Fatalf("should return error when pad revision does not exist")
	}
}

func TestGetPadOnNonExistingPad(t *testing.T) {
	m := NewMemoryDataStore()
	_, err := m.GetPad("nonexistentPad")
	if err == nil || err.Error() != "pad not found" {
		t.Fatalf("should return error for nonexistent pad")
	}
}

func TestGetReadonlyPadOnNonExistingPad(t *testing.T) {
	m := NewMemoryDataStore()
	_, err := m.GetReadonlyPad("nonexistentPad")
	if err == nil || err.Error() != "read only id not found" {
		t.Fatalf("should return error for nonexistent readonly pad mapping")
	}
}

func TestGetAuthorOnNonExistingAuthor(t *testing.T) {
	m := NewMemoryDataStore()
	_, err := m.GetAuthor("nonexistentAuthor")
	if err == nil || err.Error() != "Author not found" {
		t.Fatalf("should return error for nonexistent author")
	}
}

func TestGetAuthorByTokenOnNonExistingToken(t *testing.T) {
	m := NewMemoryDataStore()
	_, err := m.GetAuthorByToken("nonexistentToken")
	if err == nil || err.Error() != "no author available for token" {
		t.Fatalf("should return error for nonexistent token")
	}
}

func TestSaveAuthorNameOnNonExistingAuthor(t *testing.T) {
	m := NewMemoryDataStore()
	err := m.SaveAuthorName("nonexistentAuthor", "NewName")
	if err == nil || err.Error() != "author not found" {
		t.Fatalf("should return error for nonexistent author")
	}
}

func TestQueryPadSortingAndPattern(t *testing.T) {
	m := NewMemoryDataStore()

	makePad := func(name string, rev int, ts int64) {
		text := apool.AText{}
		pool := apool.APool{}
		author := name + "_a"
		p := modeldb.PadDB{
			RevNum:         rev,
			SavedRevisions: map[int]modeldb.PadRevision{rev: {Content: "c", PadDBMeta: modeldb.PadDBMeta{AText: &text, Pool: &pool, Author: &author, Timestamp: ts}}},
		}
		m.CreatePad(name, p)
	}

	makePad("bPad", 1, 20)
	makePad("aPad", 2, 30)
	makePad("cPad", 3, 10)

	res, err := m.QueryPad(0, 2, "padName", true, "")
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

	res2, err := m.QueryPad(0, 10, "", true, "bPad")
	if err != nil {
		t.Fatalf("QueryPad with pattern failed: %v", err)
	}
	if res2.TotalPads != 1 || res2.Pads[0].Padname != "bPad" {
		t.Fatalf("pattern filtering failed: %#v", res2)
	}

	res3, err := m.QueryPad(0, 3, "padName", false, "")
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
	m := NewMemoryDataStore()
	err := m.SaveChatHeadOfPad("nonexistentPad", 10)
	if err == nil || err.Error() != "pad not found" {
		t.Fatalf("should return error for nonexistent pad")
	}
}

func TestRemovePad2ReadOnly(t *testing.T) {
	m := NewMemoryDataStore()
	m.CreatePad2ReadOnly("padROTest", "ro1")
	err := m.RemovePad2ReadOnly("padROTest")
	if err != nil {
		t.Fatalf("RemovePad2ReadOnly failed: %v", err)
	}
	_, err = m.GetReadonlyPad("padROTest")
	if err == nil {
		t.Fatalf("GetReadonlyPad failed: %v", err)
	}
}

func TestGetGroupNonExistingGroup(t *testing.T) {
	m := NewMemoryDataStore()
	group, err := m.GetGroup("nonexistentGroup")
	if err == nil || group != nil {
		t.Fatalf("GetGroup should return error and nil for nonexistent group")
	}
}

func TestGetGroupOnExistingGroup(t *testing.T) {
	m := NewMemoryDataStore()

	m.SaveGroup("group1")
	group, err := m.GetGroup("group1")
	if err != nil || group == nil || *group != "group1" {
		t.Fatalf("GetGroup failed: %v %#v", err, group)
	}
}

func TestChatSaveGetAndHead(t *testing.T) {
	m := NewMemoryDataStore()
	m.CreatePad("padX", CreateRandomPad())
	author := "auth1"
	m.SaveAuthor(modeldb.AuthorDB{ID: author})
	m.authorMapper[author] = "Display"

	if err := m.SaveChatMessage("padX", 0, &author, 1000, "hello"); err != nil {
		t.Fatalf("SaveChatMessage failed: %v", err)
	}
	if err := m.SaveChatMessage("padX", 1, nil, 1001, "anon"); err != nil {
		t.Fatalf("SaveChatMessage failed: %v", err)
	}

	chats, err := m.GetChatsOfPad("padX", 0, 1)
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

	if err := m.SaveChatHeadOfPad("padX", 5); err != nil {
		t.Fatalf("SaveChatHeadOfPad failed: %v", err)
	}
	pad, _ := m.GetPad("padX")
	if pad.ChatHead != 5 {
		t.Fatalf("ChatHead not updated, got %d", pad.ChatHead)
	}
}

func TestSessionsTokensAuthors(t *testing.T) {
	m := NewMemoryDataStore()

	s := sessionmodel.Session{}
	m.SetSessionById("sess1", s)
	got := m.GetSessionById("sess1")
	if got == nil {
		t.Fatalf("GetSessionById returned nil")
	}
	removed := m.RemoveSessionById("sess1")
	if removed == nil {
		t.Fatalf("RemoveSessionById did not return session")
	}
	if m.GetSessionById("sess1") != nil {
		t.Fatalf("session should be removed")
	}

	if err := m.SetAuthorByToken("tok1", "authX"); err != nil {
		t.Fatalf("SetAuthorByToken failed: %v", err)
	}
	authorPtr, err := m.GetAuthorByToken("tok1")
	if err != nil || authorPtr == nil || *authorPtr != "authX" {
		t.Fatalf("GetAuthorByToken mismatch: %v %v", err, authorPtr)
	}

	a := modeldb.AuthorDB{ID: "authY", Name: nil, ColorId: ""}
	m.SaveAuthor(a)
	gotA, err := m.GetAuthor("authY")
	if err != nil || gotA.ID != "authY" {
		t.Fatalf("GetAuthor failed: %v %#v", err, gotA)
	}
	m.SaveAuthorName("authY", "NameY")
	m.SaveAuthorColor("authY", "blue")
	gotA2, _ := m.GetAuthor("authY")
	if gotA2.Name == nil || *gotA2.Name != "NameY" || gotA2.ColorId != "blue" {
		t.Fatalf("author updates failed: %#v", gotA2)
	}
}

func TestRemoveRevisionsOfPadNonExistingPad(t *testing.T) {
	m := NewMemoryDataStore()
	err := m.RemoveRevisionsOfPad("nonexistentPad")
	if err == nil || err.Error() != "pad not found" {
		t.Fatalf("should return error for nonexistent pad")
	}
}

func TestReadonlyMappingsAndRemoveRevisions(t *testing.T) {
	m := NewMemoryDataStore()
	m.CreatePad2ReadOnly("padR", "r1")
	gotRO, err := m.GetReadonlyPad("padR")
	if err != nil || gotRO == nil || *gotRO != "r1" {
		t.Fatalf("CreatePad2ReadOnly/GetReadonlyPad failed")
	}
	m.CreateReadOnly2Pad("padR", "r1")
	rev := m.GetReadOnly2Pad("r1")
	if rev == nil || *rev != "padR" {
		t.Fatalf("CreateReadOnly2Pad/GetReadOnly2Pad failed")
	}
	if err := m.RemoveReadOnly2Pad("r1"); err != nil {
		t.Fatalf("RemoveReadOnly2Pad failed: %v", err)
	}
	if m.GetReadOnly2Pad("r1") != nil {
		t.Fatalf("RemoveReadOnly2Pad did not remove mapping")
	}

	text := apool.AText{}
	pool := apool.APool{}
	author := "a"
	pad := modeldb.PadDB{
		RevNum:         2,
		SavedRevisions: map[int]modeldb.PadRevision{0: {Content: "c", PadDBMeta: modeldb.PadDBMeta{AText: &text, Pool: &pool, Author: &author, Timestamp: 1}}},
	}
	m.CreatePad("padRem", pad)
	if err := m.RemoveRevisionsOfPad("padRem"); err != nil {
		t.Fatalf("RemoveRevisionsOfPad failed: %v", err)
	}
	gotPad, _ := m.GetPad("padRem")
	if len(gotPad.SavedRevisions) != 0 || gotPad.RevNum != -1 {
		t.Fatalf("RemoveRevisionsOfPad did not clear revisions: %#v", gotPad)
	}
}

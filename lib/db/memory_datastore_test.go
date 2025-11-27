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

	// Create pad
	pad := modeldb.PadDB{
		RevNum:         0,
		SavedRevisions: make(map[int]modeldb.PadRevision),
	}
	m.CreatePad("padA", pad)
	if !m.DoesPadExist("padA") {
		t.Fatalf("padA should exist after CreatePad")
	}

	// GetPad
	gotPad, err := m.GetPad("padA")
	if err != nil {
		t.Fatalf("GetPad failed: %v", err)
	}
	if gotPad.RevNum != 0 {
		t.Fatalf("unexpected RevNum, got %d", gotPad.RevNum)
	}

	// GetPadIds
	m.CreatePad("padB", pad)
	ids := m.GetPadIds()
	if !containsString(ids, "padA") || !containsString(ids, "padB") {
		t.Fatalf("GetPadIds missing pads: %v", ids)
	}

	// RemovePad
	if err := m.RemovePad("padA"); err != nil {
		t.Fatalf("RemovePad returned error: %v", err)
	}
	if m.DoesPadExist("padA") {
		t.Fatalf("padA should not exist after RemovePad")
	}
}

func TestSaveAndGetRevisionAndMetaData(t *testing.T) {
	m := NewMemoryDataStore()
	text := apool.AText{}
	pool := apool.APool{}
	author := "author1"

	// create pad
	pad := modeldb.PadDB{
		RevNum:         -1,
		SavedRevisions: make(map[int]modeldb.PadRevision),
	}
	m.CreatePad("pad1", pad)

	// Save revision via SaveRevision
	if err := m.SaveRevision("pad1", 0, "changeset0", text, pool, &author, 12345); err != nil {
		t.Fatalf("SaveRevision failed: %v", err)
	}

	// GetRevision
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

	// GetRevisions range
	list, err := m.GetRevisions("pad1", 0, 0)
	if err != nil {
		t.Fatalf("GetRevisions failed: %v", err)
	}
	if len(*list) != 1 || (*list)[0].RevNum != 0 {
		t.Fatalf("GetRevisions returned unexpected: %#v", list)
	}

	// GetPadMetaData
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

func TestQueryPadSortingAndPattern(t *testing.T) {
	m := NewMemoryDataStore()
	// helper to create pads with given name and timestamp
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

	// Query sorted by padName ascending
	res, err := m.QueryPad(0, 2, "padName", true, "")
	if err != nil {
		t.Fatalf("QueryPad failed: %v", err)
	}
	if res.TotalPads < 3 || len(res.Pads) != 2 {
		t.Fatalf("QueryPad unexpected result: %#v", res)
	}
	// Names should be alphabetically ordered
	names := []string{res.Pads[0].Padname, res.Pads[1].Padname}
	expected := []string{"aPad", "bPad"}
	if !reflect.DeepEqual(names, expected) {
		t.Fatalf("expected %v got %v", expected, names)
	}

	// Query with pattern
	res2, err := m.QueryPad(0, 10, "", true, "bPad")
	if err != nil {
		t.Fatalf("QueryPad with pattern failed: %v", err)
	}
	if res2.TotalPads != 1 || res2.Pads[0].Padname != "bPad" {
		t.Fatalf("pattern filtering failed: %#v", res2)
	}

	// Descending
	res3, err := m.QueryPad(0, 3, "padName", false, "")
	if err != nil {
		t.Fatalf("QueryPad desc failed: %v", err)
	}
	// check descending order by sorting copy and comparing
	gotNames := []string{}
	for _, p := range res3.Pads {
		gotNames = append(gotNames, p.Padname)
	}
	sorted := append([]string(nil), gotNames...)
	sort.Sort(sort.Reverse(sort.StringSlice(sorted)))
	if !reflect.DeepEqual(sorted, gotNames) {
		t.Fatalf("expected descending order, got %v", gotNames)
	}
}

func TestChatSaveGetAndHead(t *testing.T) {
	m := NewMemoryDataStore()
	author := "auth1"
	m.SaveAuthor(modeldb.AuthorDB{ID: author})
	m.authorMapper[author] = "Display"

	// Save chat messages
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
	// Check display name for first message
	if (*chats)[0].ChatMessageDB.AuthorId == nil {
		t.Fatalf("expected author id on first chat")
	}
	if (*chats)[0].DisplayName == nil || *(*chats)[0].DisplayName != "Display" {
		t.Fatalf("expected display name 'Display', got %#v", (*chats)[0].DisplayName)
	}

	// SaveChatHeadOfPad
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

	// Sessions
	s := sessionmodel.Session{} // zero value is fine for testing map storage
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

	// Token author mapping
	if err := m.SetAuthorByToken("tok1", "authX"); err != nil {
		t.Fatalf("SetAuthorByToken failed: %v", err)
	}
	authorPtr, err := m.GetAuthorByToken("tok1")
	if err != nil || authorPtr == nil || *authorPtr != "authX" {
		t.Fatalf("GetAuthorByToken mismatch: %v %v", err, authorPtr)
	}

	// Author store operations
	a := modeldb.AuthorDB{ID: "authY", Name: nil, ColorId: ""}
	m.SaveAuthor(a)
	gotA, err := m.GetAuthor("authY")
	if err != nil || gotA.ID != "authY" {
		t.Fatalf("GetAuthor failed: %v %#v", err, gotA)
	}
	// Save name/color
	m.SaveAuthorName("authY", "NameY")
	m.SaveAuthorColor("authY", "blue")
	gotA2, _ := m.GetAuthor("authY")
	if gotA2.Name == nil || *gotA2.Name != "NameY" || gotA2.ColorId != "blue" {
		t.Fatalf("author updates failed: %#v", gotA2)
	}
}

func TestReadonlyMappingsAndRemoveRevisions(t *testing.T) {
	m := NewMemoryDataStore()
	// read only mappings
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
	// RemoveReadOnly2Pad
	if err := m.RemoveReadOnly2Pad("r1"); err != nil {
		t.Fatalf("RemoveReadOnly2Pad failed: %v", err)
	}
	if m.GetReadOnly2Pad("r1") != nil {
		t.Fatalf("RemoveReadOnly2Pad did not remove mapping")
	}

	// RemoveRevisionsOfPad
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

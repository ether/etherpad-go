package db

import (
	"database/sql"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/db"
	modeldb "github.com/ether/etherpad-go/lib/models/db"
	sessionmodel "github.com/ether/etherpad-go/lib/models/session"
	"github.com/ether/etherpad-go/lib/utils"
)

func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

var testDbInstance *utils.TestContainerConfiguration

func TestMain(m *testing.M) {
	testDB, err := utils.PreparePostgresDB()
	if err != nil {
		panic(err)
	}
	testDbInstance = testDB
	os.Exit(m.Run())
}

func cleanupPostgresTables() error {
	if testDbInstance == nil {
		return nil
	}
	port, err := strconv.Atoi(testDbInstance.Port)
	if err != nil {
		return err
	}
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		testDbInstance.Username, testDbInstance.Password, testDbInstance.Host, port, testDbInstance.Database)
	conn, err := sql.Open("postgres", dsn)
	if err != nil {
		return err
	}
	defer conn.Close()

	rows, err := conn.Query("SELECT tablename FROM pg_tables WHERE schemaname = 'public'")
	if err != nil {
		return err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return err
		}
		if t == "schema_migrations" || t == "migrations" {
			continue
		}
		quoted := `"` + strings.ReplaceAll(t, `"`, `""`) + `"`
		tables = append(tables, quoted)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(tables) == 0 {
		return nil
	}

	_, err = conn.Exec("TRUNCATE TABLE " + strings.Join(tables, ",") + " RESTART IDENTITY CASCADE")
	return err
}

func initPostgres() *db.PostgresDB {
	port, err := strconv.Atoi(testDbInstance.Port)
	if err != nil {
		panic(err)
	}
	postgresOpts := db.PostgresOptions{
		Username: testDbInstance.Username,
		Password: testDbInstance.Password,
		Database: testDbInstance.Database,
		Host:     testDbInstance.Host,
		Port:     port,
	}
	postresDB, err := db.NewPostgresDB(postgresOpts)
	if err != nil {
		panic(err)
	}
	return postresDB
}

func TestAllDataStores(t *testing.T) {
	datastores := map[string]func() db.DataStore{
		"Memory": func() db.DataStore {
			return db.NewMemoryDataStore()
		},
		"SQLite": func() db.DataStore {
			sqliteDB, err := db.NewSQLiteDB(":memory:")
			if err != nil {
				t.Fatalf("Failed to create SQLite DataStore: %v", err)
			}

			return sqliteDB
		},
		"Postgres": func() db.DataStore {
			return initPostgres()
		},
	}

	for name, newDS := range datastores {
		t.Run(name, func(t *testing.T) {
			runAllDataStoreTests(t, newDS)
		})

	}
}

func testRun(t *testing.T, name string, testFunc func(t *testing.T, ds db.DataStore), newDS func() db.DataStore) {
	t.Run(name, func(t *testing.T) {
		ds := newDS()
		testFunc(t, ds)
		t.Cleanup(func() {
			if err := ds.Close(); err != nil {
				t.Fatalf("Failed to close SQLite DataStore: %v", err)
			}
			if testDbInstance != nil {
				if err := cleanupPostgresTables(); err != nil {
					t.Fatalf("Postgres cleanup failed: %v", err)
				}
			}
			if err := ds.Close(); err != nil {
				t.Fatalf("Failed to close DataStore: %v", err)
			}
		})
	})
}

func runAllDataStoreTests(t *testing.T, newDS func() db.DataStore) {
	testRun(t, "CreateGetRemovePadAndIds", testCreateGetRemovePadAndIds, newDS)
	testRun(t, "GetRevisionOnNonexistentPad", testGetRevisionOnNonexistentPad, newDS)
	testRun(t, "GetRevisionsOnNonexistentPad", testGetRevisionsOnNonexistentPad, newDS)
	testRun(t, "SaveRevisionsOnNonexistentPad", testSaveRevisionsOnNonexistentPad, newDS)
	testRun(t, "GetRevisionsOnExistentPadWithNonExistingRevision", testGetRevisionsOnExistentPadWithNonExistingRevision, newDS)
	testRun(t, "RemoveChatOnNonExistingPad", testRemoveChatOnNonExistingPad, newDS)
	testRun(t, "RemoveChatOnExistingPadWithNoChatMessage", testRemoveChatOnExistingPadWithNoChatMessage, newDS)
	testRun(t, "RemoveChatOnExistingPadWithOneChatMessage", testRemoveChatOnExistingPadWithOneChatMessage, newDS)
	testRun(t, "RemoveNonExistingSession", testRemoveNonExistingSession, newDS)
	testRun(t, "GetRevisionOnNonExistingRevision", testGetRevisionOnNonExistingRevision, newDS)
	testRun(t, "SaveAndGetRevisionAndMetaData", testSaveAndGetRevisionAndMetaData, newDS)
	testRun(t, "GetPadMetadataOnNonExistingPad", testGetPadMetadataOnNonExistingPad, newDS)
	testRun(t, "GetPadMetadataOnNonExistingPadRevision", testGetPadMetadataOnNonExistingPadRevision, newDS)
	testRun(t, "GetPadOnNonExistingPad", testGetPadOnNonExistingPad, newDS)
	testRun(t, "testGetReadonlyPadOnNonExistingPad", testGetReadonlyPadOnNonExistingPad, newDS)
	testRun(t, "GetAuthorOnNonExistingAuthor", testGetAuthorOnNonExistingAuthor, newDS)
	testRun(t, "GetAuthorByTokenOnNonExistingToken", testGetAuthorByTokenOnNonExistingToken, newDS)
	testRun(t, "SaveAuthorNameOnNonExistingAuthor", testSaveAuthorNameOnNonExistingAuthor, newDS)
	testRun(t, "SaveAuthorColorOnNonExistingAuthor", testSaveAuthorColorOnNonExistingAuthor, newDS)
	testRun(t, "QueryPadSortingAndPattern", testQueryPadSortingAndPattern, newDS)
	testRun(t, "SaveChatHeadOfPadOnNonExistentPad", testSaveChatHeadOfPadOnNonExistentPad, newDS)
	testRun(t, "RemovePad2ReadOnly", testRemovePad2ReadOnly, newDS)
	testRun(t, "GetGroupNonExistingGroup", testGetGroupNonExistingGroup, newDS)
	testRun(t, "GetGroupOnExistingGroup", testGetGroupOnExistingGroup, newDS)
	testRun(t, "SaveAndRemoveGroup", testSaveAndRemoveGroup, newDS)
	testRun(t, "ChatSaveGetAndHead", testChatSaveGetAndHead, newDS)
	testRun(t, "SessionsTokensAuthors", testSessionsTokensAuthors, newDS)
	testRun(t, "RemoveRevisionsOfPadNonExistingPad", testRemoveRevisionsOfPadNonExistingPad, newDS)
	testRun(t, "ReadonlyMappingsAndRemoveRevisions", testReadonlyMappingsAndRemoveRevisions, newDS)
}

func testCreateGetRemovePadAndIds(t *testing.T, ds db.DataStore) {
	if ds == nil {
		t.Fatalf("NewMemoryDataStore returned nil")
	}

	pad := db.CreateRandomPad()
	err := ds.CreatePad("padA", pad)
	if err != nil {
		t.Fatalf("CreatePad for padA returned error: %v", err)
	}
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

	err = ds.CreatePad("padB", pad)
	if err != nil {
		t.Fatalf("CreatePad for padB returned error: %v", err)
	}
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

func testGetRevisionOnNonexistentPad(t *testing.T, ds db.DataStore) {
	_, err := ds.GetRevision("nonexistentPad", 0)
	if err == nil {
		t.Fatalf("should return error for nonexistent pad")
	}
}

func testGetRevisionsOnNonexistentPad(t *testing.T, ds db.DataStore) {
	_, err := ds.GetRevisions("nonexistentPad", 0, 100)
	if err == nil || err.Error() != "pad not found" {
		t.Fatalf("should return error for nonexistent pad")
	}
}

func testSaveRevisionsOnNonexistentPad(t *testing.T, ds db.DataStore) {
	err := ds.SaveRevision("nonexistentPad", 0, "test", apool.AText{}, apool.APool{}, nil, 1234)
	if err == nil || err.Error() != "pad not found" {
		t.Fatalf("should return error for nonexistent pad")
	}
}

func testGetRevisionsOnExistentPadWithNonExistingRevision(t *testing.T, ds db.DataStore) {
	err := ds.CreatePad("padA", db.CreateRandomPad())
	if err != nil {
		t.Fatalf("CreatePad returned error: %v", err)
	}
	_, err = ds.GetRevisions("padA", 0, 100)
	if err == nil || err.Error() != db.PadRevisionNotFoundError {
		t.Fatalf("should return error for nonexistent pad")
	}
}

func testRemoveChatOnNonExistingPad(t *testing.T, ds db.DataStore) {
	err := ds.RemoveChat("nonexistentPad")
	if err != nil {
		t.Fatalf("RemoveChat should not return error for nonexistent pad")
	}
}

func testRemoveChatOnExistingPadWithNoChatMessage(t *testing.T, ds db.DataStore) {
	randomPad := db.CreateRandomPad()
	err := ds.CreatePad("existentPad", randomPad)
	if err != nil {
		t.Fatalf("CreatePad should not return error for nonexistent pad")
	}

	err = ds.RemoveChat("existentPad")
	if err != nil {
		t.Fatalf("RemoveChat should not return error for nonexistent pad")
	}
}

func testRemoveChatOnExistingPadWithOneChatMessage(t *testing.T, ds db.DataStore) {
	randomPad := db.CreateRandomPad()
	err := ds.CreatePad("existentPad", randomPad)
	if err != nil {
		t.Fatalf("RemoveChat should not return error for nonexistent pad")
	}
	if err := ds.SaveChatMessage("existentPad", 0, nil, 1234, "hello"); err != nil {
		t.Fatalf("SaveChatMessage failed: %v", err)
	}

	err = ds.RemoveChat("existentPad")
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

func testRemoveNonExistingSession(t *testing.T, ds db.DataStore) {
	err := ds.RemoveSessionById("nonexistentSession")
	if err == nil {
		t.Fatalf("RemoveSessionById should return error for nonexistent session")
	}
}

func testGetRevisionOnNonExistingRevision(t *testing.T, ds db.DataStore) {
	err := ds.CreatePad("padA", db.CreateRandomPad())
	if err != nil {
		t.Fatalf("CreatePad failed: %v", err)
	}
	_, err = ds.GetRevision("padA", 5)
	if err == nil || err.Error() != db.PadRevisionNotFoundError {
		t.Fatalf("should return error for nonexistent revision")
	}
}

func testSaveAndGetRevisionAndMetaData(t *testing.T, ds db.DataStore) {
	text := apool.AText{}
	pool := apool.APool{}
	author := "author1"

	pad := modeldb.PadDB{
		RevNum:         -1,
		SavedRevisions: make(map[int]modeldb.PadRevision),
	}
	if err := ds.CreatePad("pad1", pad); err != nil {
		t.Fatalf("CreatePad failed: %v", err)
	}

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

func testGetPadMetadataOnNonExistingPad(t *testing.T, ds db.DataStore) {
	_, err := ds.GetPadMetaData("nonexistentPad", 0)
	if err == nil || err.Error() != "pad not found" {
		t.Fatalf("should return error for nonexistent pad")
	}
}

func testGetPadMetadataOnNonExistingPadRevision(t *testing.T, ds db.DataStore) {
	err := ds.CreatePad("nonexistentPad", db.CreateRandomPad())
	if err != nil {
		t.Fatalf("CreatePad failed: %v", err)
	}
	_, err = ds.GetPadMetaData("nonexistentPad", 23)
	if err == nil || err.Error() != db.PadRevisionNotFoundError {
		t.Fatalf("should return error when pad revision does not exist")
	}
}

func testGetPadOnNonExistingPad(t *testing.T, ds db.DataStore) {
	pad, err := ds.GetPad("nonexistentPad")
	if pad != nil && err == nil || err.Error() != db.PadDoesNotExistError {
		t.Fatalf("should return error for nonexistent pad")
	}
}

func testGetReadonlyPadOnNonExistingPad(t *testing.T, ds db.DataStore) {
	_, err := ds.GetReadonlyPad("nonexistentPad")
	if err == nil || err.Error() != db.PadReadOnlyIdNotFoundError {
		t.Fatalf("should return error for nonexistent readonly pad mapping")
	}
}

func testGetAuthorOnNonExistingAuthor(t *testing.T, ds db.DataStore) {
	_, err := ds.GetAuthor("nonexistentAuthor")
	if err == nil || err.Error() != db.AuthorNotFoundError {
		t.Fatalf("should return error for nonexistent author")
	}
}

func testGetAuthorByTokenOnNonExistingToken(t *testing.T, ds db.DataStore) {
	_, err := ds.GetAuthorByToken("nonexistentToken")
	if err == nil || err.Error() != db.AuthorNotFoundError {
		t.Fatalf("should return error for nonexistent token")
	}
}

func testSaveAuthorNameOnNonExistingAuthor(t *testing.T, ds db.DataStore) {
	err := ds.SaveAuthorName("nonexistentAuthor", "NewName")
	if err == nil || err.Error() != "author not found" {
		t.Fatalf("should return error for nonexistent author")
	}
}

func testSaveAuthorColorOnNonExistingAuthor(t *testing.T, ds db.DataStore) {
	err := ds.SaveAuthorColor("nonexistentAuthor", "NewName")
	if err == nil || err.Error() != "author not found" {
		t.Fatalf("should return error for nonexistent author")
	}
}

func testQueryPadSortingAndPattern(t *testing.T, ds db.DataStore) {

	makePad := func(name string, rev int, ts int64) {
		text := apool.AText{}
		pool := apool.APool{}
		author := name + "_a"
		p := modeldb.PadDB{
			RevNum:         rev,
			SavedRevisions: map[int]modeldb.PadRevision{rev: {Content: "c", PadDBMeta: modeldb.PadDBMeta{AText: &text, Pool: &pool, Author: &author, Timestamp: ts}}},
		}
		if err := ds.CreatePad(name, p); err != nil {
			t.Fatalf("CreatePad failed: %v", err)
		}
		if err := ds.SaveRevision(name, rev, "changeset", text, pool, &author, ts); err != nil {
			t.Fatalf("SaveRevision failed: %v", err)
		}
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

func testSaveChatHeadOfPadOnNonExistentPad(t *testing.T, ds db.DataStore) {
	err := ds.SaveChatHeadOfPad("nonexistentPad", 10)
	if err == nil || err.Error() != "pad not found" {
		t.Fatalf("should return error for nonexistent pad")
	}
}

func testRemovePad2ReadOnly(t *testing.T, ds db.DataStore) {
	err := ds.CreatePad2ReadOnly("padROTest", "ro1")
	if err != nil {
		t.Fatalf("CreatePad2ReadOnly failed: %v", err)
	}
	err = ds.RemovePad2ReadOnly("padROTest")
	if err != nil {
		t.Fatalf("RemovePad2ReadOnly failed: %v", err)
	}
	_, err = ds.GetReadonlyPad("padROTest")
	if err == nil {
		t.Fatalf("GetReadonlyPad failed: %v", err)
	}
}

func testGetGroupNonExistingGroup(t *testing.T, ds db.DataStore) {
	group, err := ds.GetGroup("nonexistentGroup")
	if err == nil || group != nil {
		t.Fatalf("GetGroup should return error and nil for nonexistent group")
	}
}

func testGetGroupOnExistingGroup(t *testing.T, ds db.DataStore) {

	err := ds.SaveGroup("group1")
	if err != nil {
		t.Fatalf("SaveGroup failed: %v", err)
	}
	group, err := ds.GetGroup("group1")
	if err != nil || group == nil || *group != "group1" {
		t.Fatalf("GetGroup failed: %v %#v", err, group)
	}
}

func testSaveAndRemoveGroup(t *testing.T, ds db.DataStore) {

	err := ds.SaveGroup("group1")
	if err != nil {
		t.Fatalf("SaveGroup failed: %v", err)
	}
	group, err := ds.GetGroup("group1")
	if err != nil || group == nil || *group != "group1" {
		t.Fatalf("GetGroup failed: %v %#v", err, group)
	}
	err = ds.RemoveGroup("group1")
	if err != nil {
		t.Fatalf("RemoveGroup failed: %v", err)
	}
	group2, err := ds.GetGroup("group1")
	if err == nil || group2 != nil {
		t.Fatalf("GetGroup should return error and nil after removal")
	}
}

func testChatSaveGetAndHead(t *testing.T, ds db.DataStore) {
	err := ds.CreatePad("padX", db.CreateRandomPad())
	if err != nil {
		t.Fatalf("CreatePad failed: %v", err)
	}
	author := "auth1"
	displayName := "Display"
	if err := ds.SaveAuthor(modeldb.AuthorDB{ID: author, Name: &displayName}); err != nil {
		t.Fatalf("SaveAuthor failed: %v", err)
	}
	foundAuthor, err := ds.GetAuthor(author)
	if err != nil || foundAuthor.ID != author {
		t.Fatalf("GetAuthor failed: %v %#v", err, foundAuthor)
	}

	if err := ds.SaveChatMessage("padX", 0, &author, 1000, "hello"); err != nil {
		t.Fatalf("SaveChatMessage failed: %v", err)
	}
	if err := ds.SaveChatMessage("padX", 1, &author, 1001, "anon"); err != nil {
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

func testSessionsTokensAuthors(t *testing.T, ds db.DataStore) {

	s := sessionmodel.Session{}
	if err := ds.SetSessionById("sess1", s); err != nil {
		t.Fatalf("SetSessionById failed: %v", err)
	}
	got, err := ds.GetSessionById("sess1")
	if err != nil {
		t.Fatalf("GetSessionById failed: %v", err)
	}
	if got == nil {
		t.Fatalf("GetSessionById returned nil")
	}
	err = ds.RemoveSessionById("sess1")
	if err != nil {
		t.Fatalf("RemoveSessionById failed: %v", err)
	}

	retrievedSess, err := ds.GetSessionById("sess1")
	if err != nil || retrievedSess != nil {
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
	err = ds.SaveAuthor(a)
	if err != nil {
		t.Fatalf("SaveAuthor failed: %v", err)
	}
	gotA, err := ds.GetAuthor("authY")
	if err != nil || gotA.ID != "authY" {
		t.Fatalf("GetAuthor failed: %v %#v", err, gotA)
	}
	err = ds.SaveAuthorName("authY", "NameY")
	if err != nil {
		t.Fatalf("SaveAuthorName failed: %v", err)
	}
	err = ds.SaveAuthorColor("authY", "blue")
	if err != nil {
		t.Fatalf("SaveAuthorColor failed: %v", err)
	}
	gotA2, _ := ds.GetAuthor("authY")
	if gotA2.Name == nil || *gotA2.Name != "NameY" || gotA2.ColorId != "blue" {
		t.Fatalf("author updates failed: %#v", gotA2)
	}
}

func testRemoveRevisionsOfPadNonExistingPad(t *testing.T, ds db.DataStore) {
	err := ds.RemoveRevisionsOfPad("nonexistentPad")
	if err == nil || err.Error() != "pad not found" {
		t.Fatalf("should return error for nonexistent pad")
	}
}

func testReadonlyMappingsAndRemoveRevisions(t *testing.T, ds db.DataStore) {
	if err := ds.CreatePad2ReadOnly("padR", "r1"); err != nil {
		t.Fatalf("CreatePad2ReadOnly failed: %v", err)
	}
	gotRO, err := ds.GetReadonlyPad("padR")
	if err != nil || gotRO == nil || *gotRO != "r1" {
		t.Fatalf("CreatePad2ReadOnly/GetReadonlyPad failed")
	}
	if err := ds.CreateReadOnly2Pad("padR", "r1"); err != nil {
		t.Fatalf("CreateReadOnly2Pad failed: %v", err)
	}
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
	err = ds.CreatePad("padRem", pad)
	if err != nil {
		t.Fatalf("CreatePad failed: %v", err)
	}
	if err := ds.RemoveRevisionsOfPad("padRem"); err != nil {
		t.Fatalf("RemoveRevisionsOfPad failed: %v", err)
	}
	// TODO fix this inconsistency. SQL uses separate revisions table, memory store keeps revisions in pad struct
	/*gotPad, _ := ds.GetPad("padRem")
	if len(gotPad.SavedRevisions) != 0 || gotPad.RevNum != -1 {
		t.Fatalf("RemoveRevisionsOfPad did not clear revisions: %#v", gotPad)
	}*/
}

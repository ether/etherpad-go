package author

import (
	"fmt"
	"testing"
	"time"

	"github.com/ether/etherpad-go/lib/author"
	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Ports of etherpad-lite's anonymizeAuthor and admin/authorSearch specs, on
// every database.
func TestAuthorPrivacy(t *testing.T) {
	h := testutils.NewTestDBHandler(t)
	h.AddTests(
		testutils.TestRunConfig{Name: "erasure zeroes the identity and counts", Test: testErasure},
		testutils.TestRunConfig{Name: "erasure is idempotent", Test: testErasureIdempotent},
		testutils.TestRunConfig{Name: "an unknown author", Test: testErasureUnknown},
		testutils.TestRunConfig{Name: "an interrupted erasure runs again", Test: testErasureInterrupted},
		testutils.TestRunConfig{Name: "dry run changes nothing", Test: testErasureDryRun},
		testutils.TestRunConfig{Name: "lastSeen", Test: testLastSeen},
		testutils.TestRunConfig{Name: "mapper", Test: testMapper},
		testutils.TestRunConfig{Name: "search", Test: testSearch},
		testutils.TestRunConfig{Name: "search cap", Test: testSearchCap},
		testutils.TestRunConfig{Name: "token transfers", Test: testTokenTransferStore},
	)
	h.StartTestDBHandler()
}

func uniq(prefix string) string { return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano()) }

func ptr(s string) *string { return &s }

func testErasure(t *testing.T, ds testutils.TestDataStore) {
	m := ds.AuthorManager
	mapper := uniq("mapper")
	a, err := m.CreateAuthorIfNotExistsFor(mapper, ptr("Alice"))
	require.NoError(t, err)
	// A browser token as well.
	token := "t." + fmt.Sprint(time.Now().UnixNano())
	require.NoError(t, ds.DS.SetAuthorByToken(token, a.Id))
	// A pad and a chat message by Alice.
	padID := uniq("erasurePad")
	_, err = ds.PadManager.GetPad(padID, ptr("hello"), &a.Id)
	require.NoError(t, err)
	require.NoError(t, ds.DS.SaveChatMessage(padID, 0, &a.Id, time.Now().UnixMilli(), "hi"))

	res, err := m.Anonymize(a.Id, false)
	require.NoError(t, err)
	assert.Equal(t, author.ErasureResult{AffectedPads: 1, RemovedTokenMappings: 1, RemovedExternalMappings: 1, ClearedChatMessages: 1}, res)

	rec, err := ds.DS.GetAuthor(a.Id)
	require.NoError(t, err)
	assert.Nil(t, rec.Name)
	assert.Equal(t, "0", rec.ColorId)
	row, err := ds.DS.GetAuthorRow(a.Id)
	require.NoError(t, err)
	assert.True(t, row.Erased)
	assert.NotZero(t, row.ErasedAt)
	assert.Nil(t, row.Mapper)

	_, err = ds.DS.GetAuthorByToken(token)
	assert.Error(t, err, "the token no longer leads to the author")
	id, err := ds.DS.GetAuthorByMapper(mapper)
	require.NoError(t, err)
	assert.Equal(t, "", id, "the mapper no longer leads to the author")
	again, err := m.CreateAuthorIfNotExistsFor(mapper, nil)
	require.NoError(t, err)
	assert.NotEqual(t, a.Id, again.Id, "the same mapper gets a new author")

	n, err := ds.DS.CountChatMessagesOfAuthor(a.Id)
	require.NoError(t, err)
	assert.Equal(t, 0, n)
	pad, err := ds.PadManager.GetPad(padID, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "hello\n", pad.Text(), "the pad text stays")
}

func testErasureIdempotent(t *testing.T, ds testutils.TestDataStore) {
	a, err := ds.AuthorManager.CreateAuthorIfNotExistsFor(uniq("m"), ptr("Carol"))
	require.NoError(t, err)
	_, err = ds.AuthorManager.Anonymize(a.Id, false)
	require.NoError(t, err)
	second, err := ds.AuthorManager.Anonymize(a.Id, false)
	require.NoError(t, err)
	assert.Equal(t, author.ErasureResult{}, second)
	require.NoError(t, ds.AuthorManager.AnonymizeAuthor(a.Id), "the old call still works")
}

func testErasureUnknown(t *testing.T, ds testutils.TestDataStore) {
	for _, dry := range []bool{false, true} {
		res, err := ds.AuthorManager.Anonymize("a.does-not-exist", dry)
		assert.EqualError(t, err, db.AuthorNotFoundError)
		assert.Equal(t, author.ErasureResult{}, res)
	}
}

func testErasureInterrupted(t *testing.T, ds testutils.TestDataStore) {
	mapper := uniq("m")
	a, err := ds.AuthorManager.CreateAuthorIfNotExistsFor(mapper, ptr("Dan"))
	require.NoError(t, err)
	// A run that zeroed the name but stopped before marking the author.
	require.NoError(t, ds.DS.SaveAuthorName(a.Id, ""))
	res, err := ds.AuthorManager.Anonymize(a.Id, false)
	require.NoError(t, err)
	assert.Equal(t, 1, res.RemovedExternalMappings, "the retry still drops the mapper")
	row, _ := ds.DS.GetAuthorRow(a.Id)
	assert.True(t, row.Erased)
}

func testErasureDryRun(t *testing.T, ds testutils.TestDataStore) {
	mapper := uniq("m")
	a, err := ds.AuthorManager.CreateAuthorIfNotExistsFor(mapper, ptr("Eve"))
	require.NoError(t, err)
	preview, err := ds.AuthorManager.Anonymize(a.Id, true)
	require.NoError(t, err)
	assert.Equal(t, 1, preview.RemovedExternalMappings)

	name, err := ds.AuthorManager.GetAuthorName(a.Id)
	require.NoError(t, err)
	assert.Equal(t, "Eve", *name)
	row, _ := ds.DS.GetAuthorRow(a.Id)
	assert.False(t, row.Erased)
	id, _ := ds.DS.GetAuthorByMapper(mapper)
	assert.Equal(t, a.Id, id, "the mapper still leads to the author")
}

func testLastSeen(t *testing.T, ds testutils.TestDataStore) {
	before := time.Now().UnixMilli()
	a, err := ds.AuthorManager.CreateAuthorIfNotExistsFor(uniq("m"), ptr("Dora"))
	require.NoError(t, err)
	created, _ := ds.DS.GetAuthorRow(a.Id)
	assert.GreaterOrEqual(t, created.LastSeen, before)

	time.Sleep(5 * time.Millisecond)
	require.NoError(t, ds.AuthorManager.SetAuthorName(a.Id, "Dora2"))
	renamed, _ := ds.DS.GetAuthorRow(a.Id)
	assert.Greater(t, renamed.LastSeen, created.LastSeen)

	time.Sleep(5 * time.Millisecond)
	require.NoError(t, ds.AuthorManager.SetAuthorColor(a.Id, "12"))
	recolored, _ := ds.DS.GetAuthorRow(a.Id)
	assert.Greater(t, recolored.LastSeen, renamed.LastSeen)
}

func testMapper(t *testing.T, ds testutils.TestDataStore) {
	mapper := uniq("moodle-user")
	a, err := ds.AuthorManager.CreateAuthorIfNotExistsFor(mapper, ptr("Frank"))
	require.NoError(t, err)
	b, err := ds.AuthorManager.CreateAuthorIfNotExistsFor(mapper, nil)
	require.NoError(t, err)
	assert.Equal(t, a.Id, b.Id, "the same mapper is the same author")
	name, _ := ds.AuthorManager.GetAuthorName(a.Id)
	assert.Equal(t, "Frank", *name, "no name keeps the name")

	// A new author's mapper is no token: a browser cannot present it.
	_, err = ds.DS.GetAuthorByToken(mapper)
	assert.Error(t, err)

	// An author an older version made, with the mapper as its token, is found.
	legacyMapper := uniq("legacy")
	legacy, err := ds.AuthorManager.CreateAuthor(nil)
	require.NoError(t, err)
	require.NoError(t, ds.DS.SetAuthorByToken(legacyMapper, legacy.Id))
	found, err := ds.AuthorManager.CreateAuthorIfNotExistsFor(legacyMapper, ptr("Grace"))
	require.NoError(t, err)
	assert.Equal(t, legacy.Id, found.Id)
	id, _ := ds.DS.GetAuthorByMapper(legacyMapper)
	assert.Equal(t, legacy.Id, id, "its mapper is recorded")
}

func testSearch(t *testing.T, ds testutils.TestDataStore) {
	m := ds.AuthorManager
	search := func(pattern, sortBy string, asc, erased bool) author.AuthorSearchResult {
		res, err := m.SearchAuthors(pattern, 0, 12, sortBy, asc, erased)
		require.NoError(t, err)
		return res
	}
	assert.Equal(t, author.AuthorSearchResult{Results: []author.AuthorListEntry{}}, search(uniq("nothing"), "name", true, false))

	tag := uniq("findme")
	_, err := m.CreateAuthorIfNotExistsFor(tag+"-1", ptr("Bob "+tag))
	require.NoError(t, err)
	_, err = m.CreateAuthorIfNotExistsFor(tag+"-2", ptr("alice "+tag))
	require.NoError(t, err)
	res := search(tag, "name", true, false)
	require.Equal(t, 2, res.Total)
	assert.Equal(t, "alice "+tag, *res.Results[0].Name, "case-insensitive order")
	assert.Equal(t, "Bob "+tag, *res.Results[1].Name)
	assert.Equal(t, 2, search(stringsUpper(tag), "name", true, false).Total, "case-insensitive match")

	mtag := uniq("mapper-tag")
	_, err = m.CreateAuthorIfNotExistsFor(mtag+"-x", ptr("Carol"))
	require.NoError(t, err)
	res = search(mtag, "name", true, false)
	require.Equal(t, 1, res.Total, "matched by mapper")
	assert.Equal(t, []string{mtag + "-x"}, res.Results[0].Mapper)

	assert.Equal(t, 0, search("100%", "name", true, false).Total, "% is literal")
	assert.Equal(t, 0, search("a_b"+tag, "name", true, false).Total, "_ is literal")

	etag := uniq("era")
	e, err := m.CreateAuthorIfNotExistsFor(etag, ptr("Erasable"))
	require.NoError(t, err)
	_, err = m.Anonymize(e.Id, false)
	require.NoError(t, err)
	suffix := e.Id[2:10]
	for _, r := range search(suffix, "name", true, false).Results {
		assert.NotEqual(t, e.Id, r.AuthorID, "erased authors are hidden")
	}
	var found *author.AuthorListEntry
	for _, r := range search(suffix, "name", true, true).Results {
		if r.AuthorID == e.Id {
			r := r
			found = &r
		}
	}
	require.NotNil(t, found, "and listed when asked, by id")
	assert.True(t, found.Erased)
	assert.NotNil(t, found.ErasedAt)

	stag := uniq("sort")
	a1, _ := m.CreateAuthorIfNotExistsFor(stag+"-a", ptr("SortA"))
	time.Sleep(10 * time.Millisecond)
	a2, _ := m.CreateAuthorIfNotExistsFor(stag+"-b", ptr("SortB"))
	asc := search(stag, "lastSeen", true, false)
	require.Equal(t, 2, asc.Total)
	assert.Equal(t, []string{a1.Id, a2.Id}, []string{asc.Results[0].AuthorID, asc.Results[1].AuthorID})
	desc := search(stag, "lastSeen", false, false)
	assert.Equal(t, a2.Id, desc.Results[0].AuthorID)

	page, err := m.SearchAuthors(tag, 1, 1, "name", true, false)
	require.NoError(t, err)
	assert.Equal(t, 2, page.Total)
	require.Len(t, page.Results, 1)
	assert.Equal(t, "Bob "+tag, *page.Results[0].Name, "offset and limit")
}

func stringsUpper(s string) string {
	out := []rune(s)
	for i, r := range out {
		if r >= 'a' && r <= 'z' {
			out[i] = r - 32
		}
	}
	return string(out)
}

func testSearchCap(t *testing.T, ds testutils.TestDataStore) {
	if testing.Short() {
		t.Skip("creates 1010 authors")
	}
	tag := uniq("cap")
	for i := 0; i < 1010; i++ {
		_, err := ds.AuthorManager.CreateAuthorIfNotExistsFor(fmt.Sprintf("%s-%05d", tag, i), nil)
		require.NoError(t, err)
	}
	res, err := ds.AuthorManager.SearchAuthors(tag, 0, 12, "name", true, false)
	require.NoError(t, err)
	require.NotNil(t, res.CappedAt)
	assert.Equal(t, 1000, *res.CappedAt)
	assert.Equal(t, 1000, res.Total)
	assert.Len(t, res.Results, 12)
}

func testTokenTransferStore(t *testing.T, ds testutils.TestDataStore) {
	now := time.Now().Truncate(time.Millisecond)
	require.NoError(t, ds.DS.CreateTokenTransfer(db.TokenTransfer{ID: "aa", Token: "t.one", Prefs: `{"x":1}`, CreatedAt: now}))
	require.NoError(t, ds.DS.CreateTokenTransfer(db.TokenTransfer{ID: "old", Token: "t.two", Prefs: "", CreatedAt: now.Add(-time.Hour)}))

	got, err := ds.DS.TakeTokenTransfer("aa")
	require.NoError(t, err)
	assert.Equal(t, "t.one", got.Token)
	assert.Equal(t, `{"x":1}`, got.Prefs)
	assert.True(t, got.CreatedAt.Equal(now))
	_, err = ds.DS.TakeTokenTransfer("aa")
	assert.ErrorIs(t, err, db.ErrTokenTransferNotFound, "once")

	require.NoError(t, ds.DS.RemoveTokenTransfersBefore(now.Add(-time.Minute)))
	_, err = ds.DS.TakeTokenTransfer("old")
	assert.ErrorIs(t, err, db.ErrTokenTransferNotFound, "expired ones are removed")
}

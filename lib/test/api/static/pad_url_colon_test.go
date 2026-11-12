package static

import (
	"net/http/httptest"
	"testing"

	staticapi "github.com/ether/etherpad-go/lib/api/static"
	db2 "github.com/ether/etherpad-go/lib/models/db"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// etherpad-lite: padUrlColonRedirect, padIdColonLegacyPad (GHSA-wg58-mhwv-35pq).
func TestPadURLColon(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(
		testutils.TestRunConfig{Name: "a URL with ':' redirects to the '_' form", Test: testColonRedirect},
		testutils.TestRunConfig{Name: "a URL with whitespace redirects to the '_' form", Test: testWhitespaceRedirect},
		testutils.TestRunConfig{Name: "the timeslider URL redirects relative to its depth", Test: testColonRedirectTimeslider},
		testutils.TestRunConfig{Name: "a pad id with '$' is 404", Test: testDollarIs404},
		testutils.TestRunConfig{Name: "a clean pad id is served without redirect", Test: testCleanPadServed},
		testutils.TestRunConfig{Name: "an existing pad with ':' stays reachable", Test: testLegacyColonPad},
	)
	defer testDb.StartTestDBHandler()
}

func get(t *testing.T, tsStore testutils.TestDataStore, url string) (int, string) {
	t.Helper()
	initStore := tsStore.ToInitStore()
	staticapi.Init(initStore)
	resp, err := initStore.C.Test(httptest.NewRequest("GET", url, nil))
	require.NoError(t, err)
	return resp.StatusCode, resp.Header.Get("Location")
}

func testColonRedirect(t *testing.T, tsStore testutils.TestDataStore) {
	status, location := get(t, tsStore, "/p/foo:bar?lang=de")
	assert.Equal(t, 302, status)
	assert.Equal(t, "foo_bar?lang=de", location)
}

func testWhitespaceRedirect(t *testing.T, tsStore testutils.TestDataStore) {
	status, location := get(t, tsStore, "/p/foo%20bar")
	assert.Equal(t, 302, status)
	assert.Equal(t, "foo_bar", location)
}

func testColonRedirectTimeslider(t *testing.T, tsStore testutils.TestDataStore) {
	status, location := get(t, tsStore, "/p/foo:bar/timeslider")
	assert.Equal(t, 302, status)
	assert.Equal(t, "../foo_bar/timeslider", location)
}

func testDollarIs404(t *testing.T, tsStore testutils.TestDataStore) {
	status, _ := get(t, tsStore, "/p/foo$bar")
	assert.Equal(t, 404, status)
}

func testCleanPadServed(t *testing.T, tsStore testutils.TestDataStore) {
	status, location := get(t, tsStore, "/p/CleanPadId123")
	assert.NotEqual(t, 404, status)
	assert.NotEqual(t, 302, status)
	assert.Empty(t, location)
}

// plantLegacyPad stores a pad under an id with ":" the way an older version
// did; such an id can no longer be created through the pad manager.
func plantLegacyPad(t *testing.T, tsStore testutils.TestDataStore, id, text string) {
	t.Helper()
	donorID := "legacydonor"
	createStaticTestPad(t, tsStore, donorID, text)
	src, err := tsStore.DS.GetPad(donorID)
	require.NoError(t, err)
	dst := *src
	dst.ID = id
	dst.ReadOnlyId = nil
	require.NoError(t, tsStore.DS.CreatePad(id, dst))
	revs, err := tsStore.DS.GetRevisions(donorID, 0, src.Head)
	require.NoError(t, err)
	for _, rev := range *revs {
		pool := db2.RevPool{}
		if rev.Pool != nil {
			pool = *rev.Pool
		}
		require.NoError(t, tsStore.DS.SaveRevision(id, rev.RevNum, rev.Changeset, rev.AText, pool, rev.AuthorId, rev.Timestamp))
	}
}

func testLegacyColonPad(t *testing.T, tsStore testutils.TestDataStore) {
	const legacyID = "wg58legacy:pad"
	_, err := tsStore.PadManager.GetPad(legacyID, nil, nil)
	require.Error(t, err, "a new pad id with ':' is refused")

	plantLegacyPad(t, tsStore, legacyID, "legacy content\n")

	p, err := tsStore.PadManager.GetPad(legacyID, nil, nil)
	require.NoError(t, err, "an existing pad with ':' is still served")
	assert.Equal(t, legacyID, p.Id)
	assert.Contains(t, p.Text(), "legacy content")
	require.NoError(t, p.Check())

	system := "" // attributed to the system author, which needs no author row
	require.NoError(t, p.SpliceText(0, 0, "edited ", &system), "and can be edited")

	status, location := get(t, tsStore, "/p/"+legacyID)
	assert.NotEqual(t, 404, status, "its URL is not 404ed")
	assert.NotEqual(t, 302, status, "and not redirected to another pad")
	assert.Empty(t, location)

	_, err = tsStore.PadManager.GetPad("wg58legacy:other", nil, nil)
	assert.Error(t, err, "a ':' id that is no existing pad stays refused")
}

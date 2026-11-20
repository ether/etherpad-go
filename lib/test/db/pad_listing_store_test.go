package db

import (
	"testing"

	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPadListingStore(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(testutils.TestRunConfig{Name: "pad listing", Test: testPadListingStore})
	testDb.StartTestDBHandler()
}

func testPadListingStore(t *testing.T, ds testutils.TestDataStore) {
	for _, id := range []string{"team-a", "team-b", "team_c", "Team-D", "other", "50%off"} {
		_, err := ds.PadManager.GetPad(id, nil, nil)
		require.NoError(t, err)
	}
	_, err := ds.PadManager.GetTypedPad("team-sheet", "sheet", nil)
	require.NoError(t, err)
	s := ds.DS

	all, err := s.ListPadIDs(db.PadListing{})
	require.NoError(t, err)
	assert.Len(t, all, 7)

	got, err := s.ListPadIDs(db.PadListing{Prefix: "team-", DocumentType: "text"})
	require.NoError(t, err)
	assert.Equal(t, []string{"team-a", "team-b"}, got, "the prefix is literal (_ is no wildcard) and case counts")

	got, err = s.ListPadIDs(db.PadListing{Prefix: "team", DocumentType: "sheet"})
	require.NoError(t, err)
	assert.Equal(t, []string{"team-sheet"}, got)

	got, err = s.ListPadIDs(db.PadListing{Contains: "TEAM-", DocumentType: "text"})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"Team-D", "team-a", "team-b"}, got, "contains ignores case")

	got, err = s.ListPadIDs(db.PadListing{Contains: "%"})
	require.NoError(t, err)
	assert.Equal(t, []string{"50%off"}, got, "% is literal")

	// Page by page, in order.
	first, err := s.ListPadIDs(db.PadListing{Prefix: "team", Limit: 1})
	require.NoError(t, err)
	require.Len(t, first, 1)
	rest, err := s.ListPadIDs(db.PadListing{Prefix: "team", After: first[0], Limit: 10})
	require.NoError(t, err)
	assert.NotContains(t, rest, first[0])
	assert.Len(t, append(first, rest...), 4)
}

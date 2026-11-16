package pad

import (
	"testing"

	"github.com/ether/etherpad-go/lib/apool"
	padModel "github.com/ether/etherpad-go/lib/models/pad"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ReplaceContent without an author (copyPadWithoutHistory through the API
// without authorId) gives the runs without an author to the system author;
// like every author of the pad it must resolve, or opening and exporting
// the pad fail.
func TestReplaceContentAuthorsResolve(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(testutils.TestRunConfig{Name: "system author of replaced content resolves", Test: testReplaceContentAuthorsResolve})
	defer testDb.StartTestDBHandler()
}

func testReplaceContentAuthorsResolve(t *testing.T, ds testutils.TestDataStore) {
	empty := ""
	p, err := ds.PadManager.GetPad("replaceContentAuthors", &empty, nil)
	require.NoError(t, err)
	src := apool.NewAPool()
	_, err = p.ReplaceContent(apool.AText{Text: "no author here\n", Attribs: "+e|1+1"}, &src, nil)
	require.NoError(t, err)

	authors := p.GetAllAuthors()
	require.Contains(t, authors, padModel.SystemAuthorId)
	resolved, err := ds.AuthorManager.GetAuthors(authors)
	require.NoError(t, err)
	ids := map[string]bool{}
	for _, r := range *resolved {
		ids[r.Id] = true
	}
	for _, id := range authors {
		assert.True(t, ids[id], "author %s of the pad does not resolve", id)
	}
}

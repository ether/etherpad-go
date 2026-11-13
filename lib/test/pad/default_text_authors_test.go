package pad

import (
	"testing"

	padModel "github.com/ether/etherpad-go/lib/models/pad"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A pad opened in the browser gets the default text, attributed to the
// system author. Every author in the pad must resolve, or the client never
// gets its CLIENT_VARS and the pad does not open.
func TestDefaultTextAuthorsResolve(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(testutils.TestRunConfig{Name: "authors of a new pad resolve", Test: testDefaultTextAuthorsResolve})
	defer testDb.StartTestDBHandler()
}

func testDefaultTextAuthorsResolve(t *testing.T, ds testutils.TestDataStore) {
	saved := settings.Displayed.DefaultPadText
	settings.Displayed.DefaultPadText = "Welcome to Etherpad!\n"
	t.Cleanup(func() { settings.Displayed.DefaultPadText = saved })
	a, err := ds.AuthorManager.CreateAuthor(nil)
	require.NoError(t, err)
	p, err := ds.PadManager.GetPad("defaultTextAuthors", nil, &a.Id)
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
		assert.True(t, ids[id], "author %s of the new pad does not resolve", id)
	}
}

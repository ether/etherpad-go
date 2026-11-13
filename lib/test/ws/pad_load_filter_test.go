package ws

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/ether/etherpad-go/lib/models/ws/admin"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Port of etherpad-lite's admin/padLoadFilter: the filter is applied on
// the server before paging, so total counts the filtered pads.
func TestPadLoadFilter(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(testutils.TestRunConfig{Name: "padLoad filters before paging", Test: testPadLoadFilter})
	defer testDb.StartTestDBHandler()
}

func testPadLoadFilter(t *testing.T, ds testutils.TestDataStore) {
	tag := "padLoadFilter"
	a, err := ds.AuthorManager.CreateAuthor(nil)
	require.NoError(t, err)
	empty := ""
	for i := 0; i < 5; i++ {
		_, err := ds.PadManager.GetPad(fmt.Sprintf("%s-empty-%d", tag, i), &empty, &a.Id)
		require.NoError(t, err)
	}
	for i := 0; i < 3; i++ {
		p, err := ds.PadManager.GetPad(fmt.Sprintf("%s-edited-%d", tag, i), &empty, &a.Id)
		require.NoError(t, err)
		require.NoError(t, p.SetText(fmt.Sprintf("seed-%d\n", i), &a.Id))
	}

	load := func(q admin.PadLoadData) admin.PadDefinition {
		t.Helper()
		q.Pattern = tag
		q.SortBy = "padName"
		q.Ascending = true
		name, raw := adminRequest(t, ds, "padLoad", q)
		require.Equal(t, "results:padLoad", name)
		var res admin.PadDefinition
		require.NoError(t, json.Unmarshal(raw, &res))
		return res
	}

	t.Run("empty returns only pads at revision 0", func(t *testing.T) {
		res := load(admin.PadLoadData{Limit: 12, Filter: "empty"})
		assert.Equal(t, 5, res.Total)
		require.Len(t, res.Results, 5)
		for _, r := range res.Results {
			assert.Equal(t, 0, r.RevisionNumber, r.PadName)
		}
	})
	t.Run("empty with limit 2 still reports the full total", func(t *testing.T) {
		res := load(admin.PadLoadData{Limit: 2, Filter: "empty"})
		assert.Equal(t, 5, res.Total)
		assert.Len(t, res.Results, 2)
		next := load(admin.PadLoadData{Offset: 4, Limit: 2, Filter: "empty"})
		assert.Equal(t, 5, next.Total)
		require.Len(t, next.Results, 1)
		assert.Equal(t, tag+"-empty-4", next.Results[0].PadName)
	})
	t.Run("no filter (older client) means all", func(t *testing.T) {
		assert.Equal(t, 8, load(admin.PadLoadData{Limit: 12}).Total)
	})
	t.Run("all matches no filter", func(t *testing.T) {
		assert.Equal(t, 8, load(admin.PadLoadData{Limit: 12, Filter: "all"}).Total)
	})
	t.Run("active excludes pads nobody has open", func(t *testing.T) {
		res := load(admin.PadLoadData{Limit: 12, Filter: "active"})
		assert.Equal(t, 0, res.Total)
		assert.Empty(t, res.Results)
	})
	t.Run("recent includes pads edited just now", func(t *testing.T) {
		assert.Equal(t, 8, load(admin.PadLoadData{Limit: 12, Filter: "recent"}).Total)
		assert.Equal(t, 0, load(admin.PadLoadData{Limit: 12, Filter: "stale"}).Total)
	})
	t.Run("unknown filter is an error, not an empty list", func(t *testing.T) {
		res := load(admin.PadLoadData{Limit: 12, Filter: "bogus"})
		assert.Contains(t, res.Error, "unknown filter")
		assert.Empty(t, res.Results)
	})
}

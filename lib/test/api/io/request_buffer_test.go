package io

import (
	"testing"

	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/assert"
)

// A pad id from one request must not change when fiber reuses the request
// buffer (see server.FiberConfig): importing into "d" made "s" vanish from
// the memory store.
func TestPadIdsOutliveTheRequest(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(testutils.TestRunConfig{Name: "two imports", Test: func(t *testing.T, ds testutils.TestDataStore) {
		app := ieApp(t, ds, false)
		ieImport(t, app, "s", "seed.txt", []byte(rtSample), "")
		ieImport(t, app, "d", "file.txt", []byte("x\n"), "")
		ids, err := ds.DS.GetPadIds()
		assert.NoError(t, err)
		assert.ElementsMatch(t, []string{"s", "d"}, *ids)
		assert.Equal(t, rtSample, iePadText_(t, ds, "s"))
		assert.Equal(t, "x\n", iePadText_(t, ds, "d"))
	}})
	testDb.StartTestDBHandler()
}

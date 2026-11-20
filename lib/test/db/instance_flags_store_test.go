package db

import (
	"testing"

	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstanceFlagStore(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(testutils.TestRunConfig{Name: "instance flags", Test: testInstanceFlagStore})
	testDb.StartTestDBHandler()
}

func testInstanceFlagStore(t *testing.T, ds testutils.TestDataStore) {
	s := ds.DS
	v, err := s.GetInstanceFlag("apikey-file-taken-over")
	require.NoError(t, err)
	assert.Nil(t, v)

	set, err := s.SetInstanceFlag("apikey-file-taken-over", "2026-11-20T18:00:00Z")
	require.NoError(t, err)
	assert.True(t, set)
	// Set once: a second call (another instance) changes nothing.
	set, err = s.SetInstanceFlag("apikey-file-taken-over", "later")
	require.NoError(t, err)
	assert.False(t, set)
	v, err = s.GetInstanceFlag("apikey-file-taken-over")
	require.NoError(t, err)
	require.NotNil(t, v)
	assert.Equal(t, "2026-11-20T18:00:00Z", *v)

	other, err := s.GetInstanceFlag("something-else")
	require.NoError(t, err)
	assert.Nil(t, other)
}

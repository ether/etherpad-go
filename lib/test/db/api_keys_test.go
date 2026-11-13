package db

import (
	"testing"
	"time"

	"github.com/ether/etherpad-go/lib/apikeys"
	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The API key store on every database.
func TestAPIKeyStore(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(testutils.TestRunConfig{Name: "api keys", Test: testAPIKeyStore})
	testDb.StartTestDBHandler()
}

func testAPIKeyStore(t *testing.T, ds testutils.TestDataStore) {
	m := apikeys.NewManager(ds.DS)
	keys, err := m.List()
	require.NoError(t, err)
	assert.Empty(t, keys)

	moodle, mk, err := m.Create("Moodle – Kurse")
	require.NoError(t, err)
	time.Sleep(2 * time.Millisecond)
	_, bk, err := m.Create("Backup")
	require.NoError(t, err)

	keys, err = m.List()
	require.NoError(t, err)
	require.Len(t, keys, 2)
	assert.Equal(t, "Moodle – Kurse", keys[0].Description, "oldest first")
	assert.Equal(t, mk.Prefix, keys[0].Prefix)
	assert.Nil(t, keys[0].LastUsed)
	assert.False(t, keys[0].Disabled)

	_, ok := m.Check(moodle)
	assert.True(t, ok)
	keys, _ = m.List()
	assert.NotNil(t, keys[0].LastUsed)

	require.NoError(t, m.SetDisabled(mk.ID, true))
	require.NoError(t, m.SetDisabled(mk.ID, true), "disabling twice is fine")
	_, ok = m.Check(moodle)
	assert.False(t, ok)
	keys, _ = m.List()
	assert.True(t, keys[0].Disabled)

	require.NoError(t, m.Delete(bk.ID))
	assert.ErrorIs(t, m.Delete(bk.ID), db.ErrAPIKeyNotFound)
	assert.ErrorIs(t, m.SetDisabled(bk.ID, false), db.ErrAPIKeyNotFound)
	keys, _ = m.List()
	assert.Len(t, keys, 1)

	// ids compare as they are (MySQL too)
	upper, err := ds.DS.GetAPIKeyByHash(apikeys.Hash(moodle))
	require.NoError(t, err)
	require.NotNil(t, upper)
	none, err := ds.DS.GetAPIKeyByHash("NOPE")
	require.NoError(t, err)
	assert.Nil(t, none)
}

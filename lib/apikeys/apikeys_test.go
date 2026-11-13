package apikeys

import (
	"strings"
	"testing"
	"time"

	"github.com/ether/etherpad-go/lib/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManager(t *testing.T) {
	store := db.NewMemoryDataStore()
	m := NewManager(store)
	now := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	m.now = func() time.Time { return now }

	_, _, err := m.Create("   ")
	assert.ErrorIs(t, err, ErrNoDescription)
	_, _, err = m.Create(strings.Repeat("x", MaxDescription+1))
	assert.ErrorIs(t, err, ErrDescriptionTooLong)

	moodle, mk, err := m.Create("  Moodle  ")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(moodle, "ep_"))
	assert.Len(t, moodle, 43)
	assert.Equal(t, "Moodle", mk.Description)
	assert.Equal(t, moodle[:10], mk.Prefix)
	backup, bk, err := m.Create("Backup")
	require.NoError(t, err)
	assert.NotEqual(t, moodle, backup)

	keys, err := m.List()
	require.NoError(t, err)
	require.Len(t, keys, 2)
	for _, k := range keys {
		assert.NotContains(t, k.Hash, moodle, "only the hash is stored")
		assert.NotEqual(t, moodle, k.Hash)
	}

	got, ok := m.Check(moodle)
	require.True(t, ok)
	assert.Equal(t, mk.ID, got.ID)
	_, ok = m.Check("ep_wrong")
	assert.False(t, ok)
	_, ok = m.Check("")
	assert.False(t, ok)

	// the last use is written, at most once a minute
	keys, _ = m.List()
	require.NotNil(t, keys[0].LastUsed)
	assert.Equal(t, now, *keys[0].LastUsed)
	now = now.Add(30 * time.Second)
	m.Check(moodle)
	keys, _ = m.List()
	assert.Equal(t, now.Add(-30*time.Second), *keys[0].LastUsed)
	now = now.Add(time.Minute)
	m.Check(moodle)
	keys, _ = m.List()
	assert.Equal(t, now, *keys[0].LastUsed)

	// disabling one key leaves the other working
	require.NoError(t, m.SetDisabled(mk.ID, true))
	_, ok = m.Check(moodle)
	assert.False(t, ok)
	_, ok = m.Check(backup)
	assert.True(t, ok)
	require.NoError(t, m.SetDisabled(mk.ID, false))
	_, ok = m.Check(moodle)
	assert.True(t, ok)

	require.NoError(t, m.Delete(bk.ID))
	_, ok = m.Check(backup)
	assert.False(t, ok)
	assert.ErrorIs(t, m.Delete(bk.ID), db.ErrAPIKeyNotFound)
	assert.ErrorIs(t, m.SetDisabled("nope", true), db.ErrAPIKeyNotFound)

	// importing an APIKEY.txt key once
	stored, err := m.Import("legacy-key-123\n", "APIKEY.txt")
	require.NoError(t, err)
	assert.True(t, stored)
	stored, err = m.Import("legacy-key-123", "APIKEY.txt")
	require.NoError(t, err)
	assert.False(t, stored)
	_, ok = m.Check("legacy-key-123")
	assert.True(t, ok)
}

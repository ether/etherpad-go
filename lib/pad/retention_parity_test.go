package pad

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/hooks"
	modeldb "github.com/ether/etherpad-go/lib/models/db"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// Port of etherpad-lite's padRetention spec (the CLIENT_READY cases are
// CheckOnOpen here; the websocket handler sends its notice as-is).

const day = 24 * time.Hour

func retentionCfg(t *testing.T, mod func(*settings.PadRetention)) *retentionConfig {
	t.Helper()
	s := settings.PadRetention{Enabled: true, DeleteAfterDays: 30, WarnBeforeDays: 7, SweepIntervalHours: 1, ExcludeGroupPads: true}
	if mod != nil {
		mod(&s)
	}
	cfg, err := parseRetentionConfig(s)
	require.NoError(t, err)
	return cfg
}

func retentionWith(t *testing.T, store db.DataStore, cfg *retentionConfig) (*Retention, *observer.ObservedLogs) {
	t.Helper()
	core, logs := observer.New(zap.InfoLevel)
	r := &Retention{manager: newManagerForTest(store), store: store, setting: &settings.Settings{}, logger: zap.New(core).Sugar()}
	r.cfg = cfg
	return r, logs
}

func TestRetentionParseConfig(t *testing.T) {
	cfg, err := parseRetentionConfig(settings.PadRetention{Enabled: false, DeleteAfterDays: 30})
	require.NoError(t, err)
	assert.Nil(t, cfg, "off")
	for name, s := range map[string]settings.PadRetention{
		"negative deleteAfterDays": {Enabled: true, DeleteAfterDays: -5, SweepIntervalHours: 1},
	} {
		_, err := parseRetentionConfig(s)
		assert.Error(t, err, name)
		assert.Contains(t, err.Error(), "padRetention.", name)
	}

	t.Run("an invalid config stays off and deletes nothing on open", func(t *testing.T) {
		store := db.NewMemoryDataStore()
		addRetentionTestPad(t, store, "old", time.Now().Add(-100*day))
		core, logs := observer.New(zap.WarnLevel)
		r := NewRetention(newManagerForTest(store), store, &settings.Settings{PadRetention: settings.PadRetention{
			Enabled: true, DeleteAfterDays: -1, SweepIntervalHours: 1}}, zap.New(core).Sugar())
		r.Start()
		defer r.Stop()
		found := false
		for _, e := range logs.All() {
			if s, ok := e.ContextMap()["error"].(string); ok && contains(s, "deleteAfterDays") {
				found = true
			}
		}
		assert.True(t, found, "the problem is logged")
		assert.Nil(t, r.CheckOnOpen("old"))
		exists, _ := store.DoesPadExist("old")
		assert.True(t, *exists)
	})
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }

func newManagerForTest(store db.DataStore) *Manager {
	h := hooks.NewHook()
	return NewManager(store, &h)
}

func TestRetentionCheckPad(t *testing.T) {
	now := time.Now()
	store := db.NewMemoryDataStore()
	cfg := retentionCfg(t, nil)
	r, _ := retentionWith(t, store, cfg)

	addRetentionTestPad(t, store, "old31", now.Add(-31*day))
	st, err := r.status("old31", cfg, now)
	require.NoError(t, err)
	assert.LessOrEqual(t, st.remaining, time.Duration(0), "expires")

	addRetentionTestPad(t, store, "recent", now.Add(-2*day))
	st, err = r.status("recent", cfg, now)
	require.NoError(t, err)
	assert.InDelta(t, float64(28*day), float64(st.remaining), float64(time.Minute), "kept, expires in 28 days")

	// Never edited: revision 0 is the creation, so it counts from there.
	addRetentionTestPad(t, store, "created", now)
	st, err = r.status("created", cfg, now)
	require.NoError(t, err)
	assert.Greater(t, st.remaining, time.Duration(0))

	addRetentionTestPad(t, store, "future", now.Add(3*day))
	st, err = r.status("future", cfg, now)
	require.NoError(t, err)
	assert.Equal(t, cfg.deleteAfter, st.remaining, "clock skew keeps the pad")

	group := "g.1234567890123456$retention"
	addRetentionTestPad(t, store, group, now.Add(-100*day))
	st, err = r.status(group, cfg, now)
	require.NoError(t, err)
	assert.Nil(t, st, "group pads are skipped")
	withGroups := retentionCfg(t, func(s *settings.PadRetention) { s.ExcludeGroupPads = false })
	removed, err := r.expirePad(group, withGroups, now)
	require.NoError(t, err)
	assert.True(t, removed)
	exists, _ := store.DoesPadExist(group)
	assert.False(t, *exists)

	addRetentionTestPad(t, store, "retention_keepme_x", now.Add(-100*day))
	keepMe := retentionCfg(t, func(s *settings.PadRetention) { s.ExcludePattern = "^retention_keepme_" })
	st, err = r.status("retention_keepme_x", keepMe, now)
	require.NoError(t, err)
	assert.Nil(t, st, "excludePattern")

	removed, err = r.expirePad("doesNotExist", cfg, now)
	require.NoError(t, err)
	assert.False(t, removed, "a missing pad is skipped")
}

type brokenRevisions struct {
	db.DataStore
	broken string
}

func (b *brokenRevisions) GetRevision(padId string, rev int) (*modeldb.PadSingleRevision, error) {
	if padId == b.broken {
		return nil, errors.New("corrupt revision")
	}
	return b.DataStore.GetRevision(padId, rev)
}

func TestRetentionSweep(t *testing.T) {
	now := time.Now()
	t.Run("deletes expired pads and keeps fresh ones", func(t *testing.T) {
		store := db.NewMemoryDataStore()
		addRetentionTestPad(t, store, "old", now.Add(-31*day))
		addRetentionTestPad(t, store, "fresh", now.Add(-1*day))
		ro := NewReadOnlyManager(store).GetReadOnlyId("old")
		r, _ := retentionWith(t, store, retentionCfg(t, nil))
		report := r.sweepOnce(r.cfg)
		assert.Equal(t, 1, report.deleted)
		exists, _ := store.DoesPadExist("old")
		assert.False(t, *exists)
		_, err := store.GetRevision("old", 0)
		assert.Error(t, err)
		id, err := store.GetPadByReadOnlyId(ro)
		assert.True(t, err != nil || id == nil, "the read-only id is gone too")
		exists, _ = store.DoesPadExist("fresh")
		assert.True(t, *exists)
	})
	t.Run("carries on past a broken revision record", func(t *testing.T) {
		mem := db.NewMemoryDataStore()
		addRetentionTestPad(t, mem, "broken", now.Add(-31*day))
		addRetentionTestPad(t, mem, "old", now.Add(-31*day))
		store := &brokenRevisions{DataStore: mem, broken: "broken"}
		r, _ := retentionWith(t, store, retentionCfg(t, nil))
		report := r.sweepOnce(r.cfg)
		assert.GreaterOrEqual(t, report.errors, 1)
		exists, _ := mem.DoesPadExist("old")
		assert.False(t, *exists)
	})
	t.Run("deletes nothing in dry run and logs each pad", func(t *testing.T) {
		store := db.NewMemoryDataStore()
		addRetentionTestPad(t, store, "old", now.Add(-31*day))
		r, logs := retentionWith(t, store, retentionCfg(t, func(s *settings.PadRetention) { s.DryRun = true }))
		report := r.sweepOnce(r.cfg)
		assert.Equal(t, 1, report.deleted, "counted as would-delete")
		entries := logs.FilterMessage("Pad retention would delete pad").All()
		require.Len(t, entries, 1)
		assert.Equal(t, "old", entries[0].ContextMap()["padId"])
		exists, _ := store.DoesPadExist("old")
		assert.True(t, *exists)
	})
	t.Run("stops between pads when cancelled", func(t *testing.T) {
		store := db.NewMemoryDataStore()
		addRetentionTestPad(t, store, "old", now.Add(-31*day))
		r, _ := retentionWith(t, store, retentionCfg(t, func(s *settings.PadRetention) { s.DryRun = true }))
		report := r.sweep(r.cfg, func() bool { return true })
		assert.Equal(t, 0, report.checked)
	})
}

func TestRetentionOnOpen(t *testing.T) {
	now := time.Now()
	t.Run("nothing when the feature is off", func(t *testing.T) {
		store := db.NewMemoryDataStore()
		addRetentionTestPad(t, store, "old", now.Add(-100*day))
		r, _ := retentionWith(t, store, nil)
		assert.Nil(t, r.CheckOnOpen("old"))
		exists, _ := store.DoesPadExist("old")
		assert.True(t, *exists)
	})
	t.Run("deletes an overdue pad", func(t *testing.T) {
		store := db.NewMemoryDataStore()
		addRetentionTestPad(t, store, "old", now.Add(-31*day))
		r, _ := retentionWith(t, store, retentionCfg(t, nil))
		n := r.CheckOnOpen("old")
		require.NotNil(t, n)
		assert.Equal(t, RetentionNotice{State: "expired", DeleteAfterDays: 30}, *n)
		exists, _ := store.DoesPadExist("old")
		assert.False(t, *exists, "the pad that opens is a new one")
	})
	t.Run("warns inside warnBeforeDays", func(t *testing.T) {
		store := db.NewMemoryDataStore()
		addRetentionTestPad(t, store, "soon", now.Add(-25*day))
		r, _ := retentionWith(t, store, retentionCfg(t, nil))
		n := r.CheckOnOpen("soon")
		require.NotNil(t, n)
		assert.Equal(t, "expiring", n.State)
		assert.InDelta(t, float64((5 * day).Milliseconds()), float64(n.MsRemaining), float64(time.Minute.Milliseconds()))
	})
	t.Run("says nothing outside the warning window", func(t *testing.T) {
		store := db.NewMemoryDataStore()
		addRetentionTestPad(t, store, "fresh", now.Add(-1*day))
		r, _ := retentionWith(t, store, retentionCfg(t, nil))
		assert.Nil(t, r.CheckOnOpen("fresh"))
	})
	t.Run("neither deletes nor warns in dry run", func(t *testing.T) {
		store := db.NewMemoryDataStore()
		addRetentionTestPad(t, store, "old", now.Add(-31*day))
		addRetentionTestPad(t, store, "soon", now.Add(-25*day))
		r, _ := retentionWith(t, store, retentionCfg(t, func(s *settings.PadRetention) { s.DryRun = true }))
		assert.Nil(t, r.CheckOnOpen("old"))
		assert.Nil(t, r.CheckOnOpen("soon"))
		exists, _ := store.DoesPadExist("old")
		assert.True(t, *exists)
	})
}

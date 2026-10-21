package pad

import (
	"testing"
	"time"

	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/hooks"
	modeldb "github.com/ether/etherpad-go/lib/models/db"
	"github.com/ether/etherpad-go/lib/settings"
	"go.uber.org/zap"
)

func addRetentionTestPad(t *testing.T, store *db.MemoryDataStore, id string, editedAt time.Time) {
	t.Helper()
	if err := store.CreatePad(id, modeldb.PadDB{ID: id, Head: 0}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveRevision(id, 0, "", modeldb.AText{Text: "\n"}, modeldb.RevPool{}, nil, editedAt.UnixMilli()); err != nil {
		t.Fatal(err)
	}
}

func newTestRetention(t *testing.T, store *db.MemoryDataStore, config settings.PadRetention) *Retention {
	t.Helper()
	createdHooks := hooks.NewHook()
	manager := NewManager(store, &createdHooks)
	retention := NewRetention(manager, store, &settings.Settings{PadRetention: config}, zap.NewNop().Sugar())
	retention.Start()
	t.Cleanup(retention.Stop)
	return retention
}

func TestRetentionCheckOnOpenDeletesExpiredPad(t *testing.T) {
	store := db.NewMemoryDataStore()
	addRetentionTestPad(t, store, "old", time.Now().Add(-48*time.Hour))
	retention := newTestRetention(t, store, settings.PadRetention{
		Enabled:            true,
		DeleteAfterDays:    1,
		WarnBeforeDays:     1,
		SweepIntervalHours: 1,
	})

	notice := retention.CheckOnOpen("old")
	if notice == nil || notice.State != "expired" {
		t.Fatalf("expected expired notice, got %#v", notice)
	}
	exists, err := store.DoesPadExist("old")
	if err != nil {
		t.Fatal(err)
	}
	if *exists {
		t.Fatal("expired pad still exists")
	}
}

func TestRetentionDryRunAndGroupExclusion(t *testing.T) {
	store := db.NewMemoryDataStore()
	addRetentionTestPad(t, store, "old", time.Now().Add(-48*time.Hour))
	addRetentionTestPad(t, store, "g.1234567890123456$group", time.Now().Add(-48*time.Hour))
	retention := newTestRetention(t, store, settings.PadRetention{
		Enabled:            true,
		DryRun:             true,
		DeleteAfterDays:    1,
		SweepIntervalHours: 1,
		ExcludeGroupPads:   true,
	})

	if notice := retention.CheckOnOpen("old"); notice == nil || notice.State != "expired" {
		t.Fatalf("expected dry-run expired notice, got %#v", notice)
	}
	if notice := retention.CheckOnOpen("g.1234567890123456$group"); notice != nil {
		t.Fatalf("expected excluded group pad to have no notice, got %#v", notice)
	}
	for _, id := range []string{"old", "g.1234567890123456$group"} {
		exists, err := store.DoesPadExist(id)
		if err != nil {
			t.Fatal(err)
		}
		if !*exists {
			t.Fatalf("pad %q was deleted during dry-run/exclusion", id)
		}
	}
}

func TestParseRetentionConfigRejectsInvalidValues(t *testing.T) {
	for name, config := range map[string]settings.PadRetention{
		"delete-after":   {Enabled: true, DeleteAfterDays: 0, SweepIntervalHours: 1},
		"warn-before":    {Enabled: true, DeleteAfterDays: 1, WarnBeforeDays: -1, SweepIntervalHours: 1},
		"sweep-interval": {Enabled: true, DeleteAfterDays: 1, SweepIntervalHours: 0},
		"exclude-regex":  {Enabled: true, DeleteAfterDays: 1, SweepIntervalHours: 1, ExcludePattern: "["},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseRetentionConfig(config); err == nil {
				t.Fatal("expected invalid retention configuration")
			}
		})
	}
}

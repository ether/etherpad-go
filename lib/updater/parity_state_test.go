package updater

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Port of etherpad-lite's updater state spec: a hand-edited or damaged
// state file resets to the empty state instead of driving the updater
// (a rollback without a backup, an apply at no time).

func writeState(t *testing.T, v any) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "var", "update-state.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	b, err := json.Marshal(v)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, b, 0o600))
	return path
}

func stateWith(exec map[string]any, extra map[string]any) map[string]any {
	s := map[string]any{"schemaVersion": 1, "execution": exec}
	for k, v := range extra {
		s[k] = v
	}
	return s
}

func TestParityEmptyState(t *testing.T) {
	s := EmptyState()
	assert.Equal(t, Execution{Status: StatusIdle}, s.Execution)
	assert.Equal(t, 0, s.BootCount)
	assert.Nil(t, s.LastResult)
	assert.Nil(t, s.Latest)
}

func TestParityLoadStateRejects(t *testing.T) {
	for name, v := range map[string]any{
		"unknown schema":         map[string]any{"schemaVersion": 99, "execution": map[string]any{"status": "idle"}},
		"latest is an array":     stateWith(map[string]any{"status": "idle"}, map[string]any{"latest": []any{1}}),
		"latest without fields":  stateWith(map[string]any{"status": "idle"}, map[string]any{"latest": map[string]any{"version": "2.7.3"}}),
		"latest with unsafe tag": stateWith(map[string]any{"status": "idle"}, map[string]any{"latest": map[string]any{"version": "2.7.3", "tag": "-rf"}}),
		"execution not object":   stateWith(nil, map[string]any{"execution": "not-an-object"}),
		"unknown status":         stateWith(map[string]any{"status": "totally-bogus"}, nil),
		"pending-verification without backup": stateWith(map[string]any{
			"status": "pending-verification", "targetTag": "v2.7.3", "fromVersion": "2.7.2", "deadlineAt": "2026-05-08T00:00:00Z"}, nil),
		"rolling-back without reason": stateWith(map[string]any{
			"status": "rolling-back", "targetTag": "v2.7.3", "backupPath": "/x.bak"}, nil),
		"rolling-back without target": stateWith(map[string]any{
			"status": "rolling-back", "backupPath": "/x.bak", "reason": "timeout"}, nil),
		"empty required field": stateWith(map[string]any{
			"status": "executing", "targetTag": "", "fromVersion": "2.7.2"}, nil),
		"unsafe target tag":      stateWith(map[string]any{"status": "verified", "targetTag": "../x"}, nil),
		"scheduled without time": stateWith(map[string]any{"status": "scheduled", "targetTag": "v2.7.3"}, nil),
		"scheduled bad time": stateWith(map[string]any{
			"status": "scheduled", "targetTag": "v2.7.3", "scheduledFor": "tomorrow-ish"}, nil),
		"bad timestamp elsewhere": stateWith(map[string]any{"status": "preflight", "targetTag": "v2.7.3", "at": "not-a-date"}, nil),
		"unknown outcome": stateWith(map[string]any{"status": "idle"}, map[string]any{
			"lastResult": map[string]any{"targetTag": "v2.7.3", "outcome": "totally-made-up", "at": "2026-05-08T00:00:00Z"}}),
		"non-numeric bootCount": stateWith(map[string]any{"status": "idle"}, map[string]any{"bootCount": "one"}),
		"negative bootCount":    stateWith(map[string]any{"status": "idle"}, map[string]any{"bootCount": -1}),
	} {
		assert.Equal(t, EmptyState(), LoadState(writeState(t, v)), name)
	}
}

func TestParityLoadStateAccepts(t *testing.T) {
	// An older file without the execution block gets an idle one.
	path := writeState(t, map[string]any{"schemaVersion": 1, "lastCheckAt": "2026-05-01T00:00:00Z", "lastEtag": `W/"abc"`})
	s := LoadState(path)
	assert.Equal(t, Execution{Status: StatusIdle}, s.Execution)
	assert.Equal(t, "2026-05-01T00:00:00Z", s.LastCheckAt)
	assert.Equal(t, `W/"abc"`, s.LastETag)

	pending := stateWith(map[string]any{
		"status": "pending-verification", "targetTag": "v2.7.3", "fromVersion": "2.7.2",
		"backupPath": "/opt/etherpad.bak", "deadlineAt": "2026-05-08T00:00:00Z"}, map[string]any{"bootCount": 1,
		"latest": map[string]any{"version": "2.7.3", "tag": "v2.7.3", "body": "notes", "publishedAt": "2026-05-01T00:00:00Z", "htmlUrl": "https://x"}})
	s = LoadState(writeState(t, pending))
	assert.Equal(t, StatusPendingVerification, s.Execution.Status)
	assert.Equal(t, 1, s.BootCount)
	assert.Equal(t, "v2.7.3", s.Latest.Tag)
}

func TestParityStateRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "dir", "update-state.json")
	for _, want := range []UpdateState{
		{Execution: Execution{Status: StatusPendingVerification, TargetTag: "v2.7.3", FromVersion: "2.7.2", BackupPath: "/x.bak", DeadlineAt: "2026-05-08T00:00:00Z", At: "2026-05-07T23:59:00Z"}, BootCount: 2},
		{Execution: Execution{Status: StatusScheduled, TargetTag: "v2.7.3", ScheduledFor: "2026-05-08T03:00:00Z"}},
		{Execution: Execution{Status: StatusIdle}, LastResult: &LastResult{TargetTag: "v2.7.3", FromVersion: "2.7.2", Outcome: "rolled-back", Reason: "health-check-timeout", At: "2026-05-08T00:00:00Z"}},
	} {
		require.NoError(t, SaveState(path, want), "creates the directory")
		want.SchemaVersion = SchemaVersion
		assert.Equal(t, want, LoadState(path))
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	assert.Len(t, entries, 1, "no temp file left behind")
}

// Saves at once each write a whole file; the last one wins and nothing is
// left over (etherpad-lite #8412). The updater saves under its mutex.
func TestParityConcurrentSaves(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows refuses a rename onto a file another rename is replacing; the updater saves one at a time")
	}
	path := filepath.Join(t.TempDir(), "update-state.json")
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s := EmptyState()
			s.BootCount = i
			assert.NoError(t, SaveState(path, s))
		}(i)
	}
	wg.Wait()
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	var s UpdateState
	require.NoError(t, json.Unmarshal(b, &s), "a whole file")
	entries, _ := os.ReadDir(filepath.Dir(path))
	assert.Len(t, entries, 1)
}

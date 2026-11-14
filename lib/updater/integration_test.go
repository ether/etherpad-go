package updater

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// Ports of etherpad-lite's updater-integration, updater-scheduler-integration
// and updater-window-integration specs: the whole updater against a fake
// GitHub, from the check to the restart into the new binary and its
// verification or rollback.

type fakeGitHub struct {
	srv       *httptest.Server
	binary    []byte
	checksum  string // overrides the sha256 in checksums.txt
	tag       string
	downloads int
	mu        sync.Mutex
}

func newFakeGitHub(t *testing.T, tag string, binary []byte) *fakeGitHub {
	f := &fakeGitHub{binary: binary, tag: tag}
	asset := fmt.Sprintf("etherpad-go-%s-%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		asset += ".exe"
	}
	mux := http.NewServeMux()
	release := func(w http.ResponseWriter) {
		_ = json.NewEncoder(w).Encode(ghRelease{TagName: f.tag, Assets: []ghAsset{
			{Name: asset, BrowserDownloadURL: f.srv.URL + "/dl/" + asset},
			{Name: "checksums.txt", BrowserDownloadURL: f.srv.URL + "/dl/checksums.txt"},
		}})
	}
	mux.HandleFunc("/repos/test/repo/releases/latest", func(w http.ResponseWriter, _ *http.Request) { release(w) })
	mux.HandleFunc("/repos/test/repo/releases/tags/", func(w http.ResponseWriter, _ *http.Request) { release(w) })
	mux.HandleFunc("/dl/"+asset, func(w http.ResponseWriter, _ *http.Request) {
		f.mu.Lock()
		f.downloads++
		f.mu.Unlock()
		_, _ = w.Write(f.binary)
	})
	mux.HandleFunc("/dl/checksums.txt", func(w http.ResponseWriter, _ *http.Request) {
		sum := sha256.Sum256(f.binary)
		h := hex.EncodeToString(sum[:])
		if f.checksum != "" {
			h = f.checksum
		}
		fmt.Fprintf(w, "%s  %s\n", h, asset)
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

// boot builds an updater as Start would after a (re)start of version.
func boot(t *testing.T, dir, exe, version string, gh *fakeGitHub, tier Tier) (*Updater, chan int) {
	t.Helper()
	exits := make(chan int, 1)
	checker := NewVersionChecker("test/repo")
	checker.apiBase = gh.srv.URL
	cfg := Config{
		Tier: tier, Repo: "test/repo", CurrentVersion: version, CheckInterval: time.Hour,
		HealthCheckSeconds: 60, StatePath: filepath.Join(dir, "var", "update-state.json"),
		LockPath: filepath.Join(dir, "var", "update.lock"), LogPath: filepath.Join(dir, "var", "update.log"),
	}
	logger := withUpdateLog(zap.NewNop().Sugar(), cfg.LogPath)
	u := &Updater{
		cfg: cfg, logger: logger, checker: checker, executor: NewExecutor(checker, exe, SignatureVerifier{}, logger),
		exePath: exe, installMethod: InstallBinary, now: time.Now, done: make(chan struct{}),
		exit: func(code int) { exits <- code },
	}
	u.accepting.Store(true)
	u.state = LoadState(cfg.StatePath)
	u.checkPendingVerification()
	u.resumeLocked()
	t.Cleanup(u.Stop)
	return u, exits
}

func waitExit(t *testing.T, exits chan int) int {
	t.Helper()
	select {
	case code := <-exits:
		return code
	case <-time.After(10 * time.Second):
		t.Fatal("the updater did not exit")
		return 0
	}
}

func TestIntegrationAutoUpdate(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "etherpad-go")
	require.NoError(t, os.WriteFile(exe, []byte("OLD-1.0.0"), 0o755))
	gh := newFakeGitHub(t, "v1.1.0", []byte("NEW-1.1.0"))

	// Check: a minor release is out; auto with no grace schedules it now.
	u, exits := boot(t, dir, exe, "1.0.0", gh, TierAuto)
	u.performCheck()
	assert.Equal(t, 75, waitExit(t, exits), "restart into the new binary")
	b, _ := os.ReadFile(exe)
	assert.Equal(t, "NEW-1.1.0", string(b))
	st := LoadState(u.cfg.StatePath)
	require.Equal(t, StatusPendingVerification, st.Execution.Status)
	assert.FileExists(t, st.Execution.BackupPath)

	// Restart into 1.1.0: a healthy boot verifies it.
	u2, _ := boot(t, dir, exe, "1.1.0", gh, TierAuto)
	u2.MarkBootHealthy()
	st = LoadState(u2.cfg.StatePath)
	assert.Equal(t, StatusVerified, st.Execution.Status)
	assert.Equal(t, "verified", st.LastResult.Outcome)

	// Nothing more to do: up to date.
	u2.performCheck()
	assert.Equal(t, StatusVerified, LoadState(u2.cfg.StatePath).Execution.Status)
	assert.NotEmpty(t, u2.LogTail(50), "the log tells what happened")
}

func TestIntegrationRollbackOnTimeout(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "etherpad-go")
	require.NoError(t, os.WriteFile(exe, []byte("OLD-1.0.0"), 0o755))
	gh := newFakeGitHub(t, "v1.1.0", []byte("NEW-BROKEN"))

	u, exits := boot(t, dir, exe, "1.0.0", gh, TierManual)
	u.performCheck()
	require.NoError(t, u.ApplyNow())
	require.Equal(t, 75, waitExit(t, exits))

	// The new binary boots but never gets healthy: the health check rolls back.
	u2, exits2 := boot(t, dir, exe, "1.1.0", gh, TierManual)
	u2.mu.Lock()
	timer := u2.healthTimer
	u2.mu.Unlock()
	require.NotNil(t, timer)
	timer.Reset(time.Millisecond)
	assert.Equal(t, 75, waitExit(t, exits2))
	b, _ := os.ReadFile(exe)
	assert.Equal(t, "OLD-1.0.0", string(b), "the old binary is back")
	st := LoadState(u2.cfg.StatePath)
	assert.Equal(t, StatusRolledBack, st.Execution.Status)
	assert.Equal(t, "rolled-back", st.LastResult.Outcome)

	// Back on 1.0.0 the same release is not tried again automatically.
	u3, _ := boot(t, dir, exe, "1.0.0", gh, TierAuto)
	u3.performCheck()
	assert.Nil(t, u3.applyTimer)
	assert.Equal(t, StatusRolledBack, LoadState(u3.cfg.StatePath).Execution.Status)
}

func TestIntegrationChecksumMismatch(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "etherpad-go")
	require.NoError(t, os.WriteFile(exe, []byte("OLD"), 0o755))
	gh := newFakeGitHub(t, "v1.1.0", []byte("TAMPERED"))
	gh.checksum = hex.EncodeToString(make([]byte, 32))

	u, exits := boot(t, dir, exe, "1.0.0", gh, TierManual)
	u.performCheck()
	u.runApply(*u.state.Latest)
	select {
	case code := <-exits:
		t.Fatalf("exited %d", code)
	default:
	}
	b, _ := os.ReadFile(exe)
	assert.Equal(t, "OLD", string(b), "untouched")
	st := LoadState(u.cfg.StatePath)
	assert.Equal(t, StatusIdle, st.Execution.Status)
	assert.Equal(t, "failed", st.LastResult.Outcome)
	assert.Equal(t, "checksum-mismatch", st.LastResult.Reason)
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		assert.NotContains(t, e.Name(), ".ep-update-new", "no download left behind")
	}
}

// A schedule made before a restart fires after it (scheduler integration),
// and an autonomous one waits for its window (window integration).
func TestIntegrationScheduleAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "etherpad-go")
	require.NoError(t, os.WriteFile(exe, []byte("OLD"), 0o755))
	gh := newFakeGitHub(t, "v1.1.0", []byte("NEW"))

	u, _ := boot(t, dir, exe, "1.0.0", gh, TierAuto)
	u.cfg.GraceMinutes = 60
	u.performCheck()
	require.Equal(t, StatusScheduled, LoadState(u.cfg.StatePath).Execution.Status)
	u.Stop()

	// The process restarts during the grace period, and the time has come.
	st := LoadState(u.cfg.StatePath)
	st.Execution.ScheduledFor = time.Now().Add(-time.Second).UTC().Format(time.RFC3339)
	require.NoError(t, SaveState(u.cfg.StatePath, st))
	_, exits := boot(t, dir, exe, "1.0.0", gh, TierAuto)
	assert.Equal(t, 75, waitExit(t, exits), "the resumed schedule applied the update")
	b, _ := os.ReadFile(exe)
	assert.Equal(t, "NEW", string(b))
}

func TestIntegrationAutonomousWaitsForWindow(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "etherpad-go")
	require.NoError(t, os.WriteFile(exe, []byte("OLD"), 0o755))
	gh := newFakeGitHub(t, "v1.1.0", []byte("NEW"))

	u, _ := boot(t, dir, exe, "1.0.0", gh, TierAutonomous)
	// A window that is not now.
	now := time.Now().UTC()
	start := now.Add(3 * time.Hour)
	w, ok := ParseWindow(start.Format("15:04"), start.Add(time.Hour).Format("15:04"), "utc")
	require.True(t, ok)
	u.cfg.Window, u.cfg.WindowConfigured = w, true
	u.performCheck()

	st := LoadState(u.cfg.StatePath)
	require.Equal(t, StatusScheduled, st.Execution.Status)
	at, err := time.Parse(time.RFC3339, st.Execution.ScheduledFor)
	require.NoError(t, err)
	assert.True(t, w.InWindow(at), "scheduled inside the window: %s", at)
	assert.True(t, at.After(now.Add(2*time.Hour)))
	gh.mu.Lock()
	assert.Equal(t, 0, gh.downloads, "nothing downloaded before the window")
	gh.mu.Unlock()
	b, _ := os.ReadFile(exe)
	assert.Equal(t, "OLD", string(b))
}

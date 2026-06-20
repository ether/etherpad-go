package updater

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.uber.org/zap"
)

func newTestUpdater(t *testing.T, exePath string) (*Updater, *int) {
	t.Helper()
	dir := t.TempDir()
	exitCode := -1
	u := &Updater{
		cfg: Config{
			Tier:               TierManual,
			CurrentVersion:     "1.0.0",
			HealthCheckSeconds: 60,
			StatePath:          filepath.Join(dir, "update-state.json"),
			LockPath:           filepath.Join(dir, "update.lock"),
		},
		logger:        zap.NewNop().Sugar(),
		installMethod: InstallBinary,
		now:           time.Now,
		exePath:       exePath,
		done:          make(chan struct{}),
	}
	u.checker = NewVersionChecker("test/repo")
	u.executor = NewExecutor(u.checker, exePath, SignatureVerifier{}, u.logger)
	u.exit = func(code int) { exitCode = code }
	u.accepting.Store(true)
	return u, &exitCode
}

func TestMarkBootHealthyVerifies(t *testing.T) {
	u, _ := newTestUpdater(t, filepath.Join(t.TempDir(), "etherpad"))
	u.state = EmptyState()
	u.state.Execution = Execution{Status: StatusPendingVerification, TargetTag: "v2.0.0", FromVersion: "1.0.0"}
	u.state.BootCount = 1

	u.MarkBootHealthy()

	if u.state.Execution.Status != StatusVerified {
		t.Fatalf("expected verified, got %q", u.state.Execution.Status)
	}
	if u.state.BootCount != 0 {
		t.Errorf("bootCount should reset, got %d", u.state.BootCount)
	}
	if u.state.LastResult == nil || u.state.LastResult.Outcome != "verified" {
		t.Errorf("lastResult should be verified, got %+v", u.state.LastResult)
	}
}

func TestMarkBootHealthyNoopWhenNotPending(t *testing.T) {
	u, _ := newTestUpdater(t, filepath.Join(t.TempDir(), "etherpad"))
	u.state = EmptyState() // idle
	u.MarkBootHealthy()
	if u.state.Execution.Status != StatusIdle {
		t.Errorf("idle state should be untouched, got %q", u.state.Execution.Status)
	}
}

func TestCheckPendingVerificationCrashLoopRollsBack(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "etherpad")
	backup := exe + ".bak"
	if err := os.WriteFile(exe, []byte("NEW-BROKEN"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(backup, []byte("OLD-GOOD"), 0o755); err != nil {
		t.Fatal(err)
	}

	u, exitCode := newTestUpdater(t, exe)
	u.state = EmptyState()
	u.state.Execution = Execution{Status: StatusPendingVerification, TargetTag: "v2.0.0", FromVersion: "1.0.0", BackupPath: backup}
	u.state.BootCount = maxBoots // next boot exceeds the limit

	u.checkPendingVerification()

	if got, _ := os.ReadFile(exe); string(got) != "OLD-GOOD" {
		t.Errorf("binary should be rolled back to OLD-GOOD, got %q", got)
	}
	if u.state.Execution.Status != StatusRolledBack {
		t.Errorf("expected rolled-back, got %q", u.state.Execution.Status)
	}
	if *exitCode != ExitCodeRestart {
		t.Errorf("expected exit %d, got %d", ExitCodeRestart, *exitCode)
	}
}

func TestAcknowledge(t *testing.T) {
	u, _ := newTestUpdater(t, filepath.Join(t.TempDir(), "etherpad"))

	// Nothing to acknowledge when not in rollback-failed.
	u.state = EmptyState()
	if err := u.Acknowledge(); err == nil {
		t.Error("Acknowledge should error when not rollback-failed")
	}

	// Clears the terminal state back to idle.
	u.state.Execution = Execution{Status: StatusRollbackFailed, TargetTag: "v2.0.0", Reason: "boom"}
	u.state.BootCount = 5
	if err := u.Acknowledge(); err != nil {
		t.Fatalf("Acknowledge should succeed on rollback-failed: %v", err)
	}
	if u.state.Execution.Status != StatusIdle {
		t.Errorf("expected idle after acknowledge, got %q", u.state.Execution.Status)
	}
	if u.state.BootCount != 0 {
		t.Errorf("bootCount should reset, got %d", u.state.BootCount)
	}
}

func TestCheckPendingVerificationArmsHealthTimer(t *testing.T) {
	u, _ := newTestUpdater(t, filepath.Join(t.TempDir(), "etherpad"))
	u.state = EmptyState()
	u.state.Execution = Execution{Status: StatusPendingVerification, TargetTag: "v2.0.0"}
	u.state.BootCount = 0

	u.checkPendingVerification()

	u.mu.Lock()
	armed := u.healthTimer != nil
	u.mu.Unlock()
	if !armed {
		t.Error("expected health timer to be armed")
	}
	if u.state.BootCount != 1 {
		t.Errorf("bootCount should be 1, got %d", u.state.BootCount)
	}
	// Clean up the timer so it cannot fire a rollback after the test.
	u.Stop()
}

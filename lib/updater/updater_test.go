package updater

import (
	"os"
	"path/filepath"
	"sync"
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
	u.cfg.CurrentVersion = "2.0.0" // booted into the new binary
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
	u.cfg.CurrentVersion = "2.0.0"
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

// A schedule from before a restart fires: the check after the restart sees
// "already scheduled" and would never arm the lost timer.
func TestResumeArmsPersistedSchedule(t *testing.T) {
	u, _ := newTestUpdater(t, filepath.Join(t.TempDir(), "etherpad"))
	u.cfg.Tier = TierAuto
	at := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	u.state = EmptyState()
	u.state.Execution = Execution{Status: StatusScheduled, TargetTag: "v2.0.0", ScheduledFor: at.Format(time.RFC3339)}

	u.resumeLocked()
	if u.applyTimer == nil || u.timerTargetTag != "v2.0.0" {
		t.Fatalf("timer not armed: %v %q", u.applyTimer, u.timerTargetTag)
	}
	u.Stop()
}

// An apply the process died in (preflight, draining, executing) is recorded
// as failed and leaves the state idle, so it does not block later updates.
func TestResumeClearsInterruptedApply(t *testing.T) {
	for _, st := range []ExecutionStatus{StatusPreflight, StatusDraining, StatusExecuting} {
		u, _ := newTestUpdater(t, filepath.Join(t.TempDir(), "etherpad"))
		u.state = EmptyState()
		u.state.Execution = Execution{Status: st, TargetTag: "v2.0.0", FromVersion: "1.0.0"}

		u.resumeLocked()
		if u.state.Execution.Status != StatusIdle {
			t.Errorf("%s: status %q, want idle", st, u.state.Execution.Status)
		}
		if r := u.state.LastResult; r == nil || r.Outcome != "failed" || r.TargetTag != "v2.0.0" {
			t.Errorf("%s: lastResult %+v", st, r)
		}
		if LoadState(u.cfg.StatePath).Execution.Status != StatusIdle {
			t.Errorf("%s: not persisted", st)
		}
		// The failed release is not scheduled again automatically.
		d := DecideSchedule(ScheduleInput{CanAuto: true, Tier: TierAuto, Latest: &ReleaseInfo{Version: "2.0.0", Tag: "v2.0.0"},
			Execution: u.state.Execution, LastResult: u.state.LastResult, Now: time.Now()})
		if d.Action != ScheduleNone {
			t.Errorf("%s: re-scheduled the interrupted release: %+v", st, d)
		}
	}
}

// Pending verification is left to checkPendingVerification.
func TestResumeLeavesOtherStates(t *testing.T) {
	for _, st := range []Execution{{Status: StatusIdle}, {Status: StatusVerified, TargetTag: "v2.0.0"}, {Status: StatusRollbackFailed, TargetTag: "v2.0.0"}} {
		u, _ := newTestUpdater(t, filepath.Join(t.TempDir(), "etherpad"))
		u.state = EmptyState()
		u.state.Execution = st
		u.resumeLocked()
		if u.state.Execution != st || u.applyTimer != nil {
			t.Errorf("%s changed: %+v", st.Status, u.state.Execution)
		}
	}
}

// Port of etherpad-lite's updateActions cases (cancel, acknowledge, busy,
// stale pending verification).

func TestStalePendingVerificationLeftToAdmin(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "etherpad")
	backup := exe + ".bak"
	_ = os.WriteFile(exe, []byte("HAND-INSTALLED"), 0o755)
	_ = os.WriteFile(backup, []byte("OLD"), 0o755)
	u, exitCode := newTestUpdater(t, exe)
	u.cfg.CurrentVersion = "2.1.0" // an admin installed another version by hand
	u.state = EmptyState()
	u.state.Execution = Execution{Status: StatusPendingVerification, TargetTag: "v2.0.0", FromVersion: "1.0.0", BackupPath: backup}
	u.state.BootCount = maxBoots

	u.checkPendingVerification()
	if got, _ := os.ReadFile(exe); string(got) != "HAND-INSTALLED" || *exitCode != -1 {
		t.Fatalf("rolled back over the running version: %q exit %d", got, *exitCode)
	}
	if u.healthTimer != nil || u.state.Execution.Status != StatusPendingVerification {
		t.Fatalf("expected it left alone: %+v", u.state.Execution)
	}
	if err := u.Acknowledge(); err != nil {
		t.Fatalf("an admin can clear it: %v", err)
	}
	if u.state.Execution.Status != StatusIdle {
		t.Errorf("status %q", u.state.Execution.Status)
	}
}

func TestAcknowledgeTerminalStates(t *testing.T) {
	for _, st := range []Execution{
		{Status: StatusRollbackFailed, TargetTag: "v2.0.0", Reason: "x"},
		{Status: StatusPreflightFailed, TargetTag: "v2.0.0", Reason: "insufficient-disk-space"},
		{Status: StatusRolledBack, TargetTag: "v2.0.0", Reason: "health-check-timeout"},
	} {
		u, _ := newTestUpdater(t, filepath.Join(t.TempDir(), "etherpad"))
		u.state = EmptyState()
		u.state.Execution = st
		u.state.LastResult = &LastResult{TargetTag: "v2.0.0", Outcome: "rolled-back", At: "2026-01-01T00:00:00Z"}
		if err := u.Acknowledge(); err != nil {
			t.Fatalf("%s: %v", st.Status, err)
		}
		if u.state.Execution.Status != StatusIdle {
			t.Errorf("%s: %q", st.Status, u.state.Execution.Status)
		}
		// The release may be scheduled automatically again.
		d := DecideSchedule(ScheduleInput{CanAuto: true, Tier: TierAuto, Latest: &ReleaseInfo{Version: "2.0.0", Tag: "v2.0.0"},
			Execution: u.state.Execution, LastResult: u.state.LastResult, Now: time.Now()})
		if d.Action != ScheduleArm {
			t.Errorf("%s: not scheduled after acknowledging: %+v", st.Status, d)
		}
		if LoadState(u.cfg.StatePath).Execution.Status != StatusIdle {
			t.Errorf("%s: not persisted (or the state file is invalid)", st.Status)
		}
	}
	for _, st := range []Execution{{Status: StatusIdle}, {Status: StatusScheduled, TargetTag: "v2.0.0", ScheduledFor: "2026-01-01T00:00:00Z"},
		{Status: StatusDraining, TargetTag: "v2.0.0"}, {Status: StatusExecuting, TargetTag: "v2.0.0"}, {Status: StatusVerified, TargetTag: "v2.0.0"}} {
		u, _ := newTestUpdater(t, filepath.Join(t.TempDir(), "etherpad"))
		u.state = EmptyState()
		u.state.Execution = st
		if err := u.Acknowledge(); err != ErrNotTerminal {
			t.Errorf("%s: %v, want ErrNotTerminal", st.Status, err)
		}
	}
	// A verification whose timer runs is not stale.
	u, _ := newTestUpdater(t, filepath.Join(t.TempDir(), "etherpad"))
	u.cfg.CurrentVersion = "2.0.0"
	u.state = EmptyState()
	u.state.Execution = Execution{Status: StatusPendingVerification, TargetTag: "v2.0.0", FromVersion: "1.0.0", BackupPath: "x", DeadlineAt: "2026-01-01T00:00:00Z"}
	u.checkPendingVerification()
	if err := u.Acknowledge(); err != ErrNotTerminal {
		t.Errorf("running verification acknowledged: %v", err)
	}
	u.Stop()
}

func TestCancelSchedule(t *testing.T) {
	u, _ := newTestUpdater(t, filepath.Join(t.TempDir(), "etherpad"))
	u.state = EmptyState()
	if err := u.Cancel(); err != ErrNothingToCancel {
		t.Errorf("idle: %v", err)
	}
	u.state.Execution = Execution{Status: StatusScheduled, TargetTag: "v2.0.0", ScheduledFor: time.Now().Add(time.Hour).UTC().Format(time.RFC3339)}
	u.mu.Lock()
	u.armApplyTimerLocked("v2.0.0", time.Now().Add(time.Hour))
	u.mu.Unlock()

	if err := u.Cancel(); err != nil {
		t.Fatal(err)
	}
	if u.state.Execution.Status != StatusIdle || u.applyTimer != nil {
		t.Errorf("not cancelled: %+v timer %v", u.state.Execution, u.applyTimer)
	}
	if r := u.state.LastResult; r == nil || r.Outcome != "cancelled" || r.TargetTag != "v2.0.0" {
		t.Errorf("lastResult %+v", r)
	}
	d := DecideSchedule(ScheduleInput{CanAuto: true, Tier: TierAuto, Latest: &ReleaseInfo{Version: "2.0.0", Tag: "v2.0.0"},
		Execution: u.state.Execution, LastResult: u.state.LastResult, Now: time.Now()})
	if d.Action != ScheduleNone {
		t.Errorf("the cancelled release was scheduled again: %+v", d)
	}
	if LoadState(u.cfg.StatePath).LastResult == nil {
		t.Error("not persisted (or the state file is invalid)")
	}
}

func TestCancelDrain(t *testing.T) {
	u, _ := newTestUpdater(t, filepath.Join(t.TempDir(), "etherpad"))
	var notices []int
	u.broadcast = func(s int) { notices = append(notices, s) }
	u.state = EmptyState()
	u.state.Execution = Execution{Status: StatusDraining, TargetTag: "v2.0.0", DrainEndsAt: "2026-01-01T00:00:00Z"}
	if err := u.Cancel(); err != ErrNothingToCancel {
		t.Errorf("no drainer running: %v", err)
	}

	d := NewDrainer(60, u.setAccepting, u.broadcast)
	u.drainer = d
	done := make(chan DrainResult)
	go func() { done <- d.Start() }()
	time.Sleep(20 * time.Millisecond)
	if u.IsAcceptingConnections() {
		t.Error("draining should refuse new connections")
	}
	if err := u.Cancel(); err != nil {
		t.Fatal(err)
	}
	if res := <-done; res != DrainCancelled {
		t.Errorf("drain %v", res)
	}
	if !u.IsAcceptingConnections() {
		t.Error("connections accepted again after cancel")
	}
}

func TestApplyNowBusy(t *testing.T) {
	for _, st := range []ExecutionStatus{StatusPreflight, StatusDraining, StatusExecuting, StatusPendingVerification, StatusRollingBack} {
		u, _ := newTestUpdater(t, filepath.Join(t.TempDir(), "etherpad"))
		u.state = EmptyState()
		u.state.Latest = &ReleaseInfo{Version: "2.0.0", Tag: "v2.0.0"}
		u.state.Execution = Execution{Status: st, TargetTag: "v2.0.0"}
		if err := u.ApplyNow(); err != ErrBusy {
			t.Errorf("%s: %v, want ErrBusy", st, err)
		}
	}
	u, _ := newTestUpdater(t, filepath.Join(t.TempDir(), "etherpad"))
	u.state = EmptyState()
	if err := u.ApplyNow(); err == nil {
		t.Error("no known release: error expected")
	}
	u.cfg.Tier = TierNotify
	u.state.Latest = &ReleaseInfo{Version: "2.0.0", Tag: "v2.0.0"}
	if err := u.ApplyNow(); err == nil {
		t.Error("notify tier: manual apply not permitted")
	}
}

// etherpad-lite's SessionDrainer: announcements below the window only, and
// connections refused during the drain and accepted again after it.
func TestDrainerShortWindow(t *testing.T) {
	var mu sync.Mutex
	var announced []int
	accepting := true
	var seenRefused bool
	d := NewDrainer(2, func(b bool) {
		mu.Lock()
		defer mu.Unlock()
		if !b {
			seenRefused = true
		}
		accepting = b
	}, func(s int) {
		mu.Lock()
		defer mu.Unlock()
		announced = append(announced, s)
	})
	if res := d.Start(); res != DrainCompleted {
		t.Fatalf("result %v", res)
	}
	mu.Lock()
	defer mu.Unlock()
	if !seenRefused || !accepting {
		t.Errorf("refused during, accepting after: %v %v", seenRefused, accepting)
	}
	// 60, 30 and 10 are not below a 2s window: only the start is announced.
	if len(announced) != 1 || announced[0] != 2 {
		t.Errorf("announced %v, want [2]", announced)
	}
}

// etherpad-lite's RollbackHandler: a rollback that cannot restore the
// backup ends in rollback-failed and still restarts.
func TestRollbackFailureIsTerminal(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "etherpad")
	_ = os.WriteFile(exe, []byte("NEW"), 0o755)
	u, exitCode := newTestUpdater(t, exe)
	u.cfg.CurrentVersion = "2.0.0"
	u.state = EmptyState()
	u.state.Execution = Execution{Status: StatusPendingVerification, TargetTag: "v2.0.0", FromVersion: "1.0.0",
		BackupPath: filepath.Join(dir, "missing.bak"), DeadlineAt: "2026-01-01T00:00:00Z"}

	u.rollback("health-check-timeout")
	if u.state.Execution.Status != StatusRollbackFailed || *exitCode != ExitCodeRestart {
		t.Fatalf("status %q exit %d", u.state.Execution.Status, *exitCode)
	}
	if u.state.LastResult == nil || u.state.LastResult.Outcome != "rollback-failed" {
		t.Errorf("lastResult %+v", u.state.LastResult)
	}
	if p := EvaluatePolicy(PolicyInput{Tier: TierAuto, InstallMethod: InstallBinary, CurrentVersion: "1.0.0",
		Latest: &ReleaseInfo{Version: "2.0.0", Tag: "v2.0.0"}, ExecutionStatus: u.state.Execution.Status}); p.CanAuto {
		t.Error("auto must stay off after a failed rollback")
	}
}

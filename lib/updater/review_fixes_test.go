package updater

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A rollback the process was stopped in is resolved at the next boot by the
// version that runs: the old one means it got through, the target one
// means it did not (and it runs again), anything else is left to the admin.
func TestResumeInterruptedRollback(t *testing.T) {
	rollingBack := func(dir string) Execution {
		return Execution{Status: StatusRollingBack, TargetTag: "v2.0.0", FromVersion: "1.0.0",
			BackupPath: filepath.Join(dir, "etherpad.bak"), Reason: "health-check-timeout"}
	}

	t.Run("old version running", func(t *testing.T) {
		u, exitCode := newTestUpdater(t, filepath.Join(t.TempDir(), "etherpad"))
		u.cfg.CurrentVersion = "1.0.0"
		u.state = EmptyState()
		u.state.Execution = rollingBack(t.TempDir())
		u.resumeLocked()
		assert.Equal(t, StatusRolledBack, u.state.Execution.Status)
		assert.Equal(t, "rolled-back", u.state.LastResult.Outcome)
		assert.Equal(t, -1, *exitCode, "no restart")
		assert.True(t, u.canAcknowledgeLocked())
		assert.Equal(t, StatusRolledBack, LoadState(u.cfg.StatePath).Execution.Status, "saved")
	})

	t.Run("target still running: rolls back again", func(t *testing.T) {
		dir := t.TempDir()
		exe := filepath.Join(dir, "etherpad")
		require.NoError(t, os.WriteFile(exe, []byte("NEW"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "etherpad.bak"), []byte("OLD"), 0o755))
		u, exitCode := newTestUpdater(t, exe)
		u.cfg.CurrentVersion = "2.0.0"
		u.state = EmptyState()
		u.state.Execution = rollingBack(dir)
		u.resumeLocked()
		assert.Equal(t, StatusRolledBack, u.state.Execution.Status)
		assert.Equal(t, ExitCodeRestart, *exitCode)
		b, err := os.ReadFile(exe)
		require.NoError(t, err)
		assert.Equal(t, "OLD", string(b), "the backup is back")
	})

	t.Run("another version running", func(t *testing.T) {
		u, exitCode := newTestUpdater(t, filepath.Join(t.TempDir(), "etherpad"))
		u.cfg.CurrentVersion = "3.0.0"
		u.state = EmptyState()
		u.state.Execution = rollingBack(t.TempDir())
		u.resumeLocked()
		assert.Equal(t, StatusRollbackFailed, u.state.Execution.Status)
		assert.Equal(t, -1, *exitCode)
		require.True(t, u.canAcknowledgeLocked(), "the admin can clear it")
		require.NoError(t, u.Acknowledge())
		assert.Equal(t, StatusIdle, u.state.Execution.Status)
	})
}

// After a failed, cancelled or interrupted apply the state is idle, but the
// release is not scheduled again; acknowledging re-enables it.
func TestAcknowledgeIdleFailure(t *testing.T) {
	latest := &ReleaseInfo{Version: "2.0.0", Tag: "v2.0.0"}
	for _, outcome := range []string{"failed", "cancelled", "preflight-failed"} {
		u, _ := newTestUpdater(t, filepath.Join(t.TempDir(), "etherpad"))
		u.state = EmptyState()
		u.state.LastResult = &LastResult{TargetTag: "v2.0.0", Outcome: outcome}
		in := func() ScheduleInput {
			return ScheduleInput{CanAuto: true, Tier: TierAuto, Latest: latest, Execution: u.state.Execution, LastResult: u.state.LastResult, Now: time.Now()}
		}
		require.Equal(t, ScheduleNone, DecideSchedule(in()).Action, outcome)
		require.True(t, u.Status().CanAcknowledge, outcome)
		require.NoError(t, u.Acknowledge(), outcome)
		assert.Equal(t, "acknowledged-"+outcome, u.state.LastResult.Outcome)
		assert.Equal(t, ScheduleArm, DecideSchedule(in()).Action, "%s: scheduled again", outcome)
		assert.ErrorIs(t, u.Acknowledge(), ErrNotTerminal, "nothing left")
	}
	u, _ := newTestUpdater(t, filepath.Join(t.TempDir(), "etherpad"))
	u.state = EmptyState()
	assert.ErrorIs(t, u.Acknowledge(), ErrNotTerminal, "a plain idle state")
}

// Cancelling the drain records "cancelled", like cancelling the schedule:
// the next check does not arm the release again.
func TestCancelledDrainIsNotRearmed(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "etherpad")
	require.NoError(t, os.WriteFile(exe, []byte("OLD"), 0o755))
	u, _ := newTestUpdater(t, exe)
	u.cfg.DrainSeconds = 60
	u.broadcast = func(int) {}
	u.state = EmptyState()
	target := ReleaseInfo{Version: "2.0.0", Tag: "v2.0.0"}
	u.state.Latest = &target

	done := make(chan struct{})
	go func() { u.runApply(target); close(done) }()
	require.Eventually(t, func() bool { return u.Status().CanCancel }, 5*time.Second, 10*time.Millisecond)
	require.NoError(t, u.Cancel())
	<-done

	assert.Equal(t, StatusIdle, u.state.Execution.Status)
	require.NotNil(t, u.state.LastResult)
	assert.Equal(t, "cancelled", u.state.LastResult.Outcome)
	assert.Equal(t, "cancelled", LoadState(u.cfg.StatePath).LastResult.Outcome, "saved")
	d := DecideSchedule(ScheduleInput{CanAuto: true, Tier: TierAuto, Latest: &target, Execution: u.state.Execution, LastResult: u.state.LastResult, Now: time.Now()})
	assert.Equal(t, ScheduleNone, d.Action)
}

// The rollback saves its state before it waits for a slow mail server, and
// the updater stays responsive meanwhile.
func TestRollbackSavesBeforeMailing(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "etherpad")
	require.NoError(t, os.WriteFile(exe, []byte("NEW"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "etherpad.bak"), []byte("OLD"), 0o755))
	u, exitCode := newTestUpdater(t, exe)
	u.cfg.CurrentVersion = "2.0.0"
	u.cfg.AdminEmail = func() string { return "admin@example.com" }
	release := make(chan struct{})
	var mailing atomic.Bool
	u.cfg.SendMail = func(string, string, string) error {
		mailing.Store(true)
		<-release
		return nil
	}
	u.state = EmptyState()
	u.state.Execution = Execution{Status: StatusPendingVerification, TargetTag: "v2.0.0", FromVersion: "1.0.0", BackupPath: filepath.Join(dir, "etherpad.bak")}

	done := make(chan struct{})
	go func() { u.rollback("health-check-timeout"); close(done) }()
	require.Eventually(t, mailing.Load, 5*time.Second, 5*time.Millisecond)
	// The mail hangs; the state is saved, and Status does not wait for it.
	assert.Equal(t, StatusRolledBack, LoadState(u.cfg.StatePath).Execution.Status)
	statusDone := make(chan Status, 1)
	go func() { statusDone <- u.Status() }()
	select {
	case st := <-statusDone:
		assert.Equal(t, StatusRolledBack, st.Execution.Status)
	case <-time.After(2 * time.Second):
		t.Fatal("Status blocked on the mail")
	}
	close(release)
	<-done
	assert.Equal(t, ExitCodeRestart, *exitCode)
}

// A notice counts as sent only once the server took it: after a failed send
// (or without a sender) it goes out at the next occasion.
func TestFailedMailIsSentAgain(t *testing.T) {
	u, _ := newTestUpdater(t, filepath.Join(t.TempDir(), "etherpad"))
	u.cfg.AdminEmail = func() string { return "admin@example.com" }
	var mu sync.Mutex
	fail := true
	var sent []string
	u.cfg.SendMail = func(_, subject, _ string) error {
		mu.Lock()
		defer mu.Unlock()
		if fail {
			return errors.New("connection refused")
		}
		sent = append(sent, subject)
		return nil
	}
	settle := func() {
		require.Eventually(t, func() bool {
			u.mailMu.Lock()
			defer u.mailMu.Unlock()
			return len(u.mailsInFlight) == 0
		}, 5*time.Second, 5*time.Millisecond)
	}
	u.state = EmptyState()
	release := ReleaseInfo{Version: "2.0.0", Tag: "v2.0.0"}

	u.recordResult(release, "failed", "download")
	settle()
	assert.Empty(t, LoadState(u.cfg.StatePath).Email.LastFailureKey, "not marked: it was not sent")

	mu.Lock()
	fail = false
	mu.Unlock()
	u.recordResult(release, "failed", "download")
	settle()
	u.recordResult(release, "failed", "download")
	settle()
	mu.Lock()
	assert.Len(t, sent, 1, "sent once it worked, and once only")
	mu.Unlock()
	assert.Equal(t, "failed:v2.0.0", LoadState(u.cfg.StatePath).Email.LastFailureKey)

	// Without a sender nothing is marked either.
	u2, _ := newTestUpdater(t, filepath.Join(t.TempDir(), "etherpad"))
	u2.cfg.AdminEmail = func() string { return "admin@example.com" }
	u2.state = EmptyState()
	u2.recordResult(release, "failed", "download")
	assert.Empty(t, u2.state.Email.LastFailureKey)

	// The "scheduled" notice is offered again at the next check while the
	// apply is scheduled.
	u3, _ := newTestUpdater(t, filepath.Join(t.TempDir(), "etherpad"))
	u3.cfg.AdminEmail = func() string { return "admin@example.com" }
	var grace atomic.Int32
	var graceFail atomic.Bool
	graceFail.Store(true)
	u3.cfg.SendMail = func(string, string, string) error {
		if graceFail.Load() {
			return errors.New("timeout")
		}
		grace.Add(1)
		return nil
	}
	u3.state = EmptyState()
	u3.state.Latest = &release
	u3.state.Execution = Execution{Status: StatusScheduled, TargetTag: "v2.0.0", ScheduledFor: time.Now().Add(time.Hour).UTC().Format(time.RFC3339)}
	u3.mu.Lock()
	u3.graceMailLocked()
	u3.mu.Unlock()
	require.Eventually(t, func() bool {
		u3.mailMu.Lock()
		defer u3.mailMu.Unlock()
		return len(u3.mailsInFlight) == 0
	}, 5*time.Second, 5*time.Millisecond)
	graceFail.Store(false)
	u3.mu.Lock()
	u3.graceMailLocked()
	u3.mu.Unlock()
	require.Eventually(t, func() bool { return grace.Load() == 1 }, 5*time.Second, 5*time.Millisecond)
	u3.mu.Lock()
	assert.Equal(t, "v2.0.0", u3.state.Email.GraceStartTag)
	u3.mu.Unlock()
}

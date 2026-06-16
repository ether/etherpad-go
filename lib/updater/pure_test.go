package updater

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestUpdateAvailable(t *testing.T) {
	cases := []struct {
		cur, lat string
		want     bool
	}{
		{"1.0.0", "1.0.1", true},
		{"1.0.0", "1.0.0", false},
		{"1.2.0", "1.1.9", false},
		{"deadbeef", "1.0.0", false}, // dev build, treated as up-to-date
		{"1.0.0", "", false},
	}
	for _, c := range cases {
		if got := updateAvailable(c.cur, c.lat); got != c.want {
			t.Errorf("updateAvailable(%q,%q)=%v want %v", c.cur, c.lat, got, c.want)
		}
	}
}

func TestIsMinorOrMoreBehind(t *testing.T) {
	cases := []struct {
		cur, lat string
		want     bool
	}{
		{"1.0.0", "1.0.9", false},
		{"1.0.0", "1.1.0", true},
		{"1.5.0", "2.0.0", true},
		{"2.0.0", "1.9.0", false},
	}
	for _, c := range cases {
		if got := isMinorOrMoreBehind(c.cur, c.lat); got != c.want {
			t.Errorf("isMinorOrMoreBehind(%q,%q)=%v want %v", c.cur, c.lat, got, c.want)
		}
	}
}

func TestParseWindow(t *testing.T) {
	if _, ok := ParseWindow("02:00", "04:00", "utc"); !ok {
		t.Error("valid window rejected")
	}
	if _, ok := ParseWindow("02:00", "02:00", "utc"); ok {
		t.Error("zero-length window accepted")
	}
	if _, ok := ParseWindow("25:00", "04:00", "utc"); ok {
		t.Error("invalid hour accepted")
	}
	if _, ok := ParseWindow("02:00", "04:00", "mars"); ok {
		t.Error("invalid tz accepted")
	}
}

func TestWindowInWindow(t *testing.T) {
	w, _ := ParseWindow("02:00", "04:00", "utc")
	in := time.Date(2026, 1, 1, 3, 0, 0, 0, time.UTC)
	out := time.Date(2026, 1, 1, 5, 0, 0, 0, time.UTC)
	if !w.InWindow(in) {
		t.Error("03:00 should be in 02:00-04:00")
	}
	if w.InWindow(out) {
		t.Error("05:00 should be outside 02:00-04:00")
	}

	cross, _ := ParseWindow("22:00", "04:00", "utc")
	if !cross.InWindow(time.Date(2026, 1, 1, 23, 0, 0, 0, time.UTC)) {
		t.Error("23:00 should be in cross-midnight 22:00-04:00")
	}
	if !cross.InWindow(time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC)) {
		t.Error("01:00 should be in cross-midnight 22:00-04:00")
	}
	if cross.InWindow(time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)) {
		t.Error("12:00 should be outside cross-midnight 22:00-04:00")
	}
}

func TestNextWindowStart(t *testing.T) {
	w, _ := ParseWindow("02:00", "04:00", "utc")
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	next := w.NextWindowStart(now)
	if next.Hour() != 2 || next.Day() != 2 {
		t.Errorf("expected next start 02:00 the following day, got %v", next)
	}
}

func TestEvaluatePolicy(t *testing.T) {
	latest := &ReleaseInfo{Version: "2.0.0", Tag: "v2.0.0"}

	// tier off
	if r := EvaluatePolicy(PolicyInput{Tier: TierOff, Latest: latest, CurrentVersion: "1.0.0"}); r.CanNotify || r.Reason != "tier-off" {
		t.Errorf("tier-off: %+v", r)
	}
	// up to date
	if r := EvaluatePolicy(PolicyInput{Tier: TierAuto, Latest: &ReleaseInfo{Version: "1.0.0"}, CurrentVersion: "1.0.0", InstallMethod: InstallBinary}); r.CanNotify || r.Reason != "up-to-date" {
		t.Errorf("up-to-date: %+v", r)
	}
	// notify tier, update available, binary
	if r := EvaluatePolicy(PolicyInput{Tier: TierNotify, Latest: latest, CurrentVersion: "1.0.0", InstallMethod: InstallBinary}); !r.CanNotify || r.CanManual || r.CanAuto {
		t.Errorf("notify: %+v", r)
	}
	// non-writable install even on auto: notify only
	if r := EvaluatePolicy(PolicyInput{Tier: TierAuto, Latest: latest, CurrentVersion: "1.0.0", InstallMethod: InstallDocker}); !r.CanNotify || r.CanAuto || r.Reason != "install-method-not-writable" {
		t.Errorf("docker auto: %+v", r)
	}
	// auto tier binary
	if r := EvaluatePolicy(PolicyInput{Tier: TierAuto, Latest: latest, CurrentVersion: "1.0.0", InstallMethod: InstallBinary}); !r.CanManual || !r.CanAuto || r.CanAutonomous {
		t.Errorf("auto binary: %+v", r)
	}
	// autonomous valid window
	if r := EvaluatePolicy(PolicyInput{Tier: TierAutonomous, Latest: latest, CurrentVersion: "1.0.0", InstallMethod: InstallBinary, WindowConfigured: true, WindowValid: true}); !r.CanAutonomous {
		t.Errorf("autonomous valid window: %+v", r)
	}
	// autonomous invalid window
	if r := EvaluatePolicy(PolicyInput{Tier: TierAutonomous, Latest: latest, CurrentVersion: "1.0.0", InstallMethod: InstallBinary, WindowConfigured: true, WindowValid: false}); r.CanAutonomous || r.Reason != "maintenance-window-invalid" {
		t.Errorf("autonomous invalid window: %+v", r)
	}
	// rollback-failed blocks auto, keeps manual
	if r := EvaluatePolicy(PolicyInput{Tier: TierAuto, Latest: latest, CurrentVersion: "1.0.0", InstallMethod: InstallBinary, ExecutionStatus: StatusRollbackFailed}); r.CanAuto || !r.CanManual || r.Reason != "rollback-failed-terminal" {
		t.Errorf("rollback-failed: %+v", r)
	}
}

func TestDecideSchedule(t *testing.T) {
	latest := &ReleaseInfo{Version: "2.0.0", Tag: "v2.0.0"}
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	// arm when idle
	d := DecideSchedule(ScheduleInput{CanAuto: true, Tier: TierAuto, Latest: latest, Execution: Execution{Status: StatusIdle}, Now: now, GraceMinutes: 30})
	if d.Action != ScheduleArm || d.TargetTag != "v2.0.0" || !d.ScheduledFor.Equal(now.Add(30*time.Minute)) {
		t.Errorf("arm: %+v", d)
	}
	// cancel when scheduled but auto revoked
	d = DecideSchedule(ScheduleInput{CanAuto: false, Latest: latest, Execution: Execution{Status: StatusScheduled, TargetTag: "v2.0.0"}, Now: now})
	if d.Action != ScheduleCancel {
		t.Errorf("cancel: %+v", d)
	}
	// none when already armed for same tag
	d = DecideSchedule(ScheduleInput{CanAuto: true, Tier: TierAuto, Latest: latest, Execution: Execution{Status: StatusScheduled, TargetTag: "v2.0.0"}, Now: now})
	if d.Action != ScheduleNone {
		t.Errorf("already-armed: %+v", d)
	}
	// autonomous snaps outside window to next window start
	w, _ := ParseWindow("02:00", "04:00", "utc")
	d = DecideSchedule(ScheduleInput{CanAuto: true, CanAutonomous: true, Tier: TierAutonomous, Latest: latest, Execution: Execution{Status: StatusIdle}, Now: now, GraceMinutes: 0, Window: w})
	if d.Action != ScheduleArm || d.ScheduledFor.Hour() != 2 {
		t.Errorf("autonomous snap: %+v", d)
	}
}

func TestDecideTriggerApply(t *testing.T) {
	latest := &ReleaseInfo{Version: "2.0.0", Tag: "v2.0.0"}
	now := time.Date(2026, 1, 1, 3, 0, 0, 0, time.UTC)
	sched := Execution{Status: StatusScheduled, TargetTag: "v2.0.0"}

	if d := DecideTriggerApply(TriggerInput{Execution: sched, TimerTargetTag: "v2.0.0", Latest: latest, CanAuto: true, Tier: TierAuto, Now: now}); d.Action != TriggerFire {
		t.Errorf("fire: %+v", d)
	}
	if d := DecideTriggerApply(TriggerInput{Execution: Execution{Status: StatusIdle}, TimerTargetTag: "v2.0.0", Latest: latest, CanAuto: true, Now: now}); d.Action != TriggerAbort {
		t.Errorf("abort on non-scheduled: %+v", d)
	}
	if d := DecideTriggerApply(TriggerInput{Execution: sched, TimerTargetTag: "v2.0.0", Latest: latest, CanAuto: false, Tier: TierAuto, Now: now}); d.Action != TriggerClear {
		t.Errorf("clear on revoked auto: %+v", d)
	}
	// autonomous outside window defers
	w, _ := ParseWindow("02:00", "04:00", "utc")
	noon := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	if d := DecideTriggerApply(TriggerInput{Execution: sched, TimerTargetTag: "v2.0.0", Latest: latest, CanAuto: true, CanAutonomous: true, Tier: TierAutonomous, Window: w, Now: noon}); d.Action != TriggerDefer || d.NextAt.Hour() != 2 {
		t.Errorf("defer outside window: %+v", d)
	}
}

func TestStateRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "var", "update-state.json")

	// Missing file -> empty state.
	if s := LoadState(path); s.Execution.Status != StatusIdle {
		t.Errorf("missing file should be idle, got %q", s.Execution.Status)
	}

	want := EmptyState()
	want.LastETag = "abc"
	want.Latest = &ReleaseInfo{Version: "2.0.0", Tag: "v2.0.0"}
	want.Execution = Execution{Status: StatusScheduled, TargetTag: "v2.0.0"}
	if err := SaveState(path, want); err != nil {
		t.Fatal(err)
	}
	got := LoadState(path)
	if got.LastETag != "abc" || got.Latest == nil || got.Latest.Tag != "v2.0.0" || got.Execution.Status != StatusScheduled {
		t.Errorf("round-trip mismatch: %+v", got)
	}

	// Corrupt file -> empty state.
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if s := LoadState(path); s.Execution.Status != StatusIdle {
		t.Errorf("corrupt file should reset to idle, got %q", s.Execution.Status)
	}
}

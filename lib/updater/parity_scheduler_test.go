package updater

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Port of etherpad-lite's Scheduler spec (decideSchedule and
// decideTriggerApply; the runner's timer is tested in updater_test.go).
// Its grace-start emails have no counterpart: etherpad-go sends no mail.

var schedNow = utc("2026-05-11T12:00:00Z")

func rel(tag string) *ReleaseInfo { return &ReleaseInfo{Tag: tag, Version: tag[1:]} }

func sched(mod func(*ScheduleInput)) ScheduleDecision {
	in := ScheduleInput{CanAuto: true, Tier: TierAuto, Latest: rel("v2.0.1"), Execution: Execution{Status: StatusIdle}, Now: schedNow, GraceMinutes: 15}
	mod(&in)
	return DecideSchedule(in)
}

func TestParityDecideSchedule(t *testing.T) {
	none := ScheduleDecision{Action: ScheduleNone}
	assert.Equal(t, none, sched(func(i *ScheduleInput) { i.Latest = nil }), "no latest release")
	assert.Equal(t, none, sched(func(i *ScheduleInput) { i.CanAuto = false }), "auto off, nothing scheduled")
	assert.Equal(t, ScheduleDecision{Action: ScheduleArm, TargetTag: "v2.0.1", ScheduledFor: schedNow.Add(15 * time.Minute)},
		sched(func(*ScheduleInput) {}), "from idle")

	assert.Equal(t, none, sched(func(i *ScheduleInput) {
		i.Execution = Execution{Status: StatusScheduled, TargetTag: "v2.0.1", ScheduledFor: "2026-05-11T12:10:00Z"}
	}), "the same tag stays scheduled (a restart inside the grace period)")
	assert.Equal(t, ScheduleDecision{Action: ScheduleArm, TargetTag: "v2.0.2", ScheduledFor: schedNow.Add(15 * time.Minute)},
		sched(func(i *ScheduleInput) {
			i.Latest = rel("v2.0.2")
			i.Execution = Execution{Status: StatusScheduled, TargetTag: "v2.0.1", ScheduledFor: "2026-05-11T12:10:00Z"}
		}), "a newer tag during the grace period")
	assert.Equal(t, ScheduleDecision{Action: ScheduleCancel}, sched(func(i *ScheduleInput) {
		i.CanAuto = false
		i.Execution = Execution{Status: StatusScheduled, TargetTag: "v2.0.1"}
	}), "the policy no longer allows auto")

	for _, st := range []ExecutionStatus{StatusPreflight, StatusDraining, StatusExecuting, StatusRollingBack, StatusPendingVerification} {
		assert.Equal(t, none, sched(func(i *ScheduleInput) { i.Execution = Execution{Status: st, TargetTag: "v2.0.1"} }), "in flight: %s", st)
	}
	for _, st := range []ExecutionStatus{StatusPreflightFailed, StatusRolledBack, StatusRollbackFailed} {
		assert.Equal(t, none, sched(func(i *ScheduleInput) { i.Execution = Execution{Status: st, TargetTag: "v2.0.1"} }), "terminal: %s", st)
	}
	for _, outcome := range []string{"failed", "preflight-failed", "rolled-back", "rollback-failed"} {
		assert.Equal(t, none, sched(func(i *ScheduleInput) {
			i.LastResult = &LastResult{TargetTag: "v2.0.1", Outcome: outcome}
		}), "the release failed before (%s): no automatic retry loop", outcome)
		assert.Equal(t, ScheduleArm, sched(func(i *ScheduleInput) {
			i.Latest = rel("v2.0.2")
			i.Execution = Execution{Status: StatusRolledBack, TargetTag: "v2.0.1"}
			i.LastResult = &LastResult{TargetTag: "v2.0.1", Outcome: outcome}
		}).Action, "a newer release after a failed one (%s)", outcome)
	}
	assert.Equal(t, "v2.0.2", sched(func(i *ScheduleInput) {
		i.Latest = rel("v2.0.2")
		i.Execution = Execution{Status: StatusVerified, TargetTag: "v2.0.1"}
		i.LastResult = &LastResult{TargetTag: "v2.0.1", Outcome: "verified"}
	}).TargetTag, "after a verified update")

	for _, m := range []int{-5, 0} {
		assert.Equal(t, schedNow, sched(func(i *ScheduleInput) { i.GraceMinutes = m }).ScheduledFor, "grace %d", m)
	}
	assert.Equal(t, schedNow.Add(7*24*time.Hour), sched(func(i *ScheduleInput) { i.GraceMinutes = 99999 }).ScheduledFor, "grace capped at 7 days")
}

func TestParityScheduleWindow(t *testing.T) {
	w, _ := ParseWindow("03:00", "05:00", "utc")
	auto := func(i *ScheduleInput) { i.Tier, i.CanAutonomous, i.Window = TierAutonomous, true, w }

	assert.Equal(t, utc("2026-05-12T03:00:00Z"), sched(auto).ScheduledFor, "snapped to the next window")
	assert.Equal(t, utc("2026-05-12T03:15:00Z"), sched(func(i *ScheduleInput) {
		auto(i)
		i.Now = utc("2026-05-12T03:00:00Z")
	}).ScheduledFor, "now+grace inside the window stays")
	assert.Equal(t, schedNow.Add(15*time.Minute), sched(func(i *ScheduleInput) {
		auto(i)
		i.CanAutonomous = false
	}).ScheduledFor, "without autonomous the window is ignored")
}

func TestParityDecideTriggerApply(t *testing.T) {
	scheduled := Execution{Status: StatusScheduled, TargetTag: "v2.0.1", ScheduledFor: "2026-05-11T12:15:00Z"}
	trig := func(mod func(*TriggerInput)) TriggerDecision {
		in := TriggerInput{Execution: scheduled, TimerTargetTag: "v2.0.1", Latest: rel("v2.0.1"), CanAuto: true, Tier: TierAuto, Now: schedNow}
		mod(&in)
		return DecideTriggerApply(in)
	}
	assert.Equal(t, TriggerFire, trig(func(*TriggerInput) {}).Action)
	assert.Equal(t, TriggerAbort, trig(func(i *TriggerInput) { i.Execution = Execution{Status: StatusIdle} }).Action, "an admin cancelled at the boundary")
	assert.Equal(t, TriggerAbort, trig(func(i *TriggerInput) { i.Execution.TargetTag = "v2.0.2" }).Action, "a manual apply overwrote it")
	assert.Equal(t, TriggerAbort, trig(func(i *TriggerInput) { i.Latest = nil }).Action)
	assert.Equal(t, TriggerAbort, trig(func(i *TriggerInput) { i.Latest = rel("v2.0.2") }).Action, "the timer is for an older release")
	assert.Equal(t, TriggerClear, trig(func(i *TriggerInput) { i.CanAuto = false }).Action, "tier changed during the grace period")

	w, _ := ParseWindow("03:00", "05:00", "utc")
	assert.Equal(t, TriggerDecision{Action: TriggerDefer, NextAt: utc("2026-05-12T03:00:00Z")}, trig(func(i *TriggerInput) {
		i.Tier, i.CanAutonomous, i.Window = TierAutonomous, true, w
	}), "outside the window")
	assert.Equal(t, TriggerFire, trig(func(i *TriggerInput) {
		i.Tier, i.CanAutonomous, i.Window = TierAutonomous, true, w
		i.Now = utc("2026-05-12T03:30:00Z")
	}).Action, "inside the window")
}

// The runner arms one timer, replaces it on re-arm, fires at once for a
// time in the past, and cancel is idempotent.
func TestParitySchedulerTimer(t *testing.T) {
	u, _ := newTestUpdater(t, t.TempDir()+"/etherpad")
	u.state = EmptyState()
	u.mu.Lock()
	u.armApplyTimerLocked("v2.0.1", time.Now().Add(time.Hour))
	first := u.applyTimer
	u.armApplyTimerLocked("v2.0.2", time.Now().Add(time.Hour))
	assert.NotSame(t, first, u.applyTimer)
	assert.False(t, first.Stop(), "the first timer was stopped on re-arm")
	assert.Equal(t, "v2.0.2", u.timerTargetTag)
	u.cancelApplyTimerLocked()
	u.cancelApplyTimerLocked()
	assert.Nil(t, u.applyTimer)
	assert.Equal(t, "", u.timerTargetTag)
	u.mu.Unlock()
}

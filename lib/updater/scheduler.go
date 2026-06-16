package updater

import "time"

// inFlight reports whether an apply attempt is mid-pipeline and must not be
// re-scheduled or re-triggered.
func (s ExecutionStatus) inFlight() bool {
	switch s {
	case StatusPreflight, StatusDraining, StatusExecuting, StatusPendingVerification, StatusRollingBack:
		return true
	default:
		return false
	}
}

// ScheduleAction is the outcome of DecideSchedule.
type ScheduleAction string

const (
	ScheduleNone   ScheduleAction = "none"
	ScheduleArm    ScheduleAction = "schedule"
	ScheduleCancel ScheduleAction = "cancel"
)

// ScheduleDecision tells the runner what to do with its timer.
type ScheduleDecision struct {
	Action       ScheduleAction
	TargetTag    string
	ScheduledFor time.Time
}

// ScheduleInput is the pure input to DecideSchedule (called after each check).
type ScheduleInput struct {
	CanAuto       bool
	CanAutonomous bool
	Tier          Tier
	Latest        *ReleaseInfo
	Execution     Execution
	Now           time.Time
	GraceMinutes  int
	Window        *MaintenanceWindow
}

// DecideSchedule decides whether to arm, cancel or leave the auto-apply timer.
func DecideSchedule(in ScheduleInput) ScheduleDecision {
	if in.Latest == nil || !in.CanAuto {
		if in.Execution.Status == StatusScheduled {
			return ScheduleDecision{Action: ScheduleCancel}
		}
		return ScheduleDecision{Action: ScheduleNone}
	}
	// Let in-flight or terminal attempts finish; never re-schedule over them.
	if in.Execution.Status.inFlight() || in.Execution.Status == StatusRollbackFailed {
		return ScheduleDecision{Action: ScheduleNone}
	}
	// Already armed for this exact target.
	if in.Execution.Status == StatusScheduled && in.Execution.TargetTag == in.Latest.Tag {
		return ScheduleDecision{Action: ScheduleNone}
	}

	scheduledFor := in.Now.Add(time.Duration(in.GraceMinutes) * time.Minute)
	// Autonomous tier: never land outside the maintenance window.
	if in.Tier == TierAutonomous && in.CanAutonomous && in.Window != nil && !in.Window.InWindow(scheduledFor) {
		scheduledFor = in.Window.NextWindowStart(scheduledFor)
	}
	return ScheduleDecision{Action: ScheduleArm, TargetTag: in.Latest.Tag, ScheduledFor: scheduledFor}
}

// TriggerAction is the outcome of DecideTriggerApply.
type TriggerAction string

const (
	TriggerFire  TriggerAction = "fire"  // proceed with the apply
	TriggerDefer TriggerAction = "defer" // re-arm for NextAt (outside maintenance window)
	TriggerAbort TriggerAction = "abort" // state changed; do nothing
	TriggerClear TriggerAction = "clear" // policy revoked auto; clear the schedule
)

// TriggerDecision tells the runner what to do when its timer fires.
type TriggerDecision struct {
	Action TriggerAction
	NextAt time.Time
}

// TriggerInput is the pure input to DecideTriggerApply (called on timer fire).
type TriggerInput struct {
	Execution      Execution
	TimerTargetTag string // tag the timer was armed for
	Latest         *ReleaseInfo
	CanAuto        bool
	CanAutonomous  bool
	Tier           Tier
	Window         *MaintenanceWindow
	Now            time.Time
}

// DecideTriggerApply decides whether a fired auto-apply timer should proceed.
func DecideTriggerApply(in TriggerInput) TriggerDecision {
	if in.Execution.Status != StatusScheduled {
		return TriggerDecision{Action: TriggerAbort}
	}
	if in.Latest == nil || in.Execution.TargetTag != in.TimerTargetTag || in.Latest.Tag != in.TimerTargetTag {
		return TriggerDecision{Action: TriggerAbort}
	}
	if !in.CanAuto {
		return TriggerDecision{Action: TriggerClear}
	}
	if in.Tier == TierAutonomous && in.Window != nil && !in.Window.InWindow(in.Now) {
		return TriggerDecision{Action: TriggerDefer, NextAt: in.Window.NextWindowStart(in.Now)}
	}
	return TriggerDecision{Action: TriggerFire}
}

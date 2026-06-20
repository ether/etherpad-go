package updater

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// maxBoots is the number of consecutive boots in pending-verification before a
// crash loop is assumed and the update is rolled back.
const maxBoots = 3

// Config is the fully-resolved updater configuration (built from settings by
// the server package, keeping this package settings-free and testable).
type Config struct {
	Tier               Tier
	Repo               string
	CurrentVersion     string
	CheckInterval      time.Duration
	GraceMinutes       int
	DrainSeconds       int
	HealthCheckSeconds int
	InstallMethod      InstallMethod // override; auto-detected when InstallAuto
	Window             *MaintenanceWindow
	WindowConfigured   bool
	Verifier           SignatureVerifier
	StatePath          string
	LockPath           string
}

// Updater orchestrates checking, scheduling, applying and verifying updates.
type Updater struct {
	cfg           Config
	logger        *zap.SugaredLogger
	checker       *VersionChecker
	executor      *Executor
	exePath       string
	installMethod InstallMethod

	mu             sync.Mutex
	state          UpdateState
	applyTimer     *time.Timer
	healthTimer    *time.Timer
	timerTargetTag string

	accepting atomic.Bool
	applying  atomic.Bool

	broadcast func(secsLeft int)
	now       func() time.Time
	exit      func(int)

	done     chan struct{}
	stopOnce sync.Once
}

// New builds an updater. broadcast may be nil.
func New(cfg Config, logger *zap.SugaredLogger, broadcast func(int)) *Updater {
	exe, err := os.Executable()
	if err != nil {
		exe = os.Args[0]
	}
	checker := NewVersionChecker(cfg.Repo)
	u := &Updater{
		cfg:       cfg,
		logger:    logger,
		checker:   checker,
		executor:  NewExecutor(checker, exe, cfg.Verifier, logger),
		exePath:   exe,
		broadcast: broadcast,
		now:       time.Now,
		exit:      os.Exit,
		done:      make(chan struct{}),
	}
	u.accepting.Store(true)
	return u
}

// IsAcceptingConnections reports whether new pad connections should be allowed.
// It is false only while draining for an update.
func (u *Updater) IsAcceptingConnections() bool { return u.accepting.Load() }

func (u *Updater) setAccepting(v bool) { u.accepting.Store(v) }

func (u *Updater) nowStr() string { return u.now().UTC().Format(time.RFC3339) }

// Start detects the install method, loads persisted state, resolves any
// pending verification from a previous restart, and begins polling.
func (u *Updater) Start() {
	u.installMethod = DetectInstallMethod(u.cfg.InstallMethod)
	u.mu.Lock()
	u.state = LoadState(u.cfg.StatePath)
	u.mu.Unlock()

	u.checkPendingVerification()

	if u.cfg.Tier == TierOff {
		u.logger.Info("updater: tier=off; not checking for updates")
		return
	}
	u.logger.Infof("updater: tier=%s installMethod=%s repo=%s", u.cfg.Tier, u.installMethod, u.cfg.Repo)
	go u.pollLoop()
}

// Stop halts polling and any armed timers.
func (u *Updater) Stop() {
	u.stopOnce.Do(func() { close(u.done) })
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.applyTimer != nil {
		u.applyTimer.Stop()
		u.applyTimer = nil
	}
	if u.healthTimer != nil {
		u.healthTimer.Stop()
		u.healthTimer = nil
	}
}

func (u *Updater) pollLoop() {
	u.performCheck()
	ticker := time.NewTicker(u.cfg.CheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-u.done:
			return
		case <-ticker.C:
			u.performCheck()
		}
	}
}

func (u *Updater) saveLocked() {
	if err := SaveState(u.cfg.StatePath, u.state); err != nil && u.logger != nil {
		u.logger.Warnf("updater: failed to persist state: %v", err)
	}
}

func (u *Updater) policyLocked() PolicyResult {
	return EvaluatePolicy(PolicyInput{
		Tier:             u.cfg.Tier,
		InstallMethod:    u.installMethod,
		CurrentVersion:   u.cfg.CurrentVersion,
		Latest:           u.state.Latest,
		ExecutionStatus:  u.state.Execution.Status,
		WindowConfigured: u.cfg.WindowConfigured,
		WindowValid:      u.cfg.Window != nil,
	})
}

// performCheck polls GitHub, updates state, and (re)schedules an auto-apply.
func (u *Updater) performCheck() {
	res := u.checker.CheckLatest(u.currentETag())

	u.mu.Lock()
	defer u.mu.Unlock()
	u.state.LastCheckAt = u.nowStr()

	switch res.Kind {
	case CheckUpdated:
		u.state.LastETag = res.ETag
		u.state.Latest = res.Release
		if updateAvailable(u.cfg.CurrentVersion, res.Release.Version) {
			u.logger.Infof("updater: new version %s available (current %s)", res.Release.Version, u.cfg.CurrentVersion)
		}
	case CheckPrerelease:
		if res.ETag != "" {
			u.state.LastETag = res.ETag
		}
	case CheckNotModified:
		// nothing changed
	case CheckRateLimited:
		u.logger.Warn("updater: GitHub rate limited the release check")
		u.saveLocked()
		return
	case CheckError:
		u.logger.Warnf("updater: release check failed (status %d)", res.Status)
		u.saveLocked()
		return
	}

	u.decideScheduleLocked()
	u.saveLocked()
}

func (u *Updater) currentETag() string {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.state.LastETag
}

func (u *Updater) decideScheduleLocked() {
	pol := u.policyLocked()
	d := DecideSchedule(ScheduleInput{
		CanAuto:       pol.CanAuto,
		CanAutonomous: pol.CanAutonomous,
		Tier:          u.cfg.Tier,
		Latest:        u.state.Latest,
		Execution:     u.state.Execution,
		Now:           u.now(),
		GraceMinutes:  u.cfg.GraceMinutes,
		Window:        u.cfg.Window,
	})
	switch d.Action {
	case ScheduleArm:
		u.state.Execution = Execution{
			Status:       StatusScheduled,
			TargetTag:    d.TargetTag,
			ScheduledFor: d.ScheduledFor.UTC().Format(time.RFC3339),
			At:           u.nowStr(),
		}
		u.armApplyTimerLocked(d.TargetTag, d.ScheduledFor)
		u.logger.Infof("updater: scheduled auto-apply of %s for %s", d.TargetTag, d.ScheduledFor.Format(time.RFC3339))
	case ScheduleCancel:
		u.cancelApplyTimerLocked()
		u.state.Execution = Execution{Status: StatusIdle}
	case ScheduleNone:
	}
}

func (u *Updater) armApplyTimerLocked(targetTag string, at time.Time) {
	if u.applyTimer != nil {
		u.applyTimer.Stop()
	}
	u.timerTargetTag = targetTag
	delay := max(at.Sub(u.now()), 0)
	u.applyTimer = time.AfterFunc(delay, u.onApplyTimer)
}

func (u *Updater) cancelApplyTimerLocked() {
	if u.applyTimer != nil {
		u.applyTimer.Stop()
		u.applyTimer = nil
	}
	u.timerTargetTag = ""
}

func (u *Updater) onApplyTimer() {
	u.mu.Lock()
	pol := u.policyLocked()
	dec := DecideTriggerApply(TriggerInput{
		Execution:      u.state.Execution,
		TimerTargetTag: u.timerTargetTag,
		Latest:         u.state.Latest,
		CanAuto:        pol.CanAuto,
		CanAutonomous:  pol.CanAutonomous,
		Tier:           u.cfg.Tier,
		Window:         u.cfg.Window,
		Now:            u.now(),
	})
	target := u.state.Latest
	u.mu.Unlock()

	switch dec.Action {
	case TriggerFire:
		if target != nil {
			u.runApply(*target)
		}
	case TriggerDefer:
		u.mu.Lock()
		u.armApplyTimerLocked(u.timerTargetTag, dec.NextAt)
		u.mu.Unlock()
		u.logger.Infof("updater: deferring auto-apply to next maintenance window at %s", dec.NextAt.Format(time.RFC3339))
	case TriggerClear:
		u.mu.Lock()
		u.cancelApplyTimerLocked()
		u.state.Execution = Execution{Status: StatusIdle}
		u.saveLocked()
		u.mu.Unlock()
	case TriggerAbort:
	}
}

func (u *Updater) setExecution(exec Execution) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.state.Execution = exec
	u.saveLocked()
}

func (u *Updater) recordResult(target ReleaseInfo, outcome, reason string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.state.LastResult = &LastResult{
		TargetTag:   target.Tag,
		FromVersion: u.cfg.CurrentVersion,
		Outcome:     outcome,
		Reason:      reason,
		At:          u.nowStr(),
	}
	u.saveLocked()
}

// runApply executes the full apply pipeline: lock → preflight → drain →
// execute → (success) pending-verification + exit(75). It is safe to call from
// the auto-apply timer or a manual trigger; only one runs at a time.
func (u *Updater) runApply(target ReleaseInfo) {
	if !u.applying.CompareAndSwap(false, true) {
		u.logger.Warn("updater: apply already in progress, ignoring")
		return
	}
	defer u.applying.Store(false)

	ok, err := acquireLock(u.cfg.LockPath)
	if err != nil || !ok {
		u.logger.Warn("updater: could not acquire apply lock; another instance may be updating")
		return
	}
	defer releaseLock(u.cfg.LockPath)

	if pf := RunPreflight(PreflightDeps{InstallMethod: u.installMethod, ExePath: u.exePath}); !pf.OK {
		u.logger.Warnf("updater: preflight failed: %s", pf.Reason)
		u.setExecution(Execution{Status: StatusPreflightFailed, TargetTag: target.Tag, Reason: pf.Reason, At: u.nowStr()})
		u.recordResult(target, "preflight-failed", pf.Reason)
		return
	}

	drainEnds := u.now().Add(time.Duration(u.cfg.DrainSeconds) * time.Second)
	u.setExecution(Execution{Status: StatusDraining, TargetTag: target.Tag, DrainEndsAt: drainEnds.UTC().Format(time.RFC3339), At: u.nowStr()})
	u.logger.Infof("updater: draining connections for %ds before applying %s", u.cfg.DrainSeconds, target.Tag)
	if NewDrainer(u.cfg.DrainSeconds, u.setAccepting, u.broadcast).Start() == DrainCancelled {
		u.logger.Warn("updater: drain cancelled; aborting apply")
		u.setExecution(Execution{Status: StatusIdle})
		return
	}

	u.setExecution(Execution{Status: StatusExecuting, TargetTag: target.Tag, FromVersion: u.cfg.CurrentVersion, At: u.nowStr()})
	res := u.executor.Apply(target)
	if !res.OK {
		// The binary is only swapped on full success, so a failure here leaves
		// the running binary untouched — no rollback needed.
		u.logger.Errorf("updater: apply failed: %s", res.Reason)
		u.recordResult(target, "failed", res.Reason)
		u.setExecution(Execution{Status: StatusIdle})
		return
	}

	deadline := u.now().Add(time.Duration(u.cfg.HealthCheckSeconds) * time.Second)
	u.setExecution(Execution{
		Status:      StatusPendingVerification,
		TargetTag:   target.Tag,
		FromVersion: u.cfg.CurrentVersion,
		BackupPath:  res.BackupPath,
		DeadlineAt:  deadline.UTC().Format(time.RFC3339),
		At:          u.nowStr(),
	})
	u.logger.Infof("updater: applied %s; restarting (exit %d) to verify", target.Tag, ExitCodeRestart)
	u.exit(ExitCodeRestart)
}

// checkPendingVerification runs at boot. If the previous run swapped in a new
// binary and is awaiting verification, it arms a health-check timer; after
// maxBoots consecutive boots in this state it assumes a crash loop and rolls
// back.
func (u *Updater) checkPendingVerification() {
	u.mu.Lock()
	if u.state.Execution.Status != StatusPendingVerification {
		u.mu.Unlock()
		return
	}
	u.state.BootCount++
	bootCount := u.state.BootCount
	u.saveLocked()
	u.mu.Unlock()

	if bootCount > maxBoots {
		u.logger.Warn("updater: crash loop detected after update; rolling back")
		u.rollback("health-check-failed-or-crash-loop")
		return
	}

	secs := u.cfg.HealthCheckSeconds
	if secs <= 0 {
		secs = 60
	}
	u.logger.Infof("updater: awaiting healthy boot to verify update (timeout %ds, attempt %d/%d)", secs, bootCount, maxBoots)
	u.mu.Lock()
	u.healthTimer = time.AfterFunc(time.Duration(secs)*time.Second, func() {
		u.logger.Warn("updater: health-check deadline passed; rolling back")
		u.rollback("health-check-timeout")
	})
	u.mu.Unlock()
}

// MarkBootHealthy is called once the server is up and serving. It confirms a
// pending update as verified and cancels the rollback timer.
func (u *Updater) MarkBootHealthy() {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.state.Execution.Status != StatusPendingVerification {
		return
	}
	if u.healthTimer != nil {
		u.healthTimer.Stop()
		u.healthTimer = nil
	}
	exec := u.state.Execution
	u.state.Execution = Execution{Status: StatusVerified, TargetTag: exec.TargetTag, At: u.nowStr()}
	u.state.BootCount = 0
	u.state.LastResult = &LastResult{TargetTag: exec.TargetTag, FromVersion: exec.FromVersion, Outcome: "verified", At: u.nowStr()}
	u.saveLocked()
	u.logger.Infof("updater: update to %s verified", exec.TargetTag)
}

func (u *Updater) rollback(reason string) {
	u.mu.Lock()
	exec := u.state.Execution
	u.state.Execution = Execution{Status: StatusRollingBack, TargetTag: exec.TargetTag, FromVersion: exec.FromVersion, BackupPath: exec.BackupPath, Reason: reason, At: u.nowStr()}
	u.saveLocked()
	u.mu.Unlock()

	err := u.executor.Rollback(exec.BackupPath)

	u.mu.Lock()
	if err != nil {
		u.state.Execution = Execution{Status: StatusRollbackFailed, TargetTag: exec.TargetTag, Reason: err.Error(), At: u.nowStr()}
		u.state.LastResult = &LastResult{TargetTag: exec.TargetTag, FromVersion: exec.FromVersion, Outcome: "rollback-failed", Reason: err.Error(), At: u.nowStr()}
		u.saveLocked()
		u.mu.Unlock()
		u.logger.Errorf("updater: rollback failed (manual intervention required): %v", err)
		u.exit(ExitCodeRestart)
		return
	}
	u.state.Execution = Execution{Status: StatusRolledBack, TargetTag: exec.TargetTag, Reason: reason, At: u.nowStr()}
	u.state.BootCount = 0
	u.state.LastResult = &LastResult{TargetTag: exec.TargetTag, FromVersion: exec.FromVersion, Outcome: "rolled-back", Reason: reason, At: u.nowStr()}
	u.saveLocked()
	u.mu.Unlock()
	u.logger.Warn("updater: rolled back to previous binary; restarting")
	u.exit(ExitCodeRestart)
}

// ---- admin-facing API ----

// CheckNow triggers an immediate release check in the background.
func (u *Updater) CheckNow() { go u.performCheck() }

// CheckNowSync runs a release check synchronously and returns the new status.
// Used by the admin UI's "check for updates" action.
func (u *Updater) CheckNowSync() Status {
	u.performCheck()
	return u.Status()
}

// ApplyNow manually triggers an apply of the latest known release. It returns
// an error if policy forbids it or no release is known.
func (u *Updater) ApplyNow() error {
	u.mu.Lock()
	pol := u.policyLocked()
	target := u.state.Latest
	u.mu.Unlock()
	if !pol.CanManual {
		return fmt.Errorf("manual apply not permitted: %s", pol.Reason)
	}
	if target == nil {
		return errors.New("no known release to apply")
	}
	go u.runApply(*target)
	return nil
}

// Acknowledge clears the terminal rollback-failed state so automatic applies
// can resume once an admin has resolved the underlying problem. It returns an
// error if there is nothing to acknowledge.
func (u *Updater) Acknowledge() error {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.state.Execution.Status != StatusRollbackFailed {
		return errors.New("no rollback-failed state to acknowledge")
	}
	u.state.Execution = Execution{Status: StatusIdle}
	u.state.BootCount = 0
	u.saveLocked()
	u.logger.Info("updater: rollback-failed state acknowledged; auto-apply re-enabled")
	return nil
}

// Status is a snapshot for the admin UI.
type Status struct {
	Tier            Tier          `json:"tier"`
	InstallMethod   InstallMethod `json:"installMethod"`
	CurrentVersion  string        `json:"currentVersion"`
	Latest          *ReleaseInfo  `json:"latest,omitempty"`
	UpdateAvailable bool          `json:"updateAvailable"`
	Execution       Execution     `json:"execution"`
	LastResult      *LastResult   `json:"lastResult,omitempty"`
	Policy          PolicyResult  `json:"policy"`
}

// Status returns the current updater status.
func (u *Updater) Status() Status {
	u.mu.Lock()
	defer u.mu.Unlock()
	avail := u.state.Latest != nil && updateAvailable(u.cfg.CurrentVersion, u.state.Latest.Version)
	return Status{
		Tier:            u.cfg.Tier,
		InstallMethod:   u.installMethod,
		CurrentVersion:  u.cfg.CurrentVersion,
		Latest:          u.state.Latest,
		UpdateAvailable: avail,
		Execution:       u.state.Execution,
		LastResult:      u.state.LastResult,
		Policy:          u.policyLocked(),
	}
}

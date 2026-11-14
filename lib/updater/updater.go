package updater

import (
	"errors"
	"fmt"
	"os"
	"strings"
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
	// LogPath is the update log shown on the admin page; "" keeps none.
	LogPath string
	// AdminEmail is read at every notice (settings can change); nil or ""
	// sends none. SendMail delivers one (lib/mail).
	AdminEmail func() string
	SendMail   func(to, subject, body string) error
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
	drainer        *Drainer // set while an apply drains, for Cancel
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
	if cfg.LogPath != "" {
		logger = withUpdateLog(logger, cfg.LogPath)
	}
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
	u.resumeLocked()

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

	if u.state.Latest != nil {
		to := u.adminEmail()
		mails, log := DecideEmails(to, u.cfg.CurrentVersion, u.state.Latest.Version, u.now(), u.state.Email)
		u.state.Email = log
		u.sendMails(to, mails, false)
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
		LastResult:    u.state.LastResult,
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
		if u.state.Latest != nil {
			to := u.adminEmail()
			mails, log := DecideGraceStartEmail(to, u.cfg.CurrentVersion, *u.state.Latest, d.ScheduledFor, u.state.Email)
			u.state.Email = log
			u.sendMails(to, mails, false)
		}
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
	u.outcomeMailLocked(outcome, reason, target.Tag, false)
	u.saveLocked()
}

func (u *Updater) adminEmail() string {
	if u.cfg.AdminEmail == nil {
		return ""
	}
	return strings.TrimSpace(u.cfg.AdminEmail())
}

// mailWait bounds how long a restart waits for its mail to go out.
const mailWait = 30 * time.Second

// sendMails delivers mails in the background; with wait (the process is
// about to exit) it waits for them, at most mailWait. A failed send is
// logged by the mailer and never stops the updater.
func (u *Updater) sendMails(to string, mails []Email, wait bool) {
	if len(mails) == 0 || u.cfg.SendMail == nil {
		return
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for _, m := range mails {
			if err := u.cfg.SendMail(to, m.Subject, m.Body); err != nil {
				u.logger.Warnf("updater: the %s mail was not sent: %v", m.Kind, err)
			}
		}
	}()
	if wait {
		select {
		case <-done:
		case <-time.After(mailWait):
		}
	}
}

// outcomeMailLocked mails the admin about a failed apply (deduplicated).
func (u *Updater) outcomeMailLocked(outcome, reason, tag string, wait bool) {
	if outcome == "verified" {
		return
	}
	to := u.adminEmail()
	mails, log := DecideOutcomeEmail(to, outcome, reason, tag, u.cfg.CurrentVersion, u.state.Email)
	u.state.Email = log
	u.sendMails(to, mails, wait)
}

// resumeLocked picks up what the previous run left in the state file:
//   - a scheduled apply is armed again for its time (a check finds it
//     "already scheduled" and would never arm the lost timer);
//   - an apply that was preflighting, draining or executing when the
//     process died is over: the running binary is still the old one, so it
//     is recorded as failed and the state goes back to idle instead of
//     blocking every later schedule.
func (u *Updater) resumeLocked() {
	u.mu.Lock()
	defer u.mu.Unlock()
	exec := u.state.Execution
	switch exec.Status {
	case StatusScheduled:
		at, err := time.Parse(time.RFC3339, exec.ScheduledFor)
		if err != nil {
			u.state.Execution = Execution{Status: StatusIdle}
			u.saveLocked()
			return
		}
		u.armApplyTimerLocked(exec.TargetTag, at)
		u.logger.Infof("updater: resuming scheduled apply of %s for %s", exec.TargetTag, exec.ScheduledFor)
	case StatusPreflight, StatusDraining, StatusExecuting:
		u.logger.Warnf("updater: the apply of %s was interrupted (%s); the old version keeps running", exec.TargetTag, exec.Status)
		u.state.LastResult = &LastResult{TargetTag: exec.TargetTag, FromVersion: u.cfg.CurrentVersion, Outcome: "failed", Reason: "interrupted-" + string(exec.Status), At: u.nowStr()}
		u.outcomeMailLocked("failed", "interrupted-"+string(exec.Status), exec.TargetTag, false)
		u.state.Execution = Execution{Status: StatusIdle}
		u.saveLocked()
	}
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
	drainer := NewDrainer(u.cfg.DrainSeconds, u.setAccepting, u.broadcast)
	u.mu.Lock()
	u.drainer = drainer
	u.mu.Unlock()
	result := drainer.Start()
	u.mu.Lock()
	u.drainer = nil
	u.mu.Unlock()
	if result == DrainCancelled {
		u.logger.Warn("updater: drain cancelled; aborting apply")
		if u.broadcast != nil {
			u.broadcast(DrainCancelledNotice)
		}
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
	// Only the target version can verify itself or roll back to the backup:
	// when an admin has installed another version by hand meanwhile, a
	// rollback would replace it with the old binary. That is left to the
	// admin (Acknowledge), as in etherpad-lite.
	if running := strings.TrimPrefix(u.cfg.CurrentVersion, "v"); running != strings.TrimPrefix(u.state.Execution.TargetTag, "v") {
		u.mu.Unlock()
		u.logger.Warnf("updater: an update to %s awaits verification, but %s is running; leaving it to an admin", u.state.Execution.TargetTag, u.cfg.CurrentVersion)
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
		u.outcomeMailLocked("rollback-failed", err.Error(), exec.TargetTag, true)
		u.saveLocked()
		u.mu.Unlock()
		u.logger.Errorf("updater: rollback failed (manual intervention required): %v", err)
		u.exit(ExitCodeRestart)
		return
	}
	u.state.Execution = Execution{Status: StatusRolledBack, TargetTag: exec.TargetTag, Reason: reason, At: u.nowStr()}
	u.state.BootCount = 0
	u.state.LastResult = &LastResult{TargetTag: exec.TargetTag, FromVersion: exec.FromVersion, Outcome: "rolled-back", Reason: reason, At: u.nowStr()}
	u.outcomeMailLocked("rolled-back", reason, exec.TargetTag, true)
	u.saveLocked()
	u.mu.Unlock()
	u.logger.Warn("updater: rolled back to previous binary; restarting")
	u.exit(ExitCodeRestart)
}

// ---- admin-facing API ----

// Errors of the admin actions.
var (
	ErrBusy            = errors.New("an update is already in progress")
	ErrNotTerminal     = errors.New("nothing to acknowledge")
	ErrNothingToCancel = errors.New("no scheduled or draining update to cancel")
)

// DrainCancelledNotice is passed to the broadcast callback instead of the
// seconds left when a drain is cancelled.
const DrainCancelledNotice = -1

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
	busy := u.state.Execution.Status.inFlight() || u.applying.Load()
	u.mu.Unlock()
	if busy {
		return ErrBusy
	}
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
	// A pending verification only when nobody can finish it: the target is
	// not running (see checkPendingVerification). While its timer runs it
	// is not stale.
	if !u.canAcknowledgeLocked() {
		return ErrNotTerminal
	}
	u.logger.Infof("updater: %s state of %s acknowledged; auto-apply re-enabled", u.state.Execution.Status, u.state.Execution.TargetTag)
	u.state.Execution = Execution{Status: StatusIdle}
	u.state.BootCount = 0
	// The failed release may be scheduled again now: the admin has looked.
	if u.state.LastResult != nil && failedOutcomes[u.state.LastResult.Outcome] {
		u.state.LastResult.Outcome = "acknowledged-" + u.state.LastResult.Outcome
	}
	u.saveLocked()
	return nil
}

// Cancel stops a scheduled apply, or the drain of one that is about to
// replace the binary. Once the binary is being replaced it cannot be
// cancelled.
func (u *Updater) Cancel() error {
	u.mu.Lock()
	defer u.mu.Unlock()
	switch u.state.Execution.Status {
	case StatusScheduled:
		tag := u.state.Execution.TargetTag
		u.cancelApplyTimerLocked()
		u.logger.Infof("updater: scheduled apply of %s cancelled", tag)
		u.state.Execution = Execution{Status: StatusIdle}
		// Not scheduled again by the next check: an admin decided against it.
		u.state.LastResult = &LastResult{TargetTag: tag, FromVersion: u.cfg.CurrentVersion, Outcome: "cancelled", At: u.nowStr()}
		u.saveLocked()
		return nil
	case StatusDraining:
		if u.drainer != nil {
			u.drainer.Cancel()
			return nil
		}
	}
	return ErrNothingToCancel
}

// ErrNoAdminEmail is returned by TestMail without an adminEmail setting.
var ErrNoAdminEmail = errors.New("no adminEmail is set")

// TestMail sends a test notice to the admin address, so mail settings can
// be checked from the admin page. Its error is the SMTP server's answer.
func (u *Updater) TestMail() error {
	to := u.adminEmail()
	if to == "" {
		return ErrNoAdminEmail
	}
	if u.cfg.SendMail == nil {
		return errors.New("mail is not set up")
	}
	return u.cfg.SendMail(to, "[Etherpad] Test mail", "This is a test mail from the Etherpad updater. Its notices about updates will reach you here.")
}

// LogTail is the end of the update log, for the admin page.
func (u *Updater) LogTail(n int) []string {
	if u.cfg.LogPath == "" {
		return []string{}
	}
	return TailLines(u.cfg.LogPath, n)
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
	// What the admin can do now (Cancel, Acknowledge).
	CanCancel      bool `json:"canCancel"`
	CanAcknowledge bool `json:"canAcknowledge"`
	// AdminEmail is where the notices go ("" for none).
	AdminEmail string `json:"adminEmail"`
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
		CanCancel:       u.canCancelLocked(),
		CanAcknowledge:  u.canAcknowledgeLocked(),
		AdminEmail:      u.adminEmail(),
	}
}

func (u *Updater) canCancelLocked() bool {
	switch u.state.Execution.Status {
	case StatusScheduled:
		return true
	case StatusDraining:
		return u.drainer != nil
	}
	return false
}

func (u *Updater) canAcknowledgeLocked() bool {
	switch u.state.Execution.Status {
	case StatusRollbackFailed, StatusPreflightFailed, StatusRolledBack:
		return true
	case StatusPendingVerification:
		return u.healthTimer == nil
	}
	return false
}

package pad

import (
	"errors"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/settings"
	"go.uber.org/zap"
)

// RetentionNotice is sent to the pad client when a pad is close to, or has
// reached, its configured retention limit.
type RetentionNotice struct {
	State           string
	MsRemaining     int64
	DeleteAfterDays int
}

type retentionConfig struct {
	deleteAfter    time.Duration
	warnBefore     time.Duration
	sweepInterval  time.Duration
	dryRun         bool
	excludeGroups  bool
	excludePattern *regexp.Regexp
}

type padRetentionStatus struct {
	lastEdited time.Time
	remaining  time.Duration
}

// Retention removes pads that have not been edited for the configured period.
// The feature is deliberately opt-in and dry-run by default.
type Retention struct {
	manager *Manager
	store   db.DataStore
	setting *settings.Settings
	logger  *zap.SugaredLogger

	mu     sync.RWMutex
	cfg    *retentionConfig
	cancel contextCancel
	wg     sync.WaitGroup
	delete sync.Mutex
	// isLeader, if set, reports whether this instance runs the periodic
	// sweep. In a cluster only the leader sweeps; the others would only
	// repeat its work.
	isLeader func() bool
}

// SetLeader makes the periodic sweep run only while isLeader returns true.
// Call it before Start.
func (r *Retention) SetLeader(isLeader func() bool) {
	r.isLeader = isLeader
}

// contextCancel is kept as a small interface so Retention remains easy to
// construct and test without exposing its context implementation.
type contextCancel func()

func NewRetention(manager *Manager, store db.DataStore, setting *settings.Settings, logger *zap.SugaredLogger) *Retention {
	return &Retention{manager: manager, store: store, setting: setting, logger: logger}
}

func parseRetentionConfig(s settings.PadRetention) (*retentionConfig, error) {
	if !s.Enabled {
		return nil, nil
	}
	if s.DeleteAfterDays <= 0 {
		return nil, errors.New("padRetention.deleteAfterDays must be greater than zero")
	}
	if s.WarnBeforeDays < 0 {
		return nil, errors.New("padRetention.warnBeforeDays must not be negative")
	}
	if s.SweepIntervalHours <= 0 {
		return nil, errors.New("padRetention.sweepIntervalHours must be greater than zero")
	}

	cfg := &retentionConfig{
		deleteAfter:   time.Duration(s.DeleteAfterDays) * 24 * time.Hour,
		warnBefore:    time.Duration(s.WarnBeforeDays) * 24 * time.Hour,
		sweepInterval: time.Duration(s.SweepIntervalHours) * time.Hour,
		dryRun:        s.DryRun,
		excludeGroups: s.ExcludeGroupPads,
	}
	if s.ExcludePattern != "" {
		pattern, err := regexp.Compile(s.ExcludePattern)
		if err != nil {
			return nil, errors.New("invalid padRetention.excludePattern: " + err.Error())
		}
		cfg.excludePattern = pattern
	}
	return cfg, nil
}

// Start starts the background sweep. The first sweep is delayed to avoid
// adding startup load to a large installation.
func (r *Retention) Start() {
	r.Stop()
	cfg, err := parseRetentionConfig(r.setting.PadRetention)
	if err != nil {
		r.logger.Warnw("Pad retention disabled", "error", err)
		return
	}
	if cfg == nil {
		return
	}

	stop := make(chan struct{})
	r.mu.Lock()
	r.cfg = cfg
	r.cancel = func() { close(stop) }
	r.mu.Unlock()

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		timer := time.NewTimer(time.Minute)
		defer timer.Stop()
		cancelled := func() bool {
			select {
			case <-stop:
				return true
			default:
				return false
			}
		}
		select {
		case <-timer.C:
			r.sweep(cfg, cancelled)
		case <-stop:
			return
		}

		ticker := time.NewTicker(cfg.sweepInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				r.sweep(cfg, cancelled)
			case <-stop:
				return
			}
		}
	}()
}

func (r *Retention) Stop() {
	r.mu.Lock()
	cancel := r.cancel
	r.cancel = nil
	r.cfg = nil
	r.mu.Unlock()
	if cancel != nil {
		cancel()
		r.wg.Wait()
	}
}

func (r *Retention) config() *retentionConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.cfg
}

func (r *Retention) excluded(padID string, cfg *retentionConfig) bool {
	return (cfg.excludeGroups && strings.Contains(padID, "$")) ||
		(cfg.excludePattern != nil && cfg.excludePattern.MatchString(padID))
}

func (r *Retention) status(padID string, cfg *retentionConfig, now time.Time) (*padRetentionStatus, error) {
	if r.excluded(padID, cfg) {
		return nil, nil
	}
	p, err := r.store.GetPad(padID)
	if err != nil {
		return nil, err
	}
	revision, err := r.store.GetRevision(padID, p.Head)
	if err != nil {
		return nil, err
	}
	lastEdited := time.UnixMilli(revision.Timestamp)
	if lastEdited.After(now) {
		return &padRetentionStatus{lastEdited: lastEdited, remaining: cfg.deleteAfter}, nil
	}
	return &padRetentionStatus{
		lastEdited: lastEdited,
		remaining:  cfg.deleteAfter - now.Sub(lastEdited),
	}, nil
}

func (r *Retention) expirePad(padID string, cfg *retentionConfig, now time.Time) (bool, error) {
	r.delete.Lock()
	defer r.delete.Unlock()
	status, err := r.status(padID, cfg, now)
	if err != nil {
		if err.Error() == db.PadDoesNotExistError || err.Error() == db.PadRevisionNotFoundError {
			return false, nil
		}
		return false, err
	}
	if status == nil || status.remaining > 0 {
		return false, nil
	}
	if cfg.dryRun {
		r.logger.Infow("Pad retention would delete pad", "padId", padID)
		return true, nil
	}
	if err := r.manager.RemovePad(padID); err != nil {
		return false, err
	}
	r.logger.Infow("Pad retention deleted pad", "padId", padID)
	return true, nil
}

// sweepReport counts what one sweep did.
type sweepReport struct {
	checked, deleted, errors int
}

func (r *Retention) sweepOnce(cfg *retentionConfig) sweepReport {
	return r.sweep(cfg, nil)
}

// sweep checks every pad; cancelled, if set, is asked between pads so
// stopping the server does not wait for a sweep over a large instance.
func (r *Retention) sweep(cfg *retentionConfig, cancelled func() bool) sweepReport {
	var report sweepReport
	if r.isLeader != nil && !r.isLeader() {
		r.logger.Debugw("Pad retention sweep skipped: another instance is the leader")
		return report
	}
	padIDs, err := r.store.GetPadIds()
	if err != nil {
		r.logger.Warnw("Pad retention sweep failed", "error", err)
		report.errors++
		return report
	}
	for _, padID := range *padIDs {
		if cancelled != nil && cancelled() {
			break
		}
		report.checked++
		unlock, err := r.manager.LockPad(padID)
		if err != nil {
			r.logger.Warnw("Pad retention check skipped", "padId", padID, "error", err)
			report.errors++
			continue
		}
		removed, err := r.expirePad(padID, cfg, time.Now())
		unlock()
		if err != nil {
			r.logger.Warnw("Pad retention check failed", "padId", padID, "error", err)
			report.errors++
			continue
		}
		if removed {
			report.deleted++
		}
	}
	if report.deleted > 0 {
		r.logger.Infow("Pad retention sweep completed", "deleted", report.deleted, "dryRun", cfg.dryRun)
	}
	return report
}

// CheckOnOpen checks retention immediately before a pad is loaded. It returns
// a notice for the browser and leaves the normal pad-loading path unchanged.
// The caller must hold the pad's write lock (see Manager.LockPad), since an
// expired pad is deleted right here.
func (r *Retention) CheckOnOpen(padID string) *RetentionNotice {
	cfg := r.config()
	if cfg == nil {
		return nil
	}
	now := time.Now()
	status, err := r.status(padID, cfg, now)
	if err != nil {
		if err.Error() != db.PadDoesNotExistError && err.Error() != db.PadRevisionNotFoundError {
			r.logger.Warnw("Pad retention open check failed", "padId", padID, "error", err)
		}
		return nil
	}
	if status == nil {
		return nil
	}
	// A dry run only logs: telling the user the pad was deleted (or will
	// be) would be wrong, it stays (etherpad-lite: "neither deletes nor
	// warns in dry run").
	if cfg.dryRun {
		if status.remaining <= 0 {
			r.logger.Infow("Pad retention would delete pad", "padId", padID)
		}
		return nil
	}
	if status.remaining <= 0 {
		if _, err := r.expirePad(padID, cfg, now); err != nil {
			r.logger.Warnw("Pad retention expiration failed", "padId", padID, "error", err)
		}
		return &RetentionNotice{State: "expired", DeleteAfterDays: int(cfg.deleteAfter / (24 * time.Hour))}
	}
	if status.remaining <= cfg.warnBefore {
		return &RetentionNotice{State: "expiring", MsRemaining: status.remaining.Milliseconds()}
	}
	return nil
}

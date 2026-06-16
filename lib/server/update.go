package server

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/ether/etherpad-go/lib/settings"
	"github.com/ether/etherpad-go/lib/updater"
	"go.uber.org/zap"
)

// buildUpdaterConfig resolves the updater configuration from settings, keeping
// the updater package itself free of any settings dependency.
func buildUpdaterConfig(s *settings.Settings, currentVersion string) updater.Config {
	u := s.Updates

	tier := updater.Tier(strings.ToLower(strings.TrimSpace(u.Tier)))
	if tier == "" {
		tier = updater.TierNotify
	}
	interval := time.Duration(u.CheckIntervalHours) * time.Hour
	if interval <= 0 {
		interval = 6 * time.Hour
	}
	install := updater.InstallMethod(strings.ToLower(strings.TrimSpace(u.InstallMethod)))
	if install == "" {
		install = updater.InstallAuto
	}
	repo := strings.TrimSpace(u.GithubRepo)
	if repo == "" {
		repo = "ether/etherpad-go"
	}
	statePath := strings.TrimSpace(u.StateFile)
	if statePath == "" {
		statePath = filepath.Join("var", "update-state.json")
	}
	health := u.RollbackHealthCheckSeconds
	if health <= 0 {
		health = 60
	}
	drain := max(u.DrainSeconds, 0)

	var window *updater.MaintenanceWindow
	windowConfigured := u.MaintenanceWindow.Start != "" || u.MaintenanceWindow.End != ""
	if windowConfigured {
		if w, ok := updater.ParseWindow(u.MaintenanceWindow.Start, u.MaintenanceWindow.End, strings.ToLower(strings.TrimSpace(u.MaintenanceWindow.TZ))); ok {
			window = w
		}
	}

	return updater.Config{
		Tier:               tier,
		Repo:               repo,
		CurrentVersion:     currentVersion,
		CheckInterval:      interval,
		GraceMinutes:       u.PreApplyGraceMinutes,
		DrainSeconds:       drain,
		HealthCheckSeconds: health,
		InstallMethod:      install,
		Window:             window,
		WindowConfigured:   windowConfigured,
		Verifier:           updater.SignatureVerifier{Require: u.RequireSignature, PublicKey: u.TrustedPublicKey},
		StatePath:          statePath,
		LockPath:           statePath + ".lock",
	}
}

// StartUpdater builds, starts and returns the self-update orchestrator.
func StartUpdater(logger *zap.SugaredLogger, s *settings.Settings, currentVersion string) *updater.Updater {
	upd := updater.New(buildUpdaterConfig(s, currentVersion), logger, func(secsLeft int) {
		logger.Warnf("updater: %d seconds until restart for update", secsLeft)
	})
	upd.Start()
	return upd
}

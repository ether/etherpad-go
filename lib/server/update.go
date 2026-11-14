package server

import (
	"github.com/ether/etherpad-go/lib/api/versionstatus"
	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/mail"
	"github.com/ether/etherpad-go/lib/pad"
	"github.com/gofiber/fiber/v3"
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
	// In a cluster every instance runs the same image, rolled out by the
	// orchestrator. An instance replacing its own binary would leave the
	// cluster on mixed versions, so it may only notify.
	if s.Clustered() && tier != updater.TierOff && tier != updater.TierNotify {
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
		LogPath:            filepath.Join(filepath.Dir(statePath), "update.log"),
		AdminEmail:         func() string { return s.AdminEmail },
	}
}

// StartUpdater builds, starts and returns the self-update orchestrator. The
// broadcast callback (may be nil) is invoked with the seconds remaining at each
// drain announcement so connected clients can be warned of the restart.
func StartUpdater(logger *zap.SugaredLogger, s *settings.Settings, currentVersion string, broadcast func(secsLeft int)) *updater.Updater {
	if configured := strings.ToLower(strings.TrimSpace(s.Updates.Tier)); s.Clustered() && configured != "" &&
		configured != string(updater.TierOff) && configured != string(updater.TierNotify) {
		logger.Warnf("updater: tier %q is not used in a cluster, only notifying. Update by rolling out a new image.", configured)
	}
	cfg := buildUpdaterConfig(s, currentVersion)
	cfg.SendMail = mail.New(func() mail.Config { return s.Mail }, logger).Send
	upd := updater.New(cfg, logger, func(secsLeft int) {
		if secsLeft != updater.DrainCancelledNotice {
			logger.Warnf("updater: %d seconds until restart for update", secsLeft)
		}
		if broadcast != nil {
			broadcast(secsLeft)
		}
	})
	upd.Start()
	return upd
}

// registerVersionStatus adds /api/version-status, which tells the first
// author of a pad that this instance is outdated. With the updater off the
// route does not exist, as in etherpad-lite.
func registerVersionStatus(app *fiber.App, upd *updater.Updater, pm *pad.Manager, store db.DataStore, s *settings.Settings) {
	if upd.Status().Tier == updater.TierOff {
		return
	}
	h := versionstatus.New(versionstatus.Deps{
		Status: upd.Status,
		Pool: func(padID string) (*apool.APool, bool) {
			if ok, err := pm.DoesPadExist(padID); err != nil || ok == nil || !*ok {
				return nil, false
			}
			p, err := pm.GetPad(padID, nil, nil)
			if err != nil {
				return nil, false
			}
			return &p.Pool, true
		},
		Author: func(c fiber.Ctx) string {
			token := pad.AuthorTokenFromCookies(c.Cookies, s)
			if token == "" {
				return ""
			}
			id, err := store.GetAuthorByToken(token)
			if err != nil || id == nil {
				return ""
			}
			return *id
		},
	})
	app.Get("/api/version-status", h.Serve)
}

package server

import (
	"testing"
	"time"

	"github.com/ether/etherpad-go/lib/settings"
	"github.com/ether/etherpad-go/lib/updater"
)

func TestBuildUpdaterConfigDefaults(t *testing.T) {
	s := &settings.Settings{} // all-zero Updates
	cfg := buildUpdaterConfig(s, "1.2.3")

	if cfg.Tier != updater.TierNotify {
		t.Errorf("default tier should be notify, got %q", cfg.Tier)
	}
	if cfg.Repo != "ether/etherpad-go" {
		t.Errorf("default repo wrong: %q", cfg.Repo)
	}
	if cfg.CheckInterval != 6*time.Hour {
		t.Errorf("default interval should be 6h, got %v", cfg.CheckInterval)
	}
	if cfg.InstallMethod != updater.InstallAuto {
		t.Errorf("default install method should be auto, got %q", cfg.InstallMethod)
	}
	if cfg.HealthCheckSeconds != 60 {
		t.Errorf("default health-check should be 60s, got %d", cfg.HealthCheckSeconds)
	}
	if cfg.LockPath != cfg.StatePath+".lock" {
		t.Errorf("lock path should derive from state path, got %q", cfg.LockPath)
	}
	if cfg.CurrentVersion != "1.2.3" {
		t.Errorf("current version not propagated: %q", cfg.CurrentVersion)
	}
	if cfg.Window != nil || cfg.WindowConfigured {
		t.Errorf("no window should be configured by default")
	}
}

func TestBuildUpdaterConfigWindowAndTier(t *testing.T) {
	s := &settings.Settings{}
	s.Updates.Tier = "AUTONOMOUS"
	s.Updates.MaintenanceWindow = settings.MaintenanceWindowSettings{Start: "02:00", End: "04:00", TZ: "UTC"}
	cfg := buildUpdaterConfig(s, "1.0.0")

	if cfg.Tier != updater.TierAutonomous {
		t.Errorf("tier should be parsed case-insensitively, got %q", cfg.Tier)
	}
	if !cfg.WindowConfigured || cfg.Window == nil {
		t.Fatalf("window should be configured and parsed")
	}
	if cfg.Window.StartMin != 120 || cfg.Window.EndMin != 240 {
		t.Errorf("window minutes wrong: %+v", cfg.Window)
	}
}

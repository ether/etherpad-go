package updater

// PolicyInput is the pure input to EvaluatePolicy.
type PolicyInput struct {
	Tier             Tier
	InstallMethod    InstallMethod
	CurrentVersion   string
	Latest           *ReleaseInfo
	ExecutionStatus  ExecutionStatus
	WindowConfigured bool // whether a maintenance window was configured at all
	WindowValid      bool // whether the configured window parsed successfully
}

// PolicyResult is the set of permitted operations under the current config.
type PolicyResult struct {
	CanNotify     bool   // surface availability / send notifications
	CanManual     bool   // admin may trigger an apply
	CanAuto       bool   // scheduler may auto-apply (after a grace window)
	CanAutonomous bool   // scheduler may auto-apply respecting a maintenance window
	Reason        string // machine-readable explanation
}

// EvaluatePolicy decides what the updater is permitted to do. It is pure and
// deterministic so it can be unit-tested without any I/O.
func EvaluatePolicy(in PolicyInput) PolicyResult {
	if in.Tier == TierOff {
		return PolicyResult{Reason: "tier-off"}
	}
	if in.Latest == nil {
		return PolicyResult{Reason: "no-known-latest"}
	}
	if !updateAvailable(in.CurrentVersion, in.Latest.Version) {
		return PolicyResult{Reason: "up-to-date"}
	}

	// Notifications are always allowed beyond this point.
	res := PolicyResult{CanNotify: true, Reason: "ok"}

	// Only a writable standalone binary can be replaced in place. Docker and
	// package-managed installs can still notify but never self-apply.
	if in.InstallMethod != InstallBinary {
		res.Reason = "install-method-not-writable"
		return res
	}

	switch in.Tier {
	case TierNotify:
		res.Reason = "notify-only"
		return res
	case TierManual:
		res.CanManual = true
		return res
	case TierAuto:
		res.CanManual = true
		res.CanAuto = true
	case TierAutonomous:
		res.CanManual = true
		// Autonomous mode REQUIRES a valid maintenance window. Without one it
		// must NOT degrade into unrestricted auto-apply, so CanAuto stays false
		// (the scheduler/trigger gate primarily on CanAuto). Manual stays allowed.
		if in.WindowValid {
			res.CanAuto = true
			res.CanAutonomous = true
		} else if in.WindowConfigured {
			res.Reason = "maintenance-window-invalid"
		} else {
			res.Reason = "maintenance-window-missing"
		}
	}

	// A failed rollback is terminal: block all automatic applies until an admin
	// acknowledges. Manual applies remain available so the admin can recover.
	if in.ExecutionStatus == StatusRollbackFailed {
		res.CanAuto = false
		res.CanAutonomous = false
		res.Reason = "rollback-failed-terminal"
	}
	return res
}

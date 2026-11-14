package updater

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Ports of etherpad-lite's updater specs versionCompare, refSafety,
// MaintenanceWindow and UpdatePolicy, for the Go updater's own design
// (pure_test.go has the first cases).

func TestParityParseVersion(t *testing.T) {
	for in, want := range map[string]string{
		"1.2.3": "1.2.3", "v2.7.3-rc.1": "2.7.3-rc.1", "v2.7.1": "2.7.1", "2.7.1+build.123": "2.7.1+build.123",
	} {
		v, ok := parseVersion(in)
		require.True(t, ok, in)
		assert.Equal(t, want, v.String(), in)
	}
	for _, bad := range []string{"garbage", "", "2.7", "1.2", "not-a-version", "2.7.1.4", "2.7.1.foo", "vv1.2.3", "01.2.3"} {
		_, ok := parseVersion(bad)
		assert.False(t, ok, "%q is no release version", bad)
	}
}

func TestParityCompareVersions(t *testing.T) {
	for _, c := range []struct {
		cur, lat string
		want     bool
	}{
		{"1.2.3", "1.2.4", true}, {"1.2.3", "1.2.3", false}, {"1.2.4", "1.2.3", false},
		{"2.7.1", "2.7.2", true}, {"2.99.99", "3.0.0", true}, {"3.0.0", "2.99.99", false},
		{"garbage", "2.7.1", false}, {"2.7.1", "garbage", false},
		{"2.7", "2.7.1", false}, // not a full version: no update offered
		{"v2.7.1", "2.7.2", true},
		// Go orders pre-releases before their release (lite strips them).
		{"2.7.3-rc.1", "2.7.3", true}, {"2.7.3", "2.7.3-rc.1", false},
	} {
		assert.Equal(t, c.want, updateAvailable(c.cur, c.lat), "%s -> %s", c.cur, c.lat)
	}
}

func TestParityMinorOrMoreBehind(t *testing.T) {
	for _, c := range []struct {
		cur, lat string
		want     bool
	}{
		{"3.0.0", "3.0.0", false}, {"3.1.0", "3.0.5", false}, {"2.7.3", "2.7.4", false}, {"3.0.1", "3.0.9", false},
		{"3.1.0", "3.2.0", true}, {"3.1.5", "3.2.0", true}, {"2.7.3", "3.0.0", true},
		{"garbage", "3.0.0", false}, {"3.0.0", "garbage", false},
	} {
		assert.Equal(t, c.want, isMinorOrMoreBehind(c.cur, c.lat), "%s -> %s", c.cur, c.lat)
	}
}

func TestParityValidTag(t *testing.T) {
	for _, ok := range []string{"v2.7.3", "2.7.3", "2.7.3-rc.1"} {
		assert.True(t, IsValidTag(ok), ok)
	}
	for _, bad := range []string{
		"-rf", "-fast-forward", "-no-verify", ".git", "", "v" + strings.Repeat("a", 300),
		"v2.7.3 -rf", "v2.7.3\nrm -rf", "v2.7.3\trf", "v2.7.3\x00rf", "v2.7.3\x7f",
		"v2.7..3", "v2~7~3", "v2:7:3", "v2.7.3?", "v2.7.3*", "v[7]", `v\7`, "v^7", "v2/../../etc",
	} {
		assert.False(t, IsValidTag(bad), "%q", bad)
	}
}

func TestParityParseWindow(t *testing.T) {
	w, ok := ParseWindow("03:00", "05:00", "local")
	require.True(t, ok)
	assert.Equal(t, MaintenanceWindow{StartMin: 180, EndMin: 300, TZ: "local"}, *w)
	w, ok = ParseWindow("22:00", "02:00", "utc")
	require.True(t, ok, "cross-midnight")
	assert.Equal(t, MaintenanceWindow{StartMin: 1320, EndMin: 120, TZ: "utc"}, *w)

	for _, bad := range [][3]string{
		{"3:00", "05:00", "local"}, {"03:60", "05:00", "local"}, {"24:00", "05:00", "local"}, {"oops", "05:00", "local"},
		{"+3:00", "05:00", "local"}, {" 03:00", "05:00", "local"}, {"03:00:00", "05:00", "local"},
		{"03:00", "03:00", "local"}, {"03:00", "05:00", "pacific"}, {"03:00", "", "local"}, {"", "", ""},
	} {
		_, ok := ParseWindow(bad[0], bad[1], bad[2])
		assert.False(t, ok, "%q", bad)
	}
}

func utc(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestParityInWindow(t *testing.T) {
	same, _ := ParseWindow("03:00", "05:00", "utc")
	for at, want := range map[string]bool{
		"2026-05-15T03:30:00Z": true, "2026-05-15T03:00:00Z": true, "2026-05-15T02:59:59Z": false,
		"2026-05-15T05:00:00Z": false, "2026-05-15T06:00:00Z": false,
	} {
		assert.Equal(t, want, same.InWindow(utc(at)), at)
	}
	cross, _ := ParseWindow("22:00", "02:00", "utc")
	for at, want := range map[string]bool{
		"2026-05-15T23:00:00Z": true, "2026-05-16T01:00:00Z": true, "2026-05-16T02:00:00Z": false,
		"2026-05-15T12:00:00Z": false, "2026-05-15T21:59:59Z": false,
	} {
		assert.Equal(t, want, cross.InWindow(utc(at)), at)
	}

	local, _ := ParseWindow("03:00", "05:00", "local")
	assert.True(t, local.InWindow(time.Date(2026, 5, 15, 4, 0, 0, 0, time.Local)), "the host's wall clock")
	assert.False(t, local.InWindow(time.Date(2026, 5, 15, 6, 0, 0, 0, time.Local)))
	assert.True(t, local.InWindow(time.Date(2026, 5, 15, 4, 0, 0, 0, time.Local).UTC()), "whatever zone the time is in")
}

func TestParityNextWindowStart(t *testing.T) {
	same, _ := ParseWindow("03:00", "05:00", "utc")
	for now, want := range map[string]string{
		"2026-05-15T01:00:00Z": "2026-05-15T03:00:00Z",
		"2026-05-15T03:30:00Z": "2026-05-16T03:00:00Z",
		"2026-05-15T06:00:00Z": "2026-05-16T03:00:00Z",
		"2026-05-15T03:00:00Z": "2026-05-16T03:00:00Z", // at the start: the next one
	} {
		assert.Equal(t, want, same.NextWindowStart(utc(now)).UTC().Format(time.RFC3339), now)
	}
	cross, _ := ParseWindow("22:00", "02:00", "utc")
	for now, want := range map[string]string{
		"2026-05-15T10:00:00Z": "2026-05-15T22:00:00Z",
		"2026-05-16T01:00:00Z": "2026-05-16T22:00:00Z",
		"2026-05-15T23:30:00Z": "2026-05-16T22:00:00Z",
	} {
		assert.Equal(t, want, cross.NextWindowStart(utc(now)).UTC().Format(time.RFC3339), now)
	}

	local, _ := ParseWindow("03:00", "05:00", "local")
	next := local.NextWindowStart(time.Date(2026, 5, 15, 1, 0, 0, 0, time.Local)).In(time.Local)
	assert.Equal(t, [5]int{2026, 5, 15, 3, 0}, [5]int{next.Year(), int(next.Month()), next.Day(), next.Hour(), next.Minute()})
}

// Across a DST change the next start keeps its wall-clock time; adding 24h
// would land an hour off.
func TestNextWindowStartAcrossDST(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skip("no tzdata")
	}
	saved := time.Local
	time.Local = berlin
	t.Cleanup(func() { time.Local = saved })

	w, _ := ParseWindow("03:30", "05:00", "local")
	// 2026-03-29 is the spring change in Berlin (02:00 -> 03:00).
	next := w.NextWindowStart(time.Date(2026, 3, 28, 4, 0, 0, 0, berlin)).In(berlin)
	assert.Equal(t, "2026-03-29 03:30", next.Format("2006-01-02 15:04"))
	// 2026-10-25 autumn change (03:00 -> 02:00).
	next = w.NextWindowStart(time.Date(2026, 10, 24, 4, 0, 0, 0, berlin)).In(berlin)
	assert.Equal(t, "2026-10-25 03:30", next.Format("2006-01-02 15:04"))
}

func TestParityPolicy(t *testing.T) {
	latest := &ReleaseInfo{Version: "2.7.2", Tag: "v2.7.2"}
	base := PolicyInput{Tier: TierManual, InstallMethod: InstallBinary, CurrentVersion: "2.7.1", Latest: latest, WindowConfigured: true, WindowValid: true}
	eval := func(mod func(*PolicyInput)) PolicyResult {
		in := base
		mod(&in)
		return EvaluatePolicy(in)
	}

	assert.Equal(t, PolicyResult{Reason: "tier-off"}, eval(func(i *PolicyInput) { i.Tier = TierOff }))
	assert.Equal(t, PolicyResult{CanNotify: true, Reason: "notify-only"}, eval(func(i *PolicyInput) { i.Tier = TierNotify }))
	assert.Equal(t, PolicyResult{CanNotify: true, CanManual: true, Reason: "ok"}, eval(func(i *PolicyInput) {}))
	assert.Equal(t, PolicyResult{CanNotify: true, Reason: "install-method-not-writable"},
		eval(func(i *PolicyInput) { i.InstallMethod = InstallDocker }), "docker never self-applies")
	assert.Equal(t, PolicyResult{CanNotify: true, CanManual: true, CanAuto: true, CanAutonomous: true, Reason: "ok"},
		eval(func(i *PolicyInput) { i.Tier = TierAutonomous }))
	assert.Equal(t, PolicyResult{CanNotify: true, Reason: "install-method-not-writable"},
		eval(func(i *PolicyInput) { i.Tier = TierAutonomous; i.InstallMethod = InstallManaged }))
	assert.Equal(t, PolicyResult{Reason: "up-to-date"}, eval(func(i *PolicyInput) { i.Tier = TierAutonomous; i.CurrentVersion = "2.7.2" }))
	assert.Equal(t, PolicyResult{Reason: "up-to-date"}, eval(func(i *PolicyInput) { i.Tier = TierAutonomous; i.CurrentVersion = "3.0.0" }), "dev build ahead")
	assert.Equal(t, PolicyResult{Reason: "no-known-latest"}, eval(func(i *PolicyInput) { i.Latest = nil }))

	// Terminal state.
	r := eval(func(i *PolicyInput) { i.Tier = TierAutonomous; i.ExecutionStatus = StatusRollbackFailed })
	assert.Equal(t, PolicyResult{CanNotify: true, CanManual: true, Reason: "rollback-failed-terminal"}, r)
	assert.Equal(t, eval(func(i *PolicyInput) { i.Tier = TierAutonomous }),
		eval(func(i *PolicyInput) { i.Tier = TierAutonomous; i.ExecutionStatus = StatusIdle }), "idle is no status")
	assert.Equal(t, eval(func(i *PolicyInput) { i.Tier = TierAutonomous }),
		eval(func(i *PolicyInput) { i.Tier = TierAutonomous; i.ExecutionStatus = StatusPreflightFailed }), "preflight-failed is informational")

	// Maintenance window. Go keeps auto off without a valid window (lite
	// degrades to auto): autonomous must not turn into apply-any-time.
	assert.Equal(t, PolicyResult{CanNotify: true, CanManual: true, Reason: "maintenance-window-missing"},
		eval(func(i *PolicyInput) { i.Tier = TierAutonomous; i.WindowConfigured, i.WindowValid = false, false }))
	assert.Equal(t, PolicyResult{CanNotify: true, CanManual: true, Reason: "maintenance-window-invalid"},
		eval(func(i *PolicyInput) { i.Tier = TierAutonomous; i.WindowValid = false }))
	assert.Equal(t, PolicyResult{CanNotify: true, CanManual: true, CanAuto: true, Reason: "ok"},
		eval(func(i *PolicyInput) { i.Tier = TierAuto; i.WindowConfigured, i.WindowValid = false, false }), "lower tiers ignore the window")
	assert.Equal(t, "rollback-failed-terminal",
		eval(func(i *PolicyInput) { i.Tier = TierAutonomous; i.WindowConfigured = false; i.ExecutionStatus = StatusRollbackFailed }).Reason)
}

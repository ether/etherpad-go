package updater

import (
	"regexp"
	"strconv"
	"time"
)

// MaintenanceWindow is a daily wall-clock window during which autonomous
// updates may run.
type MaintenanceWindow struct {
	StartMin int    // minutes since midnight, inclusive
	EndMin   int    // minutes since midnight, exclusive
	TZ       string // "utc" or "local"
}

// ParseWindow parses "HH:MM" start/end strings and a timezone ("utc"|"local").
// It returns ok=false for malformed input or a zero-length window. When
// end <= start the window crosses midnight, e.g. 22:00–04:00.
func ParseWindow(start, end, tz string) (*MaintenanceWindow, bool) {
	sm, ok := parseHHMM(start)
	if !ok {
		return nil, false
	}
	em, ok := parseHHMM(end)
	if !ok {
		return nil, false
	}
	if sm == em {
		return nil, false
	}
	if tz == "" {
		tz = "local"
	}
	if tz != "utc" && tz != "local" {
		return nil, false
	}
	return &MaintenanceWindow{StartMin: sm, EndMin: em, TZ: tz}, true
}

// hhmm is exactly two-digit hours and minutes ("03:00", not "3:00" or
// "+3:00"), as etherpad-lite's parseWindow.
var hhmm = regexp.MustCompile(`^([01][0-9]|2[0-3]):([0-5][0-9])$`)

func parseHHMM(s string) (int, bool) {
	m := hhmm.FindStringSubmatch(s)
	if m == nil {
		return 0, false
	}
	h, _ := strconv.Atoi(m[1])
	mins, _ := strconv.Atoi(m[2])
	return h*60 + mins, true
}

func (w *MaintenanceWindow) location() *time.Location {
	if w.TZ == "utc" {
		return time.UTC
	}
	return time.Local
}

// InWindow reports whether now falls within the window.
func (w *MaintenanceWindow) InWindow(now time.Time) bool {
	t := now.In(w.location())
	cur := t.Hour()*60 + t.Minute()
	if w.StartMin < w.EndMin {
		return cur >= w.StartMin && cur < w.EndMin
	}
	// Crosses midnight: [start, 24:00) ∪ [00:00, end).
	return cur >= w.StartMin || cur < w.EndMin
}

// NextWindowStart returns the earliest time at or after now whose wall-clock
// equals the window start.
func (w *MaintenanceWindow) NextWindowStart(now time.Time) time.Time {
	loc := w.location()
	t := now.In(loc)
	start := time.Date(t.Year(), t.Month(), t.Day(), w.StartMin/60, w.StartMin%60, 0, 0, loc)
	if !start.After(t) {
		// The next calendar day, not +24h: across a DST change a day has 23
		// or 25 hours, and the window starts at its wall-clock time.
		start = time.Date(t.Year(), t.Month(), t.Day()+1, w.StartMin/60, w.StartMin%60, 0, 0, loc)
	}
	return start
}

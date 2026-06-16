package updater

import (
	"strconv"
	"strings"
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

func parseHHMM(s string) (int, bool) {
	parts := strings.SplitN(strings.TrimSpace(s), ":", 2)
	if len(parts) != 2 {
		return 0, false
	}
	h, err := strconv.Atoi(parts[0])
	if err != nil || h < 0 || h > 23 {
		return 0, false
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil || m < 0 || m > 59 {
		return 0, false
	}
	return h*60 + m, true
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
		start = start.Add(24 * time.Hour)
	}
	return start
}

package ws

import (
	"errors"
	"testing"
	"time"

	"github.com/ether/etherpad-go/lib/models/ws/admin"
)

func TestPadFilter(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	ago := func(d time.Duration) int64 { return now.Add(-d).UnixMilli() }
	day := 24 * time.Hour
	cases := []struct {
		filter string
		pad    admin.PadDBSearch
		want   bool
	}{
		{"empty", admin.PadDBSearch{RevisionNumber: 0}, true},
		{"empty", admin.PadDBSearch{RevisionNumber: 1}, false},
		{"active", admin.PadDBSearch{UserCount: 1}, true},
		{"active", admin.PadDBSearch{UserCount: 0}, false},
		{"recent", admin.PadDBSearch{LastEdited: ago(time.Hour)}, true},
		{"recent", admin.PadDBSearch{LastEdited: ago(7 * day)}, true},
		{"recent", admin.PadDBSearch{LastEdited: ago(8 * day)}, false},
		{"recent", admin.PadDBSearch{LastEdited: 0}, false},
		{"stale", admin.PadDBSearch{LastEdited: ago(91 * day)}, true},
		{"stale", admin.PadDBSearch{LastEdited: ago(90 * day)}, false},
		{"stale", admin.PadDBSearch{LastEdited: 0}, false},
	}
	for _, c := range cases {
		keep, err := padFilter(c.filter, now)
		if err != nil {
			t.Fatalf("%s: %v", c.filter, err)
		}
		if got := keep(c.pad); got != c.want {
			t.Errorf("%s %+v: got %v, want %v", c.filter, c.pad, got, c.want)
		}
	}
	if _, err := padFilter("bogus", now); !errors.Is(err, errUnknownPadFilter) {
		t.Errorf("unknown filter: got %v", err)
	}
}

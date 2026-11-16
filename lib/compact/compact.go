// Package compact is "etherpad-go compact": etherpad-lite's bin/compactPad,
// bin/compactAllPads and bin/compactStalePads as a command of the binary.
//
// It talks to the running server through its HTTP API (listAllPads,
// getLastEdited, getRevisionsCount, compactPad), as etherpad-lite's scripts
// do: the server holds pads in memory, so changing their history behind its
// back in the database would leave it with a stale copy. Compacting is
// destructive; export what you cannot lose as .etherpad first.
package compact

import (
	"fmt"
	"io"
	"time"
)

// API is the part of the HTTP API the command uses.
type API interface {
	ListAllPads() ([]string, error)
	// GetLastEdited is the time of the pad's last edit, in milliseconds.
	GetLastEdited(padID string) (int64, error)
	// GetRevisionsCount is the pad's head revision number.
	GetRevisionsCount(padID string) (int, error)
	// CompactPad collapses the history, or keeps its last keep revisions.
	CompactPad(padID string, keep *int) error
}

// Options of a run.
type Options struct {
	// Pads to compact; empty: every pad of the instance.
	Pads []string
	// OlderThanDays > 0 compacts only pads not edited for that many days
	// (bin/compactStalePads).
	OlderThanDays int
	// Keep the last Keep revisions; nil collapses all history.
	Keep   *int
	DryRun bool
}

// Report counts what a run did. Revisions count the head plus one, as
// etherpad-lite's scripts report them.
type Report struct {
	Total, Selected, OK, Failed, SkippedFresh int
	RevsBefore, RevsAfter                     int
}

const day = 24 * time.Hour

// MaxOlderThanDays is the most days --older-than takes: a time.Duration
// holds about 106751 days, beyond that the cutoff would wrap into the
// future and select freshly edited pads.
const MaxOlderThanDays = 100_000

// Run compacts the pads opts selects. A pad that fails is logged and
// counted; the run goes on.
func Run(api API, opts Options, out, errOut io.Writer, now func() time.Time) Report {
	var r Report
	pads := opts.Pads
	if len(pads) == 0 {
		all, err := api.ListAllPads()
		if err != nil {
			fmt.Fprintf(errOut, "listAllPads failed: %v\n", err)
			r.Failed = 1
			return r
		}
		pads = all
	}
	r.Total = len(pads)
	if len(pads) == 0 {
		fmt.Fprintln(out, "No pads on this instance.")
		return r
	}

	strategy := "collapse all history"
	if opts.Keep != nil {
		strategy = fmt.Sprintf("keep last %d revisions", *opts.Keep)
	}
	dry := ""
	if opts.DryRun {
		dry = " (dry run, no writes)"
	}
	filter := ""
	if opts.OlderThanDays > 0 {
		filter = fmt.Sprintf(" Filter: not edited in %d day(s).", opts.OlderThanDays)
	}
	fmt.Fprintf(out, "Found %d pad(s).%s Strategy: %s%s.\n", len(pads), filter, strategy, dry)

	days := min(opts.OlderThanDays, MaxOlderThanDays)
	cutoff := now().Add(-time.Duration(days) * day).UnixMilli()
	fresh := func(padID string) (bool, error) {
		if opts.OlderThanDays <= 0 {
			return false, nil
		}
		last, err := api.GetLastEdited(padID)
		return last > cutoff, err
	}

	// First pass: which pads are stale. A pad whose last edit cannot be
	// read is a failure (it cannot be decided), not a stop.
	selected := make([]string, 0, len(pads))
	for _, padID := range pads {
		isFresh, err := fresh(padID)
		if err != nil {
			fmt.Fprintf(errOut, "%s: getLastEdited failed: %v\n", padID, err)
			r.Failed++
			continue
		}
		if isFresh {
			r.SkippedFresh++
			continue
		}
		selected = append(selected, padID)
	}
	r.Selected = len(selected)
	if opts.OlderThanDays > 0 {
		if len(selected) == 0 {
			fmt.Fprintf(out, "No stale pads (%d fresh, %d unreadable).\n", r.SkippedFresh, r.Failed)
			return r
		}
		fmt.Fprintf(out, "%d stale pad(s) to process (%d fresh skipped).\n", len(selected), r.SkippedFresh)
	}

	for i, padID := range selected {
		idx := fmt.Sprintf("[%d/%d]", i+1, len(selected))
		before, err := api.GetRevisionsCount(padID)
		if err != nil {
			fmt.Fprintf(errOut, "%s %s: getRevisionsCount failed: %v\n", idx, padID, err)
			r.Failed++
			continue
		}
		if opts.DryRun {
			fmt.Fprintf(out, "%s %s: %d revision(s), would compact\n", idx, padID, before+1)
			r.RevsBefore += before + 1
			continue
		}
		// Check again right before compacting: on a long run a pad may have
		// become active since it was selected, and compacting it would
		// disconnect its editors. Such a pad is skipped, not failed.
		isFresh, err := fresh(padID)
		if err != nil {
			fmt.Fprintf(errOut, "%s %s: getLastEdited recheck failed: %v\n", idx, padID, err)
			r.Failed++
			continue
		}
		if isFresh {
			fmt.Fprintf(out, "%s %s: edited during the run, skipped (now fresh)\n", idx, padID)
			r.SkippedFresh++
			r.Selected--
			continue
		}
		if err := api.CompactPad(padID, opts.Keep); err != nil {
			fmt.Fprintf(errOut, "%s %s: compactPad failed: %v\n", idx, padID, err)
			r.Failed++
			continue
		}
		r.OK++
		after, err := api.GetRevisionsCount(padID)
		if err != nil {
			// Compacted; the count afterwards is only information.
			fmt.Fprintf(out, "%s %s: compacted (no count afterwards)\n", idx, padID)
			continue
		}
		fmt.Fprintf(out, "%s %s: %d -> %d revision(s)\n", idx, padID, before+1, after+1)
		r.RevsBefore += before + 1
		r.RevsAfter += after + 1
	}

	switch {
	case opts.DryRun:
		fmt.Fprintf(out, "Dry run: %d pad(s), %d revision(s) in total.\n", r.Selected, r.RevsBefore)
	default:
		fmt.Fprintf(out, "Done: %d compacted, %d failed. Revisions %d -> %d.\n", r.OK, r.Failed, r.RevsBefore, r.RevsAfter)
	}
	return r
}

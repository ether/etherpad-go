package compact

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakePad struct {
	head       int
	lastEdited time.Time
	failRead   bool
	failCompac bool
	// editedOnRecheck makes the pad fresh after it was selected.
	editedOnRecheck bool
	reads           int
}

type fakeAPI struct {
	pads      map[string]*fakePad
	order     []string
	compacted map[string]*int
	listErr   error
	now       time.Time
}

func newFake(now time.Time) *fakeAPI {
	return &fakeAPI{pads: map[string]*fakePad{}, compacted: map[string]*int{}, now: now}
}

func (f *fakeAPI) add(id string, p *fakePad) {
	f.pads[id] = p
	f.order = append(f.order, id)
}

func (f *fakeAPI) ListAllPads() ([]string, error) { return f.order, f.listErr }

func (f *fakeAPI) GetLastEdited(id string) (int64, error) {
	p := f.pads[id]
	if p.failRead {
		return 0, errors.New("boom")
	}
	p.reads++
	if p.editedOnRecheck && p.reads > 1 {
		return f.now.UnixMilli(), nil
	}
	return p.lastEdited.UnixMilli(), nil
}

func (f *fakeAPI) GetRevisionsCount(id string) (int, error) {
	p, ok := f.pads[id]
	if !ok {
		return 0, errors.New("padID does not exist")
	}
	return p.head, nil
}

func (f *fakeAPI) CompactPad(id string, keep *int) error {
	p := f.pads[id]
	if p.failCompac {
		return errors.New("compaction refused")
	}
	f.compacted[id] = keep
	p.head = 0
	if keep != nil && *keep < p.head {
		p.head = *keep
	}
	return nil
}

var now = time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)

func run(t *testing.T, api API, args ...string) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := mainWithClock(args, &out, &errOut, api, func() time.Time { return now })
	return code, out.String(), errOut.String()
}

func TestCompactNamedPads(t *testing.T) {
	f := newFake(now)
	f.add("a", &fakePad{head: 40})
	f.add("b", &fakePad{head: 9})
	code, out, _ := run(t, f, "a", "b", "--keep", "5")
	assert.Equal(t, 0, code)
	require.Contains(t, f.compacted, "a")
	require.Contains(t, f.compacted, "b")
	assert.Equal(t, 5, *f.compacted["a"])
	assert.Contains(t, out, "[1/2] a: 41 -> 1 revision(s)")
	assert.Contains(t, out, "keep last 5 revisions")
}

func TestCompactAllToleratesFailures(t *testing.T) {
	f := newFake(now)
	f.add("ok", &fakePad{head: 3})
	f.add("broken", &fakePad{head: 3, failCompac: true})
	f.add("also", &fakePad{head: 7})
	code, out, errOut := run(t, f, "--all")
	assert.Equal(t, 1, code, "a failed pad fails the run")
	assert.Contains(t, f.compacted, "ok")
	assert.Contains(t, f.compacted, "also", "the run goes on after a failure")
	assert.Nil(t, f.compacted["ok"], "no --keep: collapse all history")
	assert.Contains(t, errOut, "broken: compactPad failed: compaction refused")
	assert.Contains(t, out, "Done: 2 compacted, 1 failed.")
}

func TestCompactStale(t *testing.T) {
	f := newFake(now)
	f.add("cold", &fakePad{head: 100, lastEdited: now.Add(-120 * day)})
	f.add("hot", &fakePad{head: 100, lastEdited: now.Add(-2 * day)})
	f.add("woken", &fakePad{head: 100, lastEdited: now.Add(-200 * day), editedOnRecheck: true})
	f.add("unreadable", &fakePad{failRead: true})
	code, out, errOut := run(t, f, "--older-than", "90")
	assert.Equal(t, 1, code, "an unreadable pad counts as a failure")
	assert.Contains(t, f.compacted, "cold")
	assert.NotContains(t, f.compacted, "hot")
	assert.NotContains(t, f.compacted, "woken", "edited since it was selected: skipped")
	assert.Contains(t, out, "2 stale pad(s) to process (1 fresh skipped).")
	assert.Contains(t, out, "woken: edited during the run, skipped (now fresh)")
	assert.Contains(t, errOut, "unreadable: getLastEdited failed")
}

func TestCompactDryRun(t *testing.T) {
	f := newFake(now)
	f.add("a", &fakePad{head: 10})
	f.add("b", &fakePad{head: 20})
	code, out, _ := run(t, f, "--all", "--dry-run")
	assert.Equal(t, 0, code)
	assert.Empty(t, f.compacted, "a dry run changes nothing")
	assert.Contains(t, out, "a: 11 revision(s), would compact")
	assert.Contains(t, out, "Dry run: 2 pad(s), 32 revision(s) in total.")
}

func TestCompactUsage(t *testing.T) {
	f := newFake(now)
	for _, args := range [][]string{
		{},
		{"--all", "pad"},
		{"--all", "--older-than", "3"},
		{"pad", "--keep", "-1"},
		{"--older-than", "-5"},
	} {
		code, _, errOut := run(t, f, args...)
		assert.Equal(t, 2, code, "%v", args)
		assert.NotEmpty(t, errOut, "%v", args)
	}
	assert.Empty(t, f.compacted)

	f.listErr = errors.New("unauthorised")
	code, _, errOut := run(t, f, "--all")
	assert.Equal(t, 1, code)
	assert.True(t, strings.Contains(errOut, "listAllPads failed: unauthorised"))
}

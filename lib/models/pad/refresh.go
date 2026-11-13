package pad

import (
	"errors"
	"fmt"

	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/changeset"
	dbstore "github.com/ether/etherpad-go/lib/db"
	db "github.com/ether/etherpad-go/lib/models/db"
	"github.com/ether/etherpad-go/lib/models/revision"
)

// ErrPadGone is returned by Refresh when the pad was deleted, for instance
// by another instance.
var ErrPadGone = errors.New("pad no longer exists")

// ErrNoRevisions is returned when a pad row exists but no revision is
// stored for it.
var ErrNoRevisions = errors.New("pad has no revisions")

// The stored changesets (padRev.changeset) are the authoritative history of
// a pad: other Etherpad instances on the same database append to them, and
// this instance's cached pad as well as the pad row may lag behind. The text
// and pool stored with a revision are a shortcut only: revisions written by
// Etherpad carry them, imported and migrated ones often only on key
// revisions.

func storedPoolUsable(rp *db.RevPool) bool {
	return rp != nil && (rp.NextNum > 0 || len(rp.NumToAttrib) > 0)
}

func poolFromRev(rp *db.RevPool) apool.APool {
	pool := apool.NewAPool()
	if rp != nil {
		pool.FromDB(db.PadPool{NumToAttrib: rp.NumToAttrib, NextNum: rp.NextNum})
	}
	return pool
}

// padState is the part of a pad that replaying revisions produces. Loading
// builds a padState and assigns it to the pad only once every revision has
// loaded and applied, so a failed load leaves the cached pad as it was.
type padState struct {
	atext apool.AText
	pool  apool.APool
	head  int
}

func (p *Pad) state() padState { return padState{atext: p.AText, pool: p.Pool, head: p.Head} }

func (p *Pad) setState(s padState) {
	p.AText = s.atext
	p.Pool = s.pool
	p.Head = s.head
}

// Refresh brings the pad up to the state stored in the database. Callers
// hold the pad's write lock. On error the pad is unchanged.
func (p *Pad) Refresh() error {
	v, err := p.db.GetPadVersion(p.Id)
	if err != nil {
		return fmt.Errorf("reading the version of pad %s: %w", p.Id, err)
	}
	if !v.Exists {
		return ErrPadGone
	}
	p.applyMeta(v.Meta)
	if v.ChatHead > p.ChatHead {
		p.ChatHead = v.ChatHead
	}
	if v.Rev == p.Head || v.Rev < 0 {
		return nil
	}
	// Usually a few small changesets are missing; apply them instead of
	// loading the whole text.
	if v.Rev > p.Head && p.Head >= 0 {
		if next, err := p.replay(p.state(), v.Rev); err == nil {
			p.setState(next)
			return nil
		}
	}
	next, err := p.loadState(v.Rev)
	if err != nil {
		return err
	}
	p.setState(next)
	return nil
}

// reloadFromStore completes loading a pad whose row was just mapped onto p.
// Text, pool and head come from the stored revisions only: the row may be
// behind them, ahead of them (written by an older version before its
// revision was stored), and its text is only updated on key revisions.
func (p *Pad) reloadFromStore() error {
	v, err := p.db.GetPadVersion(p.Id)
	if err != nil {
		return err
	}
	if !v.Exists {
		return ErrPadGone
	}
	p.applyMeta(v.Meta)
	p.ChatHead = max(p.ChatHead, v.ChatHead)
	if v.Rev < 0 {
		return ErrNoRevisions
	}
	if next, ok := p.storedState(v.Rev); ok {
		p.setState(next)
		return nil
	}
	if next, ok := p.keyRevisionState(v.Rev); ok {
		p.setState(next)
		return nil
	}
	// The row's text is no starting point: its head moves with every
	// revision, its text only on key revisions.
	next, err := p.rebuiltState(v.Rev)
	if err != nil {
		return err
	}
	p.setState(next)
	return nil
}

// loadState returns the stored state of rev: from the text stored with it,
// else from the nearest key revision, else replayed from the empty pad.
func (p *Pad) loadState(rev int) (padState, error) {
	if next, ok := p.storedState(rev); ok {
		return next, nil
	}
	if next, ok := p.keyRevisionState(rev); ok {
		return next, nil
	}
	return p.rebuiltState(rev)
}

// storedState uses the text and pool stored with rev, if it has them.
func (p *Pad) storedState(rev int) (padState, bool) {
	stored, err := p.db.GetRevision(p.Id, rev)
	if err != nil || stored.AText.Text == "" {
		return padState{}, false
	}
	next := padState{atext: apool.FromDBAText(stored.AText), pool: p.Pool, head: rev}
	if storedPoolUsable(stored.Pool) {
		next.pool = poolFromRev(stored.Pool)
	}
	return next, true
}

// rebuiltState replays every stored changeset from the empty pad.
func (p *Pad) rebuiltState(rev int) (padState, error) {
	return p.replay(padState{atext: changeset.MakeAText("\n", nil), pool: p.Pool, head: -1}, rev)
}

// replay applies the stored changesets after from.head up to toRev to from
// and returns the result; the pad itself is not changed. The pool stored
// with toRev holds every attribute they use (pools only grow); where none
// was stored, from's pool is used.
func (p *Pad) replay(from padState, toRev int) (padState, error) {
	deltas, err := p.db.GetRevisionDeltas(p.Id, from.head+1, toRev)
	if err != nil {
		return padState{}, err
	}
	if len(deltas) != toRev-from.head {
		// Name the first missing revision, as the pad check does.
		have := make(map[int]bool, len(deltas))
		for _, d := range deltas {
			have[d.Rev] = true
		}
		for r := from.head + 1; r <= toRev; r++ {
			if !have[r] {
				return padState{}, fmt.Errorf("revision is missing (pad %s revision %d)", p.Id, r)
			}
		}
		return padState{}, fmt.Errorf("pad %s: expected %d revisions after %d, got %d", p.Id, toRev-from.head, from.head, len(deltas))
	}
	pool := from.pool
	if storedPool, err := p.db.GetRevisionPool(p.Id, toRev); err == nil && storedPoolUsable(storedPool) {
		pool = poolFromRev(storedPool)
	}
	atext := from.atext
	for i, d := range deltas {
		if d.Rev != from.head+1+i {
			return padState{}, fmt.Errorf("revision is missing (pad %s revision %d)", p.Id, from.head+1+i)
		}
		next, err := changeset.ApplyToAText(d.Changeset, atext, pool)
		if err != nil {
			return padState{}, fmt.Errorf("pad %s: applying revision %d: %w", p.Id, d.Rev, err)
		}
		atext = *next
	}
	return padState{atext: atext, pool: pool, head: toRev}, nil
}

// RevisionDeltas returns the stored changesets of revisions from..to.
func (p *Pad) RevisionDeltas(from, to int) ([]dbstore.RevisionDelta, error) {
	return p.db.GetRevisionDeltas(p.Id, from, to)
}

// keyRevisionInterval is how often a revision stores the full text.
const keyRevisionInterval = 100

func isKeyRevision(rev int) bool { return rev%keyRevisionInterval == 0 }

// keyRevisionState loads the newest key revision at or before rev that has
// its text stored and applies the changesets after it.
func (p *Pad) keyRevisionState(rev int) (padState, bool) {
	for key := rev - rev%keyRevisionInterval; key >= 0; key -= keyRevisionInterval {
		base, ok := p.storedState(key)
		if !ok {
			continue
		}
		if key == rev {
			return base, true
		}
		next, err := p.replay(base, rev)
		return next, err == nil
	}
	return padState{}, false
}

// applyMeta takes over the metadata stored in the pad row. Another instance
// (or the read-only manager) may have changed it since this copy was loaded.
func (p *Pad) applyMeta(meta *dbstore.PadMeta) {
	if meta == nil {
		return
	}
	p.PublicStatus = meta.PublicStatus
	p.ReadonlyId = meta.ReadOnlyId
	saved := make([]revision.SavedRevision, len(meta.SavedRevisions))
	for i, r := range meta.SavedRevisions {
		saved[i] = revision.SavedRevision{RevNum: r.RevNum, SavedBy: r.SavedBy, Timestamp: r.Timestamp, Label: r.Label, Id: r.Id}
	}
	p.SavedRevisions = saved
	p.settingsJSON = meta.Settings
}

// MatchesVersion reports whether the pad reflects the stored version v:
// same head, no newer chat message and the same metadata.
func (p *Pad) MatchesVersion(v *dbstore.PadVersion) bool {
	if v == nil || !v.Exists || v.Rev != p.Head || v.ChatHead > p.ChatHead {
		return false
	}
	meta := v.Meta
	if meta == nil || meta.PublicStatus != p.PublicStatus || len(meta.SavedRevisions) != len(p.SavedRevisions) {
		return false
	}
	if meta.Settings != p.settingsJSON {
		return false
	}
	if (meta.ReadOnlyId == nil) != (p.ReadonlyId == nil) || (meta.ReadOnlyId != nil && *meta.ReadOnlyId != *p.ReadonlyId) {
		return false
	}
	for i, r := range meta.SavedRevisions {
		if r.Id != p.SavedRevisions[i].Id {
			return false
		}
	}
	return true
}

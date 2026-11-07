package pad

import (
	"errors"
	"fmt"

	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/changeset"
	dbstore "github.com/ether/etherpad-go/lib/db"
	db "github.com/ether/etherpad-go/lib/models/db"
)

// ErrPadGone is returned by Refresh when the pad was deleted, for instance
// by another instance.
var ErrPadGone = errors.New("pad no longer exists")

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

// Refresh brings the pad up to the state stored in the database. Callers
// hold the pad's write lock.
func (p *Pad) Refresh() error {
	v, err := p.db.GetPadVersion(p.Id)
	if err != nil {
		return fmt.Errorf("reading the version of pad %s: %w", p.Id, err)
	}
	if !v.Exists {
		return ErrPadGone
	}
	if v.ChatHead > p.ChatHead {
		p.ChatHead = v.ChatHead
	}
	if v.Rev == p.Head || v.Rev < 0 {
		return nil
	}
	// Usually a few small changesets are missing; apply them instead of
	// loading the whole text.
	if v.Rev > p.Head && p.Head >= 0 {
		if err := p.applyStoredRevisions(v.Rev); err == nil {
			return nil
		}
	}
	return p.resync(v.Rev)
}

// reloadFromStore completes loading a pad whose row was just mapped onto p:
// the row may be behind the stored revisions, or (written by an older
// version before its revision was stored) ahead of them.
func (p *Pad) reloadFromStore() error {
	v, err := p.db.GetPadVersion(p.Id)
	if err != nil {
		return err
	}
	if !v.Exists {
		return ErrPadGone
	}
	p.ChatHead = max(p.ChatHead, v.ChatHead)
	if v.Rev < 0 {
		return errors.New("pad has no revisions")
	}
	if p.useStoredText(v.Rev) || p.loadFromKeyRevision(v.Rev) {
		return nil
	}
	switch {
	case p.Head == v.Rev:
		return nil
	case p.Head >= 0 && p.Head < v.Rev:
		if err := p.applyStoredRevisions(v.Rev); err == nil {
			return nil
		}
	}
	return p.rebuild(v.Rev)
}

// resync replaces the cached state with the stored state of rev.
func (p *Pad) resync(rev int) error {
	if p.useStoredText(rev) || p.loadFromKeyRevision(rev) {
		return nil
	}
	return p.rebuild(rev)
}

// useStoredText takes text and pool stored with rev, if it has them.
func (p *Pad) useStoredText(rev int) bool {
	stored, err := p.db.GetRevision(p.Id, rev)
	if err != nil || stored.AText.Text == "" {
		return false
	}
	p.AText = apool.FromDBAText(stored.AText)
	if storedPoolUsable(stored.Pool) {
		p.Pool = poolFromRev(stored.Pool)
	}
	p.Head = rev
	return true
}

// rebuild replays every stored changeset from the empty pad.
func (p *Pad) rebuild(rev int) error {
	p.AText = changeset.MakeAText("\n", nil)
	p.Head = -1
	return p.applyStoredRevisions(rev)
}

// applyStoredRevisions applies the stored changesets after p.Head up to
// toRev. The pool stored with toRev holds every attribute they use (pools
// only grow); where none was stored, the pad's own pool is used.
func (p *Pad) applyStoredRevisions(toRev int) error {
	deltas, storedPool, err := p.db.GetRevisionDeltas(p.Id, p.Head+1, toRev)
	if err != nil {
		return err
	}
	if len(deltas) != toRev-p.Head {
		return fmt.Errorf("pad %s: expected %d revisions after %d, got %d", p.Id, toRev-p.Head, p.Head, len(deltas))
	}
	pool := p.Pool
	if storedPoolUsable(storedPool) {
		pool = poolFromRev(storedPool)
	}
	atext := p.AText
	for i, d := range deltas {
		if d.Rev != p.Head+1+i {
			return fmt.Errorf("pad %s: revision %d missing", p.Id, p.Head+1+i)
		}
		next, err := changeset.ApplyToAText(d.Changeset, atext, pool)
		if err != nil {
			return fmt.Errorf("pad %s: applying revision %d: %w", p.Id, d.Rev, err)
		}
		atext = *next
	}
	p.AText = atext
	p.Pool = pool
	p.Head = toRev
	return nil
}

// RevisionDeltas returns the stored changesets of revisions from..to.
func (p *Pad) RevisionDeltas(from, to int) ([]dbstore.RevisionDelta, error) {
	deltas, _, err := p.db.GetRevisionDeltas(p.Id, from, to)
	return deltas, err
}

// keyRevisionInterval is how often a revision stores the full text.
const keyRevisionInterval = 100

func isKeyRevision(rev int) bool { return rev%keyRevisionInterval == 0 }

// loadFromKeyRevision loads the newest key revision at or before rev that
// has its text stored and applies the changesets after it.
func (p *Pad) loadFromKeyRevision(rev int) bool {
	for key := rev - rev%keyRevisionInterval; key >= 0; key -= keyRevisionInterval {
		stored, err := p.db.GetRevision(p.Id, key)
		if err != nil || stored.AText.Text == "" {
			continue
		}
		p.AText = apool.FromDBAText(stored.AText)
		if storedPoolUsable(stored.Pool) {
			p.Pool = poolFromRev(stored.Pool)
		}
		p.Head = key
		if key == rev {
			return true
		}
		return p.applyStoredRevisions(rev) == nil
	}
	return false
}

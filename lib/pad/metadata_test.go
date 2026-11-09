package pad

import (
	"fmt"
	"testing"

	"github.com/ether/etherpad-go/lib/db"
	modelpad "github.com/ether/etherpad-go/lib/models/pad"
	"github.com/stretchr/testify/require"
)

// The read-only id is stored by the ReadOnlyManager directly in the
// database. Saving the cached pad later must not wipe it, or every
// read-only link of the pad breaks.
func TestReadOnlyIdSurvivesKeyRevisions(t *testing.T) {
	forEachSharedStore(t, func(t *testing.T, open func(t *testing.T) db.DataStore) {
		store := open(t)
		m := newInstance(t, store)
		ro := NewReadOnlyManager(store)
		const padID = "readonlyKeep"
		appendLine(t, m, padID, "first")
		id := ro.GetReadOnlyId(padID)

		for i := 0; i < 105; i++ { // crosses the key revision at 100
			appendLine(t, m, padID, fmt.Sprint("line ", i))
		}
		require.Equal(t, id, ro.GetReadOnlyId(padID), "the read-only id must not change")
	})
}

// Metadata changed through one instance must survive writes through
// another instance whose cached copy is older.
func TestPadMetadataAcrossInstances(t *testing.T) {
	forEachSharedStore(t, func(t *testing.T, open func(t *testing.T) db.DataStore) {
		a := newInstance(t, open(t))
		b := newInstanceB(open(t))
		const padID = "metaAcross"
		appendLine(t, a, padID, "start")
		appendLine(t, b, padID, "b has it cached")

		withPad := func(m *Manager, fn func(p *modelpad.Pad)) {
			unlock := mustLock(m.LockPad(padID))
			defer unlock()
			p, err := m.GetPad(padID, nil, nil)
			require.NoError(t, err)
			fn(p)
		}

		// Saved revisions from both instances are all kept.
		withPad(a, func(p *modelpad.Pad) { require.NoError(t, p.AddSavedRevision("authorA")) })
		appendLine(t, a, padID, "more")
		withPad(b, func(p *modelpad.Pad) { require.NoError(t, p.AddSavedRevision("authorB")) })

		// The public status set through b survives a key revision written
		// through a, whose copy loaded before the change.
		withPad(b, func(p *modelpad.Pad) { require.NoError(t, p.SetPublicStatus(true)) })
		for i := 0; i < 100; i++ {
			appendLine(t, a, padID, fmt.Sprint("a ", i))
		}

		for _, m := range []*Manager{a, b, newInstanceB(open(t))} {
			withPad(m, func(p *modelpad.Pad) {
				require.True(t, p.PublicStatus, "public status kept")
				var by []string
				for _, r := range p.SavedRevisions {
					by = append(by, r.SavedBy)
				}
				require.ElementsMatch(t, []string{"authorA", "authorB"}, by, "saved revisions of both instances kept")
			})
		}
	})
}

// Instances asking for the read-only id of a new pad at the same time all
// get the same one.
func TestReadOnlyIdIsTheSameOnAllInstances(t *testing.T) {
	forEachSharedStore(t, func(t *testing.T, open func(t *testing.T) db.DataStore) {
		storeA, storeB := open(t), open(t)
		appendLine(t, newInstance(t, storeA), "readonlyRace", "x")
		ids := make(chan string, 8)
		for i := 0; i < 8; i++ {
			store := storeA
			if i%2 == 1 {
				store = storeB
			}
			go func() { ids <- NewReadOnlyManager(store).GetReadOnlyId("readonlyRace") }()
		}
		first := <-ids
		for i := 1; i < 8; i++ {
			require.Equal(t, first, <-ids)
		}
	})
}

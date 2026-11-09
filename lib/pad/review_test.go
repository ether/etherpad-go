package pad

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ether/etherpad-go/lib/db"
	dbmodel "github.com/ether/etherpad-go/lib/models/db"
	"github.com/ether/etherpad-go/lib/test/testdb"
	"github.com/stretchr/testify/require"
)

// withTimeout fails the test instead of hanging when fn does not return.
func withTimeout(t *testing.T, d time.Duration, fn func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn()
	}()
	select {
	case <-done:
	case <-time.After(d):
		t.Fatalf("did not finish within %s (deadlock?)", d)
	}
}

// A pool of one connection is a valid configuration. Database locks (pad,
// migration, leader) must not take the connection the work under them
// needs.
func TestOneConnectionPool(t *testing.T) {
	for _, kind := range []testdb.Kind{testdb.Postgres, testdb.MySQL} {
		t.Run(string(kind), func(t *testing.T) {
			srv := testdb.Get(t, kind)
			if srv == nil {
				t.Skipf("%s unavailable", kind)
			}
			name := srv.NewDatabase(t)
			var store db.DataStore
			withTimeout(t, 60*time.Second, func() { // runs the migrations
				var err error
				if kind == testdb.Postgres {
					store, err = db.NewPostgresDB(db.PostgresOptions{Username: srv.AdminUser(), Password: testdb.Password, Database: name, Host: srv.Host, Port: srv.Port, MaxConns: 1})
				} else {
					store, err = db.NewMySQLDB(db.MySQLOptions{Username: srv.AdminUser(), Password: testdb.Password, Database: name, Host: srv.Host, Port: srv.Port, MaxConns: 1})
				}
				require.NoError(t, err)
			})
			t.Cleanup(func() { store.Close() })
			m := newInstance(t, store)

			withTimeout(t, 60*time.Second, func() {
				lease, ok, err := store.TryLeadership("one-conn")
				require.NoError(t, err)
				require.True(t, ok)
				defer lease.Release()

				for i := 0; i < 5; i++ {
					appendLine(t, m, "oneConnA", fmt.Sprint("a ", i))
				}
				// Two pads at once, as copy and move do.
				unlock, err := m.LockPad("oneConnA", "oneConnB")
				require.NoError(t, err)
				a, err := m.GetPad("oneConnA", nil, nil)
				require.NoError(t, err)
				require.NoError(t, a.SpliceText(0, 0, "x", nil))
				b, err := m.GetPad("oneConnB", nil, nil)
				require.NoError(t, err)
				require.NoError(t, b.SpliceText(0, 0, "y", nil))
				unlock()

				_, text := padState(t, m, "oneConnA")
				require.True(t, strings.HasPrefix(text, "x"))
				require.Contains(t, text, "a 4")
			})
		})
	}
}

// A pad whose first revision could not be stored must not leave a row
// behind that can neither be loaded nor created again.
func TestFailedFirstRevisionLeavesNoRow(t *testing.T) {
	forEachSharedStore(t, func(t *testing.T, open func(t *testing.T) db.DataStore) {
		store := newFailingStore(open(t))
		m := newInstance(t, store)
		const padID = "firstRevFails"

		store.failRev.Store(0)
		_, err := m.GetPad(padID, nil, nil)
		require.ErrorIs(t, err, errSimulated)
		exists, err := store.DoesPadExist(padID)
		require.NoError(t, err)
		require.False(t, *exists, "the row of the failed creation is removed")

		store.failRev.Store(-1)
		appendLine(t, m, padID, "created on the second try")
		_, text := padState(t, m, padID)
		require.Contains(t, text, "created on the second try")
	})
}

// Rows without revisions written before the rollback existed are cleaned up
// on the next open.
func TestStrandedRowIsRecreated(t *testing.T) {
	forEachSharedStore(t, func(t *testing.T, open func(t *testing.T) db.DataStore) {
		store := open(t)
		const padID = "strandedRow"
		require.NoError(t, store.CreatePad(padID, dbmodel.PadDB{ID: padID, Head: -1, ChatHead: -1, SavedRevisions: []dbmodel.SavedRevision{}}))
		m := newInstance(t, store)
		appendLine(t, m, padID, "works again")
		_, text := padState(t, m, padID)
		require.Contains(t, text, "works again")
	})
}

// Opening a new pad on two instances at once must give both the same pad,
// never "pad has no revisions" for the instance that lost the race.
func TestConcurrentCreationOnTwoInstances(t *testing.T) {
	forEachSharedStore(t, func(t *testing.T, open func(t *testing.T) db.DataStore) {
		instances := []*Manager{newInstance(t, open(t)), newInstanceB(open(t))}
		for round := 0; round < 5; round++ {
			padID := fmt.Sprint("createRace", round)
			var wg sync.WaitGroup
			errs := make(chan error, 8)
			for i := 0; i < 8; i++ {
				wg.Add(1)
				go func(m *Manager) {
					defer wg.Done()
					unlock, err := m.RLockPad(padID)
					if err != nil {
						errs <- err
						return
					}
					defer unlock()
					if _, err := m.GetPad(padID, nil, nil); err != nil {
						errs <- err
					}
				}(instances[i%2])
			}
			wg.Wait()
			close(errs)
			for err := range errs {
				t.Fatalf("round %d: %v", round, err)
			}
			headA, textA := padState(t, instances[0], padID)
			headB, textB := padState(t, instances[1], padID)
			require.Equal(t, 0, headA, "created once")
			require.Equal(t, headA, headB)
			require.Equal(t, textA, textB)
		}
	})
}

// A refresh that fails leaves the cached pad as it was, and the writer is
// told so instead of working on a copy that is behind.
func TestFailedRefreshStopsWriters(t *testing.T) {
	forEachSharedStore(t, func(t *testing.T, open func(t *testing.T) db.DataStore) {
		storeA := newFailingStore(open(t))
		a := newInstance(t, storeA)
		b := newInstanceB(open(t))
		const padID = "refreshFails"
		appendLine(t, a, padID, "one")
		cached := a.globalPadCache.GetPad(padID)
		require.NotNil(t, cached)
		headBefore, textBefore := cached.Head, cached.Text()

		appendLine(t, b, padID, "written on B") // A is now behind
		storeA.failReads.Store(true)
		_, err := a.LockPad(padID)
		require.ErrorIs(t, err, ErrPadUnavailable)
		require.Equal(t, headBefore, cached.Head, "a failed refresh changes nothing")
		require.Equal(t, textBefore, cached.Text())
		// The copy that is behind was dropped; loading it again fails too,
		// so no reader gets the stale text either.
		require.Nil(t, a.globalPadCache.GetPad(padID))
		func() {
			unlock, err := a.RLockPad(padID)
			require.NoError(t, err)
			defer unlock()
			_, err = a.GetPad(padID, nil, nil)
			require.Error(t, err)
		}()

		storeA.failReads.Store(false)
		appendLine(t, a, padID, "on A again")
		_, text := padState(t, a, padID)
		require.Contains(t, text, "written on B")
		require.Contains(t, text, "on A again")
	})
}

// Readers of a cached, up-to-date pad share the lock.
func TestReadersOfACachedPadShareTheLock(t *testing.T) {
	forEachSharedStore(t, func(t *testing.T, open func(t *testing.T) db.DataStore) {
		m := newInstance(t, open(t))
		const padID = "sharedReaders"
		appendLine(t, m, padID, "cached")
		require.NotNil(t, m.globalPadCache.GetPad(padID))

		unlock1, err := m.RLockPad(padID)
		require.NoError(t, err)
		defer unlock1()
		withTimeout(t, 5*time.Second, func() {
			unlock2, err := m.RLockPad(padID)
			require.NoError(t, err)
			unlock2()
		})
	})
}

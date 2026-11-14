package db_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/test/testdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Two instances on one PostgreSQL database must exclude each other with the
// pad and leader locks. With EP_TEST_POSTGRES_PGBOUNCER=1 the connections go
// through PgBouncer in transaction mode, where session-level advisory locks
// would land on server connections shared with others and stop excluding.
func TestPostgresLocksAcrossInstances(t *testing.T) {
	srv := testdb.Get(t, testdb.Postgres)
	name := srv.NewDatabase(t)
	open := func() *db.PostgresDB {
		store, err := db.NewPostgresDB(db.PostgresOptions{
			Username: srv.AdminUser(), Password: testdb.Password, Database: name,
			Host: srv.Host, Port: srv.Port, MaxConns: 4, PgBouncer: testdb.PgBouncer(),
		})
		require.NoError(t, err)
		t.Cleanup(func() { _ = store.Close() })
		return store
	}
	stores := []*db.PostgresDB{open(), open()}

	t.Run("pad locks exclude each other", func(t *testing.T) {
		var inside, overlaps, done atomic.Int32
		var wg sync.WaitGroup
		for w := 0; w < 8; w++ {
			wg.Add(1)
			go func(store *db.PostgresDB) {
				defer wg.Done()
				for i := 0; i < 15; i++ {
					release, err := store.LockPadsExclusive([]string{"contended"})
					if !assert.NoError(t, err) {
						return
					}
					if inside.Add(1) > 1 {
						overlaps.Add(1)
					}
					time.Sleep(time.Millisecond)
					inside.Add(-1)
					release()
					done.Add(1)
				}
			}(stores[w%2])
		}
		wg.Wait()
		assert.Zero(t, overlaps.Load(), "two holders of one pad lock at once")
		assert.EqualValues(t, 8*15, done.Load())
	})

	t.Run("a released pad lock is free for the other instance", func(t *testing.T) {
		release, err := stores[0].LockPadsExclusive([]string{"handover"})
		require.NoError(t, err)
		got := make(chan struct{})
		go func() {
			r, err := stores[1].LockPadsExclusive([]string{"handover"})
			if assert.NoError(t, err) {
				r()
			}
			close(got)
		}()
		select {
		case <-got:
			t.Fatal("the other instance got the lock while it was held")
		case <-time.After(300 * time.Millisecond):
		}
		release()
		select {
		case <-got:
		case <-time.After(10 * time.Second):
			t.Fatal("the lock was not released")
		}
	})

	t.Run("one leader at a time", func(t *testing.T) {
		lease, ok, err := stores[0].TryLeadership("job")
		require.NoError(t, err)
		require.True(t, ok)
		require.NoError(t, lease.Alive())

		_, ok, err = stores[1].TryLeadership("job")
		require.NoError(t, err)
		assert.False(t, ok, "a second leader")
		// Asking again on the same instance does not take it twice either.
		_, ok, err = stores[0].TryLeadership("job")
		require.NoError(t, err)
		assert.False(t, ok, "the lock was taken twice")

		lease.Release()
		lease2, ok, err := stores[1].TryLeadership("job")
		require.NoError(t, err)
		require.True(t, ok, "the released leadership is not free")
		require.NoError(t, lease2.Alive())
		lease2.Release()
	})
}

package pad

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/hooks"
	"github.com/ether/etherpad-go/lib/test/testdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Several Etherpad instances on one database are modelled as several
// Managers, each with its own pad cache. For the SQL servers every instance
// also gets its own connection pool, like separate processes.

type sharedStore struct {
	name string
	// open returns the datastore for one more instance.
	open func(t *testing.T) db.DataStore
}

func sharedStores(t *testing.T) []sharedStore {
	sqlite := func() sharedStore {
		var once sync.Once
		var store db.DataStore
		return sharedStore{"SQLite", func(t *testing.T) db.DataStore {
			once.Do(func() {
				s, err := db.NewSQLiteDB(":memory:")
				require.NoError(t, err)
				store = s
			})
			return store
		}}
	}
	memory := func() sharedStore {
		store := db.NewMemoryDataStore()
		return sharedStore{"Memory", func(*testing.T) db.DataStore { return store }}
	}
	server := func(kind testdb.Kind) sharedStore {
		var once sync.Once
		var name string
		var srv *testdb.Server
		return sharedStore{string(kind), func(t *testing.T) db.DataStore {
			once.Do(func() {
				srv = testdb.Get(t, kind)
				name = srv.NewDatabase(t)
			})
			var store db.DataStore
			var err error
			if kind == testdb.Postgres {
				store, err = db.NewPostgresDB(db.PostgresOptions{Username: srv.AdminUser(), Password: testdb.Password, Database: name, Host: srv.Host, Port: srv.Port, MaxConns: 8})
			} else {
				store, err = db.NewMySQLDB(db.MySQLOptions{Username: srv.AdminUser(), Password: testdb.Password, Database: name, Host: srv.Host, Port: srv.Port, MaxConns: 8})
			}
			require.NoError(t, err)
			t.Cleanup(func() { store.Close() })
			return store
		}}
	}
	return []sharedStore{memory(), sqlite(), server(testdb.Postgres), server(testdb.MySQL)}
}

func forEachSharedStore(t *testing.T, test func(t *testing.T, open func(t *testing.T) db.DataStore)) {
	for _, s := range sharedStores(t) {
		t.Run(s.name, func(t *testing.T) {
			t.Parallel()
			test(t, s.open)
		})
	}
}

func newInstance(t *testing.T, store db.DataStore) *Manager {
	t.Helper()
	return newInstanceB(store)
}

func newInstanceB(store db.DataStore) *Manager {
	h := hooks.NewHook()
	return NewManager(store, &h)
}

// appendLine appends line to the pad through m the way every writer does:
// under LockPad.
func appendLine(t *testing.T, m *Manager, padID, line string) {
	t.Helper()
	unlock := m.LockPad(padID)
	defer unlock()
	p, err := m.GetPad(padID, nil, nil)
	require.NoError(t, err)
	end := utf8.RuneCountInString(p.Text()) - 1
	require.NoError(t, p.SpliceText(end, 0, line+"\n", nil))
}

func countLines(text string) map[string]int {
	count := map[string]int{}
	for _, l := range strings.Split(text, "\n") {
		count[l]++
	}
	return count
}

func TestInstancesSharingADatabaseNeverLoseChanges(t *testing.T) {
	forEachSharedStore(t, func(t *testing.T, open func(t *testing.T) db.DataStore) {
		const instances, writers, perWriter = 3, 4, 10
		padID := "shared-pad"
		managers := make([]*Manager, instances)
		for i := range managers {
			managers[i] = newInstance(t, open(t))
		}
		appendLine(t, managers[0], padID, "start")

		var wg sync.WaitGroup
		for i, m := range managers {
			for w := 0; w < writers; w++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for n := 0; n < perWriter; n++ {
						appendLine(t, m, padID, fmt.Sprintf("i%d-w%d-n%d", i, w, n))
					}
				}()
			}
		}
		wg.Wait()

		// What a fresh instance loads, and what every instance sees after
		// catching up, contains every line exactly once.
		check := func(text string) {
			count := countLines(text)
			for i := 0; i < instances; i++ {
				for w := 0; w < writers; w++ {
					for n := 0; n < perWriter; n++ {
						line := fmt.Sprintf("i%d-w%d-n%d", i, w, n)
						assert.Equal(t, 1, count[line], line)
					}
				}
			}
		}
		fresh := newInstance(t, open(t))
		p, err := fresh.GetPad(padID, nil, nil)
		require.NoError(t, err)
		check(p.Text())
		assert.Equal(t, 1+instances*writers*perWriter, p.Head+0, "one revision per line plus the pad's first")
		for _, m := range managers {
			unlock := m.RLockPad(padID)
			p, err := m.GetPad(padID, nil, nil)
			require.NoError(t, err)
			assert.Equal(t, fresh.globalPadCache.GetPad(padID).Text(), p.Text())
			unlock()
		}
	})
}

// A writer that did not catch up (for instance because the database lock
// failed) gets a conflict instead of overwriting the other instance's
// revision, and its pad is left as it was.
func TestStaleWriterGetsAConflict(t *testing.T) {
	forEachSharedStore(t, func(t *testing.T, open func(t *testing.T) db.DataStore) {
		a, b := newInstance(t, open(t)), newInstance(t, open(t))
		padID := "stale-pad"
		appendLine(t, a, padID, "first")
		stale, err := b.GetPad(padID, nil, nil)
		require.NoError(t, err)
		appendLine(t, a, padID, "from-a")

		before, head := stale.Text(), stale.Head
		err = stale.SpliceText(0, 0, "from-b\n", nil)
		require.ErrorIs(t, err, db.ErrConflict)
		assert.Equal(t, before, stale.Text())
		assert.Equal(t, head, stale.Head)

		require.NoError(t, stale.Refresh())
		assert.Equal(t, a.globalPadCache.GetPad(padID).Text(), stale.Text())
		assert.Contains(t, stale.Text(), "from-a")
	})
}

func TestChatMessagesFromSeveralInstancesAreAllKept(t *testing.T) {
	forEachSharedStore(t, func(t *testing.T, open func(t *testing.T) db.DataStore) {
		a, b := newInstance(t, open(t)), newInstance(t, open(t))
		padID := "chat-pad"
		appendLine(t, a, padID, "x")
		pa, err := a.GetPad(padID, nil, nil)
		require.NoError(t, err)
		pb, err := b.GetPad(padID, nil, nil)
		require.NoError(t, err)

		// Both instances think the next message number is free.
		_, err = pa.AppendChatMessage(nil, 1, "from a")
		require.NoError(t, err)
		_, err = pb.AppendChatMessage(nil, 2, "from b")
		require.NoError(t, err)

		msgs, err := pa.GetChatMessages(0, 10)
		require.NoError(t, err)
		var texts []string
		for _, m := range *msgs {
			texts = append(texts, m.Message)
		}
		assert.ElementsMatch(t, []string{"from a", "from b"}, texts)
	})
}

// Older versions wrote the pad row before the revision; a row whose text
// was never stored as a revision must not win.
func TestLoadingIgnoresAPadRowAheadOfTheRevisions(t *testing.T) {
	forEachSharedStore(t, func(t *testing.T, open func(t *testing.T) db.DataStore) {
		store := open(t)
		m := newInstance(t, store)
		padID := "row-ahead"
		appendLine(t, m, padID, "stored")
		p := m.globalPadCache.GetPad(padID)
		stored := p.Text()

		row, err := store.GetPad(padID)
		require.NoError(t, err)
		row.Head++
		row.ATextText = "never stored\n"
		row.ATextAttribs = "|1+d"
		require.NoError(t, store.CreatePad(padID, *row))

		fresh := newInstance(t, open(t))
		loaded, err := fresh.GetPad(padID, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, stored, loaded.Text())
		assert.Equal(t, p.Head, loaded.Head)
	})
}

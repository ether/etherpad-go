package testutils

import (
	"testing"

	"github.com/ether/etherpad-go/lib/db"
)

// RunSequentialMemorySQLite runs conf against the in-process datastores only
// (Memory and SQLite), for tests that don't need the SQL servers.
func RunSequentialMemorySQLite(t *testing.T, conf TestRunConfig) {
	t.Helper()
	handler := NewTestDBHandler(t)
	datastores := []struct {
		name  string
		newDS func(t *testing.T) db.DataStore
	}{
		{"Memory", func(*testing.T) db.DataStore { return db.NewMemoryDataStore() }},
		{"SQLite", func(t *testing.T) db.DataStore {
			sqliteDB, err := db.NewSQLiteDB(":memory:")
			if err != nil {
				t.Fatalf("Failed to create SQLite DataStore: %v", err)
			}
			return sqliteDB
		}},
	}
	for _, d := range datastores {
		t.Run(d.name, func(t *testing.T) {
			handler.TestRun(t, conf, d.newDS, d.name)
		})
	}
}

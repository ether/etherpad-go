package migration

import (
	"database/sql"
	"testing"

	"github.com/ether/etherpad-go/lib/db"
	"github.com/stretchr/testify/assert"
)

func setupSQLiteSource(t *testing.T) *SQLDatabase {
	t.Helper()

	sqlDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}

	_, err = sqlDB.Exec(`
		CREATE TABLE store (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		);
	`)
	if err != nil {
		t.Fatal(err)
	}

	source, err := NewSQLDatabase(sqlDB, DriverSQLite)
	assert.NoError(t, err)

	insertData(t, sqlDB, insertKV)

	return source
}

func setupSQLiteTarget(t *testing.T) db.DataStore {
	t.Helper()

	target, err := db.NewSQLiteDB(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	return target
}

func TestMigrator_SQLite_To_SQLite(t *testing.T) {
	source := setupSQLiteSource(t)
	target := setupSQLiteTarget(t)

	startMigratorPipeline(t, source, target)
}

package migrations

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func sqliteHasTokenColumn(t *testing.T, db *sql.DB) bool {
	t.Helper()
	rows, err := db.Query(`PRAGMA table_info(globalAuthor)`)
	if err != nil {
		t.Fatalf("PRAGMA: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notnull, pk int
		var name, ctype string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if name == "token" {
			return true
		}
	}
	return false
}

func TestMigration009BackfillsMissingTokenColumn(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	// Simulate a stale DB: globalAuthor WITHOUT a token column.
	if _, err := db.Exec(`CREATE TABLE globalAuthor (id TEXT PRIMARY KEY, colorId TEXT, name TEXT, timestamp INTEGER)`); err != nil {
		t.Fatalf("create: %v", err)
	}
	if sqliteHasTokenColumn(t, db) {
		t.Fatal("precondition: token column should be absent")
	}

	if err := migration009AuthorTokenBackfill().Up(db, DialectSQLite); err != nil {
		t.Fatalf("migration up: %v", err)
	}
	if !sqliteHasTokenColumn(t, db) {
		t.Fatal("token column was not added")
	}

	// Idempotent: running again on a table that already has it must not error.
	if err := migration009AuthorTokenBackfill().Up(db, DialectSQLite); err != nil {
		t.Fatalf("second run not idempotent: %v", err)
	}
}

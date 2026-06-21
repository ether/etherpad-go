package migrations

import (
	"database/sql"
	"strings"
)

// migration009AuthorTokenBackfill ensures globalAuthor has a `token` column.
//
// The column is part of the current migration 001 schema, but databases created
// by an earlier revision of 001 (before `token` was added) never received it —
// CREATE TABLE IF NOT EXISTS does not alter an existing table. The author/token
// queries (GetAuthorByToken / SetAuthorByToken / RemoveTokenOfAuthor) then fail
// with "column token does not exist", blocking all new-author creation. This
// migration backfills the column idempotently (it is a no-op on fresh databases
// that already have it).
func migration009AuthorTokenBackfill() Migration {
	return Migration{
		Version:     9,
		Description: "Backfill globalAuthor.token column for databases created before it existed",
		Up: func(db *sql.DB, dialect Dialect) error {
			switch dialect {
			case DialectPostgres:
				// Postgres supports IF NOT EXISTS for ADD COLUMN — inherently idempotent.
				_, err := db.Exec(`ALTER TABLE globalauthor ADD COLUMN IF NOT EXISTS token TEXT`)
				return err
			case DialectMySQL:
				_, err := db.Exec(`ALTER TABLE globalAuthor ADD COLUMN token VARCHAR(255)`)
				return ignoreDuplicateColumn(err)
			default: // SQLite
				_, err := db.Exec(`ALTER TABLE globalAuthor ADD COLUMN token TEXT`)
				return ignoreDuplicateColumn(err)
			}
		},
	}
}

// ignoreDuplicateColumn swallows the "duplicate column" error so the migration
// is idempotent on databases (e.g. freshly created ones) that already have the
// column, where SQLite/MySQL lack an ADD COLUMN IF NOT EXISTS form.
func ignoreDuplicateColumn(err error) error {
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "duplicate column") {
		return nil
	}
	return err
}

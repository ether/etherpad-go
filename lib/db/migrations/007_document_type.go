package migrations

import (
	"database/sql"
)

func migration007DocumentType() Migration {
	return Migration{
		Version:     7,
		Description: "Add document_type column to pad",
		Up: func(db *sql.DB, dialect Dialect) error {
			var query string
			switch dialect {
			case DialectMySQL:
				query = `ALTER TABLE pad ADD COLUMN document_type VARCHAR(32) NOT NULL DEFAULT 'text'`
			case DialectPostgres:
				query = `ALTER TABLE pad ADD COLUMN IF NOT EXISTS document_type TEXT NOT NULL DEFAULT 'text'`
			default:
				query = `ALTER TABLE pad ADD COLUMN document_type TEXT NOT NULL DEFAULT 'text'`
			}
			if _, err := db.Exec(query); err != nil {
				return err
			}
			return nil
		},
	}
}

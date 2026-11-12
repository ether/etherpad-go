package migrations

import "database/sql"

// migration012PadSettings adds pad.settings, the pad-wide settings set by the
// pad's creator (etherpad-lite's padSettings), stored as JSON. NULL means the
// pad uses the server defaults.
func migration012PadSettings() Migration {
	return Migration{
		Version:     12,
		Description: "Add pad.settings for pad-wide settings",
		Up: func(db *sql.DB, dialect Dialect) error {
			var query string
			switch dialect {
			case DialectMySQL:
				query = `ALTER TABLE pad ADD COLUMN settings LONGTEXT NULL`
			case DialectPostgres:
				query = `ALTER TABLE pad ADD COLUMN IF NOT EXISTS settings TEXT NULL`
			default:
				query = `ALTER TABLE pad ADD COLUMN settings TEXT NULL`
			}
			_, err := db.Exec(query)
			return err
		},
	}
}

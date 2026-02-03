package migrations

import (
	"database/sql"
)

func migration002ServerVersion() Migration {
	return Migration{
		Version:     2,
		Description: "Create server_version table",
		Up: func(db *sql.DB, dialect Dialect) error {
			var query string
			switch dialect {
			case DialectMySQL:
				query = `CREATE TABLE IF NOT EXISTS server_version (
					version VARCHAR(255) PRIMARY KEY,
					updated_at TIMESTAMP(6) DEFAULT CURRENT_TIMESTAMP(6)
				)`
			case DialectPostgres:
				query = `CREATE TABLE IF NOT EXISTS server_version (
					version TEXT PRIMARY KEY,
					updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
				)`
			default:
				query = `CREATE TABLE IF NOT EXISTS server_version (
					version TEXT PRIMARY KEY,
					updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
				)`
			}

			if _, err := db.Exec(query); err != nil {
				return err
			}
			return nil
		},
	}
}

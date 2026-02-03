package migrations

import (
	"database/sql"
)

// migration002FiberSessions creates the fiber_sessions table for Fiber session storage
func migration002FiberSessions() Migration {
	return Migration{
		Version:     2,
		Description: "Create fiber_sessions table for Fiber session storage",
		Up: func(db *sql.DB, dialect Dialect) error {
			var query string

			switch dialect {
			case DialectMySQL:
				query = `CREATE TABLE IF NOT EXISTS fiber_sessions (
					session_key VARCHAR(255) PRIMARY KEY,
					session_data LONGBLOB,
					expires_at BIGINT,
					created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
					updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
						ON UPDATE CURRENT_TIMESTAMP,
					INDEX idx_fiber_sessions_expires (expires_at)
				)`
			case DialectPostgres:
				query = `CREATE TABLE IF NOT EXISTS fiber_sessions (
					session_key TEXT PRIMARY KEY,
					session_data BYTEA,
					expires_at BIGINT,
					created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
					updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
				)`
			default: // SQLite
				query = `CREATE TABLE IF NOT EXISTS fiber_sessions (
					session_key TEXT PRIMARY KEY,
					session_data BLOB,
					expires_at INTEGER,
					created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
					updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
				)`
			}

			_, err := db.Exec(query)
			if err != nil {
				return err
			}

			// Create index on expires_at for efficient cleanup (only for non-MySQL)
			if dialect != DialectMySQL {
				var indexQuery string
				switch dialect {
				case DialectPostgres:
					indexQuery = `CREATE INDEX IF NOT EXISTS idx_fiber_sessions_expires ON fiber_sessions (expires_at)`
				default:
					indexQuery = `CREATE INDEX IF NOT EXISTS idx_fiber_sessions_expires ON fiber_sessions (expires_at)`
				}

				_, err = db.Exec(indexQuery)
			}

			return err
		},
	}
}

package migrations

import (
	"database/sql"
)

// GetMigrations returns all available migrations
func GetMigrations() []Migration {
	return []Migration{
		migration001InitialSchema(),
		migration002ServerVersion(),
		migration003FiberSessions(),
	}
}

// migration001InitialSchema creates the initial database schema
func migration001InitialSchema() Migration {
	return Migration{
		Version:     1,
		Description: "Initial schema - create all tables",
		Up: func(db *sql.DB, dialect Dialect) error {
			var queries []string

			switch dialect {
			case DialectMySQL:
				queries = getMySQLInitialSchema()
			case DialectPostgres:
				queries = getPostgresInitialSchema()
			default:
				queries = getSQLiteInitialSchema()
			}

			for _, query := range queries {
				if _, err := db.Exec(query); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func getSQLiteInitialSchema() []string {
	return []string{
		// PAD
		`CREATE TABLE IF NOT EXISTS pad (
			id TEXT PRIMARY KEY,
			head INTEGER NOT NULL DEFAULT 0,
			saved_revisions TEXT DEFAULT NULL,
			readonly_id TEXT UNIQUE,
			pool TEXT DEFAULT NULL,
			chat_head INTEGER NOT NULL DEFAULT -1,
			public_status INTEGER NOT NULL DEFAULT 0,
			atext_text TEXT,
			atext_attribs TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// GLOBAL AUTHOR
		`CREATE TABLE IF NOT EXISTS globalAuthor (
			id TEXT PRIMARY KEY,
			colorId TEXT,
			name TEXT,
			timestamp INTEGER,
			token TEXT UNIQUE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// PAD REVISIONS
		`CREATE TABLE IF NOT EXISTS padRev (
			id TEXT NOT NULL,
			rev INTEGER NOT NULL,
			changeset TEXT,
			atextText TEXT,
			atextAttribs TEXT,
			authorId TEXT,
			timestamp INTEGER,
			pool TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (id, rev),
			FOREIGN KEY (id) REFERENCES pad(id) ON DELETE CASCADE,
			FOREIGN KEY (authorId) REFERENCES globalAuthor(id) ON DELETE SET NULL
		)`,

		// SESSION STORAGE
		`CREATE TABLE IF NOT EXISTS sessionstorage (
			id TEXT PRIMARY KEY,
			originalMaxAge INTEGER,
			expires TEXT,
			secure INTEGER,
			httpOnly INTEGER,
			path TEXT,
			sameSite TEXT,
			connections TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// GROUPS
		`CREATE TABLE IF NOT EXISTS groupPadGroup (
			id TEXT PRIMARY KEY,
			name TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// CHAT
		`CREATE TABLE IF NOT EXISTS padChat (
			padId TEXT NOT NULL,
			padHead INTEGER NOT NULL,
			chatText TEXT NOT NULL,
			authorId TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (padId, padHead),
			FOREIGN KEY (padId) REFERENCES pad(id) ON DELETE CASCADE,
			FOREIGN KEY (authorId) REFERENCES globalAuthor(id) ON DELETE SET NULL
		)`,
	}
}

func getPostgresInitialSchema() []string {
	return []string{
		// PAD
		`CREATE TABLE IF NOT EXISTS pad (
			id TEXT PRIMARY KEY,
			head INTEGER NOT NULL DEFAULT 0,
			saved_revisions JSONB,
			readonly_id TEXT UNIQUE,
			pool JSONB,
			chat_head INTEGER NOT NULL DEFAULT -1,
			public_status BOOLEAN NOT NULL DEFAULT FALSE,
			atext_text TEXT,
			atext_attribs TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// GLOBAL AUTHOR
		`CREATE TABLE IF NOT EXISTS globalAuthor (
			id TEXT PRIMARY KEY,
			colorId TEXT,
			name TEXT,
			timestamp BIGINT,
			token TEXT UNIQUE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// PAD REVISIONS
		`CREATE TABLE IF NOT EXISTS padRev (
			id TEXT NOT NULL,
			rev INTEGER NOT NULL,
			changeset TEXT,
			atextText TEXT,
			atextAttribs TEXT,
			authorId TEXT,
			timestamp BIGINT,
			pool JSONB,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (id, rev),
			FOREIGN KEY (id) REFERENCES pad(id) ON DELETE CASCADE,
			FOREIGN KEY (authorId) REFERENCES globalAuthor(id) ON DELETE SET NULL
		)`,

		// SESSION STORAGE
		`CREATE TABLE IF NOT EXISTS sessionstorage (
			id TEXT PRIMARY KEY,
			originalMaxAge INTEGER,
			expires TEXT,
			secure BOOLEAN,
			httpOnly BOOLEAN,
			path TEXT,
			sameSite TEXT,
			connections TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// GROUPS
		`CREATE TABLE IF NOT EXISTS groupPadGroup (
			id TEXT PRIMARY KEY,
			name TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// CHAT
		`CREATE TABLE IF NOT EXISTS padChat (
			padId TEXT NOT NULL,
			padHead INTEGER NOT NULL,
			chatText TEXT NOT NULL,
			authorId TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			timestamp BIGINT,
			PRIMARY KEY (padId, padHead),
			FOREIGN KEY (padId) REFERENCES pad(id) ON DELETE CASCADE,
			FOREIGN KEY (authorId) REFERENCES globalAuthor(id) ON DELETE SET NULL
		)`,
	}
}

func getMySQLInitialSchema() []string {
	return []string{
		// PAD
		`CREATE TABLE IF NOT EXISTS pad (
			id VARCHAR(255) PRIMARY KEY,
			head INT NOT NULL DEFAULT 0,
			saved_revisions JSON NULL,
			readonly_id VARCHAR(255) UNIQUE,
			pool JSON NULL,
			chat_head INT NOT NULL DEFAULT -1,
			public_status BOOLEAN NOT NULL DEFAULT FALSE,
			atext_text TEXT,
			atext_attribs TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
				ON UPDATE CURRENT_TIMESTAMP
		)`,

		// GLOBAL AUTHOR
		`CREATE TABLE IF NOT EXISTS globalAuthor (
			id VARCHAR(255) PRIMARY KEY,
			colorId VARCHAR(50),
			name VARCHAR(255),
			timestamp BIGINT,
			token VARCHAR(255) UNIQUE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
				ON UPDATE CURRENT_TIMESTAMP
		)`,

		// PAD REVISIONS
		`CREATE TABLE IF NOT EXISTS padRev (
			id VARCHAR(255) NOT NULL,
			rev INT NOT NULL,
			changeset TEXT,
			atextText TEXT,
			atextAttribs TEXT,
			authorId VARCHAR(255),
			timestamp BIGINT,
			pool JSON,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
				ON UPDATE CURRENT_TIMESTAMP,
			PRIMARY KEY (id, rev),
			FOREIGN KEY (id) REFERENCES pad(id) ON DELETE CASCADE,
			FOREIGN KEY (authorId) REFERENCES globalAuthor(id) ON DELETE SET NULL
		)`,

		// SESSION STORAGE
		`CREATE TABLE IF NOT EXISTS sessionstorage (
			id VARCHAR(255) PRIMARY KEY,
			originalMaxAge INT,
			expires VARCHAR(255),
			secure BOOLEAN,
			httpOnly BOOLEAN,
			path VARCHAR(255),
			sameSite VARCHAR(50),
			connections TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
				ON UPDATE CURRENT_TIMESTAMP
		)`,

		// GROUPS
		`CREATE TABLE IF NOT EXISTS groupPadGroup (
			id VARCHAR(255) PRIMARY KEY,
			name VARCHAR(255),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// CHAT
		`CREATE TABLE IF NOT EXISTS padChat (
			padId VARCHAR(255) NOT NULL,
			padHead INT NOT NULL,
			chatText TEXT NOT NULL,
			authorId VARCHAR(255),
    		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			timestamp BIGINT,
			PRIMARY KEY (padId, padHead),
			FOREIGN KEY (padId) REFERENCES pad(id) ON DELETE CASCADE,
			FOREIGN KEY (authorId) REFERENCES globalAuthor(id) ON DELETE SET NULL
		)`,
	}
}

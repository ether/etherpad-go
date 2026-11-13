package migrations

import "database/sql"

// migration014APIKeys adds the keys of the HTTP API (/api), managed in the
// admin UI. key_hash is the SHA-256 of the key; the key itself is not
// stored.
func migration014APIKeys() Migration {
	return Migration{
		Version:     14,
		Description: "Create api_keys table for HTTP API keys",
		Up: func(db *sql.DB, dialect Dialect) error {
			query := `CREATE TABLE IF NOT EXISTS api_keys (
				id TEXT PRIMARY KEY,
				description TEXT NOT NULL,
				prefix TEXT NOT NULL,
				key_hash TEXT NOT NULL UNIQUE,
				created_at BIGINT NOT NULL,
				last_used BIGINT NULL,
				disabled BOOLEAN NOT NULL DEFAULT FALSE
			)`
			if dialect == DialectMySQL {
				query = `CREATE TABLE IF NOT EXISTS api_keys (
					id VARCHAR(64) PRIMARY KEY,
					description TEXT NOT NULL,
					prefix VARCHAR(32) NOT NULL,
					key_hash VARCHAR(64) NOT NULL UNIQUE,
					created_at BIGINT NOT NULL,
					last_used BIGINT NULL,
					disabled BOOLEAN NOT NULL DEFAULT FALSE
				) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin`
			}
			_, err := db.Exec(query)
			return err
		},
	}
}

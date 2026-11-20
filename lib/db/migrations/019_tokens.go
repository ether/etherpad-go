package migrations

import "database/sql"

// migration019Tokens adds the tokens of the accounts (lib/tokens): API keys
// and, later, OAuth tokens, each with its own scopes, pad pattern and
// expiry. Only a SHA-256 hash of a key is stored. Times are Unix
// milliseconds; scopes and limits are JSON.
func migration019Tokens() Migration {
	return Migration{
		Version:     19,
		Description: "Tokens of accounts",
		Up: func(db *sql.DB, dialect Dialect) error {
			id, name, text, suffix, index := "TEXT", "TEXT", "TEXT", "", "CREATE INDEX IF NOT EXISTS"
			if dialect == DialectMySQL {
				id, name, text, suffix, index = "VARCHAR(64)", "VARCHAR(255)", "MEDIUMTEXT", " CHARACTER SET utf8mb4 COLLATE utf8mb4_bin", "CREATE INDEX"
			}
			queries := []string{
				`CREATE TABLE IF NOT EXISTS tokens (
					id ` + id + ` PRIMARY KEY,
					account_id ` + id + ` NOT NULL,
					name ` + name + ` NOT NULL,
					hash VARCHAR(64) NOT NULL UNIQUE,
					prefix VARCHAR(32) NOT NULL,
					scopes ` + text + ` NOT NULL,
					pad_pattern ` + name + ` NOT NULL,
					expires_at BIGINT NULL,
					max_expires_at BIGINT NULL,
					revoked_at BIGINT NULL,
					replaced_by ` + id + ` NOT NULL,
					predecessor ` + id + ` NOT NULL,
					client_id ` + name + ` NOT NULL,
					created_by ` + id + ` NOT NULL,
					created_at BIGINT NOT NULL,
					last_used_at BIGINT NULL,
					last_used_ip ` + name + ` NOT NULL,
					limits ` + text + ` NOT NULL
				)` + suffix,
				index + ` idx_tokens_account ON tokens (account_id)`,
			}
			for _, q := range queries {
				if _, err := db.Exec(q); err != nil && !isDuplicateIndex(err) {
					return err
				}
			}
			return nil
		},
	}
}

package migrations

import "database/sql"

// migration006SecretRotation creates a dedicated table that backs the
// SecretRotator. Each row holds one published set of key-derivation
// parameters (payload) for a given rotator namespace (prefix). Multiple
// Etherpad instances sharing the same database cooperate through this table.
func migration006SecretRotation() Migration {
	return Migration{
		Version:     6,
		Description: "Secret rotation - create table for rotated signing secret parameters",
		Up: func(db *sql.DB, dialect Dialect) error {
			switch dialect {
			case DialectMySQL:
				// MySQL/MariaDB does not support CREATE INDEX IF NOT EXISTS, so
				// the prefix index is declared inline in the table definition.
				_, err := db.Exec(`CREATE TABLE IF NOT EXISTS secret_rotation (
					id VARCHAR(255) PRIMARY KEY,
					prefix VARCHAR(255) NOT NULL,
					payload LONGTEXT NOT NULL,
					updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
					INDEX idx_secret_rotation_prefix (prefix)
				)`)
				return err
			case DialectPostgres:
				if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS secret_rotation (
					id TEXT PRIMARY KEY,
					prefix TEXT NOT NULL,
					payload TEXT NOT NULL,
					updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
				)`); err != nil {
					return err
				}
				_, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_secret_rotation_prefix ON secret_rotation (prefix)`)
				return err
			default:
				if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS secret_rotation (
					id TEXT PRIMARY KEY,
					prefix TEXT NOT NULL,
					payload TEXT NOT NULL,
					updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
				)`); err != nil {
					return err
				}
				_, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_secret_rotation_prefix ON secret_rotation (prefix)`)
				return err
			}
		},
	}
}

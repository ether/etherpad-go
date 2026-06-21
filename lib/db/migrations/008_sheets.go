package migrations

import "database/sql"

func migration008Sheets() Migration {
	return Migration{
		Version:     8,
		Description: "Create sheet and sheet_op tables",
		Up: func(db *sql.DB, dialect Dialect) error {
			var stmts []string
			switch dialect {
			case DialectMySQL:
				stmts = []string{
					`CREATE TABLE IF NOT EXISTS sheet (
						id VARCHAR(255) PRIMARY KEY,
						head INT NOT NULL DEFAULT 0,
						snapshot LONGTEXT,
						created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
						updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
						FOREIGN KEY (id) REFERENCES pad(id) ON DELETE CASCADE
					)`,
					`CREATE TABLE IF NOT EXISTS sheet_op (
						id VARCHAR(255) NOT NULL,
						rev INT NOT NULL,
						op LONGTEXT,
						author_id VARCHAR(255),
						timestamp BIGINT,
						created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
						PRIMARY KEY (id, rev),
						FOREIGN KEY (id) REFERENCES pad(id) ON DELETE CASCADE
					)`,
				}
			case DialectPostgres:
				stmts = []string{
					`CREATE TABLE IF NOT EXISTS sheet (
						id TEXT PRIMARY KEY,
						head INTEGER NOT NULL DEFAULT 0,
						snapshot TEXT,
						created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
						updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
						FOREIGN KEY (id) REFERENCES pad(id) ON DELETE CASCADE
					)`,
					`CREATE TABLE IF NOT EXISTS sheet_op (
						id TEXT NOT NULL,
						rev INTEGER NOT NULL,
						op TEXT,
						author_id TEXT,
						timestamp BIGINT,
						created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
						PRIMARY KEY (id, rev),
						FOREIGN KEY (id) REFERENCES pad(id) ON DELETE CASCADE
					)`,
				}
			default: // SQLite
				stmts = []string{
					`CREATE TABLE IF NOT EXISTS sheet (
						id TEXT PRIMARY KEY,
						head INTEGER NOT NULL DEFAULT 0,
						snapshot TEXT,
						created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
						updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
						FOREIGN KEY (id) REFERENCES pad(id) ON DELETE CASCADE
					)`,
					`CREATE TABLE IF NOT EXISTS sheet_op (
						id TEXT NOT NULL,
						rev INTEGER NOT NULL,
						op TEXT,
						author_id TEXT,
						timestamp INTEGER,
						created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
						PRIMARY KEY (id, rev),
						FOREIGN KEY (id) REFERENCES pad(id) ON DELETE CASCADE
					)`,
				}
			}
			for _, q := range stmts {
				if _, err := db.Exec(q); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

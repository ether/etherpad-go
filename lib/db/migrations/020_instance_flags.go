package migrations

import "database/sql"

// migration020InstanceFlags adds flags of the instance: steps that ran once
// for the whole database (a cluster shares them), such as taking over the
// API key of an APIKEY.txt. A flag is set once and never changed.
func migration020InstanceFlags() Migration {
	return Migration{
		Version:     20,
		Description: "Flags of the instance, set once",
		Up: func(db *sql.DB, dialect Dialect) error {
			name, text, suffix := "TEXT", "TEXT", ""
			if dialect == DialectMySQL {
				name, text, suffix = "VARCHAR(255)", "MEDIUMTEXT", " CHARACTER SET utf8mb4 COLLATE utf8mb4_bin"
			}
			_, err := db.Exec(`CREATE TABLE IF NOT EXISTS instance_flags (
				name ` + name + ` PRIMARY KEY,
				value ` + text + ` NOT NULL,
				created_at BIGINT NOT NULL
			)` + suffix)
			return err
		},
	}
}

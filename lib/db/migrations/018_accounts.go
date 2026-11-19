package migrations

import "database/sql"

// migration018Accounts adds accounts, roles, role assignments and the audit
// log of the account management (lib/accounts). A role keeps its scopes,
// included roles and limits as JSON. Times are Unix milliseconds.
func migration018Accounts() Migration {
	return Migration{
		Version:     18,
		Description: "Accounts, roles, role assignments and audit log",
		Up: func(db *sql.DB, dialect Dialect) error {
			// id and name columns are short strings on MySQL, which
			// cannot index TEXT; utf8mb4_bin compares them as written.
			id, name, text, suffix, index := "TEXT", "TEXT", "TEXT", "", "CREATE INDEX IF NOT EXISTS"
			if dialect == DialectMySQL {
				id, name, text, suffix, index = "VARCHAR(64)", "VARCHAR(255)", "MEDIUMTEXT", " CHARACTER SET utf8mb4 COLLATE utf8mb4_bin", "CREATE INDEX"
			}
			queries := []string{
				`CREATE TABLE IF NOT EXISTS accounts (
					id ` + id + ` PRIMARY KEY,
					username ` + name + ` NOT NULL UNIQUE,
					display_name ` + text + ` NOT NULL,
					email ` + text + ` NOT NULL,
					kind VARCHAR(16) NOT NULL,
					password_hash ` + text + ` NOT NULL,
					must_change_password BOOLEAN NOT NULL DEFAULT FALSE,
					disabled_at BIGINT NULL,
					author_id ` + name + ` NOT NULL,
					created_at BIGINT NOT NULL,
					last_login_at BIGINT NULL
				)` + suffix,
				`CREATE TABLE IF NOT EXISTS roles (
					id ` + id + ` PRIMARY KEY,
					name ` + name + ` NOT NULL UNIQUE,
					description ` + text + ` NOT NULL,
					builtin BOOLEAN NOT NULL DEFAULT FALSE,
					scopes ` + text + ` NOT NULL,
					includes ` + text + ` NOT NULL,
					limits ` + text + ` NOT NULL
				)` + suffix,
				`CREATE TABLE IF NOT EXISTS role_assignments (
					account_id ` + id + ` NOT NULL,
					role_id ` + id + ` NOT NULL,
					pad_pattern ` + name + ` NOT NULL,
					created_by ` + id + ` NOT NULL,
					created_at BIGINT NOT NULL,
					PRIMARY KEY (account_id, role_id, pad_pattern)
				)` + suffix,
				`CREATE TABLE IF NOT EXISTS audit_log (
					id ` + id + ` PRIMARY KEY,
					at BIGINT NOT NULL,
					actor_id ` + id + ` NOT NULL,
					action ` + name + ` NOT NULL,
					target ` + name + ` NOT NULL,
					details ` + text + ` NOT NULL
				)` + suffix,
				index + ` idx_role_assignments_role ON role_assignments (role_id)`,
				index + ` idx_audit_log_at ON audit_log (at)`,
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

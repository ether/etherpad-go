package migrations

import (
	"context"
	"database/sql"
	"fmt"
)

// migration013MySQLBinaryIDs makes MySQL compare ids as they are. MySQL's
// default collations ignore case, so the pads "Foo" and "foo" were one row:
// opening one gave the other's text, and creating the second changed the
// first. Every table is converted to utf8mb4 with the binary collation, as
// ids are on SQLite and Postgres. utf8mb4 holds every character of the
// older charsets, so no text changes. Large databases take a while: each
// table is rewritten once.
func migration013MySQLBinaryIDs() Migration {
	return Migration{
		Version:     13,
		Description: "MySQL: compare ids case-sensitively (utf8mb4_bin)",
		Up: func(db *sql.DB, dialect Dialect) error {
			if dialect != DialectMySQL {
				return nil
			}
			ctx := context.Background()
			// FOREIGN_KEY_CHECKS is per connection: the columns of a foreign
			// key may differ in collation only while both sides are converted.
			conn, err := db.Conn(ctx)
			if err != nil {
				return err
			}
			defer conn.Close()
			rows, err := conn.QueryContext(ctx, `SELECT TABLE_NAME FROM information_schema.TABLES
				WHERE TABLE_SCHEMA = DATABASE() AND TABLE_TYPE = 'BASE TABLE'
				AND (TABLE_COLLATION IS NULL OR TABLE_COLLATION <> 'utf8mb4_bin')`)
			if err != nil {
				return err
			}
			var tables []string
			for rows.Next() {
				var name string
				if err := rows.Scan(&name); err != nil {
					rows.Close()
					return err
				}
				tables = append(tables, name)
			}
			rows.Close()
			if err := rows.Err(); err != nil {
				return err
			}
			if _, err := conn.ExecContext(ctx, `SET FOREIGN_KEY_CHECKS = 0`); err != nil {
				return err
			}
			defer conn.ExecContext(ctx, `SET FOREIGN_KEY_CHECKS = 1`)
			for _, table := range tables {
				if _, err := conn.ExecContext(ctx, fmt.Sprintf(
					"ALTER TABLE `%s` CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_bin", table)); err != nil {
					return fmt.Errorf("converting table %s: %w", table, err)
				}
			}
			// Tables created later get it too. Not allowed for every database
			// user; later migrations name the collation themselves then.
			_, _ = conn.ExecContext(ctx, `ALTER DATABASE CHARACTER SET utf8mb4 COLLATE utf8mb4_bin`)
			return nil
		},
	}
}

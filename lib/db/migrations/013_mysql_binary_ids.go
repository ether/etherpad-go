package migrations

import (
	"context"
	"database/sql"
	"fmt"
)

// mysqlForeignKey is one of the foreign keys of the MySQL schema
// (migrations 1 and 8).
type mysqlForeignKey struct {
	table, column, refTable, refColumn, onDelete string
}

var mysqlForeignKeys = []mysqlForeignKey{
	{"padRev", "id", "pad", "id", "CASCADE"},
	{"padRev", "authorId", "globalAuthor", "id", "SET NULL"},
	{"padChat", "padId", "pad", "id", "CASCADE"},
	{"padChat", "authorId", "globalAuthor", "id", "SET NULL"},
	{"sheet", "id", "pad", "id", "CASCADE"},
	{"sheet_op", "id", "pad", "id", "CASCADE"},
}

// migration013MySQLBinaryIDs makes MySQL compare ids as they are. MySQL's
// default collations ignore case, so the pads "Foo" and "foo" were one row:
// opening one gave the other's text, and creating the second changed the
// first. Every table is converted to utf8mb4 with the binary collation, as
// ids are on SQLite and Postgres. utf8mb4 holds every character of the
// older charsets, so no text changes. Large databases take a while: each
// table is rewritten once.
//
// Nothing becomes unreachable: a revision or chat message stored under
// another case of its pad's id ("foo" for the pad "Foo", which matched
// before) first gets the pad's exact id. MySQL refuses to change the
// collation of foreign key columns, so the foreign keys are dropped and
// added again; a run that stopped half way adds them on the next run.
func migration013MySQLBinaryIDs() Migration {
	return Migration{
		Version:     13,
		Description: "MySQL: compare ids case-sensitively (utf8mb4_bin)",
		Up: func(db *sql.DB, dialect Dialect) error {
			if dialect != DialectMySQL {
				return nil
			}
			ctx := context.Background()
			// FOREIGN_KEY_CHECKS is per connection.
			conn, err := db.Conn(ctx)
			if err != nil {
				return err
			}
			defer conn.Close()
			if _, err := conn.ExecContext(ctx, `SET FOREIGN_KEY_CHECKS = 0`); err != nil {
				return err
			}
			defer conn.ExecContext(ctx, `SET FOREIGN_KEY_CHECKS = 1`)

			tables, err := queryStrings(ctx, conn, `SELECT TABLE_NAME FROM information_schema.TABLES
				WHERE TABLE_SCHEMA = DATABASE() AND TABLE_TYPE = 'BASE TABLE'`)
			if err != nil {
				return err
			}
			exists := map[string]bool{}
			for _, t := range tables {
				exists[t] = true
			}

			// 1. References get the exact id of what they reference.
			for _, fk := range mysqlForeignKeys {
				if !exists[fk.table] || !exists[fk.refTable] {
					continue
				}
				if _, err := conn.ExecContext(ctx, fmt.Sprintf(
					"UPDATE `%s` c JOIN `%s` p ON c.`%s` = p.`%s` SET c.`%s` = p.`%s` WHERE BINARY c.`%s` <> BINARY p.`%s`",
					fk.table, fk.refTable, fk.column, fk.refColumn, fk.column, fk.refColumn, fk.column, fk.refColumn)); err != nil {
					return fmt.Errorf("aligning %s.%s with %s.%s: %w", fk.table, fk.column, fk.refTable, fk.refColumn, err)
				}
			}

			// 2. Drop the foreign keys.
			rows, err := conn.QueryContext(ctx, `SELECT TABLE_NAME, CONSTRAINT_NAME FROM information_schema.TABLE_CONSTRAINTS
				WHERE TABLE_SCHEMA = DATABASE() AND CONSTRAINT_TYPE = 'FOREIGN KEY'`)
			if err != nil {
				return err
			}
			var drops [][2]string
			for rows.Next() {
				var table, name string
				if err := rows.Scan(&table, &name); err != nil {
					rows.Close()
					return err
				}
				drops = append(drops, [2]string{table, name})
			}
			rows.Close()
			if err := rows.Err(); err != nil {
				return err
			}
			for _, d := range drops {
				if _, err := conn.ExecContext(ctx, fmt.Sprintf("ALTER TABLE `%s` DROP FOREIGN KEY `%s`", d[0], d[1])); err != nil {
					return fmt.Errorf("dropping foreign key %s of %s: %w", d[1], d[0], err)
				}
			}

			// 3. Convert the tables.
			pending, err := queryStrings(ctx, conn, `SELECT TABLE_NAME FROM information_schema.TABLES
				WHERE TABLE_SCHEMA = DATABASE() AND TABLE_TYPE = 'BASE TABLE'
				AND (TABLE_COLLATION IS NULL OR TABLE_COLLATION <> 'utf8mb4_bin')`)
			if err != nil {
				return err
			}
			for _, table := range pending {
				if _, err := conn.ExecContext(ctx, fmt.Sprintf(
					"ALTER TABLE `%s` CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_bin", table)); err != nil {
					return fmt.Errorf("converting table %s: %w", table, err)
				}
			}

			// 4. Add the foreign keys again. Existing rows are not checked,
			// so none is lost.
			for _, fk := range mysqlForeignKeys {
				if !exists[fk.table] || !exists[fk.refTable] {
					continue
				}
				if _, err := conn.ExecContext(ctx, fmt.Sprintf(
					"ALTER TABLE `%s` ADD CONSTRAINT `fk_%s_%s` FOREIGN KEY (`%s`) REFERENCES `%s`(`%s`) ON DELETE %s",
					fk.table, fk.table, fk.column, fk.column, fk.refTable, fk.refColumn, fk.onDelete)); err != nil {
					return fmt.Errorf("adding foreign key %s.%s: %w", fk.table, fk.column, err)
				}
			}

			// Tables created later get it too. Not allowed for every database
			// user; later migrations name the collation themselves then.
			_, _ = conn.ExecContext(ctx, `ALTER DATABASE CHARACTER SET utf8mb4 COLLATE utf8mb4_bin`)
			return nil
		},
	}
}

func queryStrings(ctx context.Context, conn *sql.Conn, query string) ([]string, error) {
	rows, err := conn.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

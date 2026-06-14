package migrations

import "database/sql"

// migration005LargeTextColumns widens the MySQL pad-content columns from TEXT
// (max 65,535 bytes) to LONGTEXT (max 4 GiB) so large pads, revisions, and
// imports are not truncated/rejected. On SQLite and Postgres, TEXT has no
// practical size limit, so this migration is a no-op there.
func migration005LargeTextColumns() Migration {
	return Migration{
		Version:     5,
		Description: "Widen MySQL pad content columns from TEXT to LONGTEXT",
		Up: func(db *sql.DB, dialect Dialect) error {
			if dialect != DialectMySQL {
				// TEXT is effectively unbounded on SQLite and Postgres.
				return nil
			}
			// MODIFY preserves each column's existing nullability; padChat.chatText
			// is NOT NULL and must stay NOT NULL.
			alters := []string{
				"ALTER TABLE pad MODIFY atext_text LONGTEXT",
				"ALTER TABLE pad MODIFY atext_attribs LONGTEXT",
				"ALTER TABLE padRev MODIFY changeset LONGTEXT",
				"ALTER TABLE padRev MODIFY atextText LONGTEXT",
				"ALTER TABLE padRev MODIFY atextAttribs LONGTEXT",
				"ALTER TABLE padChat MODIFY chatText LONGTEXT NOT NULL",
			}
			for _, q := range alters {
				if _, err := db.Exec(q); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

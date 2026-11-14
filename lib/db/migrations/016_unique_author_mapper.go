package migrations

import (
	"database/sql"
	"strings"
)

// migration016UniqueAuthorMapper makes globalAuthor.mapper unique, so two
// concurrent createAuthorIfNotExistsFor calls for one external id cannot
// both create an author for it: the second one's assignment fails, and it
// takes the author of the first. Several authors without a mapper (NULL)
// stay allowed in every database. Duplicates from before keep the mapper
// on the oldest author id.
func migration016UniqueAuthorMapper() Migration {
	return Migration{
		Version:     16,
		Description: "Unique author mapper",
		Up: func(db *sql.DB, dialect Dialect) error {
			var stmts []string
			switch dialect {
			case DialectMySQL:
				stmts = []string{
					`UPDATE globalAuthor g JOIN (
						SELECT mapper, MIN(id) AS keep_id FROM globalAuthor
						WHERE mapper IS NOT NULL GROUP BY mapper HAVING COUNT(*) > 1
					) d ON g.mapper = d.mapper SET g.mapper = NULL WHERE g.id <> d.keep_id`,
					`CREATE UNIQUE INDEX uq_globalAuthor_mapper ON globalAuthor (mapper)`,
					`DROP INDEX idx_globalAuthor_mapper ON globalAuthor`,
				}
			case DialectPostgres:
				stmts = []string{
					`UPDATE globalauthor g SET mapper = NULL WHERE mapper IS NOT NULL
						AND id <> (SELECT MIN(h.id) FROM globalauthor h WHERE h.mapper = g.mapper)`,
					`CREATE UNIQUE INDEX IF NOT EXISTS uq_globalauthor_mapper ON globalauthor (mapper)`,
					`DROP INDEX IF EXISTS idx_globalauthor_mapper`,
				}
			default:
				stmts = []string{
					`UPDATE globalAuthor SET mapper = NULL WHERE mapper IS NOT NULL
						AND id <> (SELECT MIN(h.id) FROM globalAuthor h WHERE h.mapper = globalAuthor.mapper)`,
					`CREATE UNIQUE INDEX IF NOT EXISTS uq_globalAuthor_mapper ON globalAuthor (mapper)`,
					`DROP INDEX IF EXISTS idx_globalAuthor_mapper`,
				}
			}
			// MySQL commits each DDL statement: a run that stopped
			// half-way finds the unique index there, or the old one gone.
			for _, q := range stmts {
				if _, err := db.Exec(q); err != nil && !isDuplicateIndex(err) && !isMissingIndex(err) {
					return err
				}
			}
			return nil
		},
	}
}

func isMissingIndex(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "can't drop") || strings.Contains(msg, "check that column/key exists")
}

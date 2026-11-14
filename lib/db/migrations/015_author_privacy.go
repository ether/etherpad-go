package migrations

import (
	"database/sql"
	"strings"
)

// migration015AuthorPrivacy adds what erasing an author (GDPR Art. 17) and
// the admin's author list need, and the table for moving an author's
// identity to another browser (etherpad-lite's tokenTransfer):
//   - globalAuthor.mapper: the external id of createAuthorIfNotExistsFor
//     (an LMS user id). It was stored as the author's token; authors made
//     that way keep working (the token is still looked up), new ones get
//     the mapper here, so a browser cannot present it as its token.
//   - last_seen (ms), erased, erased_at (ms).
//   - token_transfers: one-time, short-lived transfer codes.
func migration015AuthorPrivacy() Migration {
	return Migration{
		Version:     15,
		Description: "Author mapper, last seen and erasure; token transfers",
		Up: func(db *sql.DB, dialect Dialect) error {
			var stmts []string
			switch dialect {
			case DialectMySQL:
				stmts = []string{
					`ALTER TABLE globalAuthor ADD COLUMN mapper VARCHAR(255) NULL`,
					`ALTER TABLE globalAuthor ADD COLUMN last_seen BIGINT NULL`,
					`ALTER TABLE globalAuthor ADD COLUMN erased BOOLEAN NOT NULL DEFAULT FALSE`,
					`ALTER TABLE globalAuthor ADD COLUMN erased_at BIGINT NULL`,
					`CREATE INDEX idx_globalAuthor_mapper ON globalAuthor (mapper)`,
					`CREATE TABLE IF NOT EXISTS token_transfers (
						id VARCHAR(64) PRIMARY KEY,
						token VARCHAR(255) NOT NULL,
						prefs TEXT NOT NULL,
						created_at BIGINT NOT NULL
					) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin`,
				}
			case DialectPostgres:
				stmts = []string{
					`ALTER TABLE globalauthor ADD COLUMN IF NOT EXISTS mapper TEXT NULL`,
					`ALTER TABLE globalauthor ADD COLUMN IF NOT EXISTS last_seen BIGINT NULL`,
					`ALTER TABLE globalauthor ADD COLUMN IF NOT EXISTS erased BOOLEAN NOT NULL DEFAULT FALSE`,
					`ALTER TABLE globalauthor ADD COLUMN IF NOT EXISTS erased_at BIGINT NULL`,
					`CREATE INDEX IF NOT EXISTS idx_globalauthor_mapper ON globalauthor (mapper)`,
					`CREATE TABLE IF NOT EXISTS token_transfers (
						id TEXT PRIMARY KEY,
						token TEXT NOT NULL,
						prefs TEXT NOT NULL,
						created_at BIGINT NOT NULL
					)`,
				}
			default:
				stmts = []string{
					`ALTER TABLE globalAuthor ADD COLUMN mapper TEXT NULL`,
					`ALTER TABLE globalAuthor ADD COLUMN last_seen BIGINT NULL`,
					`ALTER TABLE globalAuthor ADD COLUMN erased BOOLEAN NOT NULL DEFAULT FALSE`,
					`ALTER TABLE globalAuthor ADD COLUMN erased_at BIGINT NULL`,
					`CREATE INDEX IF NOT EXISTS idx_globalAuthor_mapper ON globalAuthor (mapper)`,
					`CREATE TABLE IF NOT EXISTS token_transfers (
						id TEXT PRIMARY KEY,
						token TEXT NOT NULL,
						prefs TEXT NOT NULL,
						created_at BIGINT NOT NULL
					)`,
				}
			}
			// Each statement may already have run if an earlier attempt
			// stopped half-way (MySQL commits every DDL statement).
			for _, q := range stmts {
				if _, err := db.Exec(q); ignoreDuplicateColumn(err) != nil && !isDuplicateIndex(err) {
					return err
				}
			}
			return nil
		},
	}
}

func isDuplicateIndex(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "duplicate key name")
}

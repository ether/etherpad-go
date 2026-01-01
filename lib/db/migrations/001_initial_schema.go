package migrations

import (
	"database/sql"
)

// GetMigrations returns all available migrations
func GetMigrations() []Migration {
	return []Migration{
		migration001InitialSchema(),
	}
}

// migration001InitialSchema creates the initial database schema
func migration001InitialSchema() Migration {
	return Migration{
		Version:     1,
		Description: "Initial schema - create all tables",
		Up: func(db *sql.DB, dialect Dialect) error {
			var queries []string

			switch dialect {
			case DialectMySQL:
				queries = getMySQLInitialSchema()
			case DialectPostgres:
				queries = getPostgresInitialSchema()
			default:
				queries = getSQLiteInitialSchema()
			}

			for _, query := range queries {
				if _, err := db.Exec(query); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func getSQLiteInitialSchema() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS pad (id TEXT PRIMARY KEY, data TEXT)`,
		`CREATE TABLE IF NOT EXISTS globalAuthor(id TEXT PRIMARY KEY, colorId TEXT, name TEXT, timestamp BIGINT)`,
		`CREATE TABLE IF NOT EXISTS globalAuthorPads(id TEXT NOT NULL, padID TEXT NOT NULL, PRIMARY KEY(id, padID), FOREIGN KEY(id) REFERENCES globalAuthor(id) ON DELETE CASCADE, FOREIGN KEY(padID) REFERENCES pad(id) ON DELETE CASCADE)`,
		`CREATE TABLE IF NOT EXISTS padRev(id TEXT, rev INTEGER, changeset TEXT, atextText TEXT, atextAttribs TEXT, authorId TEXT, timestamp INTEGER, pool TEXT, PRIMARY KEY (id, rev), FOREIGN KEY(id) REFERENCES pad(id) ON DELETE CASCADE, FOREIGN KEY(authorId) REFERENCES globalAuthor(id) ON DELETE SET NULL)`,
		`CREATE TABLE IF NOT EXISTS token2author(token TEXT PRIMARY KEY, author TEXT, FOREIGN KEY(author) REFERENCES globalAuthor(id) ON DELETE CASCADE)`,
		`CREATE TABLE IF NOT EXISTS pad2readonly(id TEXT PRIMARY KEY, data TEXT, FOREIGN KEY(id) REFERENCES pad(id) ON DELETE CASCADE)`,
		`CREATE TABLE IF NOT EXISTS readonly2pad(id TEXT PRIMARY KEY, data TEXT, FOREIGN KEY(data) REFERENCES pad(id) ON DELETE CASCADE)`,
		`CREATE TABLE IF NOT EXISTS sessionstorage(id TEXT PRIMARY KEY, originalMaxAge INTEGER, expires TEXT, secure BOOLEAN, httpOnly BOOLEAN, path TEXT, sameSite TEXT, connections TEXT)`,
		`CREATE TABLE IF NOT EXISTS groupPadGroup(id TEXT PRIMARY KEY, name TEXT)`,
		`CREATE TABLE IF NOT EXISTS padChat(padId TEXT NOT NULL, padHead INTEGER, chatText TEXT NOT NULL, authorId TEXT, timestamp BIGINT, PRIMARY KEY(padId, padHead), FOREIGN KEY(padId) REFERENCES pad(id) ON DELETE CASCADE)`,
	}
}

func getPostgresInitialSchema() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS pad (id TEXT PRIMARY KEY, data TEXT)`,
		`CREATE TABLE IF NOT EXISTS globalAuthor(id TEXT PRIMARY KEY, colorId TEXT, name TEXT, timestamp BIGINT)`,
		`CREATE TABLE IF NOT EXISTS padRev(id TEXT, rev INTEGER, changeset TEXT, atextText TEXT, atextAttribs TEXT, authorId TEXT, timestamp BIGINT, pool TEXT, PRIMARY KEY (id, rev), FOREIGN KEY(id) REFERENCES pad(id) ON DELETE CASCADE)`,
		`CREATE TABLE IF NOT EXISTS token2author(token TEXT PRIMARY KEY, author TEXT, FOREIGN KEY(author) REFERENCES globalAuthor(id) ON DELETE CASCADE)`,
		`CREATE TABLE IF NOT EXISTS globalAuthorPads(id TEXT NOT NULL, padID TEXT NOT NULL, PRIMARY KEY(id, padID), FOREIGN KEY(id) REFERENCES globalAuthor(id) ON DELETE CASCADE, FOREIGN KEY(padID) REFERENCES pad(id) ON DELETE CASCADE)`,
		`CREATE TABLE IF NOT EXISTS pad2readonly(id TEXT PRIMARY KEY, data TEXT, FOREIGN KEY(id) REFERENCES pad(id) ON DELETE CASCADE)`,
		`CREATE TABLE IF NOT EXISTS readonly2pad(id TEXT PRIMARY KEY, data TEXT, FOREIGN KEY(data) REFERENCES pad(id) ON DELETE CASCADE)`,
		`CREATE TABLE IF NOT EXISTS sessionstorage(id TEXT PRIMARY KEY, originalMaxAge INTEGER, expires TEXT, secure BOOLEAN, httpOnly BOOLEAN, path TEXT, sameSite TEXT, connections TEXT)`,
		`CREATE TABLE IF NOT EXISTS groupPadGroup(id TEXT PRIMARY KEY, name TEXT)`,
		`CREATE TABLE IF NOT EXISTS padChat(padId TEXT NOT NULL, padHead INTEGER, chatText TEXT NOT NULL, authorId TEXT, timestamp BIGINT, PRIMARY KEY(padId, padHead), FOREIGN KEY(padId) REFERENCES pad(id) ON DELETE CASCADE)`,
	}
}

func getMySQLInitialSchema() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS pad (id VARCHAR(255) PRIMARY KEY, data TEXT)`,
		`CREATE TABLE IF NOT EXISTS globalAuthor(id VARCHAR(255) PRIMARY KEY, colorId VARCHAR(50), name VARCHAR(255), timestamp BIGINT)`,
		`CREATE TABLE IF NOT EXISTS padRev(id VARCHAR(255), rev INTEGER, changeset TEXT, atextText TEXT, atextAttribs TEXT, authorId VARCHAR(255), timestamp BIGINT, pool TEXT NOT NULL, PRIMARY KEY (id, rev), FOREIGN KEY(id) REFERENCES pad(id) ON DELETE CASCADE)`,
		`CREATE TABLE IF NOT EXISTS token2author(token VARCHAR(255) PRIMARY KEY, author VARCHAR(255), FOREIGN KEY(author) REFERENCES globalAuthor(id) ON DELETE CASCADE)`,
		`CREATE TABLE IF NOT EXISTS globalAuthorPads(id VARCHAR(255) NOT NULL, padID VARCHAR(255) NOT NULL, PRIMARY KEY(id, padID), FOREIGN KEY(id) REFERENCES globalAuthor(id) ON DELETE CASCADE, FOREIGN KEY(padID) REFERENCES pad(id) ON DELETE CASCADE)`,
		`CREATE TABLE IF NOT EXISTS pad2readonly(id VARCHAR(255) PRIMARY KEY, data VARCHAR(255), FOREIGN KEY(id) REFERENCES pad(id) ON DELETE CASCADE)`,
		`CREATE TABLE IF NOT EXISTS readonly2pad(id VARCHAR(255) PRIMARY KEY, data VARCHAR(255), FOREIGN KEY(data) REFERENCES pad(id) ON DELETE CASCADE)`,
		`CREATE TABLE IF NOT EXISTS sessionstorage(id VARCHAR(255) PRIMARY KEY, originalMaxAge INTEGER, expires VARCHAR(255), secure BOOLEAN, httpOnly BOOLEAN, path VARCHAR(255), sameSite VARCHAR(50), connections TEXT)`,
		`CREATE TABLE IF NOT EXISTS groupPadGroup(id VARCHAR(255) PRIMARY KEY, name VARCHAR(255))`,
		`CREATE TABLE IF NOT EXISTS padChat(padId VARCHAR(255) NOT NULL, padHead INTEGER, chatText TEXT NOT NULL, authorId VARCHAR(255), timestamp BIGINT, PRIMARY KEY(padId, padHead), FOREIGN KEY(padId) REFERENCES pad(id) ON DELETE CASCADE)`,
	}
}

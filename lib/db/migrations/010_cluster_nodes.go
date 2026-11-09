package migrations

import "database/sql"

// migration010ClusterNodes adds the registry through which Etherpad
// instances on one database find each other (like Keycloak's jdbc-ping).
func migration010ClusterNodes() Migration {
	return Migration{
		Version:     10,
		Description: "Create cluster_nodes table for instance discovery",
		Up: func(db *sql.DB, dialect Dialect) error {
			query := `CREATE TABLE IF NOT EXISTS cluster_nodes (
				id TEXT PRIMARY KEY,
				address TEXT NOT NULL,
				last_seen BIGINT NOT NULL
			)`
			if dialect == DialectMySQL {
				query = `CREATE TABLE IF NOT EXISTS cluster_nodes (
					id VARCHAR(255) PRIMARY KEY,
					address VARCHAR(255) NOT NULL,
					last_seen BIGINT NOT NULL
				)`
			}
			_, err := db.Exec(query)
			return err
		},
	}
}

package migrations

import "database/sql"

// migration017ClusterNodeUsers stores each instance's connected users with
// its heartbeat, so the admin statistics can count the whole cluster.
func migration017ClusterNodeUsers() Migration {
	return Migration{
		Version:     17,
		Description: "Connected users per cluster node",
		Up: func(db *sql.DB, dialect Dialect) error {
			_, err := db.Exec(`ALTER TABLE cluster_nodes ADD COLUMN users INTEGER NOT NULL DEFAULT 0`)
			return ignoreDuplicateColumn(err)
		},
	}
}

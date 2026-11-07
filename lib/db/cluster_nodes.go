package db

import (
	"context"
	"database/sql"
	"time"
)

// ClusterNode is an Etherpad instance registered for discovery.
type ClusterNode struct {
	ID       string
	Address  string
	LastSeen time.Time
}

// ClusterMethods are the instance registry: every instance records its
// gossip address and refreshes it periodically; the others join it.
type ClusterMethods interface {
	HeartbeatClusterNode(id, address string, now time.Time) error
	ListClusterNodes(aliveSince time.Time) ([]ClusterNode, error)
	RemoveClusterNode(id string) error
	// PruneClusterNodes removes instances not seen since before.
	PruneClusterNodes(before time.Time) error
}

// --- Postgres ---

func (d PostgresDB) HeartbeatClusterNode(id, address string, now time.Time) error {
	_, err := d.pool.Exec(context.Background(),
		`INSERT INTO cluster_nodes (id, address, last_seen) VALUES ($1, $2, $3)
		 ON CONFLICT (id) DO UPDATE SET address = EXCLUDED.address, last_seen = EXCLUDED.last_seen`,
		id, address, now.UnixMilli())
	return err
}

func (d PostgresDB) ListClusterNodes(aliveSince time.Time) ([]ClusterNode, error) {
	rows, err := d.pool.Query(context.Background(),
		`SELECT id, address, last_seen FROM cluster_nodes WHERE last_seen >= $1`, aliveSince.UnixMilli())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ClusterNode
	for rows.Next() {
		var n ClusterNode
		var seen int64
		if err := rows.Scan(&n.ID, &n.Address, &seen); err != nil {
			return nil, err
		}
		n.LastSeen = time.UnixMilli(seen)
		out = append(out, n)
	}
	return out, rows.Err()
}

func (d PostgresDB) RemoveClusterNode(id string) error {
	_, err := d.pool.Exec(context.Background(), `DELETE FROM cluster_nodes WHERE id = $1`, id)
	return err
}

func (d PostgresDB) PruneClusterNodes(before time.Time) error {
	_, err := d.pool.Exec(context.Background(), `DELETE FROM cluster_nodes WHERE last_seen < $1`, before.UnixMilli())
	return err
}

// --- MySQL ---

func (d MysqlDB) HeartbeatClusterNode(id, address string, now time.Time) error {
	_, err := d.sqlDB.Exec(
		`INSERT INTO cluster_nodes (id, address, last_seen) VALUES (?, ?, ?)
		 ON DUPLICATE KEY UPDATE address = VALUES(address), last_seen = VALUES(last_seen)`,
		id, address, now.UnixMilli())
	return err
}

func (d MysqlDB) ListClusterNodes(aliveSince time.Time) ([]ClusterNode, error) {
	return listClusterNodesSQL(d.sqlDB.Query, aliveSince)
}

func (d MysqlDB) RemoveClusterNode(id string) error {
	_, err := d.sqlDB.Exec(`DELETE FROM cluster_nodes WHERE id = ?`, id)
	return err
}

func (d MysqlDB) PruneClusterNodes(before time.Time) error {
	_, err := d.sqlDB.Exec(`DELETE FROM cluster_nodes WHERE last_seen < ?`, before.UnixMilli())
	return err
}

// --- SQLite ---

func (d SQLiteDB) HeartbeatClusterNode(id, address string, now time.Time) error {
	_, err := d.sqlDB.Exec(
		`INSERT INTO cluster_nodes (id, address, last_seen) VALUES (?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET address = excluded.address, last_seen = excluded.last_seen`,
		id, address, now.UnixMilli())
	return err
}

func (d SQLiteDB) ListClusterNodes(aliveSince time.Time) ([]ClusterNode, error) {
	return listClusterNodesSQL(d.sqlDB.Query, aliveSince)
}

func (d SQLiteDB) RemoveClusterNode(id string) error {
	_, err := d.sqlDB.Exec(`DELETE FROM cluster_nodes WHERE id = ?`, id)
	return err
}

func (d SQLiteDB) PruneClusterNodes(before time.Time) error {
	_, err := d.sqlDB.Exec(`DELETE FROM cluster_nodes WHERE last_seen < ?`, before.UnixMilli())
	return err
}

func listClusterNodesSQL(query func(string, ...any) (*sql.Rows, error), aliveSince time.Time) ([]ClusterNode, error) {
	rows, err := query(`SELECT id, address, last_seen FROM cluster_nodes WHERE last_seen >= ?`, aliveSince.UnixMilli())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ClusterNode
	for rows.Next() {
		var n ClusterNode
		var seen int64
		if err := rows.Scan(&n.ID, &n.Address, &seen); err != nil {
			return nil, err
		}
		n.LastSeen = time.UnixMilli(seen)
		out = append(out, n)
	}
	return out, rows.Err()
}

// --- Memory ---

func (m *MemoryDataStore) HeartbeatClusterNode(id, address string, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clusterNodes[id] = ClusterNode{ID: id, Address: address, LastSeen: now}
	return nil
}

func (m *MemoryDataStore) ListClusterNodes(aliveSince time.Time) ([]ClusterNode, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []ClusterNode
	for _, n := range m.clusterNodes {
		if !n.LastSeen.Before(aliveSince) {
			out = append(out, n)
		}
	}
	return out, nil
}

func (m *MemoryDataStore) RemoveClusterNode(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.clusterNodes, id)
	return nil
}

func (m *MemoryDataStore) PruneClusterNodes(before time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, n := range m.clusterNodes {
		if n.LastSeen.Before(before) {
			delete(m.clusterNodes, id)
		}
	}
	return nil
}

var (
	_ ClusterMethods = PostgresDB{}
	_ ClusterMethods = MysqlDB{}
	_ ClusterMethods = SQLiteDB{}
	_ ClusterMethods = (*MemoryDataStore)(nil)
)

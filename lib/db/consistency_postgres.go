package db

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ether/etherpad-go/lib/models/db"
)

func (d PostgresDB) GetPadVersion(padID string) (*PadVersion, error) {
	v := &PadVersion{}
	var saved []byte
	var public *bool
	var readOnly, settings *string
	err := d.pool.QueryRow(context.Background(),
		`SELECT EXISTS(SELECT 1 FROM pad WHERE id = $1),
		        COALESCE((SELECT MAX(rev) FROM "padrev" WHERE id = $1), -1),
		        COALESCE((SELECT MAX("padhead") FROM "padchat" WHERE "padid" = $1), -1),
		        (SELECT saved_revisions FROM pad WHERE id = $1),
		        (SELECT public_status FROM pad WHERE id = $1),
		        (SELECT readonly_id FROM pad WHERE id = $1),
		        (SELECT settings FROM pad WHERE id = $1)`,
		padID).Scan(&v.Exists, &v.Rev, &v.ChatHead, &saved, &public, &readOnly, &settings)
	if err != nil {
		return nil, err
	}
	if v.Meta, err = scanPadMeta(v.Exists, saved, public, readOnly, settings); err != nil {
		return nil, err
	}
	return v, nil
}

func (d PostgresDB) GetRevisionDeltas(padID string, fromRev, toRev int) ([]RevisionDelta, error) {
	rows, err := d.pool.Query(context.Background(),
		`SELECT rev, changeset, "authorid", timestamp FROM "padrev"
		 WHERE id = $1 AND rev >= $2 AND rev <= $3 ORDER BY rev ASC`,
		padID, fromRev, toRev)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var deltas []RevisionDelta
	for rows.Next() {
		var r RevisionDelta
		// The schema allows revisions without a timestamp.
		var ts *int64
		if err := rows.Scan(&r.Rev, &r.Changeset, &r.AuthorId, &ts); err != nil {
			return nil, err
		}
		if ts != nil {
			r.Timestamp = *ts
		}
		deltas = append(deltas, r)
	}
	return deltas, rows.Err()
}

func (d PostgresDB) GetRevisionPool(padID string, rev int) (*db.RevPool, error) {
	var rawPool string
	if err := d.pool.QueryRow(context.Background(), `SELECT pool FROM "padrev" WHERE id = $1 AND rev = $2`, padID, rev).Scan(&rawPool); err != nil {
		return nil, fmt.Errorf("pool of revision %d: %w", rev, err)
	}
	var pool db.RevPool
	if err := json.Unmarshal([]byte(rawPool), &pool); err != nil {
		return nil, err
	}
	return &pool, nil
}

// lockTimeout bounds waiting for a database lock, so a stuck holder shows
// up as an error instead of hanging requests forever.
const lockTimeout = 30 * time.Second

func (d PostgresDB) LockPadsExclusive(padIDs []string) (func(), error) {
	return d.lockSlots.lockAll(padLockNames(padIDs), d.lockOne)
}

// lockOne takes a session-level advisory lock on a connection of the lock
// pool: if the process dies, the connection closes and Postgres releases
// the lock.
func (d PostgresDB) lockOne(ctx context.Context, name string) (func(), error) {
	conn, err := d.lockPool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	key := advisoryLockKey(name)
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, key); err != nil {
		// A cancelled wait may leave the connection in an unknown state.
		_ = conn.Conn().Close(context.Background())
		conn.Release()
		return nil, fmt.Errorf("pg_advisory_lock: %w", err)
	}
	return func() {
		if _, err := conn.Exec(context.Background(), `SELECT pg_advisory_unlock($1)`, key); err != nil {
			// The lock is still held by this connection; dropping it
			// releases the lock.
			_ = conn.Conn().Close(context.Background())
		}
		conn.Release()
	}, nil
}

var _ ConsistencyMethods = PostgresDB{}

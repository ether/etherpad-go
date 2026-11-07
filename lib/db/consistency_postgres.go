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
	err := d.pool.QueryRow(context.Background(),
		`SELECT EXISTS(SELECT 1 FROM pad WHERE id = $1),
		        COALESCE((SELECT MAX(rev) FROM "padrev" WHERE id = $1), -1),
		        COALESCE((SELECT MAX("padhead") FROM "padchat" WHERE "padid" = $1), -1)`,
		padID).Scan(&v.Exists, &v.Rev, &v.ChatHead)
	if err != nil {
		return nil, err
	}
	return v, nil
}

func (d PostgresDB) GetRevisionDeltas(padID string, fromRev, toRev int) ([]RevisionDelta, *db.RevPool, error) {
	ctx := context.Background()
	rows, err := d.pool.Query(ctx,
		`SELECT rev, changeset, "authorid", timestamp FROM "padrev"
		 WHERE id = $1 AND rev >= $2 AND rev <= $3 ORDER BY rev ASC`,
		padID, fromRev, toRev)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var deltas []RevisionDelta
	for rows.Next() {
		var r RevisionDelta
		if err := rows.Scan(&r.Rev, &r.Changeset, &r.AuthorId, &r.Timestamp); err != nil {
			return nil, nil, err
		}
		deltas = append(deltas, r)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	var rawPool string
	if err := d.pool.QueryRow(ctx, `SELECT pool FROM "padrev" WHERE id = $1 AND rev = $2`, padID, toRev).Scan(&rawPool); err != nil {
		return nil, nil, fmt.Errorf("pool of revision %d: %w", toRev, err)
	}
	var pool db.RevPool
	if err := json.Unmarshal([]byte(rawPool), &pool); err != nil {
		return nil, nil, err
	}
	return deltas, &pool, nil
}

// lockTimeout bounds waiting for a database lock, so a stuck holder shows
// up as an error instead of hanging requests forever.
const lockTimeout = 30 * time.Second

func (d PostgresDB) LockPadExclusive(padID string) (func(), error) {
	return d.lockExclusive(padLockName(padID))
}

func (d PostgresDB) lockExclusive(name string) (func(), error) {
	ctx, cancel := context.WithTimeout(context.Background(), lockTimeout)
	defer cancel()
	select {
	case d.lockSlots <- struct{}{}:
	case <-ctx.Done():
		return nil, fmt.Errorf("waiting for a database lock slot: %w", ctx.Err())
	}
	conn, err := d.pool.Acquire(ctx)
	if err != nil {
		<-d.lockSlots
		return nil, err
	}
	key := advisoryLockKey(name)
	// Session-level lock on this connection: if the process dies, the
	// connection closes and Postgres releases the lock.
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, key); err != nil {
		conn.Release()
		<-d.lockSlots
		return nil, fmt.Errorf("pg_advisory_lock: %w", err)
	}
	return func() {
		if _, err := conn.Exec(context.Background(), `SELECT pg_advisory_unlock($1)`, key); err != nil {
			// The lock is still held by this connection; dropping it
			// releases the lock.
			_ = conn.Conn().Close(context.Background())
		}
		conn.Release()
		<-d.lockSlots
	}, nil
}

var _ ConsistencyMethods = PostgresDB{}

package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ether/etherpad-go/lib/models/db"
	mysql2 "github.com/go-sql-driver/mysql"
)

// isDuplicateKey reports whether err is MySQL's ER_DUP_ENTRY.
func isDuplicateKey(err error) bool {
	var me *mysql2.MySQLError
	return errors.As(err, &me) && me.Number == 1062
}

func (d MysqlDB) GetPadVersion(padID string) (*PadVersion, error) {
	v := &PadVersion{}
	err := d.sqlDB.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM pad WHERE id = ?),
		        COALESCE((SELECT MAX(rev) FROM padRev WHERE id = ?), -1),
		        COALESCE((SELECT MAX(padHead) FROM padChat WHERE padId = ?), -1)`,
		padID, padID, padID).Scan(&v.Exists, &v.Rev, &v.ChatHead)
	if err != nil {
		return nil, err
	}
	return v, nil
}

func (d MysqlDB) GetRevisionDeltas(padID string, fromRev, toRev int) ([]RevisionDelta, *db.RevPool, error) {
	rows, err := d.sqlDB.Query(
		`SELECT rev, changeset, authorId, timestamp FROM padRev
		 WHERE id = ? AND rev >= ? AND rev <= ? ORDER BY rev ASC`,
		padID, fromRev, toRev)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var deltas []RevisionDelta
	for rows.Next() {
		var r RevisionDelta
		var ts sql.NullInt64
		if err := rows.Scan(&r.Rev, &r.Changeset, &r.AuthorId, &ts); err != nil {
			return nil, nil, err
		}
		r.Timestamp = ts.Int64
		deltas = append(deltas, r)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	var rawPool string
	if err := d.sqlDB.QueryRow(`SELECT pool FROM padRev WHERE id = ? AND rev = ?`, padID, toRev).Scan(&rawPool); err != nil {
		return nil, nil, fmt.Errorf("pool of revision %d: %w", toRev, err)
	}
	var pool db.RevPool
	if err := json.Unmarshal([]byte(rawPool), &pool); err != nil {
		return nil, nil, err
	}
	return deltas, &pool, nil
}

func (d MysqlDB) LockPadExclusive(padID string) (func(), error) {
	return d.lockExclusive(padLockName(padID))
}

func (d MysqlDB) lockExclusive(name string) (func(), error) {
	ctx, cancel := context.WithTimeout(context.Background(), lockTimeout)
	defer cancel()
	select {
	case d.lockSlots <- struct{}{}:
	case <-ctx.Done():
		return nil, fmt.Errorf("waiting for a database lock slot: %w", ctx.Err())
	}
	conn, err := d.sqlDB.Conn(ctx)
	if err != nil {
		<-d.lockSlots
		return nil, err
	}
	lockName := mysqlLockName(name)
	// GET_LOCK belongs to this connection and is released when it closes,
	// also when the process dies.
	var got sql.NullInt64
	if err := conn.QueryRowContext(ctx, `SELECT GET_LOCK(?, ?)`, lockName, int(lockTimeout.Seconds())).Scan(&got); err != nil || got.Int64 != 1 {
		conn.Close()
		<-d.lockSlots
		if err == nil {
			err = errors.New("timeout")
		}
		return nil, fmt.Errorf("GET_LOCK: %w", err)
	}
	return func() {
		if _, err := conn.ExecContext(context.Background(), `SELECT RELEASE_LOCK(?)`, lockName); err != nil {
			// Closing the connection releases the lock.
			_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		}
		conn.Close()
		<-d.lockSlots
	}, nil
}

var _ ConsistencyMethods = MysqlDB{}

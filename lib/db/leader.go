package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Leadership is a held leader lock. Release gives it up; Alive reports an
// error once the lock may have been lost (for example because the database
// connection holding it broke), after which the holder must stop acting as
// leader.
type Leadership interface {
	Alive() error
	Release()
}

// LeaderMethods elect one instance among those sharing a database to run a
// background job. The lock is tied to a database connection, so when an
// instance dies the database releases it and another instance takes over.
type LeaderMethods interface {
	// TryLeadership takes the named leader lock if it is free. It never
	// waits: ok is false when another instance holds it.
	TryLeadership(name string) (lease Leadership, ok bool, err error)
}

const leaderQueryTimeout = 5 * time.Second

func leaderLockName(name string) string { return "etherpad:leader:" + name }

// leaderSet is the in-process leader lock of the Memory and SQLite stores,
// which only one process uses.
type leaderSet struct {
	mu   sync.Mutex
	held map[string]bool
}

func newLeaderSet() *leaderSet { return &leaderSet{held: map[string]bool{}} }

type localLeadership struct {
	set  *leaderSet
	name string
	once sync.Once
}

func (l *localLeadership) Alive() error { return nil }

func (l *localLeadership) Release() {
	l.once.Do(func() {
		l.set.mu.Lock()
		delete(l.set.held, l.name)
		l.set.mu.Unlock()
	})
}

func (s *leaderSet) try(name string) (Leadership, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.held[name] {
		return nil, false, nil
	}
	s.held[name] = true
	return &localLeadership{set: s, name: name}, true, nil
}

func (m *MemoryDataStore) TryLeadership(name string) (Leadership, bool, error) {
	return m.leaders.try(name)
}

func (d SQLiteDB) TryLeadership(name string) (Leadership, bool, error) {
	return d.leaders.try(name)
}

// Postgres: a session-level advisory lock on a dedicated connection.

type pgLeadership struct {
	release func()
	alive   func() error
	once    sync.Once
}

func (l *pgLeadership) Alive() error { return l.alive() }
func (l *pgLeadership) Release()     { l.once.Do(l.release) }

func (d PostgresDB) TryLeadership(name string) (Leadership, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), leaderQueryTimeout)
	defer cancel()
	conn, err := d.lockPool.Acquire(ctx)
	if err != nil {
		return nil, false, err
	}
	key := advisoryLockKey(leaderLockName(name))
	var got bool
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock($1)`, key).Scan(&got); err != nil {
		conn.Release()
		return nil, false, fmt.Errorf("pg_try_advisory_lock: %w", err)
	}
	if !got {
		conn.Release()
		return nil, false, nil
	}
	var mu sync.Mutex
	lost := false
	return &pgLeadership{
		alive: func() error {
			mu.Lock()
			defer mu.Unlock()
			if lost {
				return errors.New("leader connection lost")
			}
			ctx, cancel := context.WithTimeout(context.Background(), leaderQueryTimeout)
			defer cancel()
			// The lock lives as long as this connection; if the
			// connection is broken, the lock is gone.
			if err := conn.Ping(ctx); err != nil {
				lost = true
				_ = conn.Conn().Close(context.Background())
				return err
			}
			return nil
		},
		release: func() {
			mu.Lock()
			defer mu.Unlock()
			if !lost {
				if _, err := conn.Exec(context.Background(), `SELECT pg_advisory_unlock($1)`, key); err != nil {
					_ = conn.Conn().Close(context.Background())
				}
			}
			conn.Release()
		},
	}, true, nil
}

// MySQL: GET_LOCK with timeout 0 on a dedicated connection.

func (d MysqlDB) TryLeadership(name string) (Leadership, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), leaderQueryTimeout)
	defer cancel()
	conn, err := d.lockDB.Conn(ctx)
	if err != nil {
		return nil, false, err
	}
	lockName := mysqlLockName(leaderLockName(name))
	var got sql.NullInt64
	if err := conn.QueryRowContext(ctx, `SELECT GET_LOCK(?, 0)`, lockName).Scan(&got); err != nil {
		conn.Close()
		return nil, false, fmt.Errorf("GET_LOCK: %w", err)
	}
	if got.Int64 != 1 {
		conn.Close()
		return nil, false, nil
	}
	var mu sync.Mutex
	lost := false
	return &pgLeadership{
		alive: func() error {
			mu.Lock()
			defer mu.Unlock()
			if lost {
				return errors.New("leader connection lost")
			}
			ctx, cancel := context.WithTimeout(context.Background(), leaderQueryTimeout)
			defer cancel()
			// IS_USED_LOCK returns the connection id holding the lock; it
			// must still be ours.
			var holder, self sql.NullInt64
			err := conn.QueryRowContext(ctx, `SELECT IS_USED_LOCK(?), CONNECTION_ID()`, lockName).Scan(&holder, &self)
			if err == nil && (!holder.Valid || holder.Int64 != self.Int64) {
				err = errors.New("leader lock no longer held")
			}
			if err != nil {
				lost = true
				_ = conn.Raw(func(any) error { return driver.ErrBadConn })
				return err
			}
			return nil
		},
		release: func() {
			mu.Lock()
			defer mu.Unlock()
			if !lost {
				if _, err := conn.ExecContext(context.Background(), `SELECT RELEASE_LOCK(?)`, lockName); err != nil {
					_ = conn.Raw(func(any) error { return driver.ErrBadConn })
				}
			}
			conn.Close()
		},
	}, true, nil
}

var (
	_ LeaderMethods = (*MemoryDataStore)(nil)
	_ LeaderMethods = SQLiteDB{}
	_ LeaderMethods = PostgresDB{}
	_ LeaderMethods = MysqlDB{}
)

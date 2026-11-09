package cluster

import (
	"sync"
	"time"

	"github.com/ether/etherpad-go/lib/db"
	"go.uber.org/zap"
)

// leaderCheckInterval is how often a follower tries to become leader and a
// leader checks that it still holds the lock. After a leader dies, another
// instance takes over within this interval.
const leaderCheckInterval = 5 * time.Second

// Elector keeps one instance among those sharing a database as the leader
// for a background job. It is safe to use from several goroutines.
type Elector struct {
	store    db.LeaderMethods
	name     string
	logger   *zap.SugaredLogger
	interval time.Duration

	mu    sync.Mutex
	lease db.Leadership

	stop     chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
}

// NewElector returns an elector for the job name; call Start to begin.
func NewElector(store db.LeaderMethods, name string, logger *zap.SugaredLogger) *Elector {
	return &Elector{store: store, name: name, logger: logger, interval: leaderCheckInterval, stop: make(chan struct{})}
}

// WithInterval sets how often the elector checks; for tests.
func (e *Elector) WithInterval(d time.Duration) *Elector {
	e.interval = d
	return e
}

// Start tries to become leader right away and then keeps checking.
func (e *Elector) Start() {
	e.check()
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		t := time.NewTicker(e.interval)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				e.check()
			case <-e.stop:
				return
			}
		}
	}()
}

func (e *Elector) check() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.lease != nil {
		if err := e.lease.Alive(); err != nil {
			e.logger.Warnw("lost leadership", "job", e.name, "error", err)
			e.lease.Release()
			e.lease = nil
		}
		return
	}
	lease, ok, err := e.store.TryLeadership(e.name)
	if err != nil {
		e.logger.Warnw("leader election failed", "job", e.name, "error", err)
		return
	}
	if ok {
		e.lease = lease
		e.logger.Infow("this instance is now the leader", "job", e.name)
	}
}

// IsLeader reports whether this instance currently runs the job.
func (e *Elector) IsLeader() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.lease != nil
}

// Stop gives up leadership so another instance can take over at once.
func (e *Elector) Stop() {
	e.stopOnce.Do(func() {
		close(e.stop)
		e.wg.Wait()
		e.mu.Lock()
		if e.lease != nil {
			e.lease.Release()
			e.lease = nil
		}
		e.mu.Unlock()
	})
}

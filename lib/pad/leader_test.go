package pad

import (
	"testing"
	"time"

	"github.com/ether/etherpad-go/lib/cluster"
	"github.com/ether/etherpad-go/lib/db"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestLeaderLockIsExclusiveAcrossInstances(t *testing.T) {
	forEachSharedStore(t, func(t *testing.T, open func(t *testing.T) db.DataStore) {
		a, b := open(t), open(t)

		la, ok, err := a.TryLeadership("job")
		require.NoError(t, err)
		require.True(t, ok, "a free lock is taken")
		require.NoError(t, la.Alive())

		_, ok, err = b.TryLeadership("job")
		require.NoError(t, err)
		require.False(t, ok, "a held lock is not taken twice")

		lo, ok, err := b.TryLeadership("other-job")
		require.NoError(t, err)
		require.True(t, ok, "different jobs have different leaders")
		lo.Release()

		la.Release()
		la.Release() // idempotent
		lb, ok, err := b.TryLeadership("job")
		require.NoError(t, err)
		require.True(t, ok, "a released lock can be taken")
		lb.Release()
	})
}

func TestElectorFailsOver(t *testing.T) {
	forEachSharedStore(t, func(t *testing.T, open func(t *testing.T) db.DataStore) {
		logger := zap.NewNop().Sugar()
		e1 := cluster.NewElector(open(t), "failover", logger).WithInterval(20 * time.Millisecond)
		e2 := cluster.NewElector(open(t), "failover", logger).WithInterval(20 * time.Millisecond)
		e1.Start()
		e2.Start()
		defer e2.Stop()

		require.Eventually(t, func() bool { return e1.IsLeader() != e2.IsLeader() }, 2*time.Second, 10*time.Millisecond,
			"exactly one leader")
		leader, follower := e1, e2
		if e2.IsLeader() {
			leader, follower = e2, e1
		}
		// Stays stable while the leader lives.
		time.Sleep(100 * time.Millisecond)
		require.True(t, leader.IsLeader())
		require.False(t, follower.IsLeader())

		leader.Stop()
		require.Eventually(t, follower.IsLeader, 2*time.Second, 10*time.Millisecond, "the follower takes over")
		e1.Stop()
	})
}

// That a dead leader's lock is released with its database connection is
// checked with real processes by "etherpad-go killbench" (see
// doc/cluster.md): closing a pool in-process waits for the held connection.

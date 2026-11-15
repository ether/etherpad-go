package db

import (
	"testing"
	"time"

	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The heartbeat stores the instance's connected users (migration 17), and
// a later heartbeat replaces the count.
func TestClusterNodeUsers(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(testutils.TestRunConfig{Name: "users with the heartbeat", Test: func(t *testing.T, ds testutils.TestDataStore) {
		now := time.Now()
		require.NoError(t, ds.DS.HeartbeatClusterNode("users-a", "10.0.0.1:7946", now, 5))
		require.NoError(t, ds.DS.HeartbeatClusterNode("users-b", "10.0.0.2:7946", now, 0))
		require.NoError(t, ds.DS.HeartbeatClusterNode("users-a", "10.0.0.1:7946", now.Add(time.Second), 7))
		nodes, err := ds.DS.ListClusterNodes(now.Add(-time.Minute))
		require.NoError(t, err)
		got := map[string]int{}
		for _, n := range nodes {
			got[n.ID] = n.Users
		}
		assert.Equal(t, 7, got["users-a"])
		assert.Equal(t, 0, got["users-b"])
	}})
	testDb.StartTestDBHandler()
}

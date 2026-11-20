package db

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A login code is consumed once on every database, also when callers race.
func TestConsumeAuthCode(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(testutils.TestRunConfig{Name: "consume once", Test: testConsumeAuthCode})
	testDb.StartTestDBHandler()
}

func testConsumeAuthCode(t *testing.T, ds testutils.TestDataStore) {
	s := ds.DS
	now := time.Now()
	require.NoError(t, s.CreateAuthCode("sig-1", "user_client", "req-1", "", "", "", "", now, now.Add(time.Minute)))
	row, err := s.GetAuthCode("sig-1")
	require.NoError(t, err)
	require.True(t, row.Active)

	var wins atomic.Int32
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if ok, err := s.ConsumeAuthCode("sig-1"); err == nil && ok {
				wins.Add(1)
			}
		}()
	}
	wg.Wait()
	assert.Equal(t, int32(1), wins.Load(), "exactly one caller consumes the code")
	row, err = s.GetAuthCode("sig-1")
	require.NoError(t, err)
	assert.False(t, row.Active)

	ok, err := s.ConsumeAuthCode("unknown")
	require.NoError(t, err)
	assert.False(t, ok)
}

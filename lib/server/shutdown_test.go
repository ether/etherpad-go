package server

import (
	"testing"
	"time"

	"github.com/ether/etherpad-go/lib/lifecycle"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/ether/etherpad-go/lib/ws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestShutdownDelayOnlyInACluster(t *testing.T) {
	s := &settings.Settings{DBType: "sqlite"}
	s.Cluster.ShutdownDelaySeconds = 5
	assert.Zero(t, shutdownDelay(s), "a single instance has nowhere to send users")
	s.DBType = "postgres"
	assert.Equal(t, 5*time.Second, shutdownDelay(s))
	s.Cluster.ShutdownDelaySeconds = 0
	assert.Zero(t, shutdownDelay(s))
}

func TestGracefulShutdownOrder(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	conns := []*ws.MockWebSocketConn{ws.NewActualMockWebSocketconn(), ws.NewActualMockWebSocketconn()}
	for _, c := range conns {
		client := &ws.Client{Hub: hub, Conn: c, Send: make(chan []byte, 4)}
		hub.Register <- client
		// The read loop of a closed connection unregisters it.
		go func() {
			for !c.IsClosed() {
				time.Sleep(time.Millisecond)
			}
			hub.Unregister <- client
		}()
	}
	require.Eventually(t, func() bool { return hub.NumClients() == 2 }, time.Second, time.Millisecond)

	state := &lifecycle.State{}
	state.MarkStarted()
	var steps []string
	gracefulShutdown(shutdownSteps{
		logger:    zap.NewNop().Sugar(),
		lifecycle: state,
		delay:     5 * time.Second,
		hub:       hub,
		sleep: func(d time.Duration) {
			if d == 5*time.Second {
				assert.False(t, state.Ready(), "not ready while waiting for the load balancer")
				assert.Zero(t, countClosed(conns), "connections stay open during the delay")
				steps = append(steps, "delay")
				return
			}
			time.Sleep(d)
		},
		stopJobs: func() { steps = append(steps, "jobs") },
		leaveCluster: func() {
			assert.Equal(t, 2, countClosed(conns), "connections closed before leaving the cluster")
			assert.Zero(t, hub.NumClients(), "closed connections finished")
			steps = append(steps, "cluster")
		},
		stopHTTP: func() { steps = append(steps, "http") },
	})
	assert.Equal(t, []string{"delay", "jobs", "cluster", "http"}, steps)
	assert.True(t, state.Draining())
}

func countClosed(conns []*ws.MockWebSocketConn) int {
	n := 0
	for _, c := range conns {
		if c.IsClosed() {
			n++
		}
	}
	return n
}

package server

import (
	"time"

	"github.com/ether/etherpad-go/lib/lifecycle"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/ether/etherpad-go/lib/ws"
	"go.uber.org/zap"
)

// clientDrainTimeout bounds how long shutdown waits for closed pad
// connections to finish the message they were handling.
const clientDrainTimeout = 10 * time.Second

type shutdownSteps struct {
	logger    *zap.SugaredLogger
	lifecycle *lifecycle.State
	// delay is how long to keep serving while reporting not ready.
	delay        time.Duration
	hub          *ws.Hub
	stopJobs     func()
	leaveCluster func()
	stopHTTP     func()
	// sleep is time.Sleep; replaced in tests.
	sleep func(time.Duration)
}

// shutdownDelay is the configured delay for clustered instances; a single
// instance has nowhere to send its users, so it does not wait.
func shutdownDelay(s *settings.Settings) time.Duration {
	if !s.Clustered() || s.Cluster.ShutdownDelaySeconds <= 0 {
		return 0
	}
	return time.Duration(s.Cluster.ShutdownDelaySeconds) * time.Second
}

// gracefulShutdown stops the instance without losing changes:
//
//  1. Report not ready and refuse new pad connections, then wait delay so
//     the load balancer stops sending users here.
//  2. Stop background jobs and give up leadership.
//  3. Close the pad connections ("going away"). Clients reconnect to
//     another instance. Each connection finishes the change it is handling
//     first; changes not yet stored are sent again by the client.
//  4. Leave the cluster and stop the HTTP server.
func gracefulShutdown(s shutdownSteps) {
	sleep := s.sleep
	if sleep == nil {
		sleep = time.Sleep
	}
	s.lifecycle.BeginDrain()
	if s.delay > 0 {
		s.logger.Infof("Not ready any more; closing connections in %s", s.delay)
		sleep(s.delay)
	}
	s.stopJobs()

	if n := s.hub.CloseAll(); n > 0 {
		s.logger.Infof("Closed %d connections; clients reconnect to another instance", n)
		deadline := time.Now().Add(clientDrainTimeout)
		for s.hub.NumClients() > 0 && time.Now().Before(deadline) {
			sleep(20 * time.Millisecond)
		}
		if n := s.hub.NumClients(); n > 0 {
			s.logger.Warnf("%d connections still open after %s", n, clientDrainTimeout)
		}
	}

	s.leaveCluster()
	s.stopHTTP()
}

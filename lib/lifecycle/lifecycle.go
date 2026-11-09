// Package lifecycle tracks whether this instance should receive traffic:
// not before it has started, and no longer once it is shutting down.
package lifecycle

import "sync/atomic"

// State is safe for concurrent use. The zero value is "starting".
type State struct {
	started  atomic.Bool
	draining atomic.Bool
}

// MarkStarted is called once the instance serves requests.
func (s *State) MarkStarted() { s.started.Store(true) }

// BeginDrain is called when the instance starts shutting down.
func (s *State) BeginDrain() { s.draining.Store(true) }

// Ready reports whether a load balancer should send new traffic here.
func (s *State) Ready() bool { return s != nil && s.started.Load() && !s.draining.Load() }

// Draining reports whether the instance is shutting down.
func (s *State) Draining() bool { return s != nil && s.draining.Load() }

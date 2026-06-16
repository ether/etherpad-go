package updater

import (
	"sync"
	"time"
)

// DrainResult is the outcome of a drain.
type DrainResult string

const (
	DrainCompleted DrainResult = "completed"
	DrainCancelled DrainResult = "cancelled"
)

// Drainer blocks new pad connections for a countdown window and announces the
// countdown to connected clients, giving them time to save before the binary is
// swapped and the process restarts.
type Drainer struct {
	drainSecs int
	setAccept func(bool)         // toggles whether new connections are accepted
	broadcast func(secsLeft int) // optional: announce countdown to clients

	cancel     chan struct{}
	cancelOnce sync.Once
}

// NewDrainer constructs a drainer. setAccept must not be nil; broadcast may be.
func NewDrainer(drainSecs int, setAccept func(bool), broadcast func(int)) *Drainer {
	return &Drainer{
		drainSecs: drainSecs,
		setAccept: setAccept,
		broadcast: broadcast,
		cancel:    make(chan struct{}),
	}
}

// Start blocks until the drain window elapses or Cancel is called. New
// connections are refused for the duration and restored on return.
func (d *Drainer) Start() DrainResult {
	d.setAccept(false)
	defer d.setAccept(true)

	if d.broadcast != nil {
		d.broadcast(d.drainSecs)
	}
	if d.drainSecs <= 0 {
		return DrainCompleted
	}

	deadline := time.Now().Add(time.Duration(d.drainSecs) * time.Second)
	announced := map[int]bool{}
	thresholds := []int{60, 30, 10}

	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return DrainCompleted
		}
		secsLeft := int(remaining.Seconds())
		if d.broadcast != nil {
			for _, th := range thresholds {
				if !announced[th] && th < d.drainSecs && secsLeft <= th {
					d.broadcast(th)
					announced[th] = true
				}
			}
		}
		wait := min(remaining, time.Second)
		select {
		case <-d.cancel:
			return DrainCancelled
		case <-time.After(wait):
		}
	}
}

// Cancel aborts an in-progress drain.
func (d *Drainer) Cancel() {
	d.cancelOnce.Do(func() { close(d.cancel) })
}

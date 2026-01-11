package ratelimiter

import (
	"sync"
	"time"

	"github.com/ether/etherpad-go/lib/settings"
)

type IPAddress string

type Event struct {
	LastOccurrence int64
}

type RateLimiter struct {
	Mu          sync.RWMutex
	RateLimiter map[IPAddress][]Event
}

var rateLimiter RateLimiter

func init() {
	rateLimiter = RateLimiter{
		Mu:          sync.RWMutex{},
		RateLimiter: make(map[IPAddress][]Event),
	}
}

type ErrRateLimitExceeded struct{}

func (e ErrRateLimitExceeded) Error() string {
	return "rate limit exceeded"
}

func CheckRateLimit(ip IPAddress, limiting settings.CommitRateLimiting) error {
	if limiting.LoadTest {
		return nil
	}

	rateLimiter.Mu.RLock()
	value, ok := rateLimiter.RateLimiter[ip]
	rateLimiter.Mu.RUnlock()
	if !ok {
		rateLimiter.Mu.Lock()
		rateLimiter.RateLimiter[ip] = []Event{}
		value = rateLimiter.RateLimiter[ip]
		rateLimiter.Mu.Unlock()
	}

	// Clean up old events
	cutoff := time.Now().Add(time.Duration(-limiting.Duration) * time.Second).Unix()
	var filteredEvents []Event
	for _, event := range value {
		if event.LastOccurrence >= cutoff {
			filteredEvents = append(filteredEvents, event)
		}
	}

	// Add the new event

	filteredEvents = append(filteredEvents, Event{LastOccurrence: time.Now().Unix()})
	rateLimiter.Mu.Lock()
	defer rateLimiter.Mu.Unlock()
	rateLimiter.RateLimiter[ip] = filteredEvents
	if len(rateLimiter.RateLimiter[ip]) > limiting.Points {
		return ErrRateLimitExceeded{}
	}
	return nil
}

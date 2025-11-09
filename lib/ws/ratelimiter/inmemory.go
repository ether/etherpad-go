package ratelimiter

import (
	"time"

	"github.com/ether/etherpad-go/lib/settings"
)

type IPAddress string

type Event struct {
	LastOccurrence int64
}

var rateLimiter map[IPAddress][]Event

func init() {
	rateLimiter = make(map[IPAddress][]Event)
}

type ErrRateLimitExceeded struct{}

func (e ErrRateLimitExceeded) Error() string {
	return "rate limit exceeded"
}

func CheckRateLimit(ip IPAddress, limiting settings.CommitRateLimiting) error {
	value, ok := rateLimiter[ip]
	if !ok {
		rateLimiter[ip] = []Event{}
		value = rateLimiter[ip]
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
	rateLimiter[ip] = filteredEvents
	if len(rateLimiter[ip]) > limiting.Points {
		return ErrRateLimitExceeded{}
	}
	return nil
}

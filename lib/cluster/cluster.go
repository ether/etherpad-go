// Package cluster passes live updates between Etherpad instances that share
// one database.
//
// The database stays the only source of truth: what is sent here are hints
// ("pad X is at revision N", the receiver loads the revisions itself) and
// messages that exist only in memory (chat broadcasts, who is in a pad).
// A lost hint delays an update until the next one or the periodic catch-up,
// it never loses or corrupts data.
//
// Instances find each other through the database (table cluster_nodes, like
// Keycloak's jdbc-ping) and talk directly with hashicorp/memberlist, which
// handles membership and failure detection.
package cluster

import (
	"encoding/json"
)

// Kind is the type of an Event.
type Kind string

const (
	// KindRevision: pad Pad has new revisions (up to Rev).
	KindRevision Kind = "rev"
	// KindRoom: send Payload to the pad's clients, except session Session.
	KindRoom Kind = "room"
	// KindJoin: session Session joined pad Pad; Payload announces it to the
	// pad's clients. Receivers answer with their users (KindSession).
	KindJoin Kind = "join"
	// KindSession: send Payload to session Session.
	KindSession Kind = "session"
	// KindAuthor: send Payload to the sessions of author Author in Pad.
	KindAuthor Kind = "author"
	// KindDeleted: pad Pad was deleted.
	KindDeleted Kind = "deleted"
	// KindAll: send Payload to every client.
	KindAll Kind = "all"
)

// Event is one message between instances.
type Event struct {
	Kind    Kind            `json:"k"`
	Pad     string          `json:"p,omitempty"`
	Rev     int             `json:"r,omitempty"`
	Session string          `json:"s,omitempty"`
	Author  string          `json:"a,omitempty"`
	Payload json.RawMessage `json:"m,omitempty"`
	// Origin is the sending instance; set by the bus.
	Origin string `json:"o,omitempty"`
}

// Bus connects the instances.
type Bus interface {
	// Publish sends e to every other instance.
	Publish(e Event)
	// SendTo sends e to one instance (an Event's Origin).
	SendTo(node string, e Event)
	// Subscribe registers fn for events from other instances. Events may
	// arrive out of order and, rarely, not at all: revision hints only make
	// the receiver look at the database, which is always right.
	Subscribe(fn func(Event))
	// Enabled reports whether there may be other instances.
	Enabled() bool
	// Members is the number of instances, this one included.
	Members() int
	Close() error
}

// Local is the bus of a single instance: nothing to send.
type Local struct{}

func (Local) Publish(Event)         {}
func (Local) SendTo(string, Event)  {}
func (Local) Subscribe(func(Event)) {}
func (Local) Enabled() bool         { return false }
func (Local) Members() int          { return 1 }
func (Local) Close() error          { return nil }

package ws

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/ether/etherpad-go/lib/cluster"
	pad2 "github.com/ether/etherpad-go/lib/models/pad"
	ws "github.com/ether/etherpad-go/lib/models/ws"
)

// catchUpInterval is how often the pads with clients here are compared with
// the database, in case a revision hint from another instance got lost.
const catchUpInterval = 2 * time.Second

// clusterState is the PadMessageHandler's part of the cluster.
type clusterState struct {
	bus cluster.Bus

	mu sync.Mutex
	// catchingUp holds the pads being caught up; true means another
	// catch-up was asked for meanwhile.
	catchingUp map[string]bool
	stop       chan struct{}
}

// SetCluster connects the handler to the other instances on the database.
func (p *PadMessageHandler) SetCluster(bus cluster.Bus) {
	p.cluster = &clusterState{bus: bus, catchingUp: map[string]bool{}, stop: make(chan struct{})}
	if !bus.Enabled() {
		return
	}
	bus.Subscribe(p.onClusterEvent)
	go p.catchUpLoop()
}

// StopCluster ends the periodic catch-up.
func (p *PadMessageHandler) StopCluster() {
	if p.cluster != nil && p.cluster.bus.Enabled() {
		close(p.cluster.stop)
	}
}

func (p *PadMessageHandler) clusterBus() cluster.Bus {
	if p.cluster == nil {
		return cluster.Local{}
	}
	return p.cluster.bus
}

// publish sends e to the other instances.
func (p *PadMessageHandler) publish(e cluster.Event) {
	if bus := p.clusterBus(); bus.Enabled() {
		bus.Publish(e)
	}
}

// broadcastToRoom sends payload to the clients of the pad on every
// instance, except the session except.
func (p *PadMessageHandler) broadcastToRoom(padID string, payload []byte, except string) {
	p.sendToLocalRoom(padID, payload, except)
	p.publish(cluster.Event{Kind: cluster.KindRoom, Pad: padID, Session: except, Payload: payload})
}

func (p *PadMessageHandler) sendToLocalRoom(padID string, payload []byte, except string) {
	for _, socket := range p.GetRoomSockets(padID) {
		if except != "" && socket.SessionId == except {
			continue
		}
		socket.SafeSend(payload)
	}
}

// BroadcastToAllClients sends payload to every client on every instance.
func (p *PadMessageHandler) BroadcastToAllClients(payload []byte) {
	p.sendToAllLocal(payload)
	p.publish(cluster.Event{Kind: cluster.KindAll, Payload: payload})
}

func (p *PadMessageHandler) sendToAllLocal(payload []byte) {
	p.hub.ClientsRWMutex.RLock()
	defer p.hub.ClientsRWMutex.RUnlock()
	for client := range p.hub.Clients {
		if client != nil {
			client.SafeSend(payload)
		}
	}
}

func (p *PadMessageHandler) onClusterEvent(e cluster.Event) {
	switch e.Kind {
	case cluster.KindRevision:
		p.scheduleCatchUp(e.Pad)
	case cluster.KindRoom:
		p.sendToLocalRoom(e.Pad, e.Payload, e.Session)
	case cluster.KindJoin:
		p.sendToLocalRoom(e.Pad, e.Payload, "")
		p.answerJoin(e)
	case cluster.KindSession:
		for _, socket := range p.GetRoomSockets(e.Pad) {
			if socket.SessionId == e.Session {
				socket.SafeSend(e.Payload)
			}
		}
	case cluster.KindAuthor:
		for _, socket := range p.GetRoomSockets(e.Pad) {
			if s := p.SessionStore.getSession(socket.SessionId); s != nil && s.Author == e.Author {
				socket.SafeSend(e.Payload)
			}
		}
	case cluster.KindDeleted:
		p.KickSessionsFromPad(e.Pad)
		p.padManager.UnloadPad(e.Pad)
	case cluster.KindAll:
		p.sendToAllLocal(e.Payload)
	}
}

// answerJoin tells a client that joined on another instance who is in the
// pad here.
func (p *PadMessageHandler) answerJoin(e cluster.Event) {
	seen := map[string]bool{}
	for _, socket := range p.GetRoomSockets(e.Pad) {
		s := p.SessionStore.getSession(socket.SessionId)
		if s == nil || s.Author == "" || seen[s.Author] {
			continue
		}
		seen[s.Author] = true
		payload, ok := p.userNewInfoMessage(s.Author)
		if !ok {
			continue
		}
		p.clusterBus().SendTo(e.Origin, cluster.Event{Kind: cluster.KindSession, Pad: e.Pad, Session: e.Session, Payload: payload})
	}
}

// userNewInfoMessage is the USER_NEWINFO message announcing author.
func (p *PadMessageHandler) userNewInfoMessage(authorID string) ([]byte, bool) {
	a, err := p.authorManager.GetAuthor(authorID)
	if err != nil {
		return nil, false
	}
	msg := []interface{}{"message", ws.UserNewInfo{
		Type: "COLLABROOM",
		Data: ws.UserNewInfoData{
			Type:     "USER_NEWINFO",
			UserInfo: ws.UserNewInfoDat{UserId: authorID, Name: a.Name, ColorId: a.ColorId},
		},
	}}
	b, err := json.Marshal(msg)
	return b, err == nil
}

// scheduleCatchUp sends the pad's clients here the revisions other
// instances stored. Requests for a pad that is being caught up are folded
// into one more round.
func (p *PadMessageHandler) scheduleCatchUp(padID string) {
	c := p.cluster
	c.mu.Lock()
	if _, running := c.catchingUp[padID]; running {
		c.catchingUp[padID] = true
		c.mu.Unlock()
		return
	}
	c.catchingUp[padID] = false
	c.mu.Unlock()

	go func() {
		for {
			cached := p.padManager.CatchUp(padID, func(pd *pad2.Pad) {
				p.updatePadClients(pd, nil)
			})
			if !cached {
				p.kickIfDeleted(padID)
			}
			c.mu.Lock()
			if !c.catchingUp[padID] {
				delete(c.catchingUp, padID)
				c.mu.Unlock()
				return
			}
			c.catchingUp[padID] = false
			c.mu.Unlock()
		}
	}()
}

// kickIfDeleted disconnects the clients here of a pad that another
// instance deleted.
func (p *PadMessageHandler) kickIfDeleted(padID string) {
	if len(p.GetRoomSockets(padID)) == 0 {
		return
	}
	exists, err := p.padManager.DoesPadExist(padID)
	if err == nil && !*exists {
		p.KickSessionsFromPad(padID)
	}
}

// catchUpLoop periodically catches up every pad with clients here.
func (p *PadMessageHandler) catchUpLoop() {
	t := time.NewTicker(catchUpInterval)
	defer t.Stop()
	for {
		select {
		case <-p.cluster.stop:
			return
		case <-t.C:
			for padID := range p.localPads() {
				p.scheduleCatchUp(padID)
			}
		}
	}
}

// localPads returns the pads with clients on this instance.
func (p *PadMessageHandler) localPads() map[string]struct{} {
	pads := map[string]struct{}{}
	p.hub.ClientsRWMutex.RLock()
	defer p.hub.ClientsRWMutex.RUnlock()
	for client := range p.hub.Clients {
		if client == nil {
			continue
		}
		if s := p.SessionStore.getSession(client.SessionId); s != nil && s.PadId != "" {
			pads[s.PadId] = struct{}{}
		}
	}
	return pads
}

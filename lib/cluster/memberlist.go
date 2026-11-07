package cluster

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/hashicorp/memberlist"
	"go.uber.org/zap"
)

const (
	heartbeatInterval = 5 * time.Second
	// nodeAliveFor is how long a registry entry counts after its last
	// heartbeat; pruneAfter is when it is removed.
	nodeAliveFor = 30 * time.Second
	pruneAfter   = 10 * time.Minute
	gossipKeyKey = "cluster_gossip_key"
	// maxBestEffort is the largest message sent over UDP; larger ones and
	// all payloads go over TCP.
	maxBestEffort = 1000
)

type outgoing struct {
	to   *memberlist.Node
	data []byte
	// reliable sends over TCP, otherwise UDP.
	reliable bool
}

// Memberlist is the bus between instances on one database.
type Memberlist struct {
	ml     *memberlist.Memberlist
	name   string
	addr   string
	store  db.DataStore
	logger *zap.SugaredLogger

	mu       sync.RWMutex
	handlers []func(Event)

	inbox     chan Event
	outbox    chan outgoing
	stop      chan struct{}
	wg        sync.WaitGroup
	closeOnce sync.Once
	closeErr  error
}

// NewMemberlist starts the gossip listener, registers this instance in the
// database and joins the other registered instances.
func NewMemberlist(cfg settings.Cluster, store db.DataStore, logger *zap.SugaredLogger) (*Memberlist, error) {
	key, err := gossipKey(store)
	if err != nil {
		return nil, fmt.Errorf("cluster key: %w", err)
	}

	b := &Memberlist{
		name:   nodeName(),
		store:  store,
		logger: logger,
		inbox:  make(chan Event, 4096),
		outbox: make(chan outgoing, 4096),
		stop:   make(chan struct{}),
	}

	c := memberlist.DefaultLANConfig()
	c.Name = b.name
	c.BindAddr = cfg.BindAddr
	c.BindPort = cfg.BindPort
	c.AdvertisePort = cfg.BindPort
	if cfg.AdvertisePort > 0 {
		c.AdvertisePort = cfg.AdvertisePort
	}
	if addr := firstNonEmpty(cfg.AdvertiseAddr, os.Getenv("POD_IP")); addr != "" {
		c.AdvertiseAddr = addr
	}
	c.SecretKey = key
	c.Delegate = b
	c.Events = b
	c.Logger = log.New(memberlistLog{logger}, "", 0)

	ml, err := memberlist.Create(c)
	if err != nil {
		return nil, fmt.Errorf("starting memberlist: %w", err)
	}
	b.ml = ml
	b.addr = ml.LocalNode().Address()
	logger.Infow("cluster node started", "node", b.name, "address", b.addr)

	b.heartbeat()
	b.wg.Add(3)
	go b.receiveLoop()
	go b.sendLoop()
	go b.heartbeatLoop()
	return b, nil
}

func nodeName() string {
	host, _ := os.Hostname()
	suffix := make([]byte, 4)
	_, _ = rand.Read(suffix)
	return host + "-" + hex.EncodeToString(suffix)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// gossipKey is the AES key the instances encrypt their traffic with; the
// first instance creates it in the database. Two instances starting
// together may both write one, so it is read back after a pause and the
// stored one wins.
func gossipKey(store db.DataStore) ([]byte, error) {
	read := func() ([]byte, error) {
		raw, err := store.GetOIDCStorageValue(gossipKeyKey)
		if err != nil || raw == nil || *raw == "" {
			return nil, err
		}
		return base64.StdEncoding.DecodeString(*raw)
	}
	key, err := read()
	if err != nil || key != nil {
		return key, err
	}
	key = make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := store.SetOIDCStorageValue(gossipKeyKey, base64.StdEncoding.EncodeToString(key)); err != nil {
		return nil, err
	}
	time.Sleep(300 * time.Millisecond)
	stored, err := read()
	if err != nil || stored == nil {
		return key, err
	}
	return stored, nil
}

// heartbeat records this instance and joins registered ones it is not
// connected to: at start, and to heal after a network split.
func (b *Memberlist) heartbeat() {
	now := time.Now()
	if err := b.store.HeartbeatClusterNode(b.name, b.addr, now); err != nil {
		b.logger.Warnw("cluster heartbeat failed", "error", err)
		return
	}
	nodes, err := b.store.ListClusterNodes(now.Add(-nodeAliveFor))
	if err != nil {
		b.logger.Warnw("listing cluster nodes failed", "error", err)
		return
	}
	known := map[string]bool{}
	for _, m := range b.ml.Members() {
		known[m.Name] = true
	}
	var join []string
	for _, n := range nodes {
		if n.ID != b.name && !known[n.ID] {
			join = append(join, n.Address)
		}
	}
	if len(join) > 0 {
		if _, err := b.ml.Join(join); err != nil {
			b.logger.Debugw("joining cluster nodes failed", "nodes", join, "error", err)
		}
	}
}

func (b *Memberlist) heartbeatLoop() {
	defer b.wg.Done()
	t := time.NewTicker(heartbeatInterval)
	defer t.Stop()
	lastPrune := time.Now()
	for {
		select {
		case <-b.stop:
			return
		case <-t.C:
			b.heartbeat()
			if time.Since(lastPrune) > time.Minute {
				_ = b.store.PruneClusterNodes(time.Now().Add(-pruneAfter))
				lastPrune = time.Now()
			}
		}
	}
}

func (b *Memberlist) Publish(e Event) {
	for _, m := range b.ml.Members() {
		if m.Name != b.name {
			b.enqueue(m, e)
		}
	}
}

func (b *Memberlist) SendTo(node string, e Event) {
	for _, m := range b.ml.Members() {
		if m.Name == node {
			b.enqueue(m, e)
			return
		}
	}
}

func (b *Memberlist) enqueue(to *memberlist.Node, e Event) {
	e.Origin = b.name
	data, err := json.Marshal(e)
	if err != nil {
		return
	}
	// Revision hints are small and repeated by the periodic catch-up, so
	// UDP is enough; everything with a payload goes over TCP.
	reliable := len(e.Payload) > 0 || len(data) > maxBestEffort
	select {
	case b.outbox <- outgoing{to: to, data: data, reliable: reliable}:
	default:
		b.logger.Warnw("cluster send queue full, dropping event", "kind", e.Kind, "pad", e.Pad)
	}
}

func (b *Memberlist) sendLoop() {
	defer b.wg.Done()
	for {
		select {
		case <-b.stop:
			return
		case o := <-b.outbox:
			var err error
			if o.reliable {
				err = b.ml.SendReliable(o.to, o.data)
			} else {
				err = b.ml.SendBestEffort(o.to, o.data)
			}
			if err != nil {
				b.logger.Debugw("cluster send failed", "to", o.to.Name, "error", err)
			}
		}
	}
}

func (b *Memberlist) Subscribe(fn func(Event)) {
	b.mu.Lock()
	b.handlers = append(b.handlers, fn)
	b.mu.Unlock()
}

func (b *Memberlist) receiveLoop() {
	defer b.wg.Done()
	for {
		select {
		case <-b.stop:
			return
		case e := <-b.inbox:
			b.mu.RLock()
			handlers := b.handlers
			b.mu.RUnlock()
			for _, h := range handlers {
				h(e)
			}
		}
	}
}

func (b *Memberlist) Enabled() bool { return true }
func (b *Memberlist) Members() int  { return b.ml.NumMembers() }

// Close leaves the cluster and removes this instance from the registry.
func (b *Memberlist) Close() error {
	b.closeOnce.Do(func() {
		_ = b.ml.Leave(2 * time.Second)
		close(b.stop)
		b.wg.Wait()
		_ = b.store.RemoveClusterNode(b.name)
		b.closeErr = b.ml.Shutdown()
	})
	return b.closeErr
}

// --- memberlist.Delegate ---

func (b *Memberlist) NodeMeta(int) []byte { return nil }

func (b *Memberlist) NotifyMsg(msg []byte) {
	var e Event
	if err := json.Unmarshal(msg, &e); err != nil {
		b.logger.Debugw("invalid cluster message", "error", err)
		return
	}
	select {
	case b.inbox <- e:
	default:
		b.logger.Warnw("cluster receive queue full, dropping event", "kind", e.Kind, "pad", e.Pad)
	}
}

func (b *Memberlist) GetBroadcasts(int, int) [][]byte { return nil }
func (b *Memberlist) LocalState(bool) []byte          { return nil }
func (b *Memberlist) MergeRemoteState([]byte, bool)   {}

// --- memberlist.EventDelegate ---

func (b *Memberlist) NotifyJoin(n *memberlist.Node) {
	if n.Name != b.name {
		b.logger.Infow("cluster node joined", "node", n.Name, "address", n.Address())
	}
}

func (b *Memberlist) NotifyLeave(n *memberlist.Node) {
	b.logger.Infow("cluster node left", "node", n.Name)
}

func (b *Memberlist) NotifyUpdate(*memberlist.Node) {}

// memberlistLog forwards memberlist's log lines to zap.
type memberlistLog struct{ logger *zap.SugaredLogger }

func (l memberlistLog) Write(p []byte) (int, error) {
	line := strings.TrimSpace(string(p))
	switch {
	case strings.Contains(line, "[ERR]"):
		l.logger.Errorw("memberlist: " + line)
	case strings.Contains(line, "[WARN]"):
		l.logger.Warnw("memberlist: " + line)
	default:
		l.logger.Debugw("memberlist: " + line)
	}
	return len(p), nil
}

var _ Bus = (*Memberlist)(nil)
var _ Bus = Local{}

package ep_chat_log_join_leave

import (
	"sync"
	"testing"
	"time"

	"github.com/ether/etherpad-go/lib/hooks/events"
)

type chatLog struct {
	mu   sync.Mutex
	msgs []string
}

func (c *chatLog) ctx(pad, author string) *events.UserJoinLeaveContext {
	return &events.UserJoinLeaveContext{PadId: pad, AuthorId: author, BroadcastChat: func(m map[string]any) {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.msgs = append(c.msgs, m["userId"].(string)+" "+m["ep_chat_log_join_leave"].(string))
	}}
}

func (c *chatLog) got() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.msgs...)
}

func newPlugin(t *testing.T) *EpChatLogJoinLeavePlugin {
	old := disconnectTimeout
	disconnectTimeout = 50 * time.Millisecond
	t.Cleanup(func() { disconnectTimeout = old })
	return &EpChatLogJoinLeavePlugin{users: make(map[string]map[string]*presence)}
}

func expectLog(t *testing.T, c *chatLog, want ...string) {
	t.Helper()
	got := c.got()
	if len(got) != len(want) {
		t.Fatalf("chat: got %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("chat: got %q, want %q", got, want)
		}
	}
}

func TestJoinIsAnnouncedOnce(t *testing.T) {
	p, c := newPlugin(t), &chatLog{}
	p.handleUserJoin(c.ctx("pad", "a.anna"))
	p.handleUserJoin(c.ctx("pad", "a.ben"))
	// A second tab of Anna: no second "joined".
	p.handleUserJoin(c.ctx("pad", "a.anna"))
	expectLog(t, c, "a.anna join", "a.ben join")
}

func TestQuickReconnectSaysNothing(t *testing.T) {
	p, c := newPlugin(t), &chatLog{}
	p.handleUserJoin(c.ctx("pad", "a.anna"))
	p.handleUserLeave(c.ctx("pad", "a.anna"))
	p.handleUserJoin(c.ctx("pad", "a.anna"))
	time.Sleep(3 * disconnectTimeout)
	expectLog(t, c, "a.anna join")
}

// A phone: the new connection arrives before the old one is noticed as gone.
func TestNewConnectionBeforeTheOldOneIsGone(t *testing.T) {
	p, c := newPlugin(t), &chatLog{}
	p.handleUserJoin(c.ctx("pad", "a.anna"))
	p.handleUserJoin(c.ctx("pad", "a.anna"))
	p.handleUserLeave(c.ctx("pad", "a.anna"))
	time.Sleep(3 * disconnectTimeout)
	expectLog(t, c, "a.anna join")
}

func TestLeaveAfterTheTimeout(t *testing.T) {
	p, c := newPlugin(t), &chatLog{}
	p.handleUserJoin(c.ctx("pad", "a.anna"))
	p.handleUserJoin(c.ctx("pad", "a.anna"))
	p.handleUserLeave(c.ctx("pad", "a.anna"))
	p.handleUserLeave(c.ctx("pad", "a.anna"))
	time.Sleep(3 * disconnectTimeout)
	expectLog(t, c, "a.anna join", "a.anna leave")
	// Coming back later is a new visit.
	p.handleUserJoin(c.ctx("pad", "a.anna"))
	expectLog(t, c, "a.anna join", "a.anna leave", "a.anna join")
}

func TestPadsAreSeparate(t *testing.T) {
	p, c := newPlugin(t), &chatLog{}
	p.handleUserJoin(c.ctx("one", "a.anna"))
	p.handleUserJoin(c.ctx("two", "a.anna"))
	expectLog(t, c, "a.anna join", "a.anna join")
}

func TestLeaveWithoutJoinIsIgnored(t *testing.T) {
	p, c := newPlugin(t), &chatLog{}
	p.handleUserLeave(c.ctx("pad", "a.anna"))
	time.Sleep(3 * disconnectTimeout)
	expectLog(t, c)
}

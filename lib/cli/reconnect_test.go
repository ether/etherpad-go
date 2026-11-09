package cli

import (
	"testing"
	"time"

	"github.com/ether/etherpad-go/lib/apool"
	"go.uber.org/zap"
)

func reconnectingPad(t *testing.T) *Pad {
	t.Helper()
	p := NewPad("http://x", "pad", nil, zap.NewNop().Sugar())
	pool := apool.NewAPool()
	p.apool = &pool
	p.atext = &apool.AText{Text: "Qhello\n", Attribs: "|1+7"}
	p.authorId = "a.me"
	p.baseRev = 4
	p.inFlight = &PadChangeset{changeset: "Z:6>1+1$Q", baseRev: 4}
	p.reconnecting = true
	return p
}

// The instance stored the change and died before acknowledging it: the
// change comes back as this author's revision and counts as accepted.
func TestReconnectTakesOwnStoredRevisionAsAccepted(t *testing.T) {
	p := reconnectingPad(t)
	accepted := make(chan int, 1)
	p.OnAcceptCommit(func(rev int) { accepted <- rev })

	p.handleClientReconnect(map[string]interface{}{
		"type": "CLIENT_RECONNECT", "newRev": float64(5), "headRev": float64(5),
		"author": "a.me", "changeset": "Z:6>1+1$Q",
	})

	select {
	case rev := <-accepted:
		if rev != 5 {
			t.Fatalf("accepted rev %d, want 5", rev)
		}
	case <-time.After(time.Second):
		t.Fatal("own stored revision not taken as accepted")
	}
	if p.inFlight != nil || p.outgoing != nil {
		t.Fatal("the stored change must not be sent again")
	}
	if p.baseRev != 5 || p.reconnecting {
		t.Fatalf("baseRev %d reconnecting %v", p.baseRev, p.reconnecting)
	}
	if p.atext.Text != "Qhello\n" {
		t.Fatalf("text changed: %q", p.atext.Text)
	}
}

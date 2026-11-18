package ws

import (
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

// closeRecorder is a WebSocketConn that records when it was closed.
type closeRecorder struct {
	mu     sync.Mutex
	closed time.Time
}

func (c *closeRecorder) ReadMessage() (int, []byte, error)         { select {} }
func (c *closeRecorder) WriteMessage(int, []byte) error            { return nil }
func (c *closeRecorder) SetWriteDeadline(time.Time) error          { return nil }
func (c *closeRecorder) SetReadDeadline(time.Time) error           { return nil }
func (c *closeRecorder) SetPongHandler(func(string) error)         {}
func (c *closeRecorder) WriteControl(int, []byte, time.Time) error { return nil }
func (c *closeRecorder) RemoteAddr() net.Addr                      { return &net.TCPAddr{} }
func (c *closeRecorder) SetReadLimit(int64)                        {}
func (c *closeRecorder) NextWriter(int) (io.WriteCloser, error)    { return nil, io.EOF }
func (c *closeRecorder) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed.IsZero() {
		c.closed = time.Now()
	}
	return nil
}

// A pod shutting down must finish the change a connection is handling and
// send its acknowledgement before it closes the connection; else the client
// reconnects elsewhere, finds the change missing and sends it again.
func TestCloseAllLetsTheHandledMessageFinish(t *testing.T) {
	hub := NewHub()
	conn := &closeRecorder{}
	c := &Client{Hub: hub, Conn: conn, Send: make(chan []byte, 4), drain: &clientDrain{}}
	hub.Clients[c] = true

	if !c.beginHandling() {
		t.Fatal("a new connection refuses messages")
	}
	var finished time.Time
	go func() {
		time.Sleep(150 * time.Millisecond) // storing the change
		c.Send <- []byte("ACCEPT_COMMIT")
		finished = time.Now()
		c.drain.handling.Done()
		time.Sleep(30 * time.Millisecond)
		<-c.Send // the write pump sends it
	}()
	hub.CloseAll()
	if conn.closed.IsZero() || conn.closed.Before(finished) {
		t.Fatalf("closed at %v, the change finished at %v", conn.closed, finished)
	}
	if len(c.Send) != 0 {
		t.Fatal("closed with the acknowledgement still queued")
	}
	if c.beginHandling() {
		t.Fatal("a closing connection still takes messages")
	}
}

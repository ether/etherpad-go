package ws

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// A message whose handling panics must not end the process: the panic is
// logged and only that client's connection is closed.
func TestRecoverClientPanic(t *testing.T) {
	core, logs := observer.New(zap.ErrorLevel)
	logger := zap.New(core).Sugar()
	conn := NewActualMockWebSocketconn()
	client := &Client{Conn: conn, ClientIP: "203.0.113.9"}

	require.NotPanics(t, func() {
		func() {
			defer recoverClientPanic(client, logger, "pad")
			panic("handler bug")
		}()
	})
	require.Equal(t, 1, logs.Len())
	assert.Contains(t, logs.All()[0].Message, "handler bug")
	assert.Contains(t, logs.All()[0].Message, "203.0.113.9")

	// Without a panic nothing is logged.
	func() { defer recoverClientPanic(client, logger, "pad") }()
	assert.Equal(t, 1, logs.Len())
}

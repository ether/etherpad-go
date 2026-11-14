package ws

import (
	"testing"

	"github.com/ether/etherpad-go/lib/models/ws"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Port of etherpad-lite's downstream/wire-socket-sequence spec: the message
// sequence and shapes every realtime client relies on (the Rust terminal
// editor and the Node CLI speak it by hand): CLIENT_READY -> CLIENT_VARS,
// then USER_CHANGES -> ACCEPT_COMMIT with the next revision.
func TestWireSocketSequence(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(
		testutils.TestRunConfig{Name: "handshake and first change", Test: testWireSocketSequence},
	)
	testDb.StartTestDBHandler()
}

func testWireSocketSequence(t *testing.T, ds testutils.TestDataStore) {
	const padID = "wireSequencePad"
	pad, err := ds.PadManager.GetPad(padID, nil, nil)
	require.NoError(t, err)
	require.NoError(t, pad.SetText("\n", nil))

	j := joinPad(t, ds, padID, "wireSeq")
	require.NotEmpty(t, j.author, "CLIENT_VARS.userId")
	assert.Equal(t, "\n", j.text, "collab_client_vars.initialAttributedText")

	ds.PadMessageHandler.HandleMessage(ws.UserChange{
		Event: "message",
		Data: ws.UserChangeData{Component: "pad", Type: "USER_CHANGES", Data: ws.UserChangeDataData{
			BaseRev:   j.rev,
			Changeset: "Z:1>5*0+5$hello",
			Apool:     ws.UserChangeDataDataApool{NumToAttrib: map[int][]string{0: {"author", j.author}}, NextNum: 1},
		}},
	}, j.client, ds.Settings, ds.Logger)

	var accept *frame
	for _, f := range frames(j.client) {
		require.Empty(t, f.Disconnect, "the change was refused")
		if f.Type == "ACCEPT_COMMIT" {
			f := f
			accept = &f
		}
	}
	require.NotNil(t, accept, "no ACCEPT_COMMIT")
	assert.EqualValues(t, j.rev+1, accept.Data["newRev"], "ACCEPT_COMMIT carries the bumped revision")

	pad, err = ds.PadManager.GetPad(padID, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "hello\n", pad.Text())
}

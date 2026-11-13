package ws

import (
	"encoding/json"
	"testing"

	"github.com/ether/etherpad-go/lib/models/ws"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Port of etherpad-lite's messages spec.
func TestMessagesParity(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(
		testutils.TestRunConfig{Name: "CHANGESET_REQ", Test: testChangesetReqParity},
		testutils.TestRunConfig{Name: "USER_CHANGES", Test: testUserChangesParity},
		testutils.TestRunConfig{Name: "CLIENT_READY: the system author is no missing author", Test: func(t *testing.T, ds testutils.TestDataStore) {
			// Text set without an author belongs to the reserved system author.
			text := "system text"
			_, err := ds.PadManager.GetPad("msgSystem", &text, nil)
			require.NoError(t, err)
			j := joinWithSettings(t, ds.PadMessageHandler, ds.Hub, padSettingsConfig(ds), ds, "msgSystem", "msgSys", nil)
			assert.Equal(t, "system text\n", collabText(j.vars))
		}},
	)
	testDb.StartTestDBHandler()
}

func testChangesetReqParity(t *testing.T, ds testutils.TestDataStore) {
	cfg := padSettingsConfig(ds)
	text := "dummy text"
	_, err := ds.PadManager.GetPad("msgPad", &text, nil)
	require.NoError(t, err)
	other := "other text"
	_, err = ds.PadManager.GetPad("msgPadOther", &other, nil)
	require.NoError(t, err)
	j := joinWithSettings(t, ds.PadMessageHandler, ds.Hub, cfg, ds, "msgPad", "msgReq", nil)

	request := func(t *testing.T, start string) map[string]any {
		t.Helper()
		var msg ws.ChangesetReq
		raw := `{"event":"message","data":{"component":"pad","type":"CHANGESET_REQ","padId":"msgPadOther",` +
			`"data":{"granularity":1,"start":` + start + `,"requestID":"requestId"}}}`
		require.NoError(t, json.Unmarshal([]byte(raw), &msg))
		ds.PadMessageHandler.HandleMessage(msg, j.client, cfg, ds.Logger)
		for _, f := range frames(j.client) {
			if f.Type == "CHANGESET_REQ" {
				return f.Data
			}
		}
		return nil
	}

	t.Run("changesets of another pad cannot be read", func(t *testing.T) {
		data := request(t, "0")
		require.NotNil(t, data)
		assert.Equal(t, "requestId", data["requestID"], "a string request id is echoed")
		fwd, _ := data["forwardsChangesets"].([]any)
		require.NotEmpty(t, fwd)
		assert.Regexp(t, `^[^$]*\$dummy text`, fwd[0])
	})
	t.Run("a revision that is no number is refused", func(t *testing.T) {
		assert.Nil(t, request(t, `"test123"`))
	})
	t.Run("a leading number is used", func(t *testing.T) {
		data := request(t, `"1test123"`)
		require.NotNil(t, data)
		assert.EqualValues(t, 0, data["start"], "start 1 is above head 0")
	})
	t.Run("a revision above head is the head", func(t *testing.T) {
		data := request(t, `"2"`)
		require.NotNil(t, data)
		assert.EqualValues(t, 0, data["start"])
	})
}

func testUserChangesParity(t *testing.T, ds testutils.TestDataStore) {
	cfg := padSettingsConfig(ds)
	j := joinWithSettings(t, ds.PadMessageHandler, ds.Hub, cfg, ds, "msgChanges", "msgChg", nil)
	pool := func(attribs ...[]string) ws.UserChangeDataDataApool {
		m := map[int][]string{}
		for i, a := range attribs {
			m[i] = a
		}
		return ws.UserChangeDataDataApool{NumToAttrib: m, NextNum: len(attribs)}
	}
	send := func(t *testing.T, cs string, p ws.UserChangeDataDataApool) (accepted bool, disconnect string, stored bool) {
		t.Helper()
		pad, err := ds.PadManager.GetPad("msgChanges", nil, nil)
		require.NoError(t, err)
		head := pad.Head
		ds.PadMessageHandler.HandleMessage(ws.UserChange{
			Event: "message",
			Data: ws.UserChangeData{Component: "pad", Type: "USER_CHANGES", Data: ws.UserChangeDataData{
				Apool: p, BaseRev: head, Changeset: cs,
			}},
		}, j.client, cfg, ds.Logger)
		for _, f := range frames(j.client) {
			if f.Type == "ACCEPT_COMMIT" {
				accepted = true
			}
			if f.Disconnect != "" {
				disconnect = f.Disconnect
			}
		}
		pad, err = ds.PadManager.GetPad("msgChanges", nil, nil)
		require.NoError(t, err)
		return accepted, disconnect, pad.Head > head
	}
	textOf := func() string {
		pad, err := ds.PadManager.GetPad("msgChanges", nil, nil)
		require.NoError(t, err)
		return pad.Text()
	}
	me := []string{"author", j.author}

	t.Run("changes are applied", func(t *testing.T) {
		accepted, _, stored := send(t, "Z:1>5*0+5$hello", pool(me))
		assert.True(t, accepted)
		assert.True(t, stored)
		assert.Equal(t, "hello\n", textOf())
	})
	t.Run("identity changeset is accepted, has no effect", func(t *testing.T) {
		accepted, disconnect, stored := send(t, "Z:6>0$", pool())
		assert.Empty(t, disconnect)
		assert.True(t, accepted)
		assert.False(t, stored)
	})
	t.Run("no net change is accepted, has no effect", func(t *testing.T) {
		accepted, disconnect, _ := send(t, "Z:6>0*0-1*0+1$h", pool(me))
		assert.Empty(t, disconnect)
		assert.True(t, accepted)
		assert.Equal(t, "hello\n", textOf())
	})
	for name, tc := range map[string]struct {
		cs   string
		pool ws.UserChangeDataDataApool
	}{
		"bad changeset":                        {"garbage", pool()},
		"insert without author":                {"Z:6>1+1$x", pool()},
		"insert claiming the system author":    {"Z:6>1*0+1$x", pool([]string{"author", "a.etherpad-system"})},
		"changeset stranding the trailing \\n": {"Z:6<1=5-1$", pool()},
		"insert impersonating another author":  {"Z:6>1*0+1$x", pool([]string{"author", "a.someoneElse00001"})},
	} {
		t.Run(name+" is rejected", func(t *testing.T) {
			_, disconnect, stored := send(t, tc.cs, tc.pool)
			assert.False(t, stored)
			assert.Equal(t, "hello\n", textOf())
			_ = disconnect
			// A rejected client is disconnected; join again for the next case.
			j = joinWithSettingsToken(t, ds.PadMessageHandler, ds.Hub, cfg, ds, "msgChanges", "msgChg"+name, "t.settingsmsgChg", nil)
			me = []string{"author", j.author}
		})
	}
	t.Run("retransmission is accepted, has no effect", func(t *testing.T) {
		pad, err := ds.PadManager.GetPad("msgChanges", nil, nil)
		require.NoError(t, err)
		base := pad.Head
		msg := ws.UserChange{Event: "message", Data: ws.UserChangeData{Component: "pad", Type: "USER_CHANGES",
			Data: ws.UserChangeDataData{Apool: pool(me), BaseRev: base, Changeset: "Z:6>1*0+1$!"}}}
		ds.PadMessageHandler.HandleMessage(msg, j.client, cfg, ds.Logger)
		ds.PadMessageHandler.HandleMessage(msg, j.client, cfg, ds.Logger)
		accepts := 0
		for _, f := range frames(j.client) {
			if f.Type == "ACCEPT_COMMIT" {
				accepts++
			}
		}
		assert.Equal(t, 2, accepts)
		pad, err = ds.PadManager.GetPad("msgChanges", nil, nil)
		require.NoError(t, err)
		assert.Equal(t, base+1, pad.Head, "stored once")
		assert.Equal(t, "!hello\n", pad.Text())
	})
}

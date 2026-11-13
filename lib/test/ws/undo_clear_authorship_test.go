package ws

import (
	"testing"

	"github.com/ether/etherpad-go/lib/models/ws"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Port of etherpad-lite's undo_clear_authorship spec (bug #2802).
func TestUndoClearAuthorship(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(testutils.TestRunConfig{Name: "cases", Test: testUndoClearAuthorship})
	testDb.StartTestDBHandler()
}

type changeResult struct {
	accepted   bool
	disconnect string
}

func sendChange(t *testing.T, ds testutils.TestDataStore, c settingsClient, padId, cs string, authors ...string) changeResult {
	t.Helper()
	pad, err := ds.PadManager.GetPad(padId, nil, nil)
	require.NoError(t, err)
	m := map[int][]string{}
	for i, a := range authors {
		m[i] = []string{"author", a}
	}
	ds.PadMessageHandler.HandleMessage(ws.UserChange{
		Event: "message",
		Data: ws.UserChangeData{Component: "pad", Type: "USER_CHANGES", Data: ws.UserChangeDataData{
			Apool: ws.UserChangeDataDataApool{NumToAttrib: m, NextNum: len(authors)}, BaseRev: pad.Head, Changeset: cs,
		}},
	}, c.client, padSettingsConfig(ds), ds.Logger)
	var r changeResult
	for _, f := range frames(c.client) {
		if f.Type == "ACCEPT_COMMIT" {
			r.accepted = true
		}
		if f.Disconnect != "" {
			r.disconnect = f.Disconnect
		}
	}
	return r
}

func testUndoClearAuthorship(t *testing.T, ds testutils.TestDataStore) {
	cfg := padSettingsConfig(ds)
	n := 0
	fresh := func(t *testing.T) string {
		n++
		padId := "undoClear" + string(rune('a'+n))
		empty := ""
		_, err := ds.PadManager.GetPad(padId, &empty, nil)
		require.NoError(t, err)
		return padId
	}
	join := func(t *testing.T, padId, name string) settingsClient {
		return joinWithSettingsToken(t, ds.PadMessageHandler, ds.Hub, cfg, ds, padId, padId+name, "t.undo"+padId+name, nil)
	}

	t.Run("undoing a clear with several authors is accepted", func(t *testing.T) {
		padId := fresh(t)
		a, b := join(t, padId, "A"), join(t, padId, "B")
		require.True(t, sendChange(t, ds, a, padId, "Z:1>5*0+5$hello", a.author).accepted)
		require.True(t, sendChange(t, ds, b, padId, "Z:6>6=5*0+6$ world", b.author).accepted)
		require.True(t, sendChange(t, ds, b, padId, "Z:c>0*0=b$", "").accepted, "clear authorship")
		r := sendChange(t, ds, b, padId, "Z:c>0*0=5*1=6$", a.author, b.author)
		assert.Empty(t, r.disconnect)
		assert.True(t, r.accepted)
	})
	t.Run("clearing with the empty author is allowed for anyone", func(t *testing.T) {
		padId := fresh(t)
		a := join(t, padId, "A")
		require.True(t, sendChange(t, ds, a, padId, "Z:1>5*0+5$hello", a.author).accepted)
		assert.True(t, sendChange(t, ds, a, padId, "Z:6>0*0=5$", "").accepted)
	})
	t.Run("restoring the own author after a clear is accepted", func(t *testing.T) {
		padId := fresh(t)
		a := join(t, padId, "A")
		require.True(t, sendChange(t, ds, a, padId, "Z:1>5*0+5$hello", a.author).accepted)
		require.True(t, sendChange(t, ds, a, padId, "Z:6>0*0=5$", "").accepted)
		assert.True(t, sendChange(t, ds, a, padId, "Z:6>0*0=5$", a.author).accepted)
	})
	t.Run("new text attributed to another author is rejected", func(t *testing.T) {
		padId := fresh(t)
		a, b := join(t, padId, "A"), join(t, padId, "B")
		r := sendChange(t, ds, b, padId, "Z:1>5*0+5$hello", a.author)
		assert.Equal(t, "badChangeset", r.disconnect)
	})
	t.Run("= with a fabricated author is rejected", func(t *testing.T) {
		padId := fresh(t)
		a := join(t, padId, "A")
		require.True(t, sendChange(t, ds, a, padId, "Z:1>5*0+5$hello", a.author).accepted)
		r := sendChange(t, ds, a, padId, "Z:6>0*0=5$", "a.fabricatedAuthorId")
		assert.Equal(t, "badChangeset", r.disconnect)
	})
	t.Run("- with a foreign author is rejected", func(t *testing.T) {
		padId := fresh(t)
		a := join(t, padId, "A")
		require.True(t, sendChange(t, ds, a, padId, "Z:1>5*0+5$hello", a.author).accepted)
		r := sendChange(t, ds, a, padId, "Z:6<1*0-1$", "a.fabricatedAuthorId")
		assert.Equal(t, "badChangeset", r.disconnect)
		pad, err := ds.PadManager.GetPad(padId, nil, nil)
		require.NoError(t, err)
		for _, a := range pad.Pool.NumToAttrib {
			assert.NotEqual(t, "a.fabricatedAuthorId", a.Value, "no pool injection")
		}
	})
}

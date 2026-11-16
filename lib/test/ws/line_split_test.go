package ws

import (
	"strings"
	"testing"

	"github.com/ether/etherpad-go/lib/changeset"
	"github.com/ether/etherpad-go/lib/models/ws"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// etherpad-lite #8495 (issue #8487): a multi-line keep or delete that ends
// mid-line (`|1=3` over "a\nb") passes the checks and applyToText, but
// browsers apply NEW_CHANGES line by line (mutateTextLines), which drops
// the partial line's characters: watching browsers show text out of order
// until they reload. The server stores and sends the line-split form.
func TestLineSplitChangesets(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(testutils.TestRunConfig{Name: "multi-line ops are split at line boundaries", Test: testLineSplitChangesets})
	defer testDb.StartTestDBHandler()
}

func testLineSplitChangesets(t *testing.T, ds testutils.TestDataStore) {
	cfg := padSettingsConfig(ds)
	padId := "lineSplit"
	for _, tc := range []struct{ name, key, cs, want string }{
		{"multi-line keep ending mid-line", "Keep", "Z:4>1|1=3*0+1$X", "a\nbX\n"},
		{"multi-line delete ending mid-line", "Delete", "Z:4<3|1-3$", "\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id := padId + tc.key
			j := joinWithSettings(t, ds.PadMessageHandler, ds.Hub, cfg, ds, id, "ls"+tc.key, nil)
			pool := ws.UserChangeDataDataApool{NumToAttrib: map[int][]string{0: {"author", j.author}}, NextNum: 1}
			send := func(cs string) {
				p, err := ds.PadManager.GetPad(id, nil, nil)
				require.NoError(t, err)
				ds.PadMessageHandler.HandleMessage(ws.UserChange{Event: "message", Data: ws.UserChangeData{
					Component: "pad", Type: "USER_CHANGES",
					Data: ws.UserChangeDataData{Apool: pool, BaseRev: p.Head, Changeset: cs},
				}}, j.client, cfg, ds.Logger)
			}
			send("Z:1>3*0|1+2*0+1$a\nb")
			p, err := ds.PadManager.GetPad(id, nil, nil)
			require.NoError(t, err)
			require.Equal(t, "a\nb\n", p.Text())
			prev := p.Text()

			send(tc.cs)
			p, err = ds.PadManager.GetPad(id, nil, nil)
			require.NoError(t, err)
			assert.Equal(t, tc.want, p.Text(), "the stored text")

			// What every browser does with the revision it gets.
			rev, err := p.GetRevision(p.Head)
			require.NoError(t, err)
			lines := changeset.SplitTextLines(prev)
			require.NoError(t, changeset.MutateTextLines(rev.Changeset, &lines))
			assert.Equal(t, tc.want, strings.Join(lines, ""), "a line-based client agrees, stored %s", rev.Changeset)

			// A retransmission of the original form is recognised against
			// the stored, line-split one and not applied twice.
			head := p.Head
			sendAt := func(cs string, base int) {
				ds.PadMessageHandler.HandleMessage(ws.UserChange{Event: "message", Data: ws.UserChangeData{
					Component: "pad", Type: "USER_CHANGES",
					Data: ws.UserChangeDataData{Apool: pool, BaseRev: base, Changeset: cs},
				}}, j.client, cfg, ds.Logger)
			}
			sendAt(tc.cs, head-1)
			p, err = ds.PadManager.GetPad(id, nil, nil)
			require.NoError(t, err)
			assert.Equal(t, head, p.Head, "a retransmission adds no revision")
			assert.Equal(t, tc.want, p.Text())
		})
	}
}

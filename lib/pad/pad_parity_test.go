package pad

import (
	"testing"
	"unicode/utf8"

	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/hooks"
	"github.com/ether/etherpad-go/lib/hooks/events"
	modelpad "github.com/ether/etherpad-go/lib/models/pad"
	"github.com/stretchr/testify/require"
)

// Cases of etherpad-lite's Pad and PadManager specs that had no Go test yet.

func TestCleanTextKeepsOtherCharacters(t *testing.T) {
	// Non-breaking spaces survive (etherpad-lite #3037). The Go port used to
	// replace the byte 0xA0, which is also the second byte of "à" and of many
	// other characters, and so produced invalid UTF-8.
	for _, in := range []string{
		"100 km\n",
		"a  b\n",
		"voilà, déjà vu\n",
		"丠〠à\n",
	} {
		got := *modelpad.CleanText(in)
		require.Equal(t, in, got)
		require.True(t, utf8.ValidString(got))
	}
}

func parityPad(t *testing.T, id string) *modelpad.Pad {
	t.Helper()
	hook := hooks.NewHook()
	m := NewManager(db.NewMemoryDataStore(), &hook)
	empty := ""
	p, err := m.GetPad(id, &empty, nil)
	require.NoError(t, err)
	return p
}

func TestNonBreakingSpaceRoundTrips(t *testing.T) {
	author := "a.test"

	t.Run("SpliceText", func(t *testing.T) {
		p := parityPad(t, "nbspSplice")
		require.NoError(t, p.SpliceText(0, 0, "100 km", &author))
		require.Equal(t, "100 km\n", p.Text())
	})

	t.Run("SetText", func(t *testing.T) {
		p := parityPad(t, "nbspSet")
		require.NoError(t, p.SetText("a b\n", &author))
		require.Equal(t, "a b\n", p.Text())
	})

	t.Run("SetText with accented text", func(t *testing.T) {
		p := parityPad(t, "accents")
		require.NoError(t, p.SetText("voilà\n", &author))
		require.Equal(t, "voilà\n", p.Text())
		require.NoError(t, p.Check())
	})
}

func TestSpliceTextWithoutAuthorUsesTheSystemAuthor(t *testing.T) {
	p := parityPad(t, "systemAuthor")
	empty := ""
	require.NoError(t, p.SpliceText(0, 0, "plugin-text", &empty))
	require.Equal(t, "plugin-text\n", p.Text())
	require.Contains(t, authorsInPool(p), modelpad.SystemAuthorId)
}

func TestPadDefaultContentHookGetsThePad(t *testing.T) {
	hook := hooks.NewHook()
	var got any
	hook.EnqueuePadDefaultContentHook(func(ctx *events.PadDefaultContentContext) { got = ctx.Pad })
	m := NewManager(db.NewMemoryDataStore(), &hook)
	p, err := m.GetPad("hookPad", nil, nil)
	require.NoError(t, err)
	require.Same(t, p, got)
}

func TestPadDefaultContentFromHookIsCleaned(t *testing.T) {
	hook := hooks.NewHook()
	hook.EnqueuePadDefaultContentHook(func(ctx *events.PadDefaultContentContext) {
		kind, content := "text", "foo\r\nbar\r\tbaz"
		ctx.Type, ctx.Content = &kind, &content
	})
	m := NewManager(db.NewMemoryDataStore(), &hook)
	p, err := m.GetPad("hookClean", nil, nil)
	require.NoError(t, err)
	require.Equal(t, "foo\nbar\n        baz\n", p.Text())
}

func TestIsValidPadId(t *testing.T) {
	hook := hooks.NewHook()
	m := NewManager(db.NewMemoryDataStore(), &hook)

	for _, id := range []string{
		"foo", "TF-EVC", "TF-LEC_IP03-EMS-CSM", "a.b", ".foo", "foo.", "a'b",
		"g.s8oes9dhwrvt0zif$bar", // group pad
	} {
		require.True(t, m.IsValidPadId(id), "%q is a valid pad id", id)
	}
	// "." and ".." are URL dot segments: the pad could never be opened.
	// Empty ids, a "$" outside the group prefix and ":" (GHSA-wg58-mhwv-35pq)
	// are invalid for new pads.
	for _, id := range []string{".", "..", "", "a$b", "a:b", "pad:x:revs:0"} {
		require.False(t, m.IsValidPadId(id), "%q is not a valid pad id", id)
		require.False(t, modelpad.IsValidPadId(id), "%q is not a valid pad id", id)
	}
}

package changeset

import (
	"fmt"
	"strings"
	"testing"

	"github.com/ether/etherpad-go/lib/test/testutils/general"
	"github.com/stretchr/testify/require"
)

// Randomised property tests in the spirit of etherpad-lite's easysync specs
// (easysync-compose, easysync-mutations). They exercise the changeset core
// that every edit, rebase and cluster catch-up goes through.

const propertyRounds = 200

func randomText() string {
	return general.RandomMultiline(20, 40) + "\n"
}

// toMultiByte maps some ASCII letters to multi-byte runes one to one, so a
// changeset built for ASCII text stays valid (lengths count runes) while the
// byte offsets no longer equal the rune offsets.
var toMultiByte = strings.NewReplacer("a", "ä", "b", "€", "c", "😀", "d", "ß", "e", "字")

func TestPropertyApplyToText(t *testing.T) {
	for i := 0; i < propertyRounds; i++ {
		text := randomText()
		cs, want := RandomTestChangeset(text, i%2 == 0)
		got, err := ApplyToText(cs, text)
		require.NoError(t, err, "cs=%q text=%q", cs, text)
		require.Equal(t, want, *got, "cs=%q text=%q", cs, text)
	}
}

func TestPropertyApplyToTextMultiByte(t *testing.T) {
	for i := 0; i < propertyRounds; i++ {
		text := randomText()
		cs, want := RandomTestChangeset(text, false)
		unpacked, err := Unpack(cs)
		require.NoError(t, err)
		mbText := toMultiByte.Replace(text)
		mbCs := Pack(unpacked.OldLen, unpacked.NewLen, unpacked.Ops, toMultiByte.Replace(unpacked.CharBank))

		got, err := ApplyToText(mbCs, mbText)
		require.NoError(t, err, "cs=%q text=%q", mbCs, mbText)
		require.Equal(t, toMultiByte.Replace(want), *got, "cs=%q text=%q", mbCs, mbText)

		// The line mutator must agree with ApplyToText.
		lines := splitKeepingNewlines(mbText)
		require.NoError(t, MutateTextLines(mbCs, &lines))
		require.Equal(t, *got, strings.Join(lines, ""))
	}
}

func TestPropertyCompose(t *testing.T) {
	pool := createPool([]string{"bold,", "bold,true", "italic,", "italic,true"})
	for i := 0; i < propertyRounds; i++ {
		x1 := randomText()
		cs1, x2 := RandomTestChangeset(x1, true)
		cs2, x3 := RandomTestChangeset(x2, true)
		composed, err := Compose(cs1, cs2, &pool)
		require.NoError(t, err, "cs1=%q cs2=%q", cs1, cs2)

		got, err := ApplyToText(*composed, x1)
		require.NoError(t, err)
		require.Equal(t, x3, *got, "cs1=%q cs2=%q composed=%q", cs1, cs2, *composed)
	}
}

// TestPropertyFollowConverges checks the property that live editing and
// the cluster rely on: two concurrent changes, each rebased over the other
// with Follow, lead to the same text in either order.
func TestPropertyFollowConverges(t *testing.T) {
	pool := createPool([]string{"bold,", "bold,true"})
	for i := 0; i < propertyRounds; i++ {
		text := randomText()
		a, textA := RandomTestChangeset(text, false)
		b, textB := RandomTestChangeset(text, false)

		bOverA, err := Follow(a, b, false, &pool)
		require.NoError(t, err, "a=%q b=%q", a, b)
		aOverB, err := Follow(b, a, true, &pool)
		require.NoError(t, err, "a=%q b=%q", a, b)

		viaA, err := ApplyToText(*bOverA, textA)
		require.NoError(t, err, "a=%q b=%q b'=%q", a, b, *bOverA)
		viaB, err := ApplyToText(*aOverB, textB)
		require.NoError(t, err, "a=%q b=%q a'=%q", a, b, *aOverB)
		require.Equal(t, viaA, viaB, "text=%q a=%q b=%q", text, a, b)

		// Composing either path must give the same result as applying it.
		ab, err := Compose(a, *bOverA, &pool)
		require.NoError(t, err)
		got, err := ApplyToText(*ab, text)
		require.NoError(t, err)
		require.Equal(t, *viaA, *got)
	}
}

func TestPropertyMakeSplice(t *testing.T) {
	for i := 0; i < propertyRounds; i++ {
		text := toMultiByte.Replace(randomText())
		runes := []rune(text)
		start := i % (len(runes) + 1)
		ndel := (i * 7) % (len(runes) - start + 1)
		ins := toMultiByte.Replace(general.RandomInlineString(i % 5))

		cs, err := MakeSplice(text, start, ndel, ins, nil, nil)
		require.NoError(t, err)
		got, err := ApplyToText(cs, text)
		require.NoError(t, err, "cs=%q", cs)
		want := string(runes[:start]) + ins + string(runes[start+ndel:])
		require.Equal(t, want, *got, fmt.Sprintf("start=%d ndel=%d ins=%q", start, ndel, ins))
	}
}

func splitKeepingNewlines(text string) []string {
	parts := strings.SplitAfter(text, "\n")
	if parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	return parts
}

func TestPropertySplitJoinAttributionLines(t *testing.T) {
	pool := createPool([]string{"apple,", "apple,true", "banana,", "banana,true"})
	for i := 0; i < propertyRounds; i++ {
		text, cs := multiByteChangeset(t, randomText(), true)
		atext := MakeAText(text, nil)
		next, err := ApplyToAText(cs, atext, pool)
		require.NoError(t, err, "cs=%q", cs)

		alines, err := SplitAttributionLines(next.Attribs, next.Text)
		require.NoError(t, err)
		require.Len(t, alines, strings.Count(next.Text, "\n"))
		for j, line := range splitKeepingNewlines(next.Text) {
			ops, err := DeserializeOps(alines[j])
			require.NoError(t, err)
			chars := 0
			for _, op := range *ops {
				chars += op.Chars
			}
			require.Equal(t, len([]rune(line)), chars, "line %d of %q", j, next.Text)
		}
		require.Equal(t, next.Attribs, JoinAttributionLines(alines))
	}
}

// multiByteChangeset builds a random changeset for the ASCII text and maps
// both to multi-byte characters (the generator counts bytes).
func multiByteChangeset(t *testing.T, text string, withAttribs bool) (string, string) {
	t.Helper()
	cs, _ := RandomTestChangeset(text, withAttribs)
	unpacked, err := Unpack(cs)
	require.NoError(t, err)
	return toMultiByte.Replace(text), Pack(unpacked.OldLen, unpacked.NewLen, unpacked.Ops, toMultiByte.Replace(unpacked.CharBank))
}

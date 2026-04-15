package changeset

import (
	"testing"

	"github.com/ether/etherpad-go/lib/apool"
)

// TestInverseRoundTripRev170Repro captures the first failure from the live
// diagnostic at revs 170/171: the inverse of a simple '=' + '+' changeset
// applied to a pad whose text contains 'ü' (multi-byte rune) produces a
// backward changeset that, when applied to the post-forward text, does NOT
// restore the pre-forward text — there is a one-rune transposition near a 'ü'.
//
// Forward : Z:1x>9=1w*0+9$smdüapsmd
// Backward: Z:26<9=1w-9$
// Pre-forward text (want): asdmasdümasüaüpsdmaüpsüapsdapsdpüasmdüpadüpasmdüpmdüpasdüpaassmmdmdmjg\n
//
// If this test fails, the bug is isolated to the changeset package (either
// Inverse or MutateTextLines handling rune-indexed positions on text that
// contains multi-byte characters).
func TestInverseRoundTripRev170Repro(t *testing.T) {
	const forward = "Z:1x>9=1w*0+9$smdüapsmd"
	const pre = "asdmasdümasüaüpsdmaüpsüapsdapsdpüasmdüpadüpasmdüpmdüpasdüpaassmmdmdmjg\n"

	preLines := []string{pre}
	// alines for a single-attribute-free line of the above length:
	// each line's alines entry is a simple "|1+<len>" op describing the line.
	// For this repro we use an empty-attribute run that covers the whole line.
	pool := apool.NewAPool()
	alines := []string{"|1+1x"} // |1 = one newline, +1x = 69 chars (including the \n)

	// 1) Generate the backward changeset from Inverse.
	backward, err := Inverse(forward, preLines, alines, &pool)
	if err != nil {
		t.Fatalf("Inverse failed: %v", err)
	}
	t.Logf("backward = %s", *backward)

	// 2) Apply forward to a copy of pre and get post-forward.
	postLines := append([]string(nil), preLines...)
	if err := MutateTextLines(forward, &postLines); err != nil {
		t.Fatalf("applying forward failed: %v", err)
	}
	if len(postLines) == 0 {
		t.Fatalf("post-forward lines empty")
	}
	t.Logf("post-forward = %q", postLines[0])

	// 3) Apply backward on top of post-forward and compare to pre.
	roundTrip := append([]string(nil), postLines...)
	if err := MutateTextLines(*backward, &roundTrip); err != nil {
		t.Fatalf("applying backward failed: %v", err)
	}

	if len(roundTrip) != len(preLines) {
		t.Fatalf("round-trip line count: got %d, want %d", len(roundTrip), len(preLines))
	}
	for i := range preLines {
		if roundTrip[i] != preLines[i] {
			t.Errorf("round-trip line %d mismatch:\n  got:  %q\n  want: %q", i, roundTrip[i], preLines[i])
		}
	}
}

package changeset

import (
	"testing"

	"github.com/ether/etherpad-go/lib/apool"
	"github.com/stretchr/testify/assert"
)

// Regression tests for panic sites in changeset.go. Malformed input that
// slips past CheckRep must surface as an error, never crash the server.

// Follow used to panic when the char bank of the first changeset was shorter
// than its '+' ops claimed (chars1.Skip failed inside the ApplyZip callback).
func TestFollow_ShortCharBankReturnsError(t *testing.T) {
	pool := apool.NewAPool()
	// "+5" claims five inserted chars but the bank only holds three.
	cs1 := "Z:0>5+5$abc"
	cs2 := "Z:0>1+1$x"

	assert.NotPanics(t, func() {
		_, err := Follow(cs1, cs2, false, &pool)
		assert.Error(t, err)
	})
}

// Inverse used to panic when an attribute line (alines) could not be
// deserialized (DeserializeOps error inside consumeAttribRuns).
func TestInverse_MalformedAlinesReturnsError(t *testing.T) {
	pool := apool.NewAPool()
	// Valid changeset removing one char, but the attribute line is garbage.
	cs := "Z:2<1-1$"
	lines := []string{"ab"}
	alines := []string{"!"}

	assert.NotPanics(t, func() {
		_, err := Inverse(cs, lines, alines, &pool)
		assert.Error(t, err)
	})
}

// Inverse used to panic when advancing to the next attribute line yielded a
// malformed alines entry (second DeserializeOps call inside consumeAttribRuns).
func TestInverse_MalformedSecondAlineReturnsError(t *testing.T) {
	pool := apool.NewAPool()
	// Remove three chars spanning two lines; second attribute line is garbage.
	cs := "Z:4<3|1-3$"
	lines := []string{"a\n", "b\n"}
	alines := []string{"|1+2", "!"}

	assert.NotPanics(t, func() {
		_, err := Inverse(cs, lines, alines, &pool)
		assert.Error(t, err)
	})
}

// Compose used to panic when SlicerZipperFunc rejected inconsistent ops
// (here: a keep op that claims more lines than chars).
func TestCompose_InvalidOpsReturnError(t *testing.T) {
	pool := apool.NewAPool()
	cs1 := "Z:5>0|2=1$"
	cs2 := "Z:5>0=1$"

	assert.NotPanics(t, func() {
		_, err := Compose(cs1, cs2, &pool)
		assert.Error(t, err)
	})
}

// Compose ignored Unpack errors entirely; garbage input must return an error.
func TestCompose_GarbageInputReturnsError(t *testing.T) {
	pool := apool.NewAPool()

	assert.NotPanics(t, func() {
		_, err := Compose("garbage", "Z:0>0$", &pool)
		assert.Error(t, err)
	})
}

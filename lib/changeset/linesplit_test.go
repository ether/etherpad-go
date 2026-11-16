package changeset

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func applyByLines(t *testing.T, cs, text string) string {
	t.Helper()
	lines := SplitTextLines(text)
	require.NoError(t, MutateTextLines(cs, &lines))
	return strings.Join(lines, "")
}

func TestSplitOpsAtLineBoundaries(t *testing.T) {
	for _, tc := range []struct{ name, cs, text, want string }{
		{"keep ending mid-line", "Z:4>1|1=3*0+1$X", "a\nb\n", "a\nbX\n"},
		{"delete ending mid-line", "Z:4<3|1-3$", "a\nb\n", "\n"},
		{"attributed keep", "Z:4>0*0|1=3|1=1$", "a\nb\n", "a\nb\n"},
		{"multi-byte text", "Z:6>1|1=5*0+1$!", "äö\nü😀\n", "äö\nü😀!\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := SplitOpsAtLineBoundaries(tc.cs, tc.text)
			require.NoError(t, err)
			applied, err := ApplyToText(got, tc.text)
			require.NoError(t, err)
			assert.Equal(t, tc.want, *applied, "same text as before")
			assert.Equal(t, tc.want, applyByLines(t, got, tc.text), "and line by line, as browsers apply it: %s", got)
			assert.True(t, EqualIgnoringLineSplits(tc.cs, got))
			if tc.name == "attributed keep" {
				assert.Equal(t, "Z:4>0*0|1=2*0=1$", got, "the attributes stay on both parts; the plain keep at the end goes")
			}
		})
	}
}

func TestSplitOpsAtLineBoundariesKeepsCanonical(t *testing.T) {
	for _, cs := range []string{"Z:4>1|1=2=1*0+1$X", "Z:4>1*0+1$X", "Z:4<2|1-2$"} {
		got, err := SplitOpsAtLineBoundaries(cs, "a\nb\n")
		require.NoError(t, err)
		assert.Equal(t, cs, got)
	}
	_, err := SplitOpsAtLineBoundaries("Z:9>0$", "a\nb\n")
	assert.Error(t, err, "a changeset for other text")
}

func TestEqualIgnoringLineSplits(t *testing.T) {
	assert.True(t, EqualIgnoringLineSplits("Z:4>1|1=3*0+1$X", "Z:4>1|1=2=1*0+1$X"))
	assert.False(t, EqualIgnoringLineSplits("Z:4>1|1=3*0+1$X", "Z:4>1|1=3*0+1$Y"), "other text")
	assert.False(t, EqualIgnoringLineSplits("Z:4>1|1=3*0+1$X", "Z:4>1|1=3*1+1$X"), "other attributes")
	assert.False(t, EqualIgnoringLineSplits("Z:4>1|1=3*0+1$X", "Z:4>1=1*0+1|1=2$X"), "inserted elsewhere")
	assert.True(t, EqualIgnoringLineSplits("Z:4>1*0+1|1=4$X", "Z:4>1*0+1$X"), "a plain keep at the end changes nothing")
	assert.False(t, EqualIgnoringLineSplits("Z:4>0*0|1=4$", "Z:4>0$"), "an attributed keep does")
	assert.False(t, EqualIgnoringLineSplits("garbage", "Z:4>0$"))
}

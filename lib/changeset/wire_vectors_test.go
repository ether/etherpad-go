package changeset

import (
	"encoding/json"
	"os"
	"regexp"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Port of etherpad-lite's downstream/wire-vectors spec. testdata/wire-
// vectors.json is etherpad-lite's committed fixture
// (src/tests/fixtures/wire-vectors.json) that other clients (the Rust
// terminal editor, the Node CLI) are checked against: the Go engine must
// read and apply each changeset exactly as etherpad-lite does.
func TestWireVectors(t *testing.T) {
	b, err := os.ReadFile("testdata/wire-vectors.json")
	require.NoError(t, err)
	var vectors []struct {
		Name        string `json:"name"`
		InitialText string `json:"initialText"`
		Changeset   string `json:"changeset"`
		Pool        struct {
			NumToAttrib map[string][]string `json:"numToAttrib"`
			NextNum     int                 `json:"nextNum"`
		} `json:"pool"`
		ResultText string `json:"resultText"`
	}
	require.NoError(t, json.Unmarshal(b, &vectors))
	require.NotEmpty(t, vectors)
	for _, v := range vectors {
		t.Run(v.Name, func(t *testing.T) {
			_, err := CheckRep(v.Changeset)
			require.NoError(t, err)
			got, err := ApplyToText(v.Changeset, v.InitialText)
			require.NoError(t, err)
			assert.Equal(t, v.ResultText, *got)
			// Every attribute the changeset names is in its pool.
			ops, err := Unpack(v.Changeset)
			require.NoError(t, err)
			for _, m := range regexp.MustCompile(`\*([0-9a-z]+)`).FindAllStringSubmatch(ops.Ops, -1) {
				n, err := strconv.ParseInt(m[1], 36, 64)
				require.NoError(t, err)
				_, ok := v.Pool.NumToAttrib[strconv.FormatInt(n, 10)]
				assert.True(t, ok, "attribute %s not in the pool", m[1])
			}
			assert.Equal(t, len(v.Pool.NumToAttrib), v.Pool.NextNum)
		})
	}
}

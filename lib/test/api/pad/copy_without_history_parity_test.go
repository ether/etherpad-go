package pad

import (
	"fmt"
	"sort"
	"testing"

	"github.com/ether/etherpad-go/lib/api/pad"
	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/changeset"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/ether/etherpad-go/lib/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Port of etherpad-lite's copyPadWithoutHistoryIntegrity spec. The copy
// used to be made from the plain text: formatting and authors were lost.
func TestCopyPadWithoutHistoryIntegrity(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(testutils.TestRunConfig{Name: "copy", Test: testCopyWithoutHistoryIntegrity})
	testDb.StartTestDBHandler()
}

func copyWithoutHistory(t *testing.T, ds testutils.TestDataStore, src, dst string) {
	t.Helper()
	status, body := postPadOperation(t, ds, "/admin/api/pads/"+src+"/copyWithoutHistory", dst, false)
	require.Equal(t, 200, status, string(body))
	ds.PadManager.UnloadPad(dst)
}

func testCopyWithoutHistoryIntegrity(t *testing.T, ds testutils.TestDataStore) {
	pad.Init(ds.ToInitStore())
	// Two authors, one bold word.
	a1, err := ds.AuthorManager.CreateAuthor(nil)
	require.NoError(t, err)
	a2, err := ds.AuthorManager.CreateAuthor(nil)
	require.NoError(t, err)
	text := "line 0"
	src, err := ds.PadManager.GetPad("copySrc", &text, &a1.Id)
	require.NoError(t, err)
	for i := 1; i < 6; i++ {
		author := a1.Id
		if i%2 == 0 {
			author = a2.Id
		}
		require.NoError(t, src.SpliceText(utils.RuneCount(src.Text())-1, 0, fmt.Sprintf("\nline %d", i), &author))
	}
	bold := src.Pool.PutAttrib(apool.Attribute{Key: "bold", Value: "true"}, nil)
	authorNum := src.Pool.PutAttrib(apool.Attribute{Key: "author", Value: a1.Id}, nil)
	_, err = src.AppendRevision(fmt.Sprintf("Z:%s>0*%s*%s=4$", utils.NumToString(utils.RuneCount(src.Text())), utils.NumToString(authorNum), utils.NumToString(bold)), &a1.Id)
	require.NoError(t, err)
	require.NoError(t, src.Check())
	srcText := src.Text()
	srcAuthors := src.GetAllAuthors()
	sort.Strings(srcAuthors)

	copyWithoutHistory(t, ds, "copySrc", "copyDst")
	dst, err := ds.PadManager.GetPad("copyDst", nil, nil)
	require.NoError(t, err)

	t.Run("the text equals the source exactly", func(t *testing.T) {
		assert.Equal(t, srcText, dst.Text())
	})
	t.Run("revision 1 is well-formed", func(t *testing.T) {
		rev, err := ds.DS.GetRevision("copyDst", 1)
		require.NoError(t, err)
		_, err = changeset.CheckRep(rev.Changeset)
		require.NoError(t, err)
		unpacked, err := changeset.Unpack(rev.Changeset)
		require.NoError(t, err)
		assert.Equal(t, utils.RuneCount(srcText), unpacked.NewLen)
	})
	t.Run("the copy passes the pad check", func(t *testing.T) {
		require.NoError(t, dst.Check())
	})
	t.Run("authors are kept", func(t *testing.T) {
		got := dst.GetAllAuthors()
		sort.Strings(got)
		assert.Equal(t, srcAuthors, got)
	})
	t.Run("formatting is kept", func(t *testing.T) {
		ops, err := changeset.DeserializeOps(dst.AText.Attribs)
		require.NoError(t, err)
		m := changeset.FromString((*ops)[0].Attribs, &dst.Pool)
		require.NotNil(t, m.Get("bold"))
		assert.Equal(t, "true", *m.Get("bold"))
	})
	t.Run("no drift when copied repeatedly", func(t *testing.T) {
		current := "copyDst"
		for i := 0; i < 3; i++ {
			next := fmt.Sprintf("copyDrift%d", i)
			copyWithoutHistory(t, ds, current, next)
			current = next
		}
		final, err := ds.PadManager.GetPad(current, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, srcText, final.Text())
		require.NoError(t, final.Check())
	})
}

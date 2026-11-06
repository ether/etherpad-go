package pad

import (
	"testing"

	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/require"
)

func TestPadCheckNonASCII(t *testing.T) {
	testutils.RunSequentialMemorySQLite(t, testutils.TestRunConfig{Name: "Check accepts a valid history with non-ASCII text", Test: func(t *testing.T, ds testutils.TestDataStore) {
		a, _ := ds.AuthorManager.CreateAuthor(nil)
		txt := "Grüße 😀"
		p, err := ds.PadManager.GetPad("probeCheck", &txt, &a.Id)
		require.NoError(t, err)
		require.NoError(t, p.Check(), "after create")
		require.NoError(t, p.SetText("Grüße 😀\nZweite Zeile äöü\n", &a.Id))
		require.NoError(t, p.Check(), "after setText")
		require.NoError(t, p.SpliceText(len([]rune(p.Text()))-1, 0, "noch mehr € text", &a.Id))
		require.NoError(t, p.Check(), "after append")
		require.NoError(t, p.SpliceText(3, 2, "ß", &a.Id))
		require.NoError(t, p.Check(), "after splice")
	}})
}

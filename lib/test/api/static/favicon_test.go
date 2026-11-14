package static

import (
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	staticapi "github.com/ether/etherpad-go/lib/api/static"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Port of etherpad-lite's favicon spec.
func TestFavicon(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(testutils.TestRunConfig{Name: "favicon", Test: testFavicon})
	defer testDb.StartTestDBHandler()
}

func testFavicon(t *testing.T, ds testutils.TestDataStore) {
	store := ds.ToInitStore()
	staticapi.Init(store)
	custom, err := os.ReadFile("testdata/favicon-custom.png")
	require.NoError(t, err)
	def, err := os.ReadFile("../../testutils/assets/images/favicon.ico")
	require.NoError(t, err)
	wd, _ := os.Getwd()
	get := func() (int, []byte, string) {
		resp, err := store.C.Test(httptest.NewRequest("GET", "/favicon.ico", nil))
		require.NoError(t, err)
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, b, resp.Header.Get("Location")
	}
	str := func(s string) *string { return &s }

	ds.Settings.Root = wd
	ds.Settings.Favicon = str("testdata/favicon-custom.png")
	status, body, _ := get()
	assert.Equal(t, 200, status, "relative path")
	assert.Equal(t, custom, body)

	ds.Settings.Favicon = str("https://etherpad.org/favicon.ico")
	status, _, location := get()
	assert.Equal(t, 302, status, "a URL")
	assert.Equal(t, "https://etherpad.org/favicon.ico", location)

	ds.Settings.Favicon = str(filepath.Join(wd, "testdata", "favicon-custom.png"))
	status, body, _ = get()
	assert.Equal(t, 200, status, "absolute path")
	assert.Equal(t, custom, body)

	ds.Settings.Favicon = str("favicon.ico")
	status, body, _ = get()
	assert.Equal(t, 200, status, "a missing file falls back")
	assert.Equal(t, def, body)

	ds.Settings.Favicon = nil
	ds.Settings.SkinName = "testskin"
	status, body, _ = get()
	assert.Equal(t, 200, status, "the skin's favicon")
	assert.Equal(t, "skin-icon", string(body))

	ds.Settings.SkinName = "colibris"
	status, body, _ = get()
	assert.Equal(t, 200, status, "default")
	assert.Equal(t, def, body)

	ds.Settings.SkinName = "../../etc"
	status, body, _ = get()
	assert.Equal(t, 200, status, "a skin name is no path")
	assert.Equal(t, def, body)
}

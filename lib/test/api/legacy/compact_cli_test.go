package legacy

import (
	"bytes"
	"net"
	"testing"

	"github.com/ether/etherpad-go/lib/compact"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// "etherpad-go compact" against a running server, through its HTTP API.
func TestCompactCommand(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(testutils.TestRunConfig{Name: "compacts pads through the HTTP API", Test: testCompactCommand})
	testDb.StartTestDBHandler()
}

func testCompactCommand(t *testing.T, ds testutils.TestDataStore) {
	ds.Settings.Cleanup.Enabled = true // compacting needs it, as in etherpad-lite
	c := newClient(t, ds)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() { _ = c.app.Listener(ln) }()
	t.Cleanup(func() { _ = c.app.Shutdown() })
	base := "http://" + ln.Addr().String()

	revisions := func(pad string) int {
		r := c.get("getRevisionsCount", map[string]string{"padID": pad})
		require.Equal(t, 0, r.Code, r.Message)
		return int(r.Data.(map[string]any)["revisions"].(float64))
	}
	for _, pad := range []string{"cliCompactA", "cliCompactB"} {
		require.Equal(t, 0, c.get("createPad", map[string]string{"padID": pad, "text": "start"}).Code)
		for _, text := range []string{"one", "two", "three", "four"} {
			require.Equal(t, 0, c.get("appendText", map[string]string{"padID": pad, "text": " " + text}).Code)
		}
		require.Greater(t, revisions(pad), 3)
	}
	textOf := func(pad string) string {
		return c.get("getText", map[string]string{"padID": pad}).Data.(map[string]any)["text"].(string)
	}
	textA := textOf("cliCompactA")

	main := func(args ...string) (int, string, string) {
		var out, errOut bytes.Buffer
		code := compact.Main(args, &out, &errOut, &compact.HTTPAPI{BaseURL: base, APIKey: c.key})
		return code, out.String(), errOut.String()
	}

	code, out, errOut := main("--all", "--dry-run")
	assert.Equal(t, 0, code, errOut)
	assert.Contains(t, out, "would compact")
	assert.Greater(t, revisions("cliCompactA"), 3, "a dry run changes nothing")

	code, out, errOut = main("cliCompactA")
	assert.Equal(t, 0, code, errOut)
	assert.Contains(t, out, "cliCompactA:")
	assert.Equal(t, 0, revisions("cliCompactA"), "all history collapsed")
	assert.Equal(t, textA, textOf("cliCompactA"), "the text stays")
	assert.Greater(t, revisions("cliCompactB"), 3, "only the named pad")

	code, _, errOut = main("cliCompactB", "--keep", "2")
	assert.Equal(t, 0, code, errOut)
	assert.LessOrEqual(t, revisions("cliCompactB"), 2)

	// Nothing was edited for a day: --older-than 1 finds no stale pad.
	code, out, errOut = main("--older-than", "1")
	assert.Equal(t, 0, code, errOut)
	assert.Contains(t, out, "No stale pads")

	wrong := compact.Main([]string{"cliCompactA"}, &bytes.Buffer{}, &bytes.Buffer{}, &compact.HTTPAPI{BaseURL: base, APIKey: "wrong"})
	assert.Equal(t, 1, wrong, "a wrong API key fails the run")

	code, _, errOut = main("noSuchPad")
	assert.Equal(t, 1, code)
	assert.Contains(t, errOut, "noSuchPad")
}

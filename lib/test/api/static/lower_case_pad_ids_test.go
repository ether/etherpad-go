package static

import (
	"testing"

	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/assert"
)

// etherpad-lite lowerCasePadIds and pads-with-spaces (#4883), the HTTP part.
func TestPadURLLowerCaseAndSpaces(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(
		testutils.TestRunConfig{Name: "lowerCasePadIds off: served as is", Test: func(t *testing.T, ds testutils.TestDataStore) {
			ds.Settings.LowerCasePadIDs = false
			// Served, not redirected (the page itself needs the UI build).
			status, location := get(t, ds, "/p/UPPERCASEpad")
			assert.NotEqual(t, 302, status)
			assert.NotEqual(t, 404, status)
			assert.Empty(t, location)
		}},
		testutils.TestRunConfig{Name: "lowerCasePadIds on: redirects to the lower-case id", Test: func(t *testing.T, ds testutils.TestDataStore) {
			ds.Settings.LowerCasePadIDs = true
			status, location := get(t, ds, "/p/UPPERCASEpad")
			assert.Equal(t, 302, status)
			assert.Equal(t, "uppercasepad", location)
		}},
		testutils.TestRunConfig{Name: "lowerCasePadIds on: an existing upper-case pad is served", Test: func(t *testing.T, ds testutils.TestDataStore) {
			text := "oldpad"
			_, err := ds.PadManager.GetPad("ALREADYexistingPad", &text, nil)
			assert.NoError(t, err)
			ds.Settings.LowerCasePadIDs = true
			status, location := get(t, ds, "/p/ALREADYexistingPad")
			assert.NotEqual(t, 302, status)
			assert.Empty(t, location)
		}},
		testutils.TestRunConfig{Name: "pads with spaces", Test: func(t *testing.T, ds testutils.TestDataStore) {
			status, location := get(t, ds, "/p/pads%20with%20spaces")
			assert.Equal(t, 302, status)
			assert.Equal(t, "pads_with_spaces", location)
			status, location = get(t, ds, "/p/pads%20with%20spaces?showChat=true&noColors=false")
			assert.Equal(t, 302, status)
			assert.Equal(t, "pads_with_spaces?showChat=true&noColors=false", location)
		}},
	)
	testDb.StartTestDBHandler()
}

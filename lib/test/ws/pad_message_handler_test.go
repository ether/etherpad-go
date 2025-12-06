package ws

import (
	"testing"

	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/assert"
)

func TestPadMessageHandler_AllMethods(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)

	testDb.AddTests(testutils.TestRunConfig{
		Name: "PadMessageHandler methods scaffold",
		Test: testInitPadMessageHandler,
	})
	testDb.StartTestDBHandler()
}

func testInitPadMessageHandler(t *testing.T, ds testutils.TestDataStore) {
	handler := ds.PadMessageHandler

	assert.NotNil(t, handler)
}

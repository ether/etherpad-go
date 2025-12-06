package ws

import (
	"testing"

	"github.com/ether/etherpad-go/lib/hooks"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/ether/etherpad-go/lib/ws"
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
	initializedHook := hooks.NewHook()
	handler := ws.NewPadMessageHandler(ds.DS, &initializedHook, ds.PadManager)

	assert.NotNil(t, handler)
}

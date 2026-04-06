package static

import (
	"io"
	"net/http/httptest"
	"testing"

	staticapi "github.com/ether/etherpad-go/lib/api/static"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/assert"
)

func TestStaticQR(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)

	testDb.AddTests(
		testutils.TestRunConfig{
			Name: "Pad QR endpoint returns PNG",
			Test: testPadQrEndpointReturnsPng,
		},
		testutils.TestRunConfig{
			Name: "Pad QR endpoint keeps readonly target on readonly routes",
			Test: testPadQrEndpointReadonlyVariants,
		},
		testutils.TestRunConfig{
			Name: "Pad QR endpoint returns 404 for missing pads",
			Test: testPadQrEndpointMissingPad,
		},
	)

	defer testDb.StartTestDBHandler()
}

func createStaticTestPad(t *testing.T, tsStore testutils.TestDataStore, padID string, text string) {
	t.Helper()
	_, err := tsStore.PadManager.GetPad(padID, &text, nil)
	assert.NoError(t, err)
}

func testPadQrEndpointReturnsPng(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	staticapi.Init(initStore)

	createStaticTestPad(t, tsStore, "qrpad", "QR endpoint test\n")

	req := httptest.NewRequest("GET", "/p/qrpad/qr", nil)
	resp, err := initStore.C.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "image/png", resp.Header.Get("Content-Type"))
	assert.Equal(t, "no-store", resp.Header.Get("Cache-Control"))

	body, _ := io.ReadAll(resp.Body)
	assert.Greater(t, len(body), 8)
	assert.Equal(t, []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}, body[:8])
}

func testPadQrEndpointReadonlyVariants(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	staticapi.Init(initStore)

	createStaticTestPad(t, tsStore, "qrreadonlypad", "QR readonly test\n")
	readOnlyID := initStore.ReadOnlyManager.GetReadOnlyId("qrreadonlypad")

	readwriteReq := httptest.NewRequest("GET", "/p/qrreadonlypad/qr", nil)
	readwriteResp, err := initStore.C.Test(readwriteReq)
	assert.NoError(t, err)
	assert.Equal(t, 200, readwriteResp.StatusCode)
	readwriteBody, _ := io.ReadAll(readwriteResp.Body)

	readonlyReq := httptest.NewRequest("GET", "/p/qrreadonlypad/qr?readonly=true", nil)
	readonlyResp, err := initStore.C.Test(readonlyReq)
	assert.NoError(t, err)
	assert.Equal(t, 200, readonlyResp.StatusCode)
	readonlyBody, _ := io.ReadAll(readonlyResp.Body)

	readonlyRouteReq := httptest.NewRequest("GET", "/p/"+readOnlyID+"/qr", nil)
	readonlyRouteResp, err := initStore.C.Test(readonlyRouteReq)
	assert.NoError(t, err)
	assert.Equal(t, 200, readonlyRouteResp.StatusCode)
	readonlyRouteBody, _ := io.ReadAll(readonlyRouteResp.Body)

	assert.NotEqual(t, readwriteBody, readonlyBody)
	assert.Equal(t, readonlyBody, readonlyRouteBody)
}

func testPadQrEndpointMissingPad(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	staticapi.Init(initStore)

	req := httptest.NewRequest("GET", "/p/missingpad/qr", nil)
	resp, err := initStore.C.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, 404, resp.StatusCode)
}

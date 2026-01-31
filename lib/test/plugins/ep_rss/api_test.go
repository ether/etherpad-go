package ep_rss

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ether/etherpad-go/lib/plugins/ep_rss"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/assert"
)

func TestEPRSS(t *testing.T) {
	testHandler := testutils.NewTestDBHandler(t)
	defer testHandler.StartTestDBHandler()

	testHandler.AddTests(testutils.TestRunConfig{
		Name: "ep_rss plugin tests",
		Test: testEpRSSFeed,
	},
		testutils.TestRunConfig{
			Name: "ep_rss feed build tests",
			Test: testFeedbuildsNewRSS,
		},
	)
}

func testEpRSSFeed(t *testing.T, th testutils.TestDataStore) {
	ep_rss.RegisterFeedRoutes(th.App, th.PadManager, th.Logger)

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{"rss redirect", "/p/test-pad/rss", "/p/test-pad/feed"},
		{"feed.rss redirect", "/p/test-pad/feed.rss", "/p/test-pad/feed"},
		{"atom.xml redirect", "/p/test-pad/atom.xml", "/p/test-pad/feed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			resp, err := th.App.Test(req)

			assert.NoError(t, err)
			assert.Equal(t, http.StatusMovedPermanently, resp.StatusCode)

			location := resp.Header.Get("Location")
			assert.Equal(t, tt.expected, location)
		})
	}
}

func testFeedbuildsNewRSS(t *testing.T, th testutils.TestDataStore) {
	ep_rss.RegisterFeedRoutes(th.App, th.PadManager, th.Logger)
	req := httptest.NewRequest(http.MethodGet, "/p/test/feed", nil)
	var helloWorld = "Hello\nWorld"
	_, err := th.PadManager.GetPad("test", &helloWorld, nil)
	if err != nil {
		t.Fatalf("Error creating pad: %v", err)
	}
	resp, err := th.App.Test(req)
	if err != nil {
		t.Fatalf("Error making request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200 OK, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)
	assert.Contains(t, bodyStr, `<?xml version="1.0" encoding="UTF-8"?>`)
	assert.Contains(t, bodyStr, `<title>test</title>`)
	assert.Contains(t, bodyStr, `<link>http://example.com/p/test</link>`)
	assert.Contains(t, bodyStr, `Hello<br/>World`)
	assert.Contains(t, bodyStr, `<![CDATA[`)
	assert.Equal(t, "application/rss+xml; charset=utf-8", resp.Header.Get("Content-Type"))
}

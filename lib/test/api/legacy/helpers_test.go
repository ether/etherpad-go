package legacy

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/ether/etherpad-go/lib/api/legacy"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

const testAPIKey = "test-api-key-0123456789abcdef0123"

// client calls the legacy API of one test server.
type client struct {
	t       *testing.T
	app     *fiber.App
	version string
	key     string
}

func newClient(t *testing.T, ds testutils.TestDataStore) *client {
	t.Helper()
	store := ds.ToInitStore()
	legacy.New(store, testAPIKey, nil).Register(store.C)
	return &client{t: t, app: store.C, version: legacy.LatestVersion, key: testAPIKey}
}

func (c *client) do(req *http.Request) (int, legacy.Response, string) {
	c.t.Helper()
	resp, err := c.app.Test(req, fiber.TestConfig{Timeout: 0})
	require.NoError(c.t, err)
	b, _ := io.ReadAll(resp.Body)
	var r legacy.Response
	if strings.Contains(resp.Header.Get("Content-Type"), "json") {
		require.NoError(c.t, json.Unmarshal(b, &r), string(b))
	}
	return resp.StatusCode, r, string(b)
}

func (c *client) path(fn string) string { return "/api/" + c.version + "/" + fn }

// get calls fn with the parameters in the query string.
func (c *client) get(fn string, params map[string]string) legacy.Response {
	c.t.Helper()
	_, r, _ := c.getStatus(fn, params)
	return r
}

func (c *client) getStatus(fn string, params map[string]string) (int, legacy.Response, string) {
	c.t.Helper()
	q := url.Values{}
	for k, v := range params {
		q.Set(k, v)
	}
	if c.key != "" {
		q.Set("apikey", c.key)
	}
	return c.do(httptest.NewRequest("GET", c.path(fn)+"?"+q.Encode(), nil))
}

// postJSON calls fn with a JSON body.
func (c *client) postJSON(fn string, body map[string]any) legacy.Response {
	c.t.Helper()
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", c.path(fn)+"?apikey="+url.QueryEscape(c.key), bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	_, r, _ := c.do(req)
	return r
}

// postForm calls fn with a multipart form; query holds more parameters.
func (c *client) postForm(fn string, query, form map[string]string) legacy.Response {
	c.t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range form {
		require.NoError(c.t, w.WriteField(k, v))
	}
	require.NoError(c.t, w.Close())
	q := url.Values{"apikey": {c.key}}
	for k, v := range query {
		q.Set(k, v)
	}
	req := httptest.NewRequest("POST", c.path(fn)+"?"+q.Encode(), &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	_, r, _ := c.do(req)
	return r
}

// postURLEncoded calls fn with an application/x-www-form-urlencoded body.
func (c *client) postURLEncoded(fn string, form map[string]string) legacy.Response {
	c.t.Helper()
	v := url.Values{"apikey": {c.key}}
	for k, val := range form {
		v.Set(k, val)
	}
	req := httptest.NewRequest("POST", c.path(fn), strings.NewReader(v.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	_, r, _ := c.do(req)
	return r
}

func ok(t *testing.T, r legacy.Response) map[string]any {
	t.Helper()
	require.Equal(t, 0, r.Code, "message: %s", r.Message)
	m, _ := r.Data.(map[string]any)
	return m
}

func failed(t *testing.T, r legacy.Response, message string) {
	t.Helper()
	require.Equal(t, 1, r.Code, "expected an error, got data %v", r.Data)
	if message != "" {
		require.Equal(t, message, r.Message)
	}
}

func jsonDecode(r io.Reader, v any) error { return json.NewDecoder(r).Decode(v) }

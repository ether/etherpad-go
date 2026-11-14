package tokentransfer

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Port of etherpad-lite's tokenTransfer spec.

type fixture struct {
	app   *fiber.App
	store *db.MemoryDataStore
	now   time.Time
}

func newFixture(prefix string) *fixture {
	f := &fixture{store: db.NewMemoryDataStore(), now: time.Unix(1_700_000_000, 0)}
	s := &settings.Settings{}
	s.Cookie.Prefix = prefix
	h := &Handler{Store: f.store, Settings: s, Now: func() time.Time { return f.now }}
	f.app = fiber.New()
	Register(f.app, h)
	return f
}

func (f *fixture) post(t *testing.T, cookie string, body string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest("POST", "/tokenTransfer", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	resp, err := f.app.Test(req)
	require.NoError(t, err)
	b, _ := io.ReadAll(resp.Body)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	return resp.StatusCode, m
}

func (f *fixture) get(t *testing.T, id string) (*http.Response, map[string]any) {
	t.Helper()
	return f.redeem(t, id, map[string]string{"Sec-Fetch-Site": "same-origin"}, "POST")
}

// redeem sends method /tokenTransfer/id with the given headers.
func (f *fixture) redeem(t *testing.T, id string, headers map[string]string, method string) (*http.Response, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(method, "/tokenTransfer/"+id, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := f.app.Test(req)
	require.NoError(t, err)
	b, _ := io.ReadAll(resp.Body)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	return resp, m
}

const token = "t.abc123def456ghi789jk"

func TestTransferHappyPath(t *testing.T) {
	f := newFixture("ep_")
	code, m := f.post(t, "ep_token="+token, `{"prefs":"{\"theme\":\"dark\"}"}`)
	require.Equal(t, 200, code)
	id, _ := m["id"].(string)
	require.Regexp(t, `^[0-9a-f]{32}$`, id)
	assert.NotContains(t, m, "token")

	resp, body := f.get(t, id)
	require.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, true, body["ok"])
	assert.Equal(t, `{"theme":"dark"}`, body["prefs"])
	assert.NotContains(t, body, "token", "the token travels only in the cookie")
	var cookie string
	for _, c := range resp.Header.Values("Set-Cookie") {
		if strings.HasPrefix(c, "ep_token=") {
			cookie = c
		}
	}
	require.NotEmpty(t, cookie)
	assert.Contains(t, cookie, "ep_token="+token)
	assert.Contains(t, strings.ToLower(cookie), "httponly")
	assert.Contains(t, strings.ToLower(cookie), "samesite=lax")
	assert.Equal(t, "no-store", resp.Header.Get("Cache-Control"))
}

// Another site must not swap a visitor's identity: no GET (a link, an
// image), no POST from another site's page. Refused requests leave the code
// unused.
func TestTransferRefusesOtherSites(t *testing.T) {
	f := newFixture("")
	_, m := f.post(t, "token="+token, `{}`)
	id := m["id"].(string)
	for _, tc := range []struct {
		method  string
		headers map[string]string
		want    int
	}{
		{"GET", map[string]string{"Sec-Fetch-Site": "same-origin"}, 405},
		{"GET", nil, 405},
		{"POST", map[string]string{"Sec-Fetch-Site": "cross-site"}, 403},
		{"POST", map[string]string{"Sec-Fetch-Site": "same-site"}, 403},
		{"POST", map[string]string{"Sec-Fetch-Site": "none"}, 403},
		{"POST", map[string]string{"Origin": "https://evil.example"}, 403},
	} {
		resp, _ := f.redeem(t, id, tc.headers, tc.method)
		assert.Equal(t, tc.want, resp.StatusCode, "%s %v", tc.method, tc.headers)
		assert.NotContains(t, strings.Join(resp.Header.Values("Set-Cookie"), ";"), token)
	}
	// Still unused: a page of this site (Origin of the host) redeems it.
	resp, _ := f.redeem(t, id, map[string]string{"Origin": "http://example.com"}, "POST")
	assert.Equal(t, 200, resp.StatusCode)

	// Creating a code from another site is refused as well.
	req := httptest.NewRequest("POST", "/tokenTransfer", strings.NewReader(`{}`))
	req.Header.Set("Cookie", "token="+token)
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	r, err := f.app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 403, r.StatusCode)
}

func TestTransferPrefs(t *testing.T) {
	f := newFixture("")
	for body, want := range map[string]string{
		`{"prefsHttp":"{\"a\":1}"}`: `{"a":1}`, // etherpad-lite's field name
		`{"prefs":"[1,2]"}`:         "",        // not an object
		`{"prefs":"not json"}`:      "",
		`{}`:                        "",
		`not json at all`:           "",
	} {
		_, m := f.post(t, "token="+token, body)
		_, got := f.get(t, m["id"].(string))
		assert.Equal(t, want, got["prefs"], body)
	}
	_, m := f.post(t, "token="+token, `{"prefs":"`+strings.Repeat("x", 9000)+`"}`)
	_, got := f.get(t, m["id"].(string))
	assert.Equal(t, "", got["prefs"], "too large")
}

func TestTransferSingleUse(t *testing.T) {
	f := newFixture("")
	_, m := f.post(t, "token="+token, `{}`)
	id := m["id"].(string)
	resp, _ := f.get(t, id)
	assert.Equal(t, 200, resp.StatusCode)
	resp, _ = f.get(t, id)
	assert.Equal(t, 404, resp.StatusCode)

	// Two redemptions at once: one wins.
	_, m = f.post(t, "token="+token, `{}`)
	id = m["id"].(string)
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, _ := f.get(t, id)
			codes <- resp.StatusCode
		}()
	}
	wg.Wait()
	close(codes)
	got := []int{}
	for c := range codes {
		got = append(got, c)
	}
	assert.ElementsMatch(t, []int{200, 404}, got)
}

func TestTransferTTL(t *testing.T) {
	f := newFixture("")
	_, m := f.post(t, "token="+token, `{}`)
	f.now = f.now.Add(TTL + time.Second)
	resp, body := f.get(t, m["id"].(string))
	assert.Equal(t, 410, resp.StatusCode)
	assert.NotContains(t, resp.Header.Get("Set-Cookie"), token)
	assert.Equal(t, "Token expired", body["error"])
	resp, _ = f.get(t, m["id"].(string))
	assert.Equal(t, 404, resp.StatusCode, "an expired code is used up too")

	// A record without a time counts as expired.
	require.NoError(t, f.store.CreateTokenTransfer(db.TokenTransfer{ID: strings.Repeat("a", 32), Token: token}))
	resp, _ = f.get(t, strings.Repeat("a", 32))
	assert.Equal(t, 410, resp.StatusCode)

	// Old codes are cleaned up when new ones are made.
	_, old := f.post(t, "token="+token, `{}`)
	f.now = f.now.Add(TTL + time.Minute)
	f.post(t, "token="+token, `{}`)
	_, err := f.store.TakeTokenTransfer(old["id"].(string))
	assert.ErrorIs(t, err, db.ErrTokenTransferNotFound)
}

func TestTransferValidation(t *testing.T) {
	f := newFixture("")
	code, _ := f.post(t, "", `{}`)
	assert.Equal(t, 400, code, "no author cookie")
	code, _ = f.post(t, "token=not-a-token", `{}`)
	assert.Equal(t, 400, code, "a malformed token")
	resp, _ := f.get(t, "does-not-exist")
	assert.Equal(t, 404, resp.StatusCode)
	resp, _ = f.get(t, strings.Repeat("b", 32))
	assert.Equal(t, 404, resp.StatusCode)
}

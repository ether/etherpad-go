package versionstatus

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/updater"
	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Port of etherpad-lite's updateStatus (/api/version-status) spec.

func pool(attribs ...apool.Attribute) *apool.APool {
	p := apool.NewAPool()
	for i, a := range attribs {
		p.NumToAttrib[i] = a
	}
	return &p
}

type fixture struct {
	current, latest string
	pads            map[string]*apool.APool
	statusCalls     int
	now             time.Time
}

func (f *fixture) deps() Deps {
	return Deps{
		Status: func() updater.Status {
			f.statusCalls++
			st := updater.Status{CurrentVersion: f.current}
			if f.latest != "" {
				st.Latest = &updater.ReleaseInfo{Version: f.latest, Tag: "v" + f.latest}
			}
			return st
		},
		Pool: func(id string) (*apool.APool, bool) { p, ok := f.pads[id]; return p, ok },
		Now:  func() time.Time { return f.now },
	}
}

func newFixture() *fixture {
	return &fixture{
		current: "2.6.0", latest: "2.7.0", now: time.Unix(1_700_000_000, 0),
		pads: map[string]*apool.APool{
			"pad": pool(apool.Attribute{Key: "bold", Value: "true"}, apool.Attribute{Key: "author", Value: ""},
				apool.Attribute{Key: "author", Value: "a.first"}, apool.Attribute{Key: "author", Value: "a.second"}),
		},
	}
}

func TestFirstAuthor(t *testing.T) {
	assert.Equal(t, "a.first", FirstAuthor(newFixture().pads["pad"]), "the lowest entry, empty ids skipped")
	assert.Equal(t, "", FirstAuthor(pool(apool.Attribute{Key: "bold", Value: "true"})))
	assert.Equal(t, "", FirstAuthor(nil))
	// Map order must not matter.
	p := apool.NewAPool()
	p.NumToAttrib[7] = apool.Attribute{Key: "author", Value: "a.late"}
	p.NumToAttrib[2] = apool.Attribute{Key: "author", Value: "a.early"}
	assert.Equal(t, "a.early", FirstAuthor(&p))
}

func TestLookup(t *testing.T) {
	minor := "minor"
	outdated := Response{Outdated: &minor, IsFirstAuthor: true}
	for name, c := range map[string]struct {
		mod         func(*fixture)
		pad, author string
		want        Response
	}{
		"no latest release":       {func(f *fixture) { f.latest = "" }, "pad", "a.first", empty},
		"current is the latest":   {func(f *fixture) { f.current = "2.7.0" }, "pad", "a.first", empty},
		"current is ahead":        {func(f *fixture) { f.current = "3.0.0" }, "pad", "a.first", empty},
		"a patch behind":          {func(f *fixture) { f.latest = "2.6.9" }, "pad", "a.first", empty},
		"no pad id":               {func(*fixture) {}, "", "a.first", empty},
		"anonymous":               {func(*fixture) {}, "pad", "", empty},
		"not the first author":    {func(*fixture) {}, "pad", "a.second", empty},
		"no such pad":             {func(*fixture) {}, "missing", "a.first", empty},
		"first author, behind":    {func(*fixture) {}, "pad", "a.first", outdated},
		"first author, major gap": {func(f *fixture) { f.latest = "3.0.0" }, "pad", "a.first", outdated},
	} {
		f := newFixture()
		c.mod(f)
		assert.Equal(t, c.want, New(f.deps()).Lookup(c.pad, c.author), name)
	}
}

func TestLookupCache(t *testing.T) {
	f := newFixture()
	h := New(f.deps())
	h.Lookup("pad", "a.first")
	h.Lookup("pad", "a.first")
	assert.Equal(t, 1, f.statusCalls, "a cache hit")

	h.Lookup("pad", "a.second")
	h.Lookup("other", "a.first")
	assert.Equal(t, 3, f.statusCalls, "separate keys")

	f.now = f.now.Add(61 * time.Second)
	h.Lookup("pad", "a.first")
	assert.Equal(t, 4, f.statusCalls, "expired after a minute")

	d := f.deps()
	d.Capacity = 2
	small := New(d)
	small.Lookup("p1", "a")
	small.Lookup("p2", "a")
	small.Lookup("p3", "a") // evicts p1
	before := f.statusCalls
	small.Lookup("p1", "a")
	assert.Equal(t, before+1, f.statusCalls, "evicted and computed again")
	small.Lookup("p3", "a")
	assert.Equal(t, before+1, f.statusCalls, "p3 is still cached")
}

func TestServe(t *testing.T) {
	f := newFixture()
	d := f.deps()
	d.Author = func(c fiber.Ctx) string { return c.Get("X-Author") }
	app := fiber.New()
	app.Get("/api/version-status", New(d).Serve)

	get := func(url, author string) (map[string]any, string) {
		req := httptest.NewRequest("GET", url, nil)
		req.Header.Set("X-Author", author)
		resp, err := app.Test(req)
		require.NoError(t, err)
		b, _ := io.ReadAll(resp.Body)
		var m map[string]any
		require.NoError(t, json.Unmarshal(b, &m), string(b))
		return m, resp.Header.Get("Cache-Control")
	}
	m, cache := get("/api/version-status?padId=pad", "a.first")
	assert.Equal(t, map[string]any{"outdated": "minor", "isFirstAuthor": true}, m)
	assert.Equal(t, "private, no-store", cache)
	m, _ = get("/api/version-status", "a.first")
	assert.Equal(t, map[string]any{"outdated": nil, "isFirstAuthor": false}, m)
}

package updater

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newCheckerForServer(srv *httptest.Server) *VersionChecker {
	c := NewVersionChecker("test/repo")
	c.apiBase = srv.URL
	return c
}

func TestCheckLatestUpdated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"etag-123"`)
		_ = json.NewEncoder(w).Encode(ghRelease{TagName: "v2.0.0", Body: "notes"})
	}))
	defer srv.Close()

	res := newCheckerForServer(srv).CheckLatest("")
	if res.Kind != CheckUpdated {
		t.Fatalf("kind = %q", res.Kind)
	}
	if res.Release == nil || res.Release.Version != "2.0.0" || res.Release.Tag != "v2.0.0" {
		t.Errorf("release = %+v", res.Release)
	}
	if res.ETag != `"etag-123"` {
		t.Errorf("etag = %q", res.ETag)
	}
}

func TestCheckLatestNotModified(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") == `"etag-123"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		_ = json.NewEncoder(w).Encode(ghRelease{TagName: "v2.0.0"})
	}))
	defer srv.Close()

	res := newCheckerForServer(srv).CheckLatest(`"etag-123"`)
	if res.Kind != CheckNotModified {
		t.Fatalf("kind = %q", res.Kind)
	}
}

func TestCheckLatestPrerelease(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(ghRelease{TagName: "v2.0.0-rc1", Prerelease: true})
	}))
	defer srv.Close()

	if res := newCheckerForServer(srv).CheckLatest(""); res.Kind != CheckPrerelease {
		t.Fatalf("kind = %q", res.Kind)
	}
}

func TestCheckLatestErrors(t *testing.T) {
	for _, tc := range []struct {
		status int
		want   CheckKind
	}{
		{http.StatusForbidden, CheckRateLimited},
		{http.StatusTooManyRequests, CheckRateLimited},
		{http.StatusInternalServerError, CheckError},
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.status)
		}))
		res := newCheckerForServer(srv).CheckLatest("")
		srv.Close()
		if res.Kind != tc.want {
			t.Errorf("status %d -> kind %q, want %q", tc.status, res.Kind, tc.want)
		}
	}
}

// etherpad-lite's VersionChecker cases beyond the ones above.
func TestCheckLatestParity(t *testing.T) {
	var gotETag string
	body := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotETag = r.Header.Get("If-None-Match")
		w.Header().Set("ETag", `"e2"`)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	c := newCheckerForServer(srv)

	body = `{"tag_name":"v2.7.3","body":"notes","published_at":"2026-05-01T00:00:00Z","html_url":"https://github.com/x/releases/v2.7.3"}`
	res := c.CheckLatest(`"e1"`)
	if gotETag != `"e1"` {
		t.Errorf("the previous ETag is sent, got %q", gotETag)
	}
	if res.Kind != CheckUpdated || res.Release.PublishedAt != "2026-05-01T00:00:00Z" || res.Release.HTMLURL == "" || res.Release.Body != "notes" {
		t.Errorf("parsed release: %+v %+v", res, res.Release)
	}

	body = `{"tag_name":"v3.0.0-rc.1","prerelease":true}`
	if res := c.CheckLatest(""); res.Kind != CheckPrerelease || res.ETag != `"e2"` {
		t.Errorf("a pre-release is skipped but keeps the ETag: %+v", res)
	}
	body = `{"tag_name":"v3.0.0","draft":true}`
	if res := c.CheckLatest(""); res.Kind != CheckPrerelease {
		t.Errorf("a draft is skipped: %+v", res)
	}
	for _, b := range []string{`{"body":"no tag"}`, `{"tag_name":null}`, `{"tag_name":"nightly"}`, `{"tag_name":"-rf"}`, `not json`, `{"tag_name":"v2.7"}`} {
		body = b
		if res := c.CheckLatest(""); res.Kind != CheckError {
			t.Errorf("%s: kind %q, want error", b, res.Kind)
		}
	}
}

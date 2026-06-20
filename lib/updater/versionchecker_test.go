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

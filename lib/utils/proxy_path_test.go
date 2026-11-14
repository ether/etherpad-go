package utils

import (
	"regexp"
	"strings"
	"testing"
)

func headers(m map[string]string) func(string) string {
	return func(n string) string { return m[n] }
}

// Port of etherpad-lite's sanitizeProxyPath spec.
func TestCleanProxyPath(t *testing.T) {
	cases := map[string]string{
		"":                   "",
		"/pad/etherpad":      "/pad/etherpad",
		"/a-b_c.d/0-9":       "/a-b_c.d/0-9",
		"/a\n/b":             "/a/b",
		"//evil.example/pwn": "/evil.example/pwn",
		"///x":               "/x",
		"/////x":             "/x",
		"/a//b":              "/a//b", // mid-path: harmless, as in etherpad-lite
		"/a/../b":            "",
		"../b":               "",
		"/a/..":              "",
		"/a/..b/c":           "/a/..b/c",
		"pad/etherpad":       "/pad/etherpad",
		"pad":                "/pad",
		"x":                  "/x",
		"//pad":              "/pad",
		"<>\"'()=&":          "",
		"\t ":                "",
		"/":                  "",
		"///":                "",
		"/pads/":             "/pads",
		"/pads//":            "/pads",
	}
	for in, want := range cases {
		if got := CleanProxyPath(in); got != want {
			t.Errorf("CleanProxyPath(%q) = %q, want %q", in, got, want)
		}
	}
	safe := regexp.MustCompile(`^(/[A-Za-z0-9\-_./]*)?$`)
	for _, in := range []string{
		`/pad"><script>alert(1)</script>`, `/x' onload='y`, `/a b`, `javascript:alert(1)`,
		`http:\\evil.example`, `\\evil.example`, `/p?x=1#y`,
	} {
		got := CleanProxyPath(in)
		if !safe.MatchString(got) || regexp.MustCompile(`[<>"'()=&:\\ ]`).MatchString(got) {
			t.Errorf("CleanProxyPath(%q) = %q is not safe", in, got)
		}
		if strings.HasSuffix(got, "/") {
			t.Errorf("CleanProxyPath(%q) = %q ends with /", in, got)
		}
		if len(got) > 1 && got[1] == '/' {
			t.Errorf("CleanProxyPath(%q) = %q is protocol-relative", in, got)
		}
	}
}

func TestProxyPathHeaders(t *testing.T) {
	all := map[string]string{"x-proxy-path": "/a", "x-forwarded-prefix": "/b", "x-ingress-path": "/c"}
	cases := []struct {
		h     map[string]string
		trust bool
		want  string
	}{
		{nil, false, ""},
		{map[string]string{"x-forwarded-prefix": "/pad"}, true, "/pad"},
		{map[string]string{"x-ingress-path": "/api/hassio_ingress/tok"}, true, "/api/hassio_ingress/tok"},
		{map[string]string{"x-forwarded-prefix": "/pad"}, false, ""},
		{map[string]string{"x-ingress-path": "/x"}, false, ""},
		{map[string]string{"x-proxy-path": "/legacy"}, false, "/legacy"},
		{all, true, "/a"},
		{map[string]string{"x-forwarded-prefix": "/b", "x-ingress-path": "/c"}, true, "/b"},
		{map[string]string{"x-forwarded-prefix": "//evil", "x-ingress-path": "/c"}, true, "/evil"},
		{map[string]string{"x-proxy-path": "/../x", "x-forwarded-prefix": "/b"}, true, "/b"},
		{map[string]string{"x-ingress-path": "<script>"}, true, "/script"},
	}
	for _, c := range cases {
		if got := ProxyPath(headers(c.h), c.trust); got != c.want {
			t.Errorf("ProxyPath(%v, trust=%v) = %q, want %q", c.h, c.trust, got, c.want)
		}
	}
	if got := ProxyPath(nil, true); got != "" {
		t.Errorf("no header function: %q", got)
	}
	if got := ProxyPathHeaders(false); len(got) != 1 || got[0] != "x-proxy-path" {
		t.Errorf("ProxyPathHeaders(false) = %v", got)
	}
	if got := ProxyPathHeaders(true); len(got) != 3 {
		t.Errorf("ProxyPathHeaders(true) = %v", got)
	}
}

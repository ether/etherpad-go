package legacy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAPIKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "APIKEY.txt")
	key, err := LoadAPIKey(path)
	if err != nil || len(key) != 32 {
		t.Fatalf("a missing file gets a random 32 character key: %q, %v", key, err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("the key file is private: %v, %v", info, err)
	}
	again, err := LoadAPIKey(path)
	if err != nil || again != key {
		t.Fatalf("the key is kept: %q != %q (%v)", again, key, err)
	}
	if err := os.WriteFile(path, []byte("  my-key\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, _ := LoadAPIKey(path); got != "my-key" {
		t.Fatalf("white space around the key is ignored: %q", got)
	}
	t.Setenv("EP_APIKEY_FILE", "/etc/etherpad/key")
	if got := APIKeyFile("/srv"); got != "/etc/etherpad/key" {
		t.Fatalf("EP_APIKEY_FILE: %q", got)
	}
}

func TestParseIntJS(t *testing.T) {
	for in, want := range map[string]int{"5": 5, " 7": 7, "12abc": 12, "1.9": 1, "-3": -3, "+4": 4} {
		if got, ok := parseIntJS(in); !ok || got != want {
			t.Errorf("parseIntJS(%q) = %d, %v; want %d", in, got, ok, want)
		}
	}
	for _, in := range []string{"", "abc", "-", " x1"} {
		if _, ok := parseIntJS(in); ok {
			t.Errorf("parseIntJS(%q) should fail", in)
		}
	}
}

func TestSummaryAndHost(t *testing.T) {
	if got := summary("getRevisionsCount"); got != "Get revisions count" {
		t.Errorf("summary: %q", got)
	}
	for _, h := range []string{"example.com", "example.com:9001", "[::1]:9001", "localhost"} {
		if !hostPattern.MatchString(h) {
			t.Errorf("%q is a host", h)
		}
	}
	for _, h := range []string{"evil/<script>", "a b", `x"y`, ""} {
		if hostPattern.MatchString(h) {
			t.Errorf("%q is not a host", h)
		}
	}
}

// Port of etherpad-lite's openapi spec (generateServerForApiVersion).
func TestServerOrigin(t *testing.T) {
	cases := []struct{ publicURL, scheme, host, want string }{
		{"https://pad.canonical.example", "http", "evil.com", "https://pad.canonical.example"},
		{"https://pad.canonical.example/", "http", "evil.com", "https://pad.canonical.example"},
		{"pad.example", "https", "pad.example", "https://pad.example"},
		{"http:///foo", "https", "pad.example", "https://pad.example"},
		{"https://user@pad.example", "https", "pad.example", "https://pad.example"},
		{"javascript:alert(1)", "https", "pad.example", "https://pad.example"},
		{"", "http", "pad.example", "http://pad.example"},
		{"", "https", "pad.example", "https://pad.example"},
		{"", "javascript", "pad.example", "http://pad.example"},
		{"", "https", "evil.com\r\nX-Injected: 1", "https://localhost"},
		{"", "https", "user@evil.com", "https://localhost"},
		{"", "https", "<script>", "https://localhost"},
		{"", "https", "*", "https://localhost"},
	}
	for _, c := range cases {
		if got := serverOrigin(c.publicURL, c.scheme, c.host); got != c.want {
			t.Errorf("serverOrigin(%q, %q, %q) = %q, want %q", c.publicURL, c.scheme, c.host, got, c.want)
		}
	}
}

package legacy

import (
	"os"

	"github.com/ether/etherpad-go/lib/accounts"
	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/tokens"
	"path/filepath"
	"testing"
)

func TestReadAPIKeyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "APIKEY.txt")
	key, err := ReadAPIKeyFile(path)
	if err != nil || key != "" {
		t.Fatalf("no file, no key: %q, %v", key, err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("the file is not created")
	}
	if err := os.WriteFile(path, []byte("  my-key\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, _ := ReadAPIKeyFile(path); got != "my-key" {
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

func newFileKeys(t *testing.T) *tokens.Manager {
	t.Helper()
	store := db.NewMemoryDataStore()
	acc := accounts.NewManager(store)
	if err := acc.EnsureDefaults(accounts.SeedConfig{Template: accounts.TemplateOpen}, nil); err != nil {
		t.Fatal(err)
	}
	return tokens.NewManager(store, acc, nil)
}

func importFile(t *testing.T, keys *tokens.Manager, path string, want FileResult) {
	t.Helper()
	got, err := ImportAPIKeyFile(keys, path)
	if err != nil || got != want {
		t.Fatalf("ImportAPIKeyFile = %v, %v; want %v", got, err, want)
	}
}

// The key of an APIKEY.txt becomes a token on the first start with
// tokens; afterwards the file is ignored, and the database remembers it.
func TestImportAPIKeyFileOnce(t *testing.T) {
	keys := newFileKeys(t)
	path := filepath.Join(t.TempDir(), "APIKEY.txt")
	if err := os.WriteFile(path, []byte("old-key\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	importFile(t, keys, path, FileStored)
	importFile(t, keys, path, FileIgnored)
	list, _ := keys.List(accounts.System, "")
	if len(list) != 1 || list[0].Name != "APIKEY.txt" {
		t.Fatalf("listed as APIKEY.txt: %+v", list)
	}
	if _, err := keys.Authenticate("old-key", ""); err != nil {
		t.Fatal("the old key still works")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("the file is left alone")
	}
	// Revoked in the admin UI, it stays revoked; a new key in the file is
	// not taken over either.
	if err := keys.Revoke(accounts.System, list[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("another-key"), 0o600); err != nil {
		t.Fatal(err)
	}
	importFile(t, keys, path, FileIgnored)
	for _, k := range []string{"old-key", "another-key"} {
		if _, err := keys.Authenticate(k, ""); err == nil {
			t.Fatalf("%s works", k)
		}
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	importFile(t, keys, path, FileNone)
}

// Without a file on the first start, a file found later is ignored.
func TestImportAPIKeyFileLater(t *testing.T) {
	keys := newFileKeys(t)
	path := filepath.Join(t.TempDir(), "APIKEY.txt")
	importFile(t, keys, path, FileNone)
	if err := os.WriteFile(path, []byte("late-key"), 0o600); err != nil {
		t.Fatal(err)
	}
	importFile(t, keys, path, FileIgnored)
	if _, err := keys.Authenticate("late-key", ""); err == nil {
		t.Fatal("a file found later became a token")
	}
}

// A key that already is a token (taken over by an earlier release on
// every start) is only marked as done.
func TestImportAPIKeyFileKnown(t *testing.T) {
	keys := newFileKeys(t)
	if _, err := keys.ImportKey("old-key", "APIKEY.txt"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "APIKEY.txt")
	if err := os.WriteFile(path, []byte("old-key"), 0o600); err != nil {
		t.Fatal(err)
	}
	importFile(t, keys, path, FileKnown)
	importFile(t, keys, path, FileIgnored)
}

// A file that cannot be read is tried again on the next start.
func TestImportAPIKeyFileRetried(t *testing.T) {
	keys := newFileKeys(t)
	path := filepath.Join(t.TempDir(), "APIKEY.txt")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := ImportAPIKeyFile(keys, path); err == nil {
		t.Fatal("an unreadable file is no error")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("old-key"), 0o600); err != nil {
		t.Fatal(err)
	}
	importFile(t, keys, path, FileStored)
}

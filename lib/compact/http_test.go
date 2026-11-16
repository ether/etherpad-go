package compact

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A redirect must not carry the API key to another host: the client does
// not follow redirects and says where the server points to.
func TestRedirectDoesNotLeakTheKey(t *testing.T) {
	var leaked string
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		leaked = r.Header.Get("apikey")
		_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"padIDs":[]}}`))
	}))
	defer other.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/" {
			_, _ = w.Write([]byte(`{"currentVersion":"1.3.1"}`))
			return
		}
		http.Redirect(w, r, other.URL+r.URL.Path, http.StatusTemporaryRedirect)
	}))
	defer server.Close()

	api := &HTTPAPI{BaseURL: server.URL, APIKey: "secret"}
	_, err := api.ListAllPads()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "redirects")
	assert.Contains(t, err.Error(), other.URL)
	assert.Empty(t, leaked, "the other host got the API key")
}

// The server speaks plain HTTP even with ssl settings (TLS is a reverse
// proxy's job), so the address from the settings is http.
func TestURLFromSettingsIsHTTP(t *testing.T) {
	file := filepath.Join(t.TempDir(), "settings.json")
	require.NoError(t, os.WriteFile(file, []byte(`{"ip": "127.0.0.1", "port": "9123", "ssl": {"key": "k.pem", "cert": "c.pem"}}`), 0o644))
	t.Setenv("ETHERPAD_SETTINGS_FILE", file)
	assert.Equal(t, "http://127.0.0.1:9123", urlFromSettings())
}

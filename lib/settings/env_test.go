package settings

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withFile points the settings at a file with content for the test.
func withFile(t *testing.T, content string) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "settings.json")
	require.NoError(t, os.WriteFile(p, []byte(content), 0o600))
	t.Setenv(SettingsFileEnv, p)
}

const ssoFile = `{
  "sso": {
    "issuer": "https://file.example",
    "clients": [
      {"client_id": "admin", "client_secret": "from-file", "type": "admin",
       "redirect_uris": ["https://file.example/admin/"], "grant_types": ["authorization_code"]}
    ]
  },
  "users": {"alice": {"password": "file-password", "is_admin": false}}
}`

// SSO clients from the environment only, as in a container without a
// settings.json.
func TestEnvCreatesSSOClients(t *testing.T) {
	t.Setenv("ETHERPAD_SSO_ISSUER", "https://pads.example")
	t.Setenv("ETHERPAD_SSO_CLIENTS_0_CLIENT_ID", "admin")
	t.Setenv("ETHERPAD_SSO_CLIENTS_0_CLIENT_SECRET", "0123")
	t.Setenv("ETHERPAD_SSO_CLIENTS_0_TYPE", "admin")
	t.Setenv("ETHERPAD_SSO_CLIENTS_0_REDIRECT_URIS", "https://pads.example/admin/, https://pads.example/admin")
	t.Setenv("ETHERPAD_SSO_CLIENTS_1_CLIENT_ID", "user")
	t.Setenv("ETHERPAD_SSO_CLIENTS_1_TYPE", "user")
	t.Setenv("ETHERPAD_SSO_CLIENTS_1_GRANT_TYPES_0", "authorization_code")
	t.Setenv("ETHERPAD_SSO_CLIENTS_1_GRANT_TYPES_1", "refresh_token")
	cfg, err := ReadConfig()
	require.NoError(t, err)
	require.NotNil(t, cfg.SSO)
	assert.Equal(t, "https://pads.example", cfg.SSO.Issuer)
	require.Len(t, cfg.SSO.Clients, 2)
	c := cfg.SSO.Clients[0]
	assert.Equal(t, "admin", c.ClientId)
	require.NotNil(t, c.ClientSecret)
	assert.Equal(t, "0123", *c.ClientSecret, "a string field keeps the text, no number")
	assert.Equal(t, []string{"https://pads.example/admin/", "https://pads.example/admin"}, c.RedirectUris)
	assert.Equal(t, "user", cfg.SSO.Clients[1].ClientId)
	assert.Equal(t, []string{"authorization_code", "refresh_token"}, cfg.SSO.Clients[1].GrantTypes)
}

// A place in a list merges into the element from the file; one past the
// end appends.
func TestEnvMergesIntoFileList(t *testing.T) {
	withFile(t, ssoFile)
	t.Setenv("ETHERPAD_SSO_CLIENTS_0_CLIENT_SECRET", "from-secret")
	t.Setenv("ETHERPAD_SSO_CLIENTS_1_CLIENT_ID", "user")
	t.Setenv("ETHERPAD_SSO_CLIENTS_1_TYPE", "user")
	cfg, err := ReadConfig()
	require.NoError(t, err)
	require.Len(t, cfg.SSO.Clients, 2)
	c := cfg.SSO.Clients[0]
	assert.Equal(t, "from-secret", *c.ClientSecret)
	assert.Equal(t, "admin", c.ClientId, "the rest of the element stays")
	assert.Equal(t, []string{"https://file.example/admin/"}, c.RedirectUris)
	assert.Equal(t, "https://file.example", cfg.SSO.Issuer)
	assert.Equal(t, "user", cfg.SSO.Clients[1].ClientId)
}

func TestEnvRelaxedNames(t *testing.T) {
	t.Setenv("ETHERPAD_PAD_OPTIONS_SHOW_CHAT", "false")
	t.Setenv("ETHERPAD_PADOPTIONS_USERNAME", "Relaxed")
	t.Setenv("ETHERPAD_CLUSTER_BINDPORT", "7947")
	t.Setenv("ETHERPAD_AVAILABLEEXPORTS", "pdf,html")
	cfg, err := ReadConfig()
	require.NoError(t, err)
	assert.False(t, cfg.PadOptions.ShowChat)
	require.NotNil(t, cfg.PadOptions.UserName)
	assert.Equal(t, "Relaxed", *cfg.PadOptions.UserName)
	assert.Equal(t, 7947, cfg.Cluster.BindPort)
	assert.Equal(t, []string{"pdf", "html"}, cfg.AvailableExports)
}

// Maps with free keys go through EP__, places in lists there too.
func TestEnvSpelledMapsAndLists(t *testing.T) {
	withFile(t, ssoFile)
	t.Setenv("EP__users__alice__password", "env-password")
	t.Setenv("EP__users__bob__password", "bob-password")
	t.Setenv("EP__users__bob__is_admin", "true")
	t.Setenv("EP__sso__clients__0__display_name", "Admin UI")
	cfg, err := ReadConfig()
	require.NoError(t, err)
	require.NotNil(t, cfg.Users["alice"].Password)
	assert.Equal(t, "env-password", *cfg.Users["alice"].Password)
	require.NotNil(t, cfg.Users["bob"].Password)
	assert.Equal(t, "bob-password", *cfg.Users["bob"].Password)
	require.NotNil(t, cfg.Users["bob"].IsAdmin)
	assert.True(t, *cfg.Users["bob"].IsAdmin)
	assert.Equal(t, "Admin UI", cfg.SSO.Clients[0].DisplayName)
	assert.Equal(t, "admin", cfg.SSO.Clients[0].ClientId)
}

func TestEnvGapStopsTheStart(t *testing.T) {
	t.Setenv("ETHERPAD_SSO_CLIENTS_0_CLIENT_ID", "admin")
	t.Setenv("ETHERPAD_SSO_CLIENTS_2_CLIENT_ID", "third")
	_, err := ReadConfig()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ETHERPAD_SSO_CLIENTS_2_CLIENT_ID")
	assert.Contains(t, err.Error(), "without gaps")
}

func TestEnvWrongTypeStopsTheStart(t *testing.T) {
	t.Setenv("ETHERPAD_PADOPTIONS_SHOWCHAT", "yes")
	_, err := ReadConfig()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ETHERPAD_PADOPTIONS_SHOWCHAT")
}

func TestEnvReportsUnknownNames(t *testing.T) {
	report, err := applyEnv([]string{
		"ETHERPAD_PADOPTIONS_SHOWCHATT=false", // a typo
		"ETHERPAD_USERS_ALICE_PASSWORD=x",     // a map: EP__ only
		"ETHERPAD_SETTINGS_FILE=/data/settings.json",
		"ETHERPAD_SERVICE_HOST=10.0.0.1", // Kubernetes service links
		"ETHERPAD_PORT_9001_TCP_ADDR=10.0.0.1",
		"ETHERPAD_PLUGINS_EP_ALIGN_ENABLED=true", // registered, viper reads it
		"EP__padOptions__noSuchOption=1",
		"EP__ep_some_plugin__option=1", // a plugin's own section
		"PATH=/usr/bin",
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"EP__padOptions__noSuchOption", "ETHERPAD_PADOPTIONS_SHOWCHATT", "ETHERPAD_USERS_ALICE_PASSWORD"}, report.Unknown)
}

// Plugin sections of any shape keep working, lists in them too.
func TestEnvPluginSectionWithList(t *testing.T) {
	withFile(t, `{"ep_example": {"servers": [{"url": "a"}, {"url": "b"}]}}`)
	t.Setenv("EP__ep_example__servers__1__url", "changed")
	t.Setenv("EP__ep_example__servers__2__url", "added")
	t.Setenv("EP__ep_example__name", "plain")
	_, err := ReadConfig()
	require.NoError(t, err)
	servers, ok := getPath("ep_example.servers").([]any)
	require.True(t, ok)
	require.Len(t, servers, 3)
	assert.Equal(t, "a", servers[0].(map[string]any)["url"])
	assert.Equal(t, "changed", servers[1].(map[string]any)["url"])
	assert.Equal(t, "added", servers[2].(map[string]any)["url"])
	assert.Equal(t, "plain", getPath("ep_example.name"))
}

func getPath(key string) any { return viperGet(key) }

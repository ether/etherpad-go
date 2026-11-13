package ws

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ether/etherpad-go/lib/models/ws/admin"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/ether/etherpad-go/lib/test/testutils"
	libws "github.com/ether/etherpad-go/lib/ws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Port of etherpad-lite's adminSettingsResolved and adminSettingsSave: the
// editor gets settings.json as it is on disk and saves it byte for byte.
func TestAdminSettingsFile(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(testutils.TestRunConfig{Name: "settings file", Test: testAdminSettingsFile})
	testDb.StartTestDBHandler()
}

func adminAsk(t *testing.T, ds testutils.TestDataStore, event string, data any) map[string]any {
	t.Helper()
	client := &libws.Client{Hub: libws.NewHub(), Conn: libws.NewActualMockWebSocketconn(), Send: make(chan []byte, 16), SessionId: "adminSettings"}
	raw, _ := json.Marshal(data)
	ds.AdminMessageHandler.HandleMessage(admin.EventMessage{Event: event, Data: raw}, ds.Settings, client)
	select {
	case msg := <-client.Send:
		var arr []json.RawMessage
		require.NoError(t, json.Unmarshal(msg, &arr))
		var payload map[string]any
		require.NoError(t, json.Unmarshal(arr[1], &payload))
		return payload
	default:
		t.Fatal("no reply")
		return nil
	}
}

func testAdminSettingsFile(t *testing.T, ds testutils.TestDataStore) {
	dir := t.TempDir()
	ds.Settings.Root = dir
	ds.Settings.ShowSettingsInAdminPage = true
	ds.Settings.TrustProxy = true
	pw := "db-secret"
	ds.Settings.DBSettings = &settings.DBSettings{Password: pw}
	file := filepath.Join(dir, "settings.json")
	original := "/* my settings */\n{\n  \"title\": \"${TITLE:Etherpad}\", // from the environment\n  \"trustProxy\": false,\n}\n"
	require.NoError(t, os.WriteFile(file, []byte(original), 0o600))

	t.Run("load sends the file, the live values redacted and flags", func(t *testing.T) {
		reply := adminAsk(t, ds, "load", nil)
		assert.Equal(t, original, reply["results"], "the raw file, comments and placeholders included")
		resolved, ok := reply["resolved"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, true, resolved["trustProxy"], "live value, not the file's")
		b, _ := json.Marshal(resolved)
		assert.NotContains(t, string(b), "db-secret")
		assert.NotNil(t, reply["flags"])
	})
	t.Run("save writes the text byte for byte", func(t *testing.T) {
		edited := "// edited\n{\n  \"title\": \"${TITLE:Pads}\", /* keep me */\n  \"ep_plugin\": {\"x\": 1},\n}\n"
		reply := adminAsk(t, ds, "saveSettings", edited)
		assert.Equal(t, true, reply["success"], reply)
		got, err := os.ReadFile(file)
		require.NoError(t, err)
		assert.Equal(t, edited, string(got))
	})
	t.Run("invalid settings are not saved", func(t *testing.T) {
		before, err := os.ReadFile(file)
		require.NoError(t, err)
		reply := adminAsk(t, ds, "saveSettings", "{ \"title\": ")
		assert.NotEmpty(t, reply["error"])
		after, err := os.ReadFile(file)
		require.NoError(t, err)
		assert.Equal(t, string(before), string(after), "nothing was written")
	})
	t.Run("nothing when showSettingsInAdminPage is off", func(t *testing.T) {
		ds.Settings.ShowSettingsInAdminPage = false
		defer func() { ds.Settings.ShowSettingsInAdminPage = true }()
		reply := adminAsk(t, ds, "load", nil)
		assert.Equal(t, "NOT_ALLOWED", reply["results"])
		_, has := reply["resolved"]
		assert.False(t, has)
		before, _ := os.ReadFile(file)
		adminAsk(t, ds, "saveSettings", "{}")
		after, _ := os.ReadFile(file)
		assert.Equal(t, string(before), string(after))
	})
}

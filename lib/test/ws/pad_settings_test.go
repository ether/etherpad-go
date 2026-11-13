package ws

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/ether/etherpad-go/lib/models/ws"
	"github.com/ether/etherpad-go/lib/pad"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/ether/etherpad-go/lib/test/testutils"
	libws "github.com/ether/etherpad-go/lib/ws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Pad-wide settings (etherpad-lite #7586): the pad's creator sets them for
// everyone, others are refused, every client of the pad hears the change.
func TestPadWideSettings(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(
		testutils.TestRunConfig{Name: "the creator changes them for everyone", Test: testPadSettingsByCreator},
		testutils.TestRunConfig{Name: "another user is refused", Test: testPadSettingsByOtherUser},
		testutils.TestRunConfig{Name: "ignored when enablePadWideSettings is off", Test: testPadSettingsDisabled},
		testutils.TestRunConfig{Name: "the creator's defaults apply to a new pad only", Test: testPadSettingsDefaults},
	)
	testDb.StartTestDBHandler()
}

// padSettingsConfig is the test's settings with the defaults of a real
// configuration (the test settings leave every option at its zero value).
func padSettingsConfig(ds testutils.TestDataStore) *settings.Settings {
	cfg := *ds.Settings
	cfg.EnablePadWideSettings = true
	cfg.PadOptions.ShowChat = true
	cfg.PadOptions.ShowLineNumbers = true
	cfg.PadOptions.FadeInactiveAuthorColors = true
	return &cfg
}

type settingsClient struct {
	client *libws.Client
	author string
	// vars is the CLIENT_VARS data.
	vars map[string]any
}

func joinWithSettings(t *testing.T, handler *libws.PadMessageHandler, hub *libws.Hub, cfg *settings.Settings, ds testutils.TestDataStore,
	padID, sessionID string, defaults map[string]any) settingsClient {
	t.Helper()
	return joinWithSettingsToken(t, handler, hub, cfg, ds, padID, sessionID, "t.settings"+sessionID, defaults)
}

func joinWithSettingsToken(t *testing.T, handler *libws.PadMessageHandler, hub *libws.Hub, cfg *settings.Settings, ds testutils.TestDataStore,
	padID, sessionID, token string, defaults map[string]any) settingsClient {
	t.Helper()
	client := createTestClient(hub, sessionID, padID, libws.NewActualMockWebSocketconn())
	t.Cleanup(func() { removeClient(hub, client) })
	handler.SessionStore.InitSessionForTest(sessionID)
	handler.HandleMessage(ws.ClientReady{
		Event: "message",
		Data: ws.ClientReadyData{Component: "pad", Type: "CLIENT_READY", PadID: padID,
			Token: token, PadSettingsDefaults: defaults},
	}, client, cfg, ds.Logger)
	sc := settingsClient{client: client}
	for _, f := range frames(client) {
		if f.Type == "CLIENT_VARS" {
			sc.vars = f.Data
		}
	}
	require.NotNil(t, sc.vars, "session %s got no CLIENT_VARS", sessionID)
	sc.author, _ = sc.vars["userId"].(string)
	return sc
}

func sendPadOptions(handler *libws.PadMessageHandler, cfg *settings.Settings, ds testutils.TestDataStore, c settingsClient, options map[string]any) {
	handler.HandleMessage(ws.ClientMessage{
		Event: "message",
		Data: ws.ClientMessageData{Component: "pad", Type: "COLLABROOM", Data: ws.ClientMessageDataData{
			Type: "CLIENT_MESSAGE",
			Payload: ws.ClientMessagePayload{
				Type: "padoptions", Options: options, Extra: map[string]any{"changedBy": "creator"},
			},
		}},
	}, c.client, cfg, ds.Logger)
}

// padOptionsReceived returns the options of the padoptions messages client got.
func padOptionsReceived(client *libws.Client) (options []map[string]any, shouts []string) {
	for _, f := range frames(client) {
		switch f.Type {
		case "CLIENT_MESSAGE":
			payload, _ := f.Data["payload"].(map[string]any)
			if payload["type"] == "padoptions" {
				opts, _ := payload["options"].(map[string]any)
				options = append(options, opts)
			}
		case "shoutMessage":
			payload, _ := f.Data["payload"].(map[string]any)
			msg, _ := payload["message"].(map[string]any)
			text, _ := msg["message"].(string)
			shouts = append(shouts, text)
		}
	}
	return options, shouts
}

func initialOptions(t *testing.T, c settingsClient) map[string]any {
	t.Helper()
	opts, ok := c.vars["initialOptions"].(map[string]any)
	require.True(t, ok, "CLIENT_VARS without initialOptions: %v", c.vars["initialOptions"])
	return opts
}

func view(opts map[string]any) map[string]any {
	v, _ := opts["view"].(map[string]any)
	return v
}

func testPadSettingsByCreator(t *testing.T, ds testutils.TestDataStore) {
	h, hub, cfg := ds.PadMessageHandler, ds.Hub, padSettingsConfig(ds)
	padID := "padSettingsCreator"

	creator := joinWithSettings(t, h, hub, cfg, ds, padID, "pscreator", nil)
	assert.Equal(t, true, creator.vars["canEditPadSettings"])
	assert.Equal(t, true, initialOptions(t, creator)["showChat"], "defaults before any change")
	other := joinWithSettings(t, h, hub, cfg, ds, padID, "psother", nil)
	assert.Equal(t, false, other.vars["canEditPadSettings"])
	frames(creator.client)

	sendPadOptions(h, cfg, ds, creator, map[string]any{
		"enforceSettings": true, "showChat": false,
		"view": map[string]any{"showLineNumbers": false},
	})
	for _, c := range []settingsClient{creator, other} {
		got, shouts := padOptionsReceived(c.client)
		require.Len(t, got, 1, "every client of the pad hears the change")
		assert.Empty(t, shouts)
		assert.Equal(t, false, got[0]["showChat"])
		assert.Equal(t, true, got[0]["enforceSettings"])
		assert.Equal(t, false, view(got[0])["showLineNumbers"])
		assert.Equal(t, true, view(got[0])["showAuthorColors"], "the complete settings are sent")
	}

	// A second change merges into the first, the view object key by key.
	sendPadOptions(h, cfg, ds, creator, map[string]any{"view": map[string]any{"rtlIsTrue": true}})
	got, _ := padOptionsReceived(other.client)
	require.Len(t, got, 1)
	assert.Equal(t, false, got[0]["showChat"])
	assert.Equal(t, false, view(got[0])["showLineNumbers"])
	assert.Equal(t, true, view(got[0])["rtlIsTrue"])

	// Stored: a client joining later, after the pad was unloaded, gets them.
	ds.PadManager.UnloadPad(padID)
	later := joinWithSettings(t, h, hub, cfg, ds, padID, "pslater", nil)
	opts := initialOptions(t, later)
	assert.Equal(t, false, opts["showChat"])
	assert.Equal(t, true, opts["enforceSettings"])
	assert.Equal(t, true, view(opts)["rtlIsTrue"])

	// A second tab of the creator's browser (same token, same author) may
	// edit the pad-wide settings too.
	secondTab := joinWithSettingsToken(t, h, hub, cfg, ds, padID, "pscreatortab2", "t.settingspscreator", nil)
	assert.Equal(t, true, secondTab.vars["canEditPadSettings"])
}

func testPadSettingsByOtherUser(t *testing.T, ds testutils.TestDataStore) {
	h, hub, cfg := ds.PadMessageHandler, ds.Hub, padSettingsConfig(ds)
	padID := "padSettingsOther"
	creator := joinWithSettings(t, h, hub, cfg, ds, padID, "pocreator", nil)
	other := joinWithSettings(t, h, hub, cfg, ds, padID, "poother", nil)
	frames(creator.client)

	sendPadOptions(h, cfg, ds, other, map[string]any{"showChat": false})
	got, shouts := padOptionsReceived(other.client)
	assert.Empty(t, got)
	assert.Equal(t, []string{"Only the pad creator can change pad settings"}, shouts)
	got, _ = padOptionsReceived(creator.client)
	assert.Empty(t, got, "nothing is broadcast")

	ds.PadManager.UnloadPad(padID)
	later := joinWithSettings(t, h, hub, cfg, ds, padID, "polater", nil)
	assert.Equal(t, true, initialOptions(t, later)["showChat"], "nothing is stored")
}

func testPadSettingsDisabled(t *testing.T, ds testutils.TestDataStore) {
	h, hub := ds.PadMessageHandler, ds.Hub
	cfg := *padSettingsConfig(ds)
	cfg.EnablePadWideSettings = false
	padID := "padSettingsDisabled"
	creator := joinWithSettings(t, h, hub, &cfg, ds, padID, "pdcreator", map[string]any{"showChat": false})
	assert.Equal(t, false, creator.vars["canEditPadSettings"])
	assert.Equal(t, true, initialOptions(t, creator)["showChat"], "creator defaults are ignored")

	sendPadOptions(h, &cfg, ds, creator, map[string]any{"showChat": false})
	got, shouts := padOptionsReceived(creator.client)
	assert.Empty(t, got)
	assert.Empty(t, shouts)
	ds.PadManager.UnloadPad(padID)
	later := joinWithSettings(t, h, hub, &cfg, ds, padID, "pdlater", nil)
	assert.Equal(t, true, initialOptions(t, later)["showChat"])
}

func testPadSettingsDefaults(t *testing.T, ds testutils.TestDataStore) {
	h, hub, cfg := ds.PadMessageHandler, ds.Hub, padSettingsConfig(ds)
	padID := "padSettingsDefaults"
	creator := joinWithSettings(t, h, hub, cfg, ds, padID, "pdfcreator",
		map[string]any{"alwaysShowChat": true, "view": map[string]any{"padFontFamily": "RobotoMono"}})
	opts := initialOptions(t, creator)
	assert.Equal(t, true, opts["alwaysShowChat"])
	assert.Equal(t, "RobotoMono", view(opts)["padFontFamily"])

	// Joining an existing pad never changes its settings.
	other := joinWithSettings(t, h, hub, cfg, ds, padID, "pdfother",
		map[string]any{"alwaysShowChat": false, "view": map[string]any{"padFontFamily": "OpenDyslexic"}})
	opts = initialOptions(t, other)
	assert.Equal(t, true, opts["alwaysShowChat"])
	assert.Equal(t, "RobotoMono", view(opts)["padFontFamily"])
}

// A change made through one instance reaches the clients of another, and a
// client joining there later gets the stored settings (the cached pad there
// is refreshed because its metadata no longer matches).
func TestPadWideSettingsAcrossInstances(t *testing.T) {
	testutils.RunSequentialMemorySQLite(t, testutils.TestRunConfig{
		Name: "pad-wide settings between two instances",
		Test: testPadSettingsAcrossInstances,
	})
}

func testPadSettingsAcrossInstances(t *testing.T, ds testutils.TestDataStore) {
	busA, busB := newBusPair()
	hubB := libws.NewHub()
	go hubB.Run()
	sessB := libws.NewSessionStore()
	managerB := pad.NewManager(ds.DS, ds.Hooks)
	handlerB := libws.NewPadMessageHandler(ds.DS, ds.Hooks, managerB, &sessB, hubB, ds.Logger, testutils.GetTestAssets(), ds.Settings)
	ds.PadMessageHandler.SetCluster(busA)
	handlerB.SetCluster(busB)
	t.Cleanup(func() { ds.PadMessageHandler.StopCluster(); handlerB.StopCluster() })
	cfg := padSettingsConfig(ds)
	padID := "padSettingsCluster"

	creator := joinWithSettings(t, ds.PadMessageHandler, ds.Hub, cfg, ds, padID, "pccreator", nil)
	onB := joinWithSettings(t, handlerB, hubB, cfg, ds, padID, "pcb", nil)
	assert.Equal(t, true, initialOptions(t, onB)["showChat"])
	frames(creator.client)

	sendPadOptions(ds.PadMessageHandler, cfg, ds, creator, map[string]any{"showChat": false, "enforceSettings": true})

	var got []map[string]any
	require.Eventually(t, func() bool {
		more, _ := padOptionsReceived(onB.client)
		got = append(got, more...)
		return len(got) > 0
	}, 5*time.Second, 20*time.Millisecond, "the client on B hears the change made on A")
	assert.Equal(t, false, got[0]["showChat"])

	laterOnB := joinWithSettings(t, handlerB, hubB, cfg, ds, padID, "pcblater", nil)
	opts := initialOptions(t, laterOnB)
	assert.Equal(t, false, opts["showChat"], "B's cached pad was refreshed")
	assert.Equal(t, true, opts["enforceSettings"])
	encoded, _ := json.Marshal(opts)
	t.Logf("settings on B: %s", encoded)
}

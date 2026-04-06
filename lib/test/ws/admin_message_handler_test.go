package ws

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ether/etherpad-go/lib/db"
	db2 "github.com/ether/etherpad-go/lib/models/db"
	"github.com/ether/etherpad-go/lib/models/ws/admin"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/ether/etherpad-go/lib/ws"
	"github.com/stretchr/testify/assert"
)

func TestAdminMessageHandler_AllMethods(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)

	testDb.AddTests(testutils.TestRunConfig{
		Name: "Handle load of settings",
		Test: testHandleLoadSettings,
	},
		testutils.TestRunConfig{
			Name: "Handle create pad with existing pad",
			Test: testHandleCreatePadWithExistingPad,
		},
		testutils.TestRunConfig{
			Name: "Handle create pad with no existing pad",
			Test: testHandleCreatePadWithNoExistingPad,
		},
		testutils.TestRunConfig{
			Name: "Handle create pad with loading a pad",
			Test: testHandlePadLoad,
		},
		testutils.TestRunConfig{
			Name: "Test get installed plugins",
			Test: testGetInstalled,
		},
		testutils.TestRunConfig{
			Name: "Handle delete pad that does not exist",
			Test: testHandleDeletePadNotExisting,
		},
		testutils.TestRunConfig{
			Name: "Handle delete pad",
			Test: testHandleDeletePad,
		},
		testutils.TestRunConfig{
			Name: "Handle create pad with loading a pad with exact pattern",
			Test: testHandlePadLoadExactPattern,
		},
		testutils.TestRunConfig{
			Name: "Handle create pad with loading a pad with fuzzy pattern",
			Test: testHandlePadLoadFuzzyPattern,
		},
		testutils.TestRunConfig{
			Name: "Handle shout message",
			Test: testHandleShout,
		},
		testutils.TestRunConfig{
			Name: "Handle checkUpdates message",
			Test: testHandleCheckUpdates,
		},
		testutils.TestRunConfig{
			Name: "Handle getConnections",
			Test: testGetConnections,
		},
		testutils.TestRunConfig{
			Name: "Handle getSystemInfo",
			Test: testGetSystemInfo,
		},
		testutils.TestRunConfig{
			Name: "Handle getPadContent",
			Test: testGetPadContent,
		},
		testutils.TestRunConfig{
			Name: "Handle searchPadContent",
			Test: testSearchPadContent,
		},
		testutils.TestRunConfig{
			Name: "Handle bulkDeletePads",
			Test: testBulkDeletePads,
		},
		testutils.TestRunConfig{
			Name: "Handle kickUser",
			Test: testKickUser,
		},
		testutils.TestRunConfig{
			Name: "Handle saveSettings",
			Test: testSaveSettings,
		},
	)
	testDb.StartTestDBHandler()
}

func testHandleLoadSettings(t *testing.T, ds testutils.TestDataStore) {
	message := admin.EventMessage{
		Event: "load",
	}
	settingsToLoad := settings.Displayed
	hub := ws.NewHub()

	client := &ws.Client{
		Hub:       hub,
		Conn:      ds.MockWebSocket,
		Send:      make(chan []byte, 256),
		Room:      "test-pad",
		SessionId: "session123",
		Handler:   nil,
	}

	// Start mock write pump to forward messages from Send channel to MockWebSocket
	wg := startMockWritePump(client, ds.MockWebSocket)

	ds.AdminMessageHandler.HandleMessage(message, &settingsToLoad, client)

	// Wait for the mock write pump to process the message
	wg.Wait()

	assert.Len(t, ds.MockWebSocket.Data, 1)
	var response []interface{}
	assert.NoError(t, json.Unmarshal(ds.MockWebSocket.Data[0].Data, &response))
	assert.Equal(t, "settings", response[0])
	var returnedSettings map[string]interface{}
	settingsBytes, err := json.Marshal(response[1])
	assert.NoError(t, err)
	assert.NoError(t, json.Unmarshal(settingsBytes, &returnedSettings))
	settingsLoaded := returnedSettings["results"]
	assert.NotNil(t, settingsLoaded)
}

func testHandleCreatePadWithExistingPad(t *testing.T, ds testutils.TestDataStore) {
	hub := ws.NewHub()
	settingsToLoad := settings.Displayed
	client := &ws.Client{
		Hub:       hub,
		Conn:      ds.MockWebSocket,
		Send:      make(chan []byte, 256),
		Room:      "test-pad",
		SessionId: "session123",
		Handler:   nil,
	}

	padCreateMessage := admin.PadCreateData{
		PadName: "test",
	}
	data, err := json.Marshal(padCreateMessage)
	assert.NoError(t, err)
	padAdminMessage := admin.EventMessage{
		Event: "createPad",
		Data:  data,
	}
	createdPad, err := ds.PadManager.GetPad("test", nil, nil)
	assert.NoError(t, err)
	assert.NotNil(t, createdPad)

	// Start mock write pump right before HandleMessage
	wg := startMockWritePump(client, ds.MockWebSocket)

	ds.AdminMessageHandler.HandleMessage(padAdminMessage, &settingsToLoad, client)
	wg.Wait()

	assert.Len(t, ds.MockWebSocket.Data, 1)
	var resp = make([]interface{}, 2)
	assert.NoError(t, json.Unmarshal(ds.MockWebSocket.Data[0].Data, &resp))
	assert.Equal(t, "results:createPad", resp[0])
	adminErrorMessage := resp[1].(map[string]interface{})
	assert.NoError(t, err)
	assert.Equal(t, "Pad already exists", adminErrorMessage["error"])
}

func testHandleShout(t *testing.T, ds testutils.TestDataStore) {
	hub := ws.NewHub()
	settingsToLoad := settings.Displayed
	client := &ws.Client{
		Hub:       hub,
		Conn:      ds.MockWebSocket,
		Send:      make(chan []byte, 256),
		Room:      "test-pad",
		SessionId: "session123",
		Handler:   nil,
	}

	// Start mock write pump
	wg := startMockWritePump(client, ds.MockWebSocket)

	ds.Hub.Clients[client] = true
	shoutMessageRequest := admin.ShoutMessageRequest{
		Message: "This is a shout message",
		Sticky:  false,
	}
	data, err := json.Marshal(shoutMessageRequest)
	assert.NoError(t, err)
	padAdminMessage := admin.EventMessage{
		Event: "shout",
		Data:  data,
	}

	ds.AdminMessageHandler.HandleMessage(padAdminMessage, &settingsToLoad, client)
	wg.Wait()

	assert.Len(t, ds.MockWebSocket.Data, 1)
	var resp = make([]interface{}, 2)
	assert.NoError(t, json.Unmarshal(ds.MockWebSocket.Data[0].Data, &resp))
	assert.Equal(t, "result:shout", resp[0])
	var adminSuccessMessage admin.ShoutMessageResponse
	respBytes, err := json.Marshal(resp[1])
	assert.NoError(t, err)
	assert.NoError(t, json.Unmarshal(respBytes, &adminSuccessMessage))

	assert.Equal(t, "This is a shout message", adminSuccessMessage.Data.Payload.Message.Message)
	assert.Equal(t, false, adminSuccessMessage.Data.Payload.Message.Sticky)
}

func testHandleCreatePadWithNoExistingPad(t *testing.T, ds testutils.TestDataStore) {
	hub := ws.NewHub()
	settingsToLoad := settings.Displayed
	client := &ws.Client{
		Hub:       hub,
		Conn:      ds.MockWebSocket,
		Send:      make(chan []byte, 256),
		Room:      "test-pad",
		SessionId: "session123",
		Handler:   nil,
	}

	// Start mock write pump
	wg := startMockWritePump(client, ds.MockWebSocket)

	padCreateMessage := admin.PadCreateData{
		PadName: "test",
	}
	data, err := json.Marshal(padCreateMessage)
	assert.NoError(t, err)
	padAdminMessage := admin.EventMessage{
		Event: "createPad",
		Data:  data,
	}

	ds.AdminMessageHandler.HandleMessage(padAdminMessage, &settingsToLoad, client)
	wg.Wait()

	assert.Len(t, ds.MockWebSocket.Data, 1)
	var resp = make([]interface{}, 2)
	assert.NoError(t, json.Unmarshal(ds.MockWebSocket.Data[0].Data, &resp))
	assert.Equal(t, "results:createPad", resp[0])
	adminErrorMessage := resp[1].(map[string]interface{})
	assert.NoError(t, err)
	assert.Equal(t, "Pad created test", adminErrorMessage["success"])
}

func testHandlePadLoad(t *testing.T, ds testutils.TestDataStore) {
	hub := ws.NewHub()
	settingsToLoad := settings.Displayed
	client := &ws.Client{
		Hub:       hub,
		Conn:      ds.MockWebSocket,
		Send:      make(chan []byte, 256),
		Room:      "test-pad",
		SessionId: "session123",
		Handler:   nil,
	}

	// Start mock write pump
	wg := startMockWritePump(client, ds.MockWebSocket)

	padCreateMessage := admin.PadLoadData{
		Limit:     10,
		Offset:    0,
		Pattern:   "test",
		SortBy:    "padName",
		Ascending: true,
	}
	data, err := json.Marshal(padCreateMessage)
	assert.NoError(t, err)
	padAdminMessage := admin.EventMessage{
		Event: "padLoad",
		Data:  data,
	}

	ds.AdminMessageHandler.HandleMessage(padAdminMessage, &settingsToLoad, client)
	wg.Wait()

	assert.Len(t, ds.MockWebSocket.Data, 1)
	var resp = make([]interface{}, 2)
	assert.NoError(t, json.Unmarshal(ds.MockWebSocket.Data[0].Data, &resp))
	assert.Equal(t, "results:padLoad", resp[0])
	adminErrorMessage := resp[1].(map[string]interface{})
	assert.NoError(t, err)
	assert.Len(t, adminErrorMessage["results"], 0)
	assert.Equal(t, adminErrorMessage["total"], float64(0))
}

func testHandlePadLoadExactPattern(t *testing.T, ds testutils.TestDataStore) {
	hub := ws.NewHub()
	settingsToLoad := settings.Displayed
	randomPad := db.CreateRandomPad()

	assert.NoError(t, ds.DS.CreatePad("test", randomPad))
	assert.NoError(t, ds.DS.SaveRevision("test", 1, "123", db2.AText{
		Text:    randomPad.ATextText,
		Attribs: randomPad.ATextAttribs,
	}, db2.RevPool{}, nil, 123))

	client := &ws.Client{
		Hub:       hub,
		Conn:      ds.MockWebSocket,
		Send:      make(chan []byte, 256),
		Room:      "test-pad",
		SessionId: "session123",
		Handler:   nil,
	}

	// Start mock write pump
	wg := startMockWritePump(client, ds.MockWebSocket)

	padCreateMessage := admin.PadLoadData{
		Limit:     10,
		Offset:    0,
		Pattern:   "test",
		SortBy:    "padName",
		Ascending: true,
	}
	data, err := json.Marshal(padCreateMessage)
	assert.NoError(t, err)
	padAdminMessage := admin.EventMessage{
		Event: "padLoad",
		Data:  data,
	}

	ds.AdminMessageHandler.HandleMessage(padAdminMessage, &settingsToLoad, client)
	wg.Wait()

	assert.Len(t, ds.MockWebSocket.Data, 1)
	var resp = make([]interface{}, 2)
	assert.NoError(t, json.Unmarshal(ds.MockWebSocket.Data[0].Data, &resp))
	assert.Equal(t, "results:padLoad", resp[0])
	adminErrorMessage := resp[1].(map[string]interface{})
	assert.NoError(t, err)
	assert.Len(t, adminErrorMessage["results"], 1)
	assert.Equal(t, adminErrorMessage["total"], float64(1))
}

func testHandlePadLoadFuzzyPattern(t *testing.T, ds testutils.TestDataStore) {
	hub := ws.NewHub()
	settingsToLoad := settings.Displayed
	randomPad := db.CreateRandomPad()

	assert.NoError(t, ds.DS.CreatePad("test123", randomPad))
	assert.NoError(t, ds.DS.SaveRevision("test123", 1, "123", db2.AText{
		Text:    randomPad.ATextText,
		Attribs: randomPad.ATextAttribs,
	}, db2.RevPool{}, nil, 123))

	client := &ws.Client{
		Hub:       hub,
		Conn:      ds.MockWebSocket,
		Send:      make(chan []byte, 256),
		Room:      "test-pad",
		SessionId: "session123",
		Handler:   nil,
	}

	// Start mock write pump
	wg := startMockWritePump(client, ds.MockWebSocket)

	padCreateMessage := admin.PadLoadData{
		Limit:     10,
		Offset:    0,
		Pattern:   "test",
		SortBy:    "padName",
		Ascending: true,
	}
	data, err := json.Marshal(padCreateMessage)
	assert.NoError(t, err)
	padAdminMessage := admin.EventMessage{
		Event: "padLoad",
		Data:  data,
	}

	ds.AdminMessageHandler.HandleMessage(padAdminMessage, &settingsToLoad, client)
	wg.Wait()

	assert.Len(t, ds.MockWebSocket.Data, 1)
	var resp = make([]interface{}, 2)
	assert.NoError(t, json.Unmarshal(ds.MockWebSocket.Data[0].Data, &resp))
	assert.Equal(t, "results:padLoad", resp[0])
	adminErrorMessage := resp[1].(map[string]interface{})
	assert.NoError(t, err)
	assert.Len(t, adminErrorMessage["results"], 1)
	assert.Equal(t, adminErrorMessage["total"], float64(1))
}

func testGetInstalled(t *testing.T, ds testutils.TestDataStore) {
	hub := ws.NewHub()
	settingsToLoad := settings.Displayed
	client := &ws.Client{
		Hub:       hub,
		Conn:      ds.MockWebSocket,
		Send:      make(chan []byte, 256),
		Room:      "test-pad",
		SessionId: "session123",
		Handler:   nil,
	}

	// Start mock write pump
	wg := startMockWritePump(client, ds.MockWebSocket)

	getInstalledRequest := admin.EventMessage{
		Event: "getInstalled",
		Data:  make(json.RawMessage, 0),
	}
	ds.AdminMessageHandler.HandleMessage(getInstalledRequest, &settingsToLoad, client)
	wg.Wait()

	assert.Len(t, ds.MockWebSocket.Data, 1)
	var resp = make([]interface{}, 2)
	assert.NoError(t, json.Unmarshal(ds.MockWebSocket.Data[0].Data, &resp))
	assert.Equal(t, "results:installed", resp[0])
	adminErrorMessage := resp[1].(map[string]interface{})
	assert.Len(t, adminErrorMessage["installed"], 15)
}

func testHandleDeletePadNotExisting(t *testing.T, ds testutils.TestDataStore) {
	hub := ws.NewHub()
	settingsToLoad := settings.Displayed
	client := &ws.Client{
		Hub:       hub,
		Conn:      ds.MockWebSocket,
		Send:      make(chan []byte, 256),
		Room:      "test-pad",
		SessionId: "session123",
		Handler:   nil,
	}
	padDelete := "nonExistingPad"
	data, err := json.Marshal(padDelete)
	assert.NoError(t, err)
	padAdminMessage := admin.EventMessage{
		Event: "deletePad",
		Data:  data,
	}

	ds.AdminMessageHandler.HandleMessage(padAdminMessage, &settingsToLoad, client)
	assert.Len(t, ds.MockWebSocket.Data, 0)
}

func testHandleDeletePad(t *testing.T, ds testutils.TestDataStore) {
	hub := ws.NewHub()
	settingsToLoad := settings.Displayed
	client := &ws.Client{
		Hub:       hub,
		Conn:      ds.MockWebSocket,
		Send:      make(chan []byte, 256),
		Room:      "test-pad",
		SessionId: "session123",
		Handler:   nil,
	}

	padDelete := "existingPad"
	createdPad, err := ds.PadManager.GetPad(padDelete, nil, nil)
	assert.NoError(t, err)
	assert.NotNil(t, createdPad)
	data, err := json.Marshal(padDelete)
	assert.NoError(t, err)
	padAdminMessage := admin.EventMessage{
		Event: "deletePad",
		Data:  data,
	}

	// Start mock write pump right before HandleMessage
	wg := startMockWritePump(client, ds.MockWebSocket)

	ds.AdminMessageHandler.HandleMessage(padAdminMessage, &settingsToLoad, client)
	wg.Wait()

	assert.Len(t, ds.MockWebSocket.Data, 1)
	var resp = make([]interface{}, 2)
	assert.NoError(t, json.Unmarshal(ds.MockWebSocket.Data[0].Data, &resp))
	assert.Equal(t, "results:deletePad", resp[0])
	assert.Equal(t, "existingPad", resp[1])
}

func testGetConnections(t *testing.T, ds testutils.TestDataStore) {
	hub := ws.NewHub()
	settingsToLoad := settings.Displayed
	client := &ws.Client{
		Hub:       hub,
		Conn:      ds.MockWebSocket,
		Send:      make(chan []byte, 256),
		Room:      "test-pad",
		SessionId: "session123",
		Handler:   nil,
	}

	wg := startMockWritePump(client, ds.MockWebSocket)

	getConnectionsMessage := admin.EventMessage{
		Event: "getConnections",
		Data:  json.RawMessage(`{}`),
	}
	ds.AdminMessageHandler.HandleMessage(getConnectionsMessage, &settingsToLoad, client)
	wg.Wait()

	assert.Len(t, ds.MockWebSocket.Data, 1)
	var resp = make([]interface{}, 2)
	assert.NoError(t, json.Unmarshal(ds.MockWebSocket.Data[0].Data, &resp))
	assert.Equal(t, "results:getConnections", resp[0])
	resultMap := resp[1].(map[string]interface{})
	_, hasConnections := resultMap["connections"]
	assert.True(t, hasConnections)
}

func testGetSystemInfo(t *testing.T, ds testutils.TestDataStore) {
	hub := ws.NewHub()
	settingsToLoad := settings.Displayed
	client := &ws.Client{
		Hub:       hub,
		Conn:      ds.MockWebSocket,
		Send:      make(chan []byte, 256),
		Room:      "test-pad",
		SessionId: "session123",
		Handler:   nil,
	}

	wg := startMockWritePump(client, ds.MockWebSocket)

	getSystemInfoMessage := admin.EventMessage{
		Event: "getSystemInfo",
		Data:  json.RawMessage(`{}`),
	}
	ds.AdminMessageHandler.HandleMessage(getSystemInfoMessage, &settingsToLoad, client)
	wg.Wait()

	assert.Len(t, ds.MockWebSocket.Data, 1)
	var resp = make([]interface{}, 2)
	assert.NoError(t, json.Unmarshal(ds.MockWebSocket.Data[0].Data, &resp))
	assert.Equal(t, "results:getSystemInfo", resp[0])
	resultMap := resp[1].(map[string]interface{})
	_, hasMemAlloc := resultMap["memAlloc"]
	assert.True(t, hasMemAlloc)
	_, hasNumGoroutine := resultMap["numGoroutine"]
	assert.True(t, hasNumGoroutine)
	_, hasGoVersion := resultMap["goVersion"]
	assert.True(t, hasGoVersion)
	_, hasNumCPU := resultMap["numCPU"]
	assert.True(t, hasNumCPU)
	assert.Greater(t, resultMap["numGoroutine"].(float64), float64(0))
	assert.NotEmpty(t, resultMap["goVersion"])
}

func testGetPadContent(t *testing.T, ds testutils.TestDataStore) {
	padText := "This is the content of testpad123"
	_, err := ds.PadManager.GetPad("testpad123", &padText, nil)
	assert.NoError(t, err)

	hub := ws.NewHub()
	settingsToLoad := settings.Displayed
	client := &ws.Client{
		Hub:       hub,
		Conn:      ds.MockWebSocket,
		Send:      make(chan []byte, 256),
		Room:      "test-pad",
		SessionId: "session123",
		Handler:   nil,
	}

	wg := startMockWritePump(client, ds.MockWebSocket)

	padNameData, err := json.Marshal("testpad123")
	assert.NoError(t, err)
	getPadContentMessage := admin.EventMessage{
		Event: "getPadContent",
		Data:  padNameData,
	}
	ds.AdminMessageHandler.HandleMessage(getPadContentMessage, &settingsToLoad, client)
	wg.Wait()

	assert.Len(t, ds.MockWebSocket.Data, 1)
	var resp = make([]interface{}, 2)
	assert.NoError(t, json.Unmarshal(ds.MockWebSocket.Data[0].Data, &resp))
	assert.Equal(t, "results:getPadContent", resp[0])
	resultMap := resp[1].(map[string]interface{})
	assert.Equal(t, "testpad123", resultMap["padId"])
	assert.Contains(t, resultMap["content"], padText)
}

func testSearchPadContent(t *testing.T, ds testutils.TestDataStore) {
	padText := "Hello unique search term World"
	_, err := ds.PadManager.GetPad("searchpad1", &padText, nil)
	assert.NoError(t, err)

	hub := ws.NewHub()
	settingsToLoad := settings.Displayed
	client := &ws.Client{
		Hub:       hub,
		Conn:      ds.MockWebSocket,
		Send:      make(chan []byte, 256),
		Room:      "test-pad",
		SessionId: "session123",
		Handler:   nil,
	}

	wg := startMockWritePump(client, ds.MockWebSocket)

	searchMessage := admin.EventMessage{
		Event: "searchPadContent",
		Data:  json.RawMessage(`{"query":"unique search term","limit":10}`),
	}
	ds.AdminMessageHandler.HandleMessage(searchMessage, &settingsToLoad, client)
	wg.Wait()

	assert.Len(t, ds.MockWebSocket.Data, 1)
	var resp = make([]interface{}, 2)
	assert.NoError(t, json.Unmarshal(ds.MockWebSocket.Data[0].Data, &resp))
	assert.Equal(t, "results:searchPadContent", resp[0])
	resultMap := resp[1].(map[string]interface{})
	results := resultMap["results"].([]interface{})
	assert.GreaterOrEqual(t, len(results), 1)
	firstResult := results[0].(map[string]interface{})
	assert.NotEmpty(t, firstResult["padId"])
	assert.True(t, strings.Contains(firstResult["snippet"].(string), "unique search term"))
}

func testBulkDeletePads(t *testing.T, ds testutils.TestDataStore) {
	padText1 := "bulk delete pad 1"
	_, err := ds.PadManager.GetPad("bulkdelete1", &padText1, nil)
	assert.NoError(t, err)
	padText2 := "bulk delete pad 2"
	_, err = ds.PadManager.GetPad("bulkdelete2", &padText2, nil)
	assert.NoError(t, err)

	hub := ws.NewHub()
	settingsToLoad := settings.Displayed
	client := &ws.Client{
		Hub:       hub,
		Conn:      ds.MockWebSocket,
		Send:      make(chan []byte, 256),
		Room:      "test-pad",
		SessionId: "session123",
		Handler:   nil,
	}

	wg := startMockWritePump(client, ds.MockWebSocket)

	bulkDeleteMessage := admin.EventMessage{
		Event: "bulkDeletePads",
		Data:  json.RawMessage(`{"padNames":["bulkdelete1","bulkdelete2"]}`),
	}
	ds.AdminMessageHandler.HandleMessage(bulkDeleteMessage, &settingsToLoad, client)
	wg.Wait()

	assert.Len(t, ds.MockWebSocket.Data, 1)
	var resp = make([]interface{}, 2)
	assert.NoError(t, json.Unmarshal(ds.MockWebSocket.Data[0].Data, &resp))
	assert.Equal(t, "results:bulkDeletePads", resp[0])
	resultMap := resp[1].(map[string]interface{})
	assert.Equal(t, float64(2), resultMap["deleted"])

	// Verify pads no longer exist in the data store
	_, err = ds.DS.GetPad("bulkdelete1")
	assert.Error(t, err)
	_, err = ds.DS.GetPad("bulkdelete2")
	assert.Error(t, err)
}

func testKickUser(t *testing.T, ds testutils.TestDataStore) {
	hub := ws.NewHub()
	settingsToLoad := settings.Displayed
	client := &ws.Client{
		Hub:       hub,
		Conn:      ds.MockWebSocket,
		Send:      make(chan []byte, 256),
		Room:      "test-pad",
		SessionId: "session123",
		Handler:   nil,
	}

	wg := startMockWritePump(client, ds.MockWebSocket)

	kickMessage := admin.EventMessage{
		Event: "kickUser",
		Data:  json.RawMessage(`{"sessionId":"nonexistent"}`),
	}
	ds.AdminMessageHandler.HandleMessage(kickMessage, &settingsToLoad, client)
	wg.Wait()

	assert.Len(t, ds.MockWebSocket.Data, 1)
	var resp = make([]interface{}, 2)
	assert.NoError(t, json.Unmarshal(ds.MockWebSocket.Data[0].Data, &resp))
	assert.Equal(t, "results:kickUser", resp[0])
	resultMap := resp[1].(map[string]interface{})
	assert.Equal(t, true, resultMap["success"])
}

func testSaveSettings(t *testing.T, ds testutils.TestDataStore) {
	hub := ws.NewHub()
	settingsToLoad := settings.Displayed
	client := &ws.Client{
		Hub:       hub,
		Conn:      ds.MockWebSocket,
		Send:      make(chan []byte, 256),
		Room:      "test-pad",
		SessionId: "session123",
		Handler:   nil,
	}

	wg := startMockWritePump(client, ds.MockWebSocket)

	settingsJSON := `{"title":"Test"}`
	data, err := json.Marshal(settingsJSON)
	assert.NoError(t, err)
	saveSettingsMessage := admin.EventMessage{
		Event: "saveSettings",
		Data:  data,
	}
	ds.AdminMessageHandler.HandleMessage(saveSettingsMessage, &settingsToLoad, client)
	wg.Wait()

	assert.Len(t, ds.MockWebSocket.Data, 1)
	var resp = make([]interface{}, 2)
	assert.NoError(t, json.Unmarshal(ds.MockWebSocket.Data[0].Data, &resp))
	assert.Equal(t, "results:saveSettings", resp[0])
	resultMap := resp[1].(map[string]interface{})
	assert.Equal(t, true, resultMap["success"])
}

func testHandleCheckUpdates(t *testing.T, ds testutils.TestDataStore) {
	hub := ws.NewHub()
	settingsToLoad := settings.Displayed
	settingsToLoad.GitVersion = "v1.0.0"

	client := &ws.Client{
		Hub:       hub,
		Conn:      ds.MockWebSocket,
		Send:      make(chan []byte, 256),
		Room:      "test-pad",
		SessionId: "session123",
		Handler:   nil,
	}

	// Case 1: No version in DB
	checkUpdatesMessage := admin.EventMessage{
		Event: "checkUpdates",
	}

	wg := startMockWritePump(client, ds.MockWebSocket)
	ds.AdminMessageHandler.HandleMessage(checkUpdatesMessage, &settingsToLoad, client)
	wg.Wait()

	assert.Len(t, ds.MockWebSocket.Data, 0)

	// Case 2: Newer version in DB
	assert.NoError(t, ds.DS.SaveServerVersion("v1.1.0"))
	ds.MockWebSocket.Data = nil // clear data

	wg = startMockWritePump(client, ds.MockWebSocket)
	ds.AdminMessageHandler.HandleMessage(checkUpdatesMessage, &settingsToLoad, client)
	wg.Wait()
	var resp []interface{}
	assert.NoError(t, json.Unmarshal(ds.MockWebSocket.Data[0].Data, &resp))
	assert.Equal(t, "results:checkUpdates", resp[0])
	result := resp[1].(map[string]interface{})
	assert.Equal(t, "v1.0.0", result["currentVersion"])
	assert.Equal(t, "v1.1.0", result["latestVersion"])
	assert.Equal(t, true, result["updateAvailable"])

	assert.Len(t, ds.MockWebSocket.Data, 1)
	assert.NoError(t, json.Unmarshal(ds.MockWebSocket.Data[0].Data, &resp))
	result = resp[1].(map[string]interface{})
	assert.Equal(t, "v1.1.0", result["latestVersion"])
	assert.Equal(t, true, result["updateAvailable"])
}

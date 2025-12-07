package ws

import (
	"encoding/json"
	"testing"

	"github.com/ether/etherpad-go/lib/db"
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
		Ctx:       nil,
		Handler:   nil,
	}

	ds.AdminMessageHandler.HandleMessage(message, &settingsToLoad, client)
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
		Ctx:       nil,
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

	ds.AdminMessageHandler.HandleMessage(padAdminMessage, &settingsToLoad, client)
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
		Ctx:       nil,
		Handler:   nil,
	}

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
		Ctx:       nil,
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

	ds.AdminMessageHandler.HandleMessage(padAdminMessage, &settingsToLoad, client)
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
		Ctx:       nil,
		Handler:   nil,
	}
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
	assert.NoError(t, ds.DS.SaveRevision("test", 1, "123", randomPad.AText, randomPad.Pool, nil, 123))

	client := &ws.Client{
		Hub:       hub,
		Conn:      ds.MockWebSocket,
		Send:      make(chan []byte, 256),
		Room:      "test-pad",
		SessionId: "session123",
		Ctx:       nil,
		Handler:   nil,
	}
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
	assert.NoError(t, ds.DS.SaveRevision("test123", 1, "123", randomPad.AText, randomPad.Pool, nil, 123))

	client := &ws.Client{
		Hub:       hub,
		Conn:      ds.MockWebSocket,
		Send:      make(chan []byte, 256),
		Room:      "test-pad",
		SessionId: "session123",
		Ctx:       nil,
		Handler:   nil,
	}
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
		Ctx:       nil,
		Handler:   nil,
	}
	getInstalledRequest := admin.EventMessage{
		Event: "getInstalled",
		Data:  make(json.RawMessage, 0),
	}
	ds.AdminMessageHandler.HandleMessage(getInstalledRequest, &settingsToLoad, client)
	assert.Len(t, ds.MockWebSocket.Data, 1)
	var resp = make([]interface{}, 2)
	assert.NoError(t, json.Unmarshal(ds.MockWebSocket.Data[0].Data, &resp))
	assert.Equal(t, "results:installed", resp[0])
	adminErrorMessage := resp[1].(map[string]interface{})
	assert.Len(t, adminErrorMessage["installed"], 1)
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
		Ctx:       nil,
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
		Ctx:       nil,
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

	ds.AdminMessageHandler.HandleMessage(padAdminMessage, &settingsToLoad, client)
	assert.Len(t, ds.MockWebSocket.Data, 1)
	var resp = make([]interface{}, 2)
	assert.NoError(t, json.Unmarshal(ds.MockWebSocket.Data[0].Data, &resp))
	assert.Equal(t, "results:deletePad", resp[0])
	assert.Equal(t, "existingPad", resp[1])
}

func testHandleCleanupRevisions(t *testing.T, ds testutils.TestDataStore) {

}

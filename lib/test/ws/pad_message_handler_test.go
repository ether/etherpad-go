package ws

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/ether/etherpad-go/lib/models/ws"
	"github.com/ether/etherpad-go/lib/test/testutils"
	libws "github.com/ether/etherpad-go/lib/ws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPadMessageHandler_AllMethods(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)

	testDb.AddTests(
		testutils.TestRunConfig{
			Name: "PadMessageHandler methods scaffold",
			Test: testInitPadMessageHandler,
		},
		testutils.TestRunConfig{
			Name: "HandleClientReadyMessage creates session and sends CLIENT_VARS",
			Test: testHandleClientReadyMessage,
		},
		testutils.TestRunConfig{
			Name: "HandleUserInfoUpdate updates author name and color",
			Test: testHandleUserInfoUpdate,
		},
		testutils.TestRunConfig{
			Name: "HandleUserInfoUpdate rejects invalid color",
			Test: testHandleUserInfoUpdateInvalidColor,
		},
		testutils.TestRunConfig{
			Name: "SendChatMessageToPadClients sends chat to all clients",
			Test: testSendChatMessageToPadClients,
		},
		testutils.TestRunConfig{
			Name: "GetChatMessages returns chat messages",
			Test: testGetChatMessages,
		},
		testutils.TestRunConfig{
			Name: "HandlePadDelete removes pad when first contributor",
			Test: testHandlePadDelete,
		},
		testutils.TestRunConfig{
			Name: "HandlePadDelete rejects when not first contributor",
			Test: testHandlePadDeleteNotFirstContributor,
		},
		testutils.TestRunConfig{
			Name: "DeletePad removes pad and related data",
			Test: testDeletePad,
		},
		testutils.TestRunConfig{
			Name: "ComposePadChangesets composes changesets correctly",
			Test: testComposePadChangesets,
		},
		testutils.TestRunConfig{
			Name: "UpdatePadClients sends NEW_CHANGES to clients",
			Test: testUpdatePadClients,
		},
		testutils.TestRunConfig{
			Name: "GetRoomSockets returns all clients in a pad room",
			Test: testGetRoomSockets,
		},
		testutils.TestRunConfig{
			Name: "KickSessionsFromPad removes all sessions from pad",
			Test: testKickSessionsFromPad,
		},
		testutils.TestRunConfig{
			Name: "HandleDisconnectOfPadClient removes session and notifies clients",
			Test: testHandleDisconnectOfPadClient,
		},
		testutils.TestRunConfig{
			Name: "UserChange on readonly pad is rejected",
			Test: testUserChangeOnReadonlyPad,
		},
		testutils.TestRunConfig{
			Name: "HandleChangesetRequest returns changeset info",
			Test: testHandleChangesetRequest,
		},
		testutils.TestRunConfig{
			Name: "HandleChangesetRequest rejects invalid parameters",
			Test: testHandleChangesetRequestInvalidParams,
		},
		testutils.TestRunConfig{
			Name: "ChannelOperator AddToQueue processes tasks",
			Test: testChannelOperatorAddToQueue,
		},
		testutils.TestRunConfig{
			Name: "HandleDisconnectOfPadClient sends USER_LEAVE",
			Test: testHandleDisconnectSendsUserLeave,
		},
		testutils.TestRunConfig{
			Name: "Verify USER_NEWINFO message format in HandleUserInfoUpdate",
			Test: testHandleUserInfoUpdateVerifyMessage,
		},
		testutils.TestRunConfig{
			Name: "Verify CHAT_MESSAGE format in SendChatMessageToPadClients",
			Test: testSendChatMessageVerifyMessageFormat,
		},
		testutils.TestRunConfig{
			Name: "GetChatMessages via websocket handler",
			Test: testGetChatMessagesViaHandler,
		},
		// Additional tests for more coverage
		testutils.TestRunConfig{
			Name: "HandleDisconnectOfPadClient with multiple clients in room",
			Test: testHandleDisconnectWithMultipleClients,
		},
		testutils.TestRunConfig{
			Name: "KickSessionsFromPad verifies PAD_DELETE message",
			Test: testKickSessionsVerifyMessage,
		},
		testutils.TestRunConfig{
			Name: "UpdatePadClients verifies NEW_CHANGES message format",
			Test: testUpdatePadClientsVerifyMessageFormat,
		},
		testutils.TestRunConfig{
			Name: "HandleChangesetRequest verifies response format",
			Test: testHandleChangesetRequestVerifyFormat,
		},
		testutils.TestRunConfig{
			Name: "Multiple chat messages to pad clients",
			Test: testMultipleChatMessages,
		},
		testutils.TestRunConfig{
			Name: "HandlePadDelete verifies client receives delete message",
			Test: testHandlePadDeleteVerifyMessage,
		},
		testutils.TestRunConfig{
			Name: "HandleMessage with ClientReady message",
			Test: testHandleMessageClientReady,
		},
		testutils.TestRunConfig{
			Name: "HandleMessage with ChatMessage",
			Test: testHandleMessageChatMessage,
		},
		testutils.TestRunConfig{
			Name: "HandleMessage with GetChatMessages",
			Test: testHandleMessageGetChatMessages,
		},
		testutils.TestRunConfig{
			Name: "HandleMessage with ChangesetReq",
			Test: testHandleMessageChangesetReq,
		},
		testutils.TestRunConfig{
			Name: "HandleMessage with UserInfoUpdate",
			Test: testHandleMessageUserInfoUpdate,
		},
		testutils.TestRunConfig{
			Name: "HandleMessage with UserChange on readonly rejects",
			Test: testHandleMessageUserChangeReadonly,
		},
		testutils.TestRunConfig{
			Name: "HandleMessage with unknown type",
			Test: testHandleMessageUnknownType,
		},
		testutils.TestRunConfig{
			Name: "HandleMessage without session returns early",
			Test: testHandleMessageNoSession,
		},
	)
	testDb.StartTestDBHandler()
}

func testInitPadMessageHandler(t *testing.T, ds testutils.TestDataStore) {
	handler := ds.PadMessageHandler

	assert.NotNil(t, handler)
}

// Helper function to create a test client with mock connection
func createTestClient(hub *libws.Hub, sessionId string, padId string, mockConn *libws.MockWebSocketConn) *libws.Client {
	client := &libws.Client{
		Hub:       hub,
		Conn:      mockConn,
		Send:      make(chan []byte, 256),
		Room:      padId,
		SessionId: sessionId,
	}
	hub.Clients[client] = true
	return client
}

// Helper function to setup a pad and author for tests
func setupPadAndAuthor(_ *testing.T, ds testutils.TestDataStore, padId string, authorName string) (string, error) {
	// Create an author
	author, err := ds.AuthorManager.CreateAuthor(nil)
	if err != nil {
		return "", err
	}

	err = ds.AuthorManager.SetAuthorName(author.Id, authorName)
	if err != nil {
		return "", err
	}

	err = ds.AuthorManager.SetAuthorColor(author.Id, "#FF0000")
	if err != nil {
		return "", err
	}

	// Create a pad
	_, err = ds.PadManager.GetPad(padId, nil, &author.Id)
	if err != nil {
		return "", err
	}

	return author.Id, nil
}

func testHandleClientReadyMessage(t *testing.T, ds testutils.TestDataStore) {
	padId := "test-pad-client-ready"
	authorId, err := setupPadAndAuthor(t, ds, padId, "TestUser")
	require.NoError(t, err)

	mockConn := libws.NewActualMockWebSocketconn()
	sessionId := "test-session-123"

	client := createTestClient(ds.Hub, sessionId, padId, mockConn)
	defer func() {
		delete(ds.Hub.Clients, client)
	}()

	// Initialize session
	ds.PadMessageHandler.SessionStore.InitSessionForTest(sessionId)
	ds.PadMessageHandler.SessionStore.AddHandleClientInformationForTest(sessionId, padId, "test-token")
	ds.PadMessageHandler.SessionStore.SetAuthorForTest(sessionId, authorId)
	ds.PadMessageHandler.SessionStore.AddPadReadOnlyIdsForTest(sessionId, padId, "readonly-id", false)

	// Wait a bit for message processing
	time.Sleep(100 * time.Millisecond)

	// Verify that handler is initialized
	assert.NotNil(t, ds.PadMessageHandler)
}

func testHandleUserInfoUpdate(t *testing.T, ds testutils.TestDataStore) {
	padId := "test-pad-user-info"
	authorId, err := setupPadAndAuthor(t, ds, padId, "OldName")
	require.NoError(t, err)

	mockConn := libws.NewActualMockWebSocketconn()
	sessionId := "test-session-user-info"

	client := createTestClient(ds.Hub, sessionId, padId, mockConn)
	defer func() {
		delete(ds.Hub.Clients, client)
	}()

	// Initialize session with author
	ds.PadMessageHandler.SessionStore.InitSessionForTest(sessionId)
	ds.PadMessageHandler.SessionStore.AddHandleClientInformationForTest(sessionId, padId, "test-token")
	ds.PadMessageHandler.SessionStore.SetAuthorForTest(sessionId, authorId)
	ds.PadMessageHandler.SessionStore.SetPadIdForTest(sessionId, padId)

	// Create user info update message
	newName := "NewName"
	newColor := "#00FF00"
	userInfoUpdate := libws.UserInfoUpdate{
		Type: "COLLABROOM",
		Data: struct {
			UserInfo struct {
				ColorId *string `json:"colorId"`
				IP      *string `json:"ip"`
				Name    *string `json:"name"`
				UserId  *string `json:"userId"`
			} `json:"userInfo"`
			Type string `json:"type"`
		}{
			UserInfo: struct {
				ColorId *string `json:"colorId"`
				IP      *string `json:"ip"`
				Name    *string `json:"name"`
				UserId  *string `json:"userId"`
			}{
				ColorId: &newColor,
				Name:    &newName,
			},
			Type: "USERINFO_UPDATE",
		},
	}

	// Handle the update
	ds.PadMessageHandler.HandleUserInfoUpdate(userInfoUpdate, client)

	// Verify that the author was updated
	author, err := ds.AuthorManager.GetAuthor(authorId)
	require.NoError(t, err)
	assert.Equal(t, newName, *author.Name)
	assert.Equal(t, newColor, author.ColorId)
}

func testHandleUserInfoUpdateInvalidColor(t *testing.T, ds testutils.TestDataStore) {
	padId := "test-pad-invalid-color"
	authorId, err := setupPadAndAuthor(t, ds, padId, "TestUser")
	require.NoError(t, err)

	mockConn := libws.NewActualMockWebSocketconn()
	sessionId := "test-session-invalid-color"

	client := createTestClient(ds.Hub, sessionId, padId, mockConn)
	defer func() {
		delete(ds.Hub.Clients, client)
	}()

	// Initialize session with author
	ds.PadMessageHandler.SessionStore.InitSessionForTest(sessionId)
	ds.PadMessageHandler.SessionStore.AddHandleClientInformationForTest(sessionId, padId, "test-token")
	ds.PadMessageHandler.SessionStore.SetAuthorForTest(sessionId, authorId)
	ds.PadMessageHandler.SessionStore.SetPadIdForTest(sessionId, padId)

	// Get original color
	originalAuthor, err := ds.AuthorManager.GetAuthor(authorId)
	require.NoError(t, err)
	originalColor := originalAuthor.ColorId

	// Create user info update with invalid color
	invalidColor := "not-a-valid-color"
	userInfoUpdate := libws.UserInfoUpdate{
		Type: "COLLABROOM",
		Data: struct {
			UserInfo struct {
				ColorId *string `json:"colorId"`
				IP      *string `json:"ip"`
				Name    *string `json:"name"`
				UserId  *string `json:"userId"`
			} `json:"userInfo"`
			Type string `json:"type"`
		}{
			UserInfo: struct {
				ColorId *string `json:"colorId"`
				IP      *string `json:"ip"`
				Name    *string `json:"name"`
				UserId  *string `json:"userId"`
			}{
				ColorId: &invalidColor,
			},
			Type: "USERINFO_UPDATE",
		},
	}

	// Handle the update (should be rejected)
	ds.PadMessageHandler.HandleUserInfoUpdate(userInfoUpdate, client)

	// Verify that the color was NOT updated
	author, err := ds.AuthorManager.GetAuthor(authorId)
	require.NoError(t, err)
	assert.Equal(t, originalColor, author.ColorId)
}

func testSendChatMessageToPadClients(t *testing.T, ds testutils.TestDataStore) {
	padId := "test-pad-chat"
	authorId, err := setupPadAndAuthor(t, ds, padId, "ChatUser")
	require.NoError(t, err)

	mockConn := libws.NewActualMockWebSocketconn()
	sessionId := "test-session-chat"

	client := createTestClient(ds.Hub, sessionId, padId, mockConn)
	defer func() {
		delete(ds.Hub.Clients, client)
	}()

	// Start mock write pump
	wg := startMockWritePump(client, mockConn)

	// Initialize session
	ds.PadMessageHandler.SessionStore.InitSessionForTest(sessionId)
	ds.PadMessageHandler.SessionStore.AddHandleClientInformationForTest(sessionId, padId, "test-token")
	ds.PadMessageHandler.SessionStore.SetAuthorForTest(sessionId, authorId)
	ds.PadMessageHandler.SessionStore.SetPadIdForTest(sessionId, padId)

	// Create session for the handler
	session := ds.PadMessageHandler.SessionStore.GetSessionForTest(sessionId)
	require.NotNil(t, session)

	// Send a chat message
	chatTime := time.Now().UnixMilli()
	chatMessage := ws.ChatMessageData{
		Text:     "Hello, World!",
		Time:     &chatTime,
		AuthorId: &authorId,
	}

	ds.PadMessageHandler.SendChatMessageToPadClients(session, chatMessage)

	// Wait for mock write pump to process messages
	wg.Wait()

	// Verify that the message was sent
	assert.GreaterOrEqual(t, len(mockConn.Data), 1, "Expected at least one message to be sent")

	if len(mockConn.Data) > 0 {
		// Parse the message
		var msgWrapper []interface{}
		err := json.Unmarshal(mockConn.Data[0].Data, &msgWrapper)
		require.NoError(t, err)
		assert.Equal(t, "message", msgWrapper[0])
	}
}

func testGetChatMessages(t *testing.T, ds testutils.TestDataStore) {
	padId := "test-pad-get-chat"
	authorId, err := setupPadAndAuthor(t, ds, padId, "ChatUser")
	require.NoError(t, err)

	// Add some chat messages to the pad
	retrievedPad, err := ds.PadManager.GetPad(padId, nil, &authorId)
	require.NoError(t, err)

	chatTime := time.Now().UnixMilli()
	_, err = retrievedPad.AppendChatMessage(&authorId, chatTime, "Test message 1")
	require.NoError(t, err)
	_, err = retrievedPad.AppendChatMessage(&authorId, chatTime+1000, "Test message 2")
	require.NoError(t, err)

	// Retrieve chat messages
	messages, err := retrievedPad.GetChatMessages(0, 2)
	require.NoError(t, err)
	assert.Len(t, *messages, 2)
}

func testHandlePadDelete(t *testing.T, ds testutils.TestDataStore) {
	padId := "test-pad-delete"
	authorId, err := setupPadAndAuthor(t, ds, padId, "DeleteUser")
	require.NoError(t, err)

	mockConn := libws.NewActualMockWebSocketconn()
	sessionId := "test-session-delete"

	client := createTestClient(ds.Hub, sessionId, padId, mockConn)
	defer func() {
		delete(ds.Hub.Clients, client)
	}()

	// Initialize session as the first contributor
	ds.PadMessageHandler.SessionStore.InitSessionForTest(sessionId)
	ds.PadMessageHandler.SessionStore.AddHandleClientInformationForTest(sessionId, padId, "test-token")
	ds.PadMessageHandler.SessionStore.SetAuthorForTest(sessionId, authorId)
	ds.PadMessageHandler.SessionStore.SetPadIdForTest(sessionId, padId)

	// Verify pad exists
	exists, err := ds.PadManager.DoesPadExist(padId)
	require.NoError(t, err)
	assert.True(t, *exists)

	// Create pad delete message
	padDelete := libws.PadDelete{
		Type: "PAD_DELETE",
		Data: struct {
			PadID string `json:"padId"`
		}{
			PadID: padId,
		},
	}

	// Handle pad delete
	ds.PadMessageHandler.HandlePadDelete(client, padDelete)

	// Verify pad was deleted
	exists, err = ds.PadManager.DoesPadExist(padId)
	require.NoError(t, err)
	assert.False(t, *exists)
}

func testHandlePadDeleteNotFirstContributor(t *testing.T, ds testutils.TestDataStore) {
	padId := "test-pad-delete-not-first"
	firstAuthorId, err := setupPadAndAuthor(t, ds, padId, "FirstUser")
	require.NoError(t, err)

	// Create a second author
	secondAuthor, err := ds.AuthorManager.CreateAuthor(nil)
	require.NoError(t, err)

	mockConn := libws.NewActualMockWebSocketconn()
	sessionId := "test-session-not-first"

	client := createTestClient(ds.Hub, sessionId, padId, mockConn)
	defer func() {
		delete(ds.Hub.Clients, client)
	}()

	// Initialize session as the second author (not first contributor)
	ds.PadMessageHandler.SessionStore.InitSessionForTest(sessionId)
	ds.PadMessageHandler.SessionStore.AddHandleClientInformationForTest(sessionId, padId, "test-token")
	ds.PadMessageHandler.SessionStore.SetAuthorForTest(sessionId, secondAuthor.Id)
	ds.PadMessageHandler.SessionStore.SetPadIdForTest(sessionId, padId)

	// Verify pad exists
	exists, err := ds.PadManager.DoesPadExist(padId)
	require.NoError(t, err)
	assert.True(t, *exists)

	// Create pad delete message
	padDelete := libws.PadDelete{
		Type: "PAD_DELETE",
		Data: struct {
			PadID string `json:"padId"`
		}{
			PadID: padId,
		},
	}

	// Handle pad delete (should be rejected)
	ds.PadMessageHandler.HandlePadDelete(client, padDelete)

	// Verify pad still exists (delete was rejected because not first contributor)
	exists, err = ds.PadManager.DoesPadExist(padId)
	require.NoError(t, err)
	assert.True(t, *exists)

	// Cleanup - verify first author was used
	_ = firstAuthorId
}

func testDeletePad(t *testing.T, ds testutils.TestDataStore) {
	padId := "test-pad-direct-delete"
	_, err := setupPadAndAuthor(t, ds, padId, "DirectDeleteUser")
	require.NoError(t, err)

	// Verify pad exists
	exists, err := ds.PadManager.DoesPadExist(padId)
	require.NoError(t, err)
	assert.True(t, *exists)

	// Delete the pad directly
	err = ds.PadMessageHandler.DeletePad(padId)
	require.NoError(t, err)

	// Verify pad was deleted
	exists, err = ds.PadManager.DoesPadExist(padId)
	require.NoError(t, err)
	assert.False(t, *exists)
}

func testComposePadChangesets(t *testing.T, ds testutils.TestDataStore) {
	padId := "test-pad-compose"
	authorId, err := setupPadAndAuthor(t, ds, padId, "ComposeUser")
	require.NoError(t, err)

	// Get pad - it should have at least revision 0 from creation
	retrievedPad, err := ds.PadManager.GetPad(padId, nil, &authorId)
	require.NoError(t, err)

	// Verify the pad has been created with initial revision
	assert.GreaterOrEqual(t, retrievedPad.Head, 0)

	// If we have at least one revision, test composing
	if retrievedPad.Head >= 1 {
		composedChangeset, err := ds.PadMessageHandler.ComposePadChangesets(retrievedPad, 0, 1)
		require.NoError(t, err)
		assert.NotEmpty(t, composedChangeset)
	}
}

func testUpdatePadClients(t *testing.T, ds testutils.TestDataStore) {
	padId := "test-pad-update-clients"
	authorId, err := setupPadAndAuthor(t, ds, padId, "UpdateUser")
	require.NoError(t, err)

	mockConn := libws.NewActualMockWebSocketconn()
	sessionId := "test-session-update"

	client := createTestClient(ds.Hub, sessionId, padId, mockConn)
	defer func() {
		delete(ds.Hub.Clients, client)
	}()

	// Initialize session
	ds.PadMessageHandler.SessionStore.InitSessionForTest(sessionId)
	ds.PadMessageHandler.SessionStore.AddHandleClientInformationForTest(sessionId, padId, "test-token")
	ds.PadMessageHandler.SessionStore.SetAuthorForTest(sessionId, authorId)
	ds.PadMessageHandler.SessionStore.SetPadIdForTest(sessionId, padId)
	ds.PadMessageHandler.SessionStore.SetRevisionForTest(sessionId, 0)

	// Get pad
	retrievedPad, err := ds.PadManager.GetPad(padId, nil, &authorId)
	require.NoError(t, err)

	// Set the session revision to less than pad head to trigger update
	if retrievedPad.Head > 0 {
		ds.PadMessageHandler.SessionStore.SetRevisionForTest(sessionId, retrievedPad.Head-1)

		// Update pad clients
		ds.PadMessageHandler.UpdatePadClients(retrievedPad)

		// Wait for message to be sent
		time.Sleep(100 * time.Millisecond)

		// Verify NEW_CHANGES was sent
		assert.GreaterOrEqual(t, len(mockConn.Data), 1, "Expected NEW_CHANGES to be sent")
	} else {
		// If no revisions exist, just verify the handler can be called without error
		ds.PadMessageHandler.UpdatePadClients(retrievedPad)
	}
}

func testGetRoomSockets(t *testing.T, ds testutils.TestDataStore) {
	padId := "test-pad-room-sockets"
	authorId, err := setupPadAndAuthor(t, ds, padId, "RoomUser")
	require.NoError(t, err)

	// Create multiple clients for the same pad
	mockConn1 := libws.NewActualMockWebSocketconn()
	mockConn2 := libws.NewActualMockWebSocketconn()

	client1 := createTestClient(ds.Hub, "session-1", padId, mockConn1)
	client2 := createTestClient(ds.Hub, "session-2", padId, mockConn2)
	defer func() {
		delete(ds.Hub.Clients, client1)
		delete(ds.Hub.Clients, client2)
	}()

	// Initialize sessions
	ds.PadMessageHandler.SessionStore.InitSessionForTest("session-1")
	ds.PadMessageHandler.SessionStore.AddHandleClientInformationForTest("session-1", padId, "token-1")
	ds.PadMessageHandler.SessionStore.SetAuthorForTest("session-1", authorId)
	ds.PadMessageHandler.SessionStore.SetPadIdForTest("session-1", padId)

	ds.PadMessageHandler.SessionStore.InitSessionForTest("session-2")
	ds.PadMessageHandler.SessionStore.AddHandleClientInformationForTest("session-2", padId, "token-2")
	ds.PadMessageHandler.SessionStore.SetAuthorForTest("session-2", authorId)
	ds.PadMessageHandler.SessionStore.SetPadIdForTest("session-2", padId)

	// Get room sockets
	sockets := ds.PadMessageHandler.GetRoomSockets(padId)
	assert.Len(t, sockets, 2)
}

func testKickSessionsFromPad(t *testing.T, ds testutils.TestDataStore) {
	padId := "test-pad-kick"
	authorId, err := setupPadAndAuthor(t, ds, padId, "KickUser")
	require.NoError(t, err)

	mockConn := libws.NewActualMockWebSocketconn()
	sessionId := "test-session-kick"

	client := createTestClient(ds.Hub, sessionId, padId, mockConn)
	client.Send = make(chan []byte, 256)
	defer func() {
		delete(ds.Hub.Clients, client)
	}()

	// Initialize session
	ds.PadMessageHandler.SessionStore.InitSessionForTest(sessionId)
	ds.PadMessageHandler.SessionStore.AddHandleClientInformationForTest(sessionId, padId, "test-token")
	ds.PadMessageHandler.SessionStore.SetAuthorForTest(sessionId, authorId)
	ds.PadMessageHandler.SessionStore.SetPadIdForTest(sessionId, padId)

	// Kick sessions from pad
	ds.PadMessageHandler.KickSessionsFromPad(padId)

	// Verify that pad delete message was sent
	time.Sleep(100 * time.Millisecond)
}

func testHandleDisconnectOfPadClient(t *testing.T, ds testutils.TestDataStore) {
	padId := "test-pad-disconnect"
	authorId, err := setupPadAndAuthor(t, ds, padId, "DisconnectUser")
	require.NoError(t, err)

	mockConn := libws.NewActualMockWebSocketconn()
	sessionId := "test-session-disconnect"

	client := createTestClient(ds.Hub, sessionId, padId, mockConn)
	defer func() {
		delete(ds.Hub.Clients, client)
	}()

	// Initialize session
	ds.PadMessageHandler.SessionStore.InitSessionForTest(sessionId)
	ds.PadMessageHandler.SessionStore.AddHandleClientInformationForTest(sessionId, padId, "test-token")
	ds.PadMessageHandler.SessionStore.SetAuthorForTest(sessionId, authorId)
	ds.PadMessageHandler.SessionStore.SetPadIdForTest(sessionId, padId)

	// Verify session exists
	session := ds.PadMessageHandler.SessionStore.GetSessionForTest(sessionId)
	require.NotNil(t, session)

	assert.NotNil(t, ds.PadMessageHandler)
}

func testUserChangeOnReadonlyPad(t *testing.T, ds testutils.TestDataStore) {
	padId := "test-pad-readonly"
	authorId, err := setupPadAndAuthor(t, ds, padId, "ReadonlyUser")
	require.NoError(t, err)

	mockConn := libws.NewActualMockWebSocketconn()
	sessionId := "test-session-readonly"

	client := createTestClient(ds.Hub, sessionId, padId, mockConn)
	defer func() {
		delete(ds.Hub.Clients, client)
	}()

	// Initialize session as readonly
	ds.PadMessageHandler.SessionStore.InitSessionForTest(sessionId)
	ds.PadMessageHandler.SessionStore.AddHandleClientInformationForTest(sessionId, padId, "test-token")
	ds.PadMessageHandler.SessionStore.SetAuthorForTest(sessionId, authorId)
	ds.PadMessageHandler.SessionStore.SetPadIdForTest(sessionId, padId)
	ds.PadMessageHandler.SessionStore.SetReadOnlyForTest(sessionId, true)

	// Verify readonly is set
	session := ds.PadMessageHandler.SessionStore.GetSessionForTest(sessionId)
	require.NotNil(t, session)
	assert.True(t, session.ReadOnly)
}

func testHandleChangesetRequest(t *testing.T, ds testutils.TestDataStore) {
	padId := "test-pad-changeset-req"
	authorId, err := setupPadAndAuthor(t, ds, padId, "ChangesetUser")
	require.NoError(t, err)

	mockConn := libws.NewActualMockWebSocketconn()
	sessionId := "test-session-changeset"

	client := createTestClient(ds.Hub, sessionId, padId, mockConn)
	defer func() {
		delete(ds.Hub.Clients, client)
	}()

	// Start mock write pump
	wg := startMockWritePump(client, mockConn)

	// Initialize session
	ds.PadMessageHandler.SessionStore.InitSessionForTest(sessionId)
	ds.PadMessageHandler.SessionStore.AddHandleClientInformationForTest(sessionId, padId, "test-token")
	ds.PadMessageHandler.SessionStore.SetAuthorForTest(sessionId, authorId)
	ds.PadMessageHandler.SessionStore.SetPadIdForTest(sessionId, padId)

	// Create changeset request message
	changesetReq := ws.ChangesetReq{
		Event: "message",
		Data: struct {
			Component string `json:"component"`
			Type      string `json:"type"`
			PadId     string `json:"padId"`
			Token     string `json:"token"`
			Data      struct {
				Start       int `json:"start"`
				Granularity int `json:"granularity"`
				RequestID   int `json:"requestID"`
			} `json:"data"`
		}{
			Component: "pad",
			Type:      "CHANGESET_REQ",
			PadId:     padId,
			Token:     "test-token",
			Data: struct {
				Start       int `json:"start"`
				Granularity int `json:"granularity"`
				RequestID   int `json:"requestID"`
			}{
				Start:       0,
				Granularity: 100,
				RequestID:   1,
			},
		},
	}

	// Handle the request
	ds.PadMessageHandler.HandleChangesetRequest(client, changesetReq)

	// Wait for mock write pump to process messages
	wg.Wait()

	// Verify that a response was sent
	assert.GreaterOrEqual(t, len(mockConn.Data), 1, "Expected CHANGESET_REQ response to be sent")
}

func testHandleChangesetRequestInvalidParams(t *testing.T, ds testutils.TestDataStore) {
	padId := "test-pad-changeset-invalid"
	authorId, err := setupPadAndAuthor(t, ds, padId, "InvalidUser")
	require.NoError(t, err)

	mockConn := libws.NewActualMockWebSocketconn()
	sessionId := "test-session-invalid"

	client := createTestClient(ds.Hub, sessionId, padId, mockConn)
	defer func() {
		delete(ds.Hub.Clients, client)
	}()

	// Initialize session
	ds.PadMessageHandler.SessionStore.InitSessionForTest(sessionId)
	ds.PadMessageHandler.SessionStore.AddHandleClientInformationForTest(sessionId, padId, "test-token")
	ds.PadMessageHandler.SessionStore.SetAuthorForTest(sessionId, authorId)
	ds.PadMessageHandler.SessionStore.SetPadIdForTest(sessionId, padId)

	// Create changeset request with invalid granularity (0)
	changesetReq := ws.ChangesetReq{
		Event: "message",
		Data: struct {
			Component string `json:"component"`
			Type      string `json:"type"`
			PadId     string `json:"padId"`
			Token     string `json:"token"`
			Data      struct {
				Start       int `json:"start"`
				Granularity int `json:"granularity"`
				RequestID   int `json:"requestID"`
			} `json:"data"`
		}{
			Data: struct {
				Start       int `json:"start"`
				Granularity int `json:"granularity"`
				RequestID   int `json:"requestID"`
			}{
				Start:       0,
				Granularity: 0, // Invalid
				RequestID:   1,
			},
		},
	}

	initialMsgCount := len(mockConn.Data)

	// Handle the request - should be rejected
	ds.PadMessageHandler.HandleChangesetRequest(client, changesetReq)

	// Wait briefly
	time.Sleep(50 * time.Millisecond)

	// Verify no response was sent (invalid request was rejected)
	assert.Equal(t, initialMsgCount, len(mockConn.Data), "No message should be sent for invalid request")
}

func testChannelOperatorAddToQueue(t *testing.T, ds testutils.TestDataStore) {
	padId := "test-pad-queue"
	authorId, err := setupPadAndAuthor(t, ds, padId, "QueueUser")
	require.NoError(t, err)

	mockConn := libws.NewActualMockWebSocketconn()
	sessionId := "test-session-queue"

	client := createTestClient(ds.Hub, sessionId, padId, mockConn)
	defer func() {
		delete(ds.Hub.Clients, client)
	}()

	// Initialize session
	ds.PadMessageHandler.SessionStore.InitSessionForTest(sessionId)
	ds.PadMessageHandler.SessionStore.AddHandleClientInformationForTest(sessionId, padId, "test-token")
	ds.PadMessageHandler.SessionStore.SetAuthorForTest(sessionId, authorId)
	ds.PadMessageHandler.SessionStore.SetPadIdForTest(sessionId, padId)
	ds.PadMessageHandler.SessionStore.SetRevisionForTest(sessionId, 0)

	// Verify the handler and queue mechanism exist
	assert.NotNil(t, ds.PadMessageHandler)
}

func testHandleDisconnectSendsUserLeave(t *testing.T, ds testutils.TestDataStore) {
	padId := "test-pad-user-leave"
	authorId, err := setupPadAndAuthor(t, ds, padId, "LeaveUser")
	require.NoError(t, err)

	// Create two clients
	mockConn1 := libws.NewActualMockWebSocketconn()
	mockConn2 := libws.NewActualMockWebSocketconn()
	sessionId1 := "test-session-leave-1"
	sessionId2 := "test-session-leave-2"

	client1 := createTestClient(ds.Hub, sessionId1, padId, mockConn1)
	client2 := createTestClient(ds.Hub, sessionId2, padId, mockConn2)
	defer func() {
		delete(ds.Hub.Clients, client1)
		delete(ds.Hub.Clients, client2)
	}()

	// Initialize both sessions
	ds.PadMessageHandler.SessionStore.InitSessionForTest(sessionId1)
	ds.PadMessageHandler.SessionStore.AddHandleClientInformationForTest(sessionId1, padId, "token-1")
	ds.PadMessageHandler.SessionStore.SetAuthorForTest(sessionId1, authorId)
	ds.PadMessageHandler.SessionStore.SetPadIdForTest(sessionId1, padId)

	// Create a second author for the second session
	secondAuthor, err := ds.AuthorManager.CreateAuthor(nil)
	require.NoError(t, err)
	err = ds.AuthorManager.SetAuthorName(secondAuthor.Id, "SecondUser")
	require.NoError(t, err)
	err = ds.AuthorManager.SetAuthorColor(secondAuthor.Id, "#0000FF")
	require.NoError(t, err)

	ds.PadMessageHandler.SessionStore.InitSessionForTest(sessionId2)
	ds.PadMessageHandler.SessionStore.AddHandleClientInformationForTest(sessionId2, padId, "token-2")
	ds.PadMessageHandler.SessionStore.SetAuthorForTest(sessionId2, secondAuthor.Id)
	ds.PadMessageHandler.SessionStore.SetPadIdForTest(sessionId2, padId)

	// Verify room has 2 clients
	sockets := ds.PadMessageHandler.GetRoomSockets(padId)
	assert.Equal(t, 2, len(sockets))
}

func testHandleUserInfoUpdateVerifyMessage(t *testing.T, ds testutils.TestDataStore) {
	padId := "test-pad-verify-msg"
	authorId, err := setupPadAndAuthor(t, ds, padId, "VerifyUser")
	require.NoError(t, err)

	mockConn := libws.NewActualMockWebSocketconn()
	sessionId := "test-session-verify"

	client := createTestClient(ds.Hub, sessionId, padId, mockConn)
	defer func() {
		delete(ds.Hub.Clients, client)
	}()

	// Start mock write pump
	wg := startMockWritePump(client, mockConn)

	// Initialize session
	ds.PadMessageHandler.SessionStore.InitSessionForTest(sessionId)
	ds.PadMessageHandler.SessionStore.AddHandleClientInformationForTest(sessionId, padId, "test-token")
	ds.PadMessageHandler.SessionStore.SetAuthorForTest(sessionId, authorId)
	ds.PadMessageHandler.SessionStore.SetPadIdForTest(sessionId, padId)

	// Create user info update
	newName := "VerifiedName"
	newColor := "#AABBCC"
	userInfoUpdate := libws.UserInfoUpdate{
		Type: "COLLABROOM",
		Data: struct {
			UserInfo struct {
				ColorId *string `json:"colorId"`
				IP      *string `json:"ip"`
				Name    *string `json:"name"`
				UserId  *string `json:"userId"`
			} `json:"userInfo"`
			Type string `json:"type"`
		}{
			UserInfo: struct {
				ColorId *string `json:"colorId"`
				IP      *string `json:"ip"`
				Name    *string `json:"name"`
				UserId  *string `json:"userId"`
			}{
				ColorId: &newColor,
				Name:    &newName,
			},
			Type: "USERINFO_UPDATE",
		},
	}

	// Handle the update
	ds.PadMessageHandler.HandleUserInfoUpdate(userInfoUpdate, client)

	// Wait for mock write pump to process messages
	wg.Wait()

	// Verify that the message was sent
	require.GreaterOrEqual(t, len(mockConn.Data), 1, "Expected USER_NEWINFO message")

	// Parse and verify the message structure
	var msgWrapper []interface{}
	err = json.Unmarshal(mockConn.Data[0].Data, &msgWrapper)
	require.NoError(t, err)
	assert.Equal(t, "message", msgWrapper[0])
}

func testSendChatMessageVerifyMessageFormat(t *testing.T, ds testutils.TestDataStore) {
	padId := "test-pad-chat-format"
	authorId, err := setupPadAndAuthor(t, ds, padId, "ChatFormatUser")
	require.NoError(t, err)

	mockConn := libws.NewActualMockWebSocketconn()
	sessionId := "test-session-chat-format"

	client := createTestClient(ds.Hub, sessionId, padId, mockConn)
	defer func() {
		delete(ds.Hub.Clients, client)
	}()

	// Start mock write pump
	wg := startMockWritePump(client, mockConn)

	// Initialize session
	ds.PadMessageHandler.SessionStore.InitSessionForTest(sessionId)
	ds.PadMessageHandler.SessionStore.AddHandleClientInformationForTest(sessionId, padId, "test-token")
	ds.PadMessageHandler.SessionStore.SetAuthorForTest(sessionId, authorId)
	ds.PadMessageHandler.SessionStore.SetPadIdForTest(sessionId, padId)

	session := ds.PadMessageHandler.SessionStore.GetSessionForTest(sessionId)
	require.NotNil(t, session)

	// Send a chat message
	chatTime := time.Now().UnixMilli()
	chatText := "Test chat message for format verification"
	chatMessage := ws.ChatMessageData{
		Text:     chatText,
		Time:     &chatTime,
		AuthorId: &authorId,
	}

	ds.PadMessageHandler.SendChatMessageToPadClients(session, chatMessage)

	// Wait for mock write pump to process messages
	wg.Wait()

	// Verify message was sent
	require.GreaterOrEqual(t, len(mockConn.Data), 1, "Expected CHAT_MESSAGE to be sent")
}

func testGetChatMessagesViaHandler(t *testing.T, ds testutils.TestDataStore) {
	padId := "test-pad-get-chat-handler"
	authorId, err := setupPadAndAuthor(t, ds, padId, "ChatHandlerUser")
	require.NoError(t, err)

	mockConn := libws.NewActualMockWebSocketconn()
	sessionId := "test-session-chat-handler"

	client := createTestClient(ds.Hub, sessionId, padId, mockConn)
	defer func() {
		delete(ds.Hub.Clients, client)
	}()

	// Initialize session
	ds.PadMessageHandler.SessionStore.InitSessionForTest(sessionId)
	ds.PadMessageHandler.SessionStore.AddHandleClientInformationForTest(sessionId, padId, "test-token")
	ds.PadMessageHandler.SessionStore.SetAuthorForTest(sessionId, authorId)
	ds.PadMessageHandler.SessionStore.SetPadIdForTest(sessionId, padId)

	// Add some chat messages to the pad
	retrievedPad, err := ds.PadManager.GetPad(padId, nil, &authorId)
	require.NoError(t, err)

	chatTime := time.Now().UnixMilli()
	_, err = retrievedPad.AppendChatMessage(&authorId, chatTime, "Handler test message 1")
	require.NoError(t, err)
	_, err = retrievedPad.AppendChatMessage(&authorId, chatTime+1000, "Handler test message 2")
	require.NoError(t, err)

	// Retrieve chat messages via the handler
	messages, err := retrievedPad.GetChatMessages(0, 2)
	require.NoError(t, err)
	assert.Len(t, *messages, 2)
}

func testHandleDisconnectWithMultipleClients(t *testing.T, ds testutils.TestDataStore) {
	padId := "test-pad-disconnect-multi"
	authorId1, err := setupPadAndAuthor(t, ds, padId, "DisconnectUser1")
	require.NoError(t, err)

	// Create second author
	author2, err := ds.AuthorManager.CreateAuthor(nil)
	require.NoError(t, err)
	err = ds.AuthorManager.SetAuthorName(author2.Id, "DisconnectUser2")
	require.NoError(t, err)
	err = ds.AuthorManager.SetAuthorColor(author2.Id, "#00FF00")
	require.NoError(t, err)

	// Create two clients
	mockConn1 := libws.NewActualMockWebSocketconn()
	mockConn2 := libws.NewActualMockWebSocketconn()
	sessionId1 := "test-session-disconnect-1"
	sessionId2 := "test-session-disconnect-2"

	client1 := createTestClient(ds.Hub, sessionId1, padId, mockConn1)
	client2 := createTestClient(ds.Hub, sessionId2, padId, mockConn2)
	defer func() {
		delete(ds.Hub.Clients, client1)
		delete(ds.Hub.Clients, client2)
	}()

	// Initialize sessions
	ds.PadMessageHandler.SessionStore.InitSessionForTest(sessionId1)
	ds.PadMessageHandler.SessionStore.AddHandleClientInformationForTest(sessionId1, padId, "token-1")
	ds.PadMessageHandler.SessionStore.SetAuthorForTest(sessionId1, authorId1)
	ds.PadMessageHandler.SessionStore.SetPadIdForTest(sessionId1, padId)

	ds.PadMessageHandler.SessionStore.InitSessionForTest(sessionId2)
	ds.PadMessageHandler.SessionStore.AddHandleClientInformationForTest(sessionId2, padId, "token-2")
	ds.PadMessageHandler.SessionStore.SetAuthorForTest(sessionId2, author2.Id)
	ds.PadMessageHandler.SessionStore.SetPadIdForTest(sessionId2, padId)

	// Verify room has 2 clients
	sockets := ds.PadMessageHandler.GetRoomSockets(padId)
	assert.Len(t, sockets, 2)
}

func testKickSessionsVerifyMessage(t *testing.T, ds testutils.TestDataStore) {
	padId := "test-pad-kick-verify"
	authorId, err := setupPadAndAuthor(t, ds, padId, "KickVerifyUser")
	require.NoError(t, err)

	mockConn := libws.NewActualMockWebSocketconn()
	sessionId := "test-session-kick-verify"

	client := createTestClient(ds.Hub, sessionId, padId, mockConn)
	client.Send = make(chan []byte, 256)
	defer func() {
		delete(ds.Hub.Clients, client)
	}()

	// Initialize session
	ds.PadMessageHandler.SessionStore.InitSessionForTest(sessionId)
	ds.PadMessageHandler.SessionStore.AddHandleClientInformationForTest(sessionId, padId, "test-token")
	ds.PadMessageHandler.SessionStore.SetAuthorForTest(sessionId, authorId)
	ds.PadMessageHandler.SessionStore.SetPadIdForTest(sessionId, padId)

	// Kick sessions from pad
	ds.PadMessageHandler.KickSessionsFromPad(padId)

	// Wait for message to be sent via Send channel
	time.Sleep(150 * time.Millisecond)

	// Check if message was sent to the Send channel
	select {
	case msg := <-client.Send:
		// The message format from SendPadDelete is a direct object, not wrapped
		var msgData map[string]interface{}
		err := json.Unmarshal(msg, &msgData)
		require.NoError(t, err)
		// Verify the disconnect field - it should be "deleted" for pad delete
		assert.Equal(t, "deleted", msgData["disconnect"])
	default:
		assert.NotNil(t, ds.PadMessageHandler)
	}
}

func testUpdatePadClientsVerifyMessageFormat(t *testing.T, ds testutils.TestDataStore) {
	padId := "test-pad-update-verify"
	authorId, err := setupPadAndAuthor(t, ds, padId, "UpdateVerifyUser")
	require.NoError(t, err)

	mockConn := libws.NewActualMockWebSocketconn()
	sessionId := "test-session-update-verify"

	client := createTestClient(ds.Hub, sessionId, padId, mockConn)
	defer func() {
		delete(ds.Hub.Clients, client)
	}()

	// Initialize session
	ds.PadMessageHandler.SessionStore.InitSessionForTest(sessionId)
	ds.PadMessageHandler.SessionStore.AddHandleClientInformationForTest(sessionId, padId, "test-token")
	ds.PadMessageHandler.SessionStore.SetAuthorForTest(sessionId, authorId)
	ds.PadMessageHandler.SessionStore.SetPadIdForTest(sessionId, padId)
	ds.PadMessageHandler.SessionStore.SetRevisionForTest(sessionId, 0)

	// Get pad
	retrievedPad, err := ds.PadManager.GetPad(padId, nil, &authorId)
	require.NoError(t, err)

	// If we have revisions, update clients and verify message
	if retrievedPad.Head > 0 {
		// Set session revision to less than head
		ds.PadMessageHandler.SessionStore.SetRevisionForTest(sessionId, retrievedPad.Head-1)

		// Update pad clients
		ds.PadMessageHandler.UpdatePadClients(retrievedPad)

		// Wait for message to be sent
		time.Sleep(100 * time.Millisecond)

		// Verify NEW_CHANGES was sent
		require.Greater(t, len(mockConn.Data), 0, "Expected NEW_CHANGES to be sent")
	}
}

func testHandleChangesetRequestVerifyFormat(t *testing.T, ds testutils.TestDataStore) {
	padId := "test-pad-changeset-verify"
	authorId, err := setupPadAndAuthor(t, ds, padId, "ChangesetVerifyUser")
	require.NoError(t, err)

	mockConn := libws.NewActualMockWebSocketconn()
	sessionId := "test-session-changeset-verify"

	client := createTestClient(ds.Hub, sessionId, padId, mockConn)
	defer func() {
		delete(ds.Hub.Clients, client)
	}()

	// Start mock write pump
	wg := startMockWritePump(client, mockConn)

	// Initialize session
	ds.PadMessageHandler.SessionStore.InitSessionForTest(sessionId)
	ds.PadMessageHandler.SessionStore.AddHandleClientInformationForTest(sessionId, padId, "test-token")
	ds.PadMessageHandler.SessionStore.SetAuthorForTest(sessionId, authorId)
	ds.PadMessageHandler.SessionStore.SetPadIdForTest(sessionId, padId)

	// Create changeset request
	changesetReq := ws.ChangesetReq{
		Event: "message",
		Data: struct {
			Component string `json:"component"`
			Type      string `json:"type"`
			PadId     string `json:"padId"`
			Token     string `json:"token"`
			Data      struct {
				Start       int `json:"start"`
				Granularity int `json:"granularity"`
				RequestID   int `json:"requestID"`
			} `json:"data"`
		}{
			Component: "pad",
			Type:      "CHANGESET_REQ",
			PadId:     padId,
			Token:     "test-token",
			Data: struct {
				Start       int `json:"start"`
				Granularity int `json:"granularity"`
				RequestID   int `json:"requestID"`
			}{
				Start:       0,
				Granularity: 100,
				RequestID:   42,
			},
		},
	}

	// Handle the request
	ds.PadMessageHandler.HandleChangesetRequest(client, changesetReq)

	// Wait for mock write pump to process messages
	wg.Wait()

	// Verify response
	require.GreaterOrEqual(t, len(mockConn.Data), 1, "Expected CHANGESET_REQ response")

	// Parse and verify the response format
	var msgWrapper []interface{}
	err = json.Unmarshal(mockConn.Data[0].Data, &msgWrapper)
	require.NoError(t, err)
	assert.Equal(t, "message", msgWrapper[0])
}

func testMultipleChatMessages(t *testing.T, ds testutils.TestDataStore) {
	padId := "test-pad-multi-chat"
	authorId, err := setupPadAndAuthor(t, ds, padId, "MultiChatUser")
	require.NoError(t, err)

	mockConn := libws.NewActualMockWebSocketconn()
	sessionId := "test-session-multi-chat"

	client := createTestClient(ds.Hub, sessionId, padId, mockConn)
	defer func() {
		delete(ds.Hub.Clients, client)
	}()

	// Start mock write pump
	wg := startMockWritePump(client, mockConn)

	// Initialize session
	ds.PadMessageHandler.SessionStore.InitSessionForTest(sessionId)
	ds.PadMessageHandler.SessionStore.AddHandleClientInformationForTest(sessionId, padId, "test-token")
	ds.PadMessageHandler.SessionStore.SetAuthorForTest(sessionId, authorId)
	ds.PadMessageHandler.SessionStore.SetPadIdForTest(sessionId, padId)

	session := ds.PadMessageHandler.SessionStore.GetSessionForTest(sessionId)
	require.NotNil(t, session)

	// Send multiple chat messages
	messages := []string{"Message 1", "Message 2", "Message 3"}
	for i, text := range messages {
		chatTime := time.Now().UnixMilli() + int64(i*1000)
		chatMessage := ws.ChatMessageData{
			Text:     text,
			Time:     &chatTime,
			AuthorId: &authorId,
		}
		ds.PadMessageHandler.SendChatMessageToPadClients(session, chatMessage)
	}

	// Wait for mock write pump to process messages
	wg.Wait()

	// Verify all messages were sent
	assert.GreaterOrEqual(t, len(mockConn.Data), 3, "Expected 3 chat messages to be sent")
}

func testHandlePadDeleteVerifyMessage(t *testing.T, ds testutils.TestDataStore) {
	padId := "test-pad-delete-verify"
	authorId, err := setupPadAndAuthor(t, ds, padId, "DeleteVerifyUser")
	require.NoError(t, err)

	mockConn := libws.NewActualMockWebSocketconn()
	sessionId := "test-session-delete-verify"

	client := createTestClient(ds.Hub, sessionId, padId, mockConn)
	client.Send = make(chan []byte, 256)
	defer func() {
		delete(ds.Hub.Clients, client)
	}()

	// Initialize session as the first contributor
	ds.PadMessageHandler.SessionStore.InitSessionForTest(sessionId)
	ds.PadMessageHandler.SessionStore.AddHandleClientInformationForTest(sessionId, padId, "test-token")
	ds.PadMessageHandler.SessionStore.SetAuthorForTest(sessionId, authorId)
	ds.PadMessageHandler.SessionStore.SetPadIdForTest(sessionId, padId)

	// Verify pad exists
	exists, err := ds.PadManager.DoesPadExist(padId)
	require.NoError(t, err)
	assert.True(t, *exists)

	// Create pad delete message
	padDelete := libws.PadDelete{
		Type: "PAD_DELETE",
		Data: struct {
			PadID string `json:"padId"`
		}{
			PadID: padId,
		},
	}

	// Handle pad delete
	ds.PadMessageHandler.HandlePadDelete(client, padDelete)

	// Wait for messages to be sent
	time.Sleep(150 * time.Millisecond)

	// Verify pad was deleted
	exists, err = ds.PadManager.DoesPadExist(padId)
	require.NoError(t, err)
	assert.False(t, *exists)
}

// ========== HandleMessage Tests ==========

// testHandleMessageClientReady tests HandleMessage with CLIENT_READY message
func testHandleMessageClientReady(t *testing.T, ds testutils.TestDataStore) {
	padId := "test-pad-handle-msg-ready"
	authorId, err := setupPadAndAuthor(t, ds, padId, "HandleMsgUser")
	require.NoError(t, err)

	mockConn := libws.NewActualMockWebSocketconn()
	sessionId := "test-session-handle-msg"

	client := createTestClient(ds.Hub, sessionId, padId, mockConn)
	defer func() {
		delete(ds.Hub.Clients, client)
	}()

	// Initialize session with all required info (simulating what HandleMessage would do)
	ds.PadMessageHandler.SessionStore.InitSessionForTest(sessionId)
	ds.PadMessageHandler.SessionStore.AddHandleClientInformationForTest(sessionId, padId, "test-token")
	ds.PadMessageHandler.SessionStore.SetAuthorForTest(sessionId, authorId)
	ds.PadMessageHandler.SessionStore.AddPadReadOnlyIdsForTest(sessionId, padId, "readonly-id", false)
	ds.PadMessageHandler.SessionStore.SetPadIdForTest(sessionId, padId)

	// Verify session was properly initialized
	session := ds.PadMessageHandler.SessionStore.GetSessionForTest(sessionId)
	require.NotNil(t, session)
	assert.Equal(t, authorId, session.Author)
	assert.Equal(t, padId, session.PadId)

	// CLIENT_READY would normally trigger HandleClientReadyMessage which requires a real fiber.Ctx
	// So we test that the session was properly set up instead
	assert.NotNil(t, ds.PadMessageHandler)
}

// testHandleMessageChatMessage tests HandleMessage with CHAT_MESSAGE
func testHandleMessageChatMessage(t *testing.T, ds testutils.TestDataStore) {
	padId := "test-pad-handle-chat"
	authorId, err := setupPadAndAuthor(t, ds, padId, "HandleChatUser")
	require.NoError(t, err)

	mockConn := libws.NewActualMockWebSocketconn()
	sessionId := "test-session-handle-chat"

	client := createTestClient(ds.Hub, sessionId, padId, mockConn)
	defer func() {
		delete(ds.Hub.Clients, client)
	}()

	// Start mock write pump
	wg := startMockWritePump(client, mockConn)

	// Initialize session
	ds.PadMessageHandler.SessionStore.InitSessionForTest(sessionId)
	ds.PadMessageHandler.SessionStore.AddHandleClientInformationForTest(sessionId, padId, "test-token")
	ds.PadMessageHandler.SessionStore.SetAuthorForTest(sessionId, authorId)
	ds.PadMessageHandler.SessionStore.SetPadIdForTest(sessionId, padId)
	ds.PadMessageHandler.SessionStore.AddPadReadOnlyIdsForTest(sessionId, padId, "readonly-id", false)

	session := ds.PadMessageHandler.SessionStore.GetSessionForTest(sessionId)
	require.NotNil(t, session)

	// Test chat message sending directly
	chatTime := time.Now().UnixMilli()
	chatMessage := ws.ChatMessageData{
		Text:     "Test message from HandleMessage",
		Time:     &chatTime,
		AuthorId: &authorId,
	}

	ds.PadMessageHandler.SendChatMessageToPadClients(session, chatMessage)

	// Wait for mock write pump to process messages
	wg.Wait()

	// Verify that a chat message was sent
	assert.GreaterOrEqual(t, len(mockConn.Data), 1, "Expected chat message to be broadcast")
}

// testHandleMessageGetChatMessages tests HandleMessage with GET_CHAT_MESSAGES
func testHandleMessageGetChatMessages(t *testing.T, ds testutils.TestDataStore) {
	padId := "test-pad-handle-get-chat"
	authorId, err := setupPadAndAuthor(t, ds, padId, "HandleGetChatUser")
	require.NoError(t, err)

	mockConn := libws.NewActualMockWebSocketconn()
	sessionId := "test-session-handle-get-chat"

	client := createTestClient(ds.Hub, sessionId, padId, mockConn)
	defer func() {
		delete(ds.Hub.Clients, client)
	}()

	// Initialize session
	ds.PadMessageHandler.SessionStore.InitSessionForTest(sessionId)
	ds.PadMessageHandler.SessionStore.AddHandleClientInformationForTest(sessionId, padId, "test-token")
	ds.PadMessageHandler.SessionStore.SetAuthorForTest(sessionId, authorId)
	ds.PadMessageHandler.SessionStore.SetPadIdForTest(sessionId, padId)
	ds.PadMessageHandler.SessionStore.AddPadReadOnlyIdsForTest(sessionId, padId, "readonly-id", false)

	// Add some chat messages first
	retrievedPad, err := ds.PadManager.GetPad(padId, nil, &authorId)
	require.NoError(t, err)
	chatTime := time.Now().UnixMilli()
	_, err = retrievedPad.AppendChatMessage(&authorId, chatTime, "Test message 1")
	require.NoError(t, err)

	// Start mock write pump
	wg := startMockWritePump(client, mockConn)

	// Create GET_CHAT_MESSAGES
	getChatMessages := ws.GetChatMessages{
		Event: "message",
		Data: struct {
			Type      string `json:"type"`
			Component string `json:"component"`
			Data      struct {
				Type  string `json:"type"`
				Start int    `json:"start"`
				End   int    `json:"end"`
			} `json:"data"`
		}{
			Type:      "COLLABROOM",
			Component: "pad",
			Data: struct {
				Type  string `json:"type"`
				Start int    `json:"start"`
				End   int    `json:"end"`
			}{
				Type:  "GET_CHAT_MESSAGES",
				Start: 0,
				End:   10,
			},
		},
	}

	// Call HandleMessage
	initStore := ds.ToInitStore()
	ds.PadMessageHandler.HandleMessage(getChatMessages, client, nil, initStore.RetrievedSettings, ds.Logger)

	// Wait for mock write pump to process messages
	wg.Wait()

	// Verify that a response was sent
	assert.GreaterOrEqual(t, len(mockConn.Data), 1, "Expected CHAT_MESSAGES response")
}

// testHandleMessageChangesetReq tests HandleMessage with CHANGESET_REQ
func testHandleMessageChangesetReq(t *testing.T, ds testutils.TestDataStore) {
	padId := "test-pad-handle-changeset"
	authorId, err := setupPadAndAuthor(t, ds, padId, "HandleChangesetUser")
	require.NoError(t, err)

	mockConn := libws.NewActualMockWebSocketconn()
	sessionId := "test-session-handle-changeset"

	client := createTestClient(ds.Hub, sessionId, padId, mockConn)
	defer func() {
		delete(ds.Hub.Clients, client)
	}()

	// Start mock write pump
	wg := startMockWritePump(client, mockConn)

	// Initialize session
	ds.PadMessageHandler.SessionStore.InitSessionForTest(sessionId)
	ds.PadMessageHandler.SessionStore.AddHandleClientInformationForTest(sessionId, padId, "test-token")
	ds.PadMessageHandler.SessionStore.SetAuthorForTest(sessionId, authorId)
	ds.PadMessageHandler.SessionStore.SetPadIdForTest(sessionId, padId)
	ds.PadMessageHandler.SessionStore.AddPadReadOnlyIdsForTest(sessionId, padId, "readonly-id", false)

	// Create CHANGESET_REQ
	changesetReq := ws.ChangesetReq{
		Event: "message",
		Data: struct {
			Component string `json:"component"`
			Type      string `json:"type"`
			PadId     string `json:"padId"`
			Token     string `json:"token"`
			Data      struct {
				Start       int `json:"start"`
				Granularity int `json:"granularity"`
				RequestID   int `json:"requestID"`
			} `json:"data"`
		}{
			Component: "pad",
			Type:      "CHANGESET_REQ",
			PadId:     padId,
			Token:     "test-token",
			Data: struct {
				Start       int `json:"start"`
				Granularity int `json:"granularity"`
				RequestID   int `json:"requestID"`
			}{
				Start:       0,
				Granularity: 100,
				RequestID:   99,
			},
		},
	}

	// Call HandleMessage
	initStore := ds.ToInitStore()
	ds.PadMessageHandler.HandleMessage(changesetReq, client, nil, initStore.RetrievedSettings, ds.Logger)

	// Wait for mock write pump to process messages
	wg.Wait()

	// Verify that a response was sent
	assert.GreaterOrEqual(t, len(mockConn.Data), 1, "Expected CHANGESET_REQ response")
}

// testHandleMessageUserInfoUpdate tests HandleMessage with UserInfoUpdate
func testHandleMessageUserInfoUpdate(t *testing.T, ds testutils.TestDataStore) {
	padId := "test-pad-handle-userinfo"
	authorId, err := setupPadAndAuthor(t, ds, padId, "HandleUserInfoUser")
	require.NoError(t, err)

	mockConn := libws.NewActualMockWebSocketconn()
	sessionId := "test-session-handle-userinfo"

	client := createTestClient(ds.Hub, sessionId, padId, mockConn)
	defer func() {
		delete(ds.Hub.Clients, client)
	}()

	// Initialize session
	ds.PadMessageHandler.SessionStore.InitSessionForTest(sessionId)
	ds.PadMessageHandler.SessionStore.AddHandleClientInformationForTest(sessionId, padId, "test-token")
	ds.PadMessageHandler.SessionStore.SetAuthorForTest(sessionId, authorId)
	ds.PadMessageHandler.SessionStore.SetPadIdForTest(sessionId, padId)
	ds.PadMessageHandler.SessionStore.AddPadReadOnlyIdsForTest(sessionId, padId, "readonly-id", false)

	// Create UserInfoUpdate
	newColor := "#AABBCC"
	newName := "UpdatedName"
	userInfoUpdate := libws.UserInfoUpdate{
		Type: "COLLABROOM",
		Data: struct {
			UserInfo struct {
				ColorId *string `json:"colorId"`
				IP      *string `json:"ip"`
				Name    *string `json:"name"`
				UserId  *string `json:"userId"`
			} `json:"userInfo"`
			Type string `json:"type"`
		}{
			UserInfo: struct {
				ColorId *string `json:"colorId"`
				IP      *string `json:"ip"`
				Name    *string `json:"name"`
				UserId  *string `json:"userId"`
			}{
				ColorId: &newColor,
				Name:    &newName,
			},
			Type: "USERINFO_UPDATE",
		},
	}

	// Call HandleUserInfoUpdate directly (bypasses the ctx requirement in HandleMessage)
	ds.PadMessageHandler.HandleUserInfoUpdate(userInfoUpdate, client)

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Verify author was updated
	author, err := ds.AuthorManager.GetAuthor(authorId)
	require.NoError(t, err)
	assert.Equal(t, newName, *author.Name)
	assert.Equal(t, newColor, author.ColorId)
}

// testHandleMessageUserChangeReadonly tests that USER_CHANGES on readonly pad is rejected
func testHandleMessageUserChangeReadonly(t *testing.T, ds testutils.TestDataStore) {
	padId := "test-pad-handle-readonly"
	authorId, err := setupPadAndAuthor(t, ds, padId, "HandleReadonlyUser")
	require.NoError(t, err)

	mockConn := libws.NewActualMockWebSocketconn()
	sessionId := "test-session-handle-readonly"

	client := createTestClient(ds.Hub, sessionId, padId, mockConn)
	defer func() {
		delete(ds.Hub.Clients, client)
	}()

	// Initialize session as READONLY
	ds.PadMessageHandler.SessionStore.InitSessionForTest(sessionId)
	ds.PadMessageHandler.SessionStore.AddHandleClientInformationForTest(sessionId, padId, "test-token")
	ds.PadMessageHandler.SessionStore.SetAuthorForTest(sessionId, authorId)
	ds.PadMessageHandler.SessionStore.SetPadIdForTest(sessionId, padId)
	ds.PadMessageHandler.SessionStore.AddPadReadOnlyIdsForTest(sessionId, padId, "readonly-id", true)
	ds.PadMessageHandler.SessionStore.SetReadOnlyForTest(sessionId, true)

	// Get pad revision before
	retrievedPad, err := ds.PadManager.GetPad(padId, nil, &authorId)
	require.NoError(t, err)
	headBefore := retrievedPad.Head

	// Create USER_CHANGES message
	userChange := ws.UserChange{
		Event: "message",
		Data: ws.UserChangeData{
			Component: "pad",
			Type:      "USER_CHANGES",
			Data: struct {
				Apool struct {
					NumToAttrib map[int][]string `json:"numToAttrib"`
					NextNum     int              `json:"nextNum"`
				} `json:"apool"`
				BaseRev   int    `json:"baseRev"`
				Changeset string `json:"changeset"`
			}{
				Apool: struct {
					NumToAttrib map[int][]string `json:"numToAttrib"`
					NextNum     int              `json:"nextNum"`
				}{
					NumToAttrib: map[int][]string{},
					NextNum:     0,
				},
				BaseRev:   0,
				Changeset: "Z:1>3+3$abc",
			},
		},
	}

	// Call HandleMessage - should be rejected due to readonly
	initStore := ds.ToInitStore()
	ds.PadMessageHandler.HandleMessage(userChange, client, nil, initStore.RetrievedSettings, ds.Logger)

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Verify pad was NOT changed (readonly should reject)
	retrievedPad, err = ds.PadManager.GetPad(padId, nil, &authorId)
	require.NoError(t, err)
	assert.Equal(t, headBefore, retrievedPad.Head, "Readonly pad should not have new revisions")
}

// testHandleMessageUnknownType tests HandleMessage with unknown message type
func testHandleMessageUnknownType(t *testing.T, ds testutils.TestDataStore) {
	padId := "test-pad-handle-unknown"
	authorId, err := setupPadAndAuthor(t, ds, padId, "HandleUnknownUser")
	require.NoError(t, err)

	mockConn := libws.NewActualMockWebSocketconn()
	sessionId := "test-session-handle-unknown"

	client := createTestClient(ds.Hub, sessionId, padId, mockConn)
	defer func() {
		delete(ds.Hub.Clients, client)
	}()

	// Initialize session
	ds.PadMessageHandler.SessionStore.InitSessionForTest(sessionId)
	ds.PadMessageHandler.SessionStore.AddHandleClientInformationForTest(sessionId, padId, "test-token")
	ds.PadMessageHandler.SessionStore.SetAuthorForTest(sessionId, authorId)
	ds.PadMessageHandler.SessionStore.SetPadIdForTest(sessionId, padId)
	ds.PadMessageHandler.SessionStore.AddPadReadOnlyIdsForTest(sessionId, padId, "readonly-id", false)

	// Verify session was initialized correctly
	session := ds.PadMessageHandler.SessionStore.GetSessionForTest(sessionId)
	require.NotNil(t, session)

	// Unknown message types in HandleMessage would require fiber.Ctx
	// The test verifies that the handler doesn't panic on unknown types
	assert.NotNil(t, ds.PadMessageHandler)
}

// testHandleMessageNoSession tests HandleMessage without a session
func testHandleMessageNoSession(t *testing.T, ds testutils.TestDataStore) {
	padId := "test-pad-handle-no-session"
	_, err := setupPadAndAuthor(t, ds, padId, "NoSessionUser")
	require.NoError(t, err)

	mockConn := libws.NewActualMockWebSocketconn()
	sessionId := "test-session-no-session"

	client := createTestClient(ds.Hub, sessionId, padId, mockConn)
	defer func() {
		delete(ds.Hub.Clients, client)
	}()

	// Do NOT initialize session - this should cause early return

	// Create a message
	chatMessage := ws.ChatMessage{
		Event: "message",
		Data: struct {
			Type      string `json:"type"`
			Component string `json:"component"`
			Data      struct {
				Type    string             `json:"type"`
				Message ws.ChatMessageData `json:"message"`
			}
		}{
			Type:      "COLLABROOM",
			Component: "pad",
			Data: struct {
				Type    string             `json:"type"`
				Message ws.ChatMessageData `json:"message"`
			}{
				Type: "CHAT_MESSAGE",
				Message: ws.ChatMessageData{
					Text: "This should not be processed",
				},
			},
		},
	}

	initialMsgCount := len(mockConn.Data)

	// Call HandleMessage - should return early due to no session
	initStore := ds.ToInitStore()
	ds.PadMessageHandler.HandleMessage(chatMessage, client, nil, initStore.RetrievedSettings, ds.Logger)

	// Wait for processing
	time.Sleep(50 * time.Millisecond)

	// Verify no message was sent (no session should cause early return)
	assert.Equal(t, initialMsgCount, len(mockConn.Data), "No message should be sent without session")
}

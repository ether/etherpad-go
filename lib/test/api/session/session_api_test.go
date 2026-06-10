package session

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ether/etherpad-go/lib/api/session"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionAPI(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)

	testDb.AddTests(
		testutils.TestRunConfig{
			Name: "CreateSession and GetSessionInfo roundtrip",
			Test: testCreateAndGetSession,
		},
		testutils.TestRunConfig{
			Name: "CreateSession validates group, author and validUntil",
			Test: testCreateSessionValidation,
		},
		testutils.TestRunConfig{
			Name: "DeleteSession removes the session",
			Test: testDeleteSession,
		},
		testutils.TestRunConfig{
			Name: "ListSessions of group and author",
			Test: testListSessions,
		},
	)

	defer testDb.StartTestDBHandler()
}

func setupGroupAndAuthor(t *testing.T, tsStore testutils.TestDataStore) (string, string) {
	t.Helper()
	groupId := "g.sessiongrp123456"
	require.NoError(t, tsStore.DS.SaveGroup(groupId))
	author, err := tsStore.AuthorManager.CreateAuthor(nil)
	require.NoError(t, err)
	return groupId, author.Id
}

func createSessionViaAPI(t *testing.T, tsStore testutils.TestDataStore, groupId string, authorId string, validUntil int64) (int, session.SessionResponse) {
	t.Helper()
	initStore := tsStore.ToInitStore()

	body, _ := json.Marshal(session.CreateSessionRequest{
		GroupID:    groupId,
		AuthorID:   authorId,
		ValidUntil: validUntil,
	})
	req := httptest.NewRequest("POST", "/admin/api/sessions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := initStore.C.Test(req)
	require.NoError(t, err)

	var response session.SessionResponse
	respBody, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(respBody, &response)
	return resp.StatusCode, response
}

func testCreateAndGetSession(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	session.Init(initStore)

	groupId, authorId := setupGroupAndAuthor(t, tsStore)
	validUntil := time.Now().Unix() + 3600

	status, created := createSessionViaAPI(t, tsStore, groupId, authorId, validUntil)
	assert.Equal(t, 200, status)
	require.NotEmpty(t, created.SessionID)
	assert.Equal(t, "s.", created.SessionID[:2])

	// Info roundtrip
	req := httptest.NewRequest("GET", "/admin/api/sessions/"+created.SessionID, nil)
	resp, err := initStore.C.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var info session.SessionInfoResponse
	body, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(body, &info)
	assert.Equal(t, groupId, info.GroupID)
	assert.Equal(t, authorId, info.AuthorID)
	assert.Equal(t, validUntil, info.ValidUntil)

	// Unknown session is a 404
	reqMissing := httptest.NewRequest("GET", "/admin/api/sessions/s.doesnotexist12345", nil)
	respMissing, _ := initStore.C.Test(reqMissing)
	assert.Equal(t, 404, respMissing.StatusCode)
}

func testCreateSessionValidation(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	session.Init(initStore)

	groupId, authorId := setupGroupAndAuthor(t, tsStore)
	future := time.Now().Unix() + 3600

	// Unknown group
	status, _ := createSessionViaAPI(t, tsStore, "g.unknowngroup1234", authorId, future)
	assert.Equal(t, 404, status)

	// Unknown author
	status, _ = createSessionViaAPI(t, tsStore, groupId, "a.unknownauthor1234", future)
	assert.Equal(t, 404, status)

	// validUntil in the past
	status, _ = createSessionViaAPI(t, tsStore, groupId, authorId, time.Now().Unix()-10)
	assert.Equal(t, 400, status)
}

func testDeleteSession(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	session.Init(initStore)

	groupId, authorId := setupGroupAndAuthor(t, tsStore)
	status, created := createSessionViaAPI(t, tsStore, groupId, authorId, time.Now().Unix()+3600)
	require.Equal(t, 200, status)

	req := httptest.NewRequest("DELETE", "/admin/api/sessions/"+created.SessionID, nil)
	resp, err := initStore.C.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	// Now gone
	reqInfo := httptest.NewRequest("GET", "/admin/api/sessions/"+created.SessionID, nil)
	respInfo, _ := initStore.C.Test(reqInfo)
	assert.Equal(t, 404, respInfo.StatusCode)

	// Deleting again is a 404
	reqAgain := httptest.NewRequest("DELETE", "/admin/api/sessions/"+created.SessionID, nil)
	respAgain, _ := initStore.C.Test(reqAgain)
	assert.Equal(t, 404, respAgain.StatusCode)
}

func testListSessions(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	session.Init(initStore)

	groupId, authorId := setupGroupAndAuthor(t, tsStore)
	future := time.Now().Unix() + 3600

	status, first := createSessionViaAPI(t, tsStore, groupId, authorId, future)
	require.Equal(t, 200, status)
	status, second := createSessionViaAPI(t, tsStore, groupId, authorId, future)
	require.Equal(t, 200, status)

	// By group
	req := httptest.NewRequest("GET", "/admin/api/groups/"+groupId+"/sessions", nil)
	resp, err := initStore.C.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var groupSessions session.SessionListResponse
	body, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(body, &groupSessions)
	sessionIds := make([]string, 0)
	for _, s := range groupSessions.Sessions {
		sessionIds = append(sessionIds, s.SessionID)
	}
	assert.Contains(t, sessionIds, first.SessionID)
	assert.Contains(t, sessionIds, second.SessionID)

	// By author
	reqAuthor := httptest.NewRequest("GET", "/admin/api/authors/"+authorId+"/sessions", nil)
	respAuthor, err := initStore.C.Test(reqAuthor)
	require.NoError(t, err)
	assert.Equal(t, 200, respAuthor.StatusCode)

	var authorSessions session.SessionListResponse
	bodyAuthor, _ := io.ReadAll(respAuthor.Body)
	_ = json.Unmarshal(bodyAuthor, &authorSessions)
	sessionIds = sessionIds[:0]
	for _, s := range authorSessions.Sessions {
		sessionIds = append(sessionIds, s.SessionID)
	}
	assert.Contains(t, sessionIds, first.SessionID)
	assert.Contains(t, sessionIds, second.SessionID)

	// Deleting a session removes it from the listings
	reqDel := httptest.NewRequest("DELETE", "/admin/api/sessions/"+first.SessionID, nil)
	respDel, err := initStore.C.Test(reqDel)
	require.NoError(t, err)
	require.Equal(t, 200, respDel.StatusCode)

	respList2, _ := initStore.C.Test(httptest.NewRequest("GET", "/admin/api/groups/"+groupId+"/sessions", nil))
	var groupSessions2 session.SessionListResponse
	bodyList2, _ := io.ReadAll(respList2.Body)
	_ = json.Unmarshal(bodyList2, &groupSessions2)
	for _, s := range groupSessions2.Sessions {
		assert.NotEqual(t, first.SessionID, s.SessionID)
	}
}

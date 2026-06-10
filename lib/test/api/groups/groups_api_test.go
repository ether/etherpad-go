package groups

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/ether/etherpad-go/lib/api/groups"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/assert"
)

func TestGroupsAPI(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)

	testDb.AddTests(
		// Create group
		testutils.TestRunConfig{
			Name: "CreateGroup successfully",
			Test: testCreateGroupSuccess,
		},
		// Delete group
		testutils.TestRunConfig{
			Name: "DeleteGroup successfully",
			Test: testDeleteGroupSuccess,
		},
		testutils.TestRunConfig{
			Name: "DeleteGroup not found returns 404",
			Test: testDeleteGroupNotFound,
		},
		// Create group pad
		testutils.TestRunConfig{
			Name: "CreateGroupPad successfully",
			Test: testCreateGroupPadSuccess,
		},
		testutils.TestRunConfig{
			Name: "CreateGroupPad invalid pad name",
			Test: testCreateGroupPadInvalidName,
		},
		testutils.TestRunConfig{
			Name: "CreateGroupPad group not found",
			Test: testCreateGroupPadGroupNotFound,
		},
		testutils.TestRunConfig{
			Name: "CreateGroupIfNotExistsFor is idempotent",
			Test: testCreateGroupIfNotExistsFor,
		},
		testutils.TestRunConfig{
			Name: "ListAllGroups returns groups",
			Test: testListAllGroups,
		},
		testutils.TestRunConfig{
			Name: "ListGroupPads returns pads of group",
			Test: testListGroupPads,
		},
	)

	defer testDb.StartTestDBHandler()
}

// Helper to create a group
func createTestGroup(t *testing.T, tsStore testutils.TestDataStore) string {
	err := tsStore.DS.SaveGroup("g.testgroup1234567")
	assert.NoError(t, err)
	return "g.testgroup1234567"
}

// ========== Create Group ==========

func testCreateGroupSuccess(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	groups.Init(initStore)

	req := httptest.NewRequest("POST", "/admin/api/groups", nil)
	resp, err := initStore.C.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var response groups.GroupIDResponse
	body, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(body, &response)

	assert.NotEmpty(t, response.GroupID)
	assert.True(t, len(response.GroupID) > 2)
	assert.Equal(t, "g.", response.GroupID[:2])
}

// ========== Delete Group ==========

func testDeleteGroupSuccess(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	groups.Init(initStore)

	// Create group first
	groupId := createTestGroup(t, tsStore)

	req := httptest.NewRequest("DELETE", "/admin/api/groups/"+groupId, nil)
	resp, err := initStore.C.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}

func testDeleteGroupNotFound(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	groups.Init(initStore)

	req := httptest.NewRequest("DELETE", "/admin/api/groups/g.nonexistent1234", nil)
	resp, err := initStore.C.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, 404, resp.StatusCode)
}

// ========== Create Group Pad ==========

func testCreateGroupPadSuccess(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	groups.Init(initStore)

	// Create group first
	groupId := createTestGroup(t, tsStore)

	reqBody := groups.CreateGroupPadRequest{
		PadName:  "testpad",
		Text:     "Initial content",
		AuthorId: "",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/admin/api/groups/"+groupId+"/pads", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := initStore.C.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var response map[string]string
	respBody, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(respBody, &response)
	assert.Contains(t, response["padID"], groupId)
}

func testCreateGroupPadInvalidName(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	groups.Init(initStore)

	// Create group first
	groupId := createTestGroup(t, tsStore)

	reqBody := groups.CreateGroupPadRequest{
		PadName: "invalid$name",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/admin/api/groups/"+groupId+"/pads", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := initStore.C.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, 400, resp.StatusCode)
}

func testCreateGroupPadGroupNotFound(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	groups.Init(initStore)

	reqBody := groups.CreateGroupPadRequest{
		PadName: "testpad",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/admin/api/groups/g.nonexistent1234/pads", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := initStore.C.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, 404, resp.StatusCode)
}

// ========== Create Group If Not Exists For ==========

func testCreateGroupIfNotExistsFor(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	groups.Init(initStore)

	body, _ := json.Marshal(groups.CreateGroupIfNotExistsForRequest{GroupMapper: "my-external-id"})
	req := httptest.NewRequest("POST", "/admin/api/groups/createIfNotExistsFor", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := initStore.C.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var first groups.GroupIDResponse
	respBody, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(respBody, &first)
	assert.Equal(t, "g.", first.GroupID[:2])

	// Same mapper returns the same group
	req2 := httptest.NewRequest("POST", "/admin/api/groups/createIfNotExistsFor", bytes.NewBuffer(body))
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := initStore.C.Test(req2)
	assert.NoError(t, err)
	var second groups.GroupIDResponse
	respBody2, _ := io.ReadAll(resp2.Body)
	_ = json.Unmarshal(respBody2, &second)
	assert.Equal(t, first.GroupID, second.GroupID)

	// Group actually exists
	_, err = tsStore.DS.GetGroup(first.GroupID)
	assert.NoError(t, err)

	// Missing mapper is a 400
	req3 := httptest.NewRequest("POST", "/admin/api/groups/createIfNotExistsFor", bytes.NewBuffer([]byte(`{}`)))
	req3.Header.Set("Content-Type", "application/json")
	resp3, _ := initStore.C.Test(req3)
	assert.Equal(t, 400, resp3.StatusCode)
}

// ========== List All Groups ==========

func testListAllGroups(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	groups.Init(initStore)

	assert.NoError(t, tsStore.DS.SaveGroup("g.listgroupsAAAAAA"))
	assert.NoError(t, tsStore.DS.SaveGroup("g.listgroupsBBBBBB"))

	req := httptest.NewRequest("GET", "/admin/api/groups", nil)
	resp, err := initStore.C.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var response groups.GroupListResponse
	body, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(body, &response)
	assert.Contains(t, response.GroupIDs, "g.listgroupsAAAAAA")
	assert.Contains(t, response.GroupIDs, "g.listgroupsBBBBBB")
}

// ========== List Group Pads ==========

func testListGroupPads(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	groups.Init(initStore)

	// pad ids require a g.<16 alphanumeric chars> group prefix
	groupId := "g.listpads12345678"[:18]
	assert.NoError(t, tsStore.DS.SaveGroup(groupId))
	padId := groupId + "$mypad"
	_, err := tsStore.PadManager.GetPad(padId, nil, nil)
	assert.NoError(t, err)

	req := httptest.NewRequest("GET", "/admin/api/groups/"+groupId+"/pads", nil)
	resp, err := initStore.C.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var response groups.PadListResponse
	body, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(body, &response)
	assert.Contains(t, response.PadIDs, padId)

	// Unknown group returns 404
	req2 := httptest.NewRequest("GET", "/admin/api/groups/g.doesnotexist1234/pads", nil)
	resp2, _ := initStore.C.Test(req2)
	assert.Equal(t, 404, resp2.StatusCode)
}

package ws

import (
	"encoding/json"
	"testing"

	"github.com/ether/etherpad-go/lib/apikeys"
	"github.com/ether/etherpad-go/lib/test/testutils"
	libws "github.com/ether/etherpad-go/lib/ws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The admin page manages the keys of the HTTP API.
func TestAdminAPIKeys(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(testutils.TestRunConfig{Name: "create, list, disable, delete", Test: testAdminAPIKeys})
	defer testDb.StartTestDBHandler()
}

func apiKeys(t *testing.T, ds testutils.TestDataStore, event string, data any) libws.APIKeysResult {
	t.Helper()
	name, raw := adminRequest(t, ds, event, data)
	require.Equal(t, "results:apiKeys", name)
	var res libws.APIKeysResult
	require.NoError(t, json.Unmarshal(raw, &res))
	return res
}

func testAdminAPIKeys(t *testing.T, ds testutils.TestDataStore) {
	res := apiKeys(t, ds, "apiKeysLoad", nil)
	assert.Empty(t, res.Error)
	assert.Empty(t, res.Keys)

	res = apiKeys(t, ds, "apiKeyCreate", map[string]string{"description": ""})
	assert.Equal(t, apikeys.ErrNoDescription.Error(), res.Error)
	assert.Nil(t, res.Created)

	res = apiKeys(t, ds, "apiKeyCreate", map[string]string{"description": "Moodle"})
	require.Empty(t, res.Error)
	require.NotNil(t, res.Created)
	moodle := res.Created.Key
	moodleID := res.Created.ID
	require.Len(t, res.Keys, 1)
	assert.Equal(t, "Moodle", res.Keys[0].Description)
	assert.Equal(t, moodle[:10], res.Keys[0].Prefix)
	assert.Equal(t, res.Created.ID, res.Keys[0].ID)

	res = apiKeys(t, ds, "apiKeyCreate", map[string]string{"description": "Backup"})
	backupID := res.Created.ID
	require.Len(t, res.Keys, 2)

	// the key itself is never listed again
	res = apiKeys(t, ds, "apiKeysLoad", nil)
	assert.Nil(t, res.Created)
	raw, _ := json.Marshal(res)
	assert.NotContains(t, string(raw), moodle)

	keys := apikeys.NewManager(ds.DS)
	_, ok := keys.Check(moodle)
	assert.True(t, ok)

	res = apiKeys(t, ds, "apiKeySetDisabled", map[string]any{"id": moodleID, "disabled": true})
	require.Empty(t, res.Error)
	for _, k := range res.Keys {
		assert.Equal(t, k.ID == moodleID, k.Disabled, k.Description)
	}
	_, ok = keys.Check(moodle)
	assert.False(t, ok, "disabled")

	res = apiKeys(t, ds, "apiKeyDelete", map[string]string{"id": backupID})
	require.Empty(t, res.Error)
	require.Len(t, res.Keys, 1)
	assert.Equal(t, "Moodle", res.Keys[0].Description)

	res = apiKeys(t, ds, "apiKeyDelete", map[string]string{"id": backupID})
	assert.Contains(t, res.Error, "not found")
	res = apiKeys(t, ds, "apiKeyCreate", "not an object")
	assert.Equal(t, "invalid request", res.Error)
}

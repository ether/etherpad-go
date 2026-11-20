package db

import (
	"testing"
	"time"

	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenStore(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(testutils.TestRunConfig{Name: "tokens", Test: testTokenStore})
	testDb.StartTestDBHandler()
}

func testTokenStore(t *testing.T, ds testutils.TestDataStore) {
	s := ds.DS
	created := time.UnixMilli(time.Now().UnixMilli())
	expires := created.Add(30 * 24 * time.Hour)
	tok := db.Token{ID: "tok1", AccountID: "acc.anna", Name: "Laptop – ünïcode", Hash: "h1", Prefix: "ep_abc",
		Scopes: []string{"pad:read", "pad:write"}, PadPattern: "claude/*", ExpiresAt: &expires,
		CreatedBy: "acc.root", CreatedAt: created, Limits: map[string]int64{"ai.requestsPerDay": 5}}
	require.NoError(t, s.CreateToken(tok))
	assert.ErrorIs(t, s.CreateToken(db.Token{ID: "tok2", AccountID: "acc.bob", Hash: "h1", CreatedAt: created}), db.ErrConflict,
		"a hash is unique")
	require.NoError(t, s.CreateToken(db.Token{ID: "tok3", AccountID: "acc.bob", Name: "Moodle", Hash: "h3", Scopes: []string{"*"},
		CreatedAt: created.Add(time.Second)}))

	got, err := s.GetTokenByHash("h1")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, tok.Name, got.Name)
	assert.Equal(t, tok.Scopes, got.Scopes)
	assert.Equal(t, "claude/*", got.PadPattern)
	assert.True(t, expires.Equal(*got.ExpiresAt))
	assert.Nil(t, got.RevokedAt)
	assert.Equal(t, int64(5), got.Limits["ai.requestsPerDay"])
	missing, err := s.GetTokenByHash("nope")
	require.NoError(t, err)
	assert.Nil(t, missing)

	all, err := s.ListTokens("")
	require.NoError(t, err)
	require.Len(t, all, 2)
	assert.Equal(t, "tok1", all[0].ID)
	bobs, err := s.ListTokens("acc.bob")
	require.NoError(t, err)
	require.Len(t, bobs, 1)
	assert.Equal(t, []string{"*"}, bobs[0].Scopes)
	assert.Nil(t, bobs[0].ExpiresAt, "never")

	revoked := created.Add(time.Minute)
	got.RevokedAt, got.ReplacedBy, got.Name = &revoked, "tok3", "renamed"
	got.ExpiresAt = nil
	require.NoError(t, s.UpdateToken(*got))
	require.NoError(t, s.UpdateToken(*got), "an update that changes nothing is no error")
	again, err := s.GetToken("tok1")
	require.NoError(t, err)
	assert.True(t, revoked.Equal(*again.RevokedAt))
	assert.Equal(t, "tok3", again.ReplacedBy)
	assert.Equal(t, "renamed", again.Name)
	assert.Nil(t, again.ExpiresAt)
	assert.Equal(t, "h1", again.Hash, "the hash never changes")
	assert.ErrorIs(t, s.UpdateToken(db.Token{ID: "nope"}), db.ErrTokenNotFound)

	used := created.Add(2 * time.Minute)
	require.NoError(t, s.TouchToken("tok3", used, "203.0.113.7"))
	moodle, err := s.GetToken("tok3")
	require.NoError(t, err)
	assert.True(t, used.Equal(*moodle.LastUsedAt))
	assert.Equal(t, "203.0.113.7", moodle.LastUsedIP)
}

package oidc

import (
	"sync"
	"testing"

	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zitadel/oidc/v3/pkg/oidc"
)

// Port of etherpad-lite's OidcProviderSecurity spec where it applies to the
// Go provider (zitadel/oidc instead of node-oidc-provider).

func TestProviderKeysAreSharedAndNotFixed(t *testing.T) {
	t.Run("the token key is random per deployment, not a fixed value", func(t *testing.T) {
		a, err := loadOrCreateCryptoKey(db.NewMemoryDataStore())
		require.NoError(t, err)
		b, err := loadOrCreateCryptoKey(db.NewMemoryDataStore())
		require.NoError(t, err)
		assert.NotEqual(t, a, b)
		assert.NotEqual(t, [32]byte{}, a)
	})

	t.Run("stable across restarts and instances on one database", func(t *testing.T) {
		store := db.NewMemoryDataStore()
		a, err := loadOrCreateCryptoKey(store)
		require.NoError(t, err)
		b, err := loadOrCreateCryptoKey(store)
		require.NoError(t, err)
		assert.Equal(t, a, b)
		k1, err := loadOrCreateSigningKey(store)
		require.NoError(t, err)
		k2, err := loadOrCreateSigningKey(store)
		require.NoError(t, err)
		assert.True(t, k1.Equal(k2))
	})

	t.Run("instances starting at once agree on one key", func(t *testing.T) {
		store := db.NewMemoryDataStore()
		var wg sync.WaitGroup
		values := make([]string, 8)
		for i := range values {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				values[i], _ = loadOrCreate(store, "race_key", func() (string, error) {
					return "value-" + string(rune('a'+i)), nil
				})
			}(i)
		}
		wg.Wait()
		for _, v := range values {
			assert.Equal(t, values[0], v, "every instance uses the stored value")
		}
		stored, err := store.GetOIDCStorageValue("race_key")
		require.NoError(t, err)
		assert.Equal(t, values[0], *stored)
	})
}

func TestOriginAllowedForOIDCClient(t *testing.T) {
	sso := &settings.SSO{Clients: []settings.SSOClient{
		{ClientId: "admin", RedirectUris: []string{"https://pad.example.com/admin/", "http://localhost:9001/admin/"}},
		{ClientId: "broken", RedirectUris: []string{"::not a url", "relative/path"}},
		{ClientId: "none"},
	}}
	var invalid []string
	allowed := allowedOrigins(sso, func(_, uri string) { invalid = append(invalid, uri) })
	assert.Equal(t, []string{"https://pad.example.com", "http://localhost:9001"}, allowed)
	assert.Equal(t, []string{"::not a url", "relative/path"}, invalid)

	assert.True(t, originAllowed("https://pad.example.com", allowed), "an origin of a registered redirect URI")
	assert.False(t, originAllowed("https://attacker.example", allowed), "an unregistered origin")
	assert.False(t, originAllowed("https://pad.example.com.attacker.example", allowed), "a look-alike suffix")
	assert.False(t, originAllowed("https://evilpad.example.com", allowed), "no substring matching")
	assert.False(t, originAllowed("http://pad.example.com", allowed), "a scheme mismatch")
	assert.False(t, originAllowed("http://localhost:9002", allowed), "another port")
	assert.False(t, originAllowed("", allowed), "a missing origin")
	assert.False(t, originAllowed("https://pad.example.com", nil), "no registered redirect URIs")
	assert.Empty(t, allowedOrigins(nil, func(string, string) {}), "no SSO configured")
}

func TestInteractiveLoginPasswordCheck(t *testing.T) {
	ts := newTestServer(t, db.NewMemoryDataStore())
	s := ts.auth.storage
	assert.True(t, s.checkPassword("admin", "adminpass"), "the correct password")
	assert.False(t, s.checkPassword("admin", "wrong"), "a wrong password")
	assert.False(t, s.checkPassword("admin", ""), "a missing password")
	assert.False(t, s.checkPassword("admin", "adminpas"), "a prefix of the password")
	assert.False(t, s.checkPassword("admin", "adminpass "), "the password with a suffix")
	assert.False(t, s.checkPassword("empty", ""), "an account with an empty password cannot log in")
	assert.False(t, s.checkPassword("empty", "anything"))
	assert.False(t, s.checkPassword("nobody", "adminpass"), "an unknown user")
	assert.False(t, s.checkPassword("", "adminpass"), "a missing login")
	assert.False(t, s.checkPassword("__proto__", ""), "no special names")
	assert.False(t, s.checkPassword("constructor", ""))

	noUsers := testSettings("http://localhost:9001")
	noUsers.Users = nil
	a, err := newAuthenticator(noUsers, db.NewMemoryDataStore())
	require.NoError(t, err)
	assert.False(t, a.storage.checkPassword("admin", "adminpass"), "no users configured")
}

func TestNonAdminTokenWhoseSubjectIsAClientIDIsRejected(t *testing.T) {
	// A user named like a client must not gain anything from that, and a
	// client's own token never carries the admin claim.
	cfg := testSettings("http://localhost:9001")
	cfg.Users[serviceClientID] = settings.User{Password: strPtr("pw"), IsAdmin: boolPtr(false)}
	a, err := newAuthenticator(cfg, db.NewMemoryDataStore())
	require.NoError(t, err)
	for _, subject := range []string{serviceClientID, adminClientID, "editor"} {
		info := newUserInfo()
		a.storage.setUserinfo(info, subject, nil)
		assert.Equal(t, false, info.Claims["admin"], subject)
	}
	info := newUserInfo()
	a.storage.setUserinfo(info, "admin", nil)
	assert.Equal(t, true, info.Claims["admin"])
}

func newUserInfo() *oidc.UserInfo { return &oidc.UserInfo{Claims: map[string]any{}} }

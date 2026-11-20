package tokens

import (
	"testing"

	"github.com/ether/etherpad-go/lib/accounts"
	"github.com/ether/etherpad-go/lib/apikeys"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTakeOverAPIKeys(t *testing.T) {
	e := newEnv(t)
	keys := apikeys.NewManager(e.store)
	moodleKey, moodle, err := keys.Create("Moodle")
	require.NoError(t, err)
	oldKey, old, err := keys.Create("old script")
	require.NoError(t, err)
	require.NoError(t, keys.SetDisabled(old.ID, true))
	keys.Check(moodleKey) // used once

	n, err := e.tok.TakeOverAPIKeys(e.store)
	require.NoError(t, err)
	assert.Equal(t, 2, n)

	// the key keeps working, with every right, as before
	use, err := e.tok.Authenticate(moodleKey, "")
	require.NoError(t, err)
	assert.Equal(t, LegacyAccount, use.Account.Username)
	assert.Equal(t, accounts.KindService, use.Account.Kind)
	assert.True(t, use.Grants.IsAdmin())
	assert.Equal(t, "Moodle", use.Token.Name)
	assert.Equal(t, moodle.Prefix, use.Token.Prefix)
	assert.Nil(t, use.Token.ExpiresAt)
	assert.NotNil(t, use.Token.LastUsedAt, "its last use came along")
	_, err = e.tok.Authenticate(oldKey, "")
	assert.ErrorIs(t, err, ErrInvalid, "a disabled key is a revoked token")

	// once only; a revoked token is not brought back
	require.NoError(t, e.tok.Revoke(e.root, use.Token.ID))
	n, err = e.tok.TakeOverAPIKeys(e.store)
	require.NoError(t, err)
	assert.Equal(t, 0, n)
	_, err = e.tok.Authenticate(moodleKey, "")
	assert.ErrorIs(t, err, ErrInvalid)

	// a key made after the take-over (an older instance in the cluster) is
	// taken over on the next start
	lateKey, _, err := keys.Create("late")
	require.NoError(t, err)
	n, err = e.tok.TakeOverAPIKeys(e.store)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	_, err = e.tok.Authenticate(lateKey, "")
	require.NoError(t, err)

	// an APIKEY.txt key
	stored, err := e.tok.ImportKey("0123456789abcdef0123456789abcdef", "APIKEY.txt")
	require.NoError(t, err)
	assert.True(t, stored)
	stored, err = e.tok.ImportKey("0123456789abcdef0123456789abcdef", "APIKEY.txt")
	require.NoError(t, err)
	assert.False(t, stored)
	use, err = e.tok.Authenticate("0123456789abcdef0123456789abcdef", "")
	require.NoError(t, err)
	assert.Equal(t, "APIKEY.txt", use.Token.Name)

	all, err := e.tok.List(e.root, "")
	require.NoError(t, err)
	assert.Len(t, all, 4)
}

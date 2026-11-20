package tokens

import (
	"errors"
	"strings"

	"github.com/ether/etherpad-go/lib/accounts"
	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/utils"
)

// LegacyAccount is the service account that holds the API keys of
// before the tokens: the keys of the former admin page "API keys" and of
// an APIKEY.txt. Like those keys, it may do everything.
const LegacyAccount = "legacy-api"

// LegacyStore is what the take-over needs: the former keys and the tokens.
type LegacyStore interface {
	Store
	db.APIKeyMethods
}

// legacyHolder returns the account of the former keys, made with the admin
// role on first use.
func (m *Manager) legacyHolder() (*db.Account, error) {
	a, err := m.accounts.AccountByUsername(LegacyAccount)
	if err != nil {
		return nil, err
	}
	if a == nil {
		a, err = m.accounts.CreateAccount(accounts.System, accounts.NewAccount{
			Username: LegacyAccount, DisplayName: "API keys of before the tokens", Kind: accounts.KindService})
		if errors.Is(err, db.ErrConflict) {
			// Another instance starting at the same time made it.
			a, err = m.accounts.AccountByUsername(LegacyAccount)
			if err == nil && a == nil {
				err = errors.New("the account " + LegacyAccount + " vanished")
			}
		}
		if err != nil {
			return nil, err
		}
	}
	if a.Kind != accounts.KindService {
		return nil, errors.New("the account " + LegacyAccount + " is not a service account")
	}
	refs, err := m.accounts.Refs(a.ID)
	if err != nil {
		return nil, err
	}
	for _, r := range refs {
		if r.Role == accounts.RoleAdmin && r.Pads == "" {
			return a, nil
		}
	}
	err = m.accounts.Assign(accounts.System, a.ID, accounts.RoleAdmin, "")
	if errors.Is(err, db.ErrConflict) {
		err = nil // assigned by another instance meanwhile
	}
	return a, err
}

// TakeOverAPIKeys makes every former API key a token of the legacy-api
// account with all scopes, never expiring, under the same hash: the keys
// keep working. A disabled key becomes a revoked token. It runs on every
// start and takes over only keys that are no token yet, so an interrupted
// take-over is finished and a revoked token is not brought back. The
// former keys stay in their table. It returns how many it took over.
func (m *Manager) TakeOverAPIKeys(store LegacyStore) (int, error) {
	keys, err := store.ListAPIKeys()
	if err != nil || len(keys) == 0 {
		return 0, err
	}
	var missing []db.APIKey
	for _, k := range keys {
		existing, err := m.store.GetTokenByHash(k.Hash)
		if err != nil {
			return 0, err
		}
		if existing == nil {
			missing = append(missing, k)
		}
	}
	if len(missing) == 0 {
		return 0, nil
	}
	// The account first: making it takes the accounts' lock itself.
	holder, err := m.legacyHolder()
	if err != nil {
		return 0, err
	}
	taken := 0
	err = m.accounts.Locked(func() error {
		taken, err = m.takeOver(holder, missing)
		return err
	})
	if taken > 0 {
		m.changed()
	}
	return taken, err
}

// takeOver stores the keys as tokens of holder, under the lock: a key
// another instance took over meanwhile is skipped, not counted twice.
func (m *Manager) takeOver(holder *db.Account, keys []db.APIKey) (int, error) {
	taken := 0
	for _, k := range keys {
		existing, err := m.store.GetTokenByHash(k.Hash)
		if err != nil {
			return taken, err
		}
		if existing != nil {
			continue
		}
		name := strings.TrimSpace(k.Description)
		if name == "" {
			name = "API key " + k.Prefix
		}
		t := db.Token{ID: "apikey-" + k.ID, AccountID: holder.ID, Name: name, Hash: k.Hash, Prefix: k.Prefix,
			Scopes: []string{accounts.AllScopes}, CreatedBy: accounts.System.ID, CreatedAt: k.CreatedAt, LastUsedAt: k.LastUsed}
		if k.Disabled {
			now := m.now()
			t.RevokedAt = &now
		}
		if err := m.store.CreateToken(t); err != nil {
			if errors.Is(err, db.ErrConflict) {
				// Taken over by another instance (no shared lock).
				continue
			}
			return taken, err
		}
		taken++
		if err := m.accounts.Record(accounts.System, "token.take-over", t.ID, map[string]any{"name": name, "prefix": k.Prefix}); err != nil {
			return taken, err
		}
	}
	return taken, nil
}

// ImportKey stores an existing key (from an APIKEY.txt) as a token of the
// legacy-api account, unless it is one already. It reports whether it
// stored it.
func (m *Manager) ImportKey(key, name string) (bool, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return false, errors.New("empty key")
	}
	existing, err := m.store.GetTokenByHash(Hash(key))
	if err != nil || existing != nil {
		return false, err
	}
	holder, err := m.legacyHolder()
	if err != nil {
		return false, err
	}
	stored := false
	err = m.accounts.Locked(func() error {
		stored, err = m.importKey(holder, key, name)
		return err
	})
	if stored {
		m.changed()
	}
	return stored, err
}

func (m *Manager) importKey(holder *db.Account, key, name string) (bool, error) {
	existing, err := m.store.GetTokenByHash(Hash(key))
	if err != nil || existing != nil {
		return false, err
	}
	t := db.Token{ID: utils.RandomString(16), AccountID: holder.ID, Name: name, Hash: Hash(key), Prefix: shown(key),
		Scopes: []string{accounts.AllScopes}, CreatedBy: accounts.System.ID, CreatedAt: m.now()}
	if err := m.store.CreateToken(t); err != nil {
		if errors.Is(err, db.ErrConflict) {
			return false, nil
		}
		return false, err
	}
	return true, m.accounts.Record(accounts.System, "token.take-over", t.ID, map[string]any{"name": name, "prefix": t.Prefix})
}

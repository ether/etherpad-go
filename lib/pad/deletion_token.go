package pad

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/utils"
)

// Pad deletion tokens (etherpad-lite #7926): the creator of a pad gets a
// secret once, so the pad can still be deleted after the browser that
// created it lost its author cookie. Only a SHA-256 hash is stored, in the
// key-value table under "pad:<id>:deletionToken".

const deletionTokenLength = 32

type storedDeletionToken struct {
	CreatedAt int64  `json:"createdAt"`
	Hash      string `json:"hash"`
}

func deletionTokenKey(padID string) string { return "pad:" + padID + ":deletionToken" }

func hashDeletionToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// DeletionTokens manages the deletion tokens of pads.
type DeletionTokens struct {
	store db.DataStore
}

func NewDeletionTokens(store db.DataStore) *DeletionTokens { return &DeletionTokens{store: store} }

// CreateIfAbsent returns a new token for the pad, or nil when the pad already
// has one: the plaintext is handed out exactly once. The insert is atomic, so
// two instances creating at once cannot both hand out a token.
func (d *DeletionTokens) CreateIfAbsent(padID string) (*string, error) {
	existing, err := d.store.GetOIDCStorageValue(deletionTokenKey(padID))
	if err == nil && existing != nil {
		return nil, nil
	}
	token := utils.RandomString(deletionTokenLength)
	payload, err := json.Marshal(storedDeletionToken{CreatedAt: time.Now().UnixMilli(), Hash: hashDeletionToken(token)})
	if err != nil {
		return nil, err
	}
	stored, err := d.store.AddOIDCStorageValue(deletionTokenKey(padID), string(payload))
	if err != nil {
		return nil, err
	}
	if stored != string(payload) {
		return nil, nil // another request was first
	}
	return &token, nil
}

// IsValid reports whether token is the pad's deletion token.
func (d *DeletionTokens) IsValid(padID, token string) bool {
	if token == "" {
		return false
	}
	raw, err := d.store.GetOIDCStorageValue(deletionTokenKey(padID))
	if err != nil || raw == nil {
		return false
	}
	var stored storedDeletionToken
	if json.Unmarshal([]byte(*raw), &stored) != nil || stored.Hash == "" {
		return false
	}
	expected, err := hex.DecodeString(stored.Hash)
	if err != nil {
		return false
	}
	actual := sha256.Sum256([]byte(token))
	return subtle.ConstantTimeCompare(expected, actual[:]) == 1
}

// Transfer hands the source pad's token to a moved pad, unless the
// destination already has one (it may have been handed out meanwhile).
// The source's token is removed with the source pad.
func (d *DeletionTokens) Transfer(srcPadID, dstPadID string) error {
	raw, err := d.store.GetOIDCStorageValue(deletionTokenKey(srcPadID))
	if err != nil || raw == nil {
		return nil
	}
	_, err = d.store.AddOIDCStorageValue(deletionTokenKey(dstPadID), *raw)
	return err
}

// Remove drops the pad's token; called when the pad is removed.
func (d *DeletionTokens) Remove(padID string) error {
	return d.store.DeleteOIDCStorageValue(deletionTokenKey(padID))
}

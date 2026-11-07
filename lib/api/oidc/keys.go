package oidc

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"

	"github.com/ether/etherpad-go/lib/db"
)

const (
	signingKeyStorageKey = "signing_key"
	cryptoKeyStorageKey  = "token_crypto_key"
	// legacyStateStorageKey held the fosite state snapshot.
	legacyStateStorageKey = "fosite_state"
)

// loadOrCreate returns the value stored under key, or stores and returns the
// one create makes. All instances on one database thereby share the value.
func loadOrCreate(store db.DataStore, key string, create func() (string, error)) (string, error) {
	raw, err := store.GetOIDCStorageValue(key)
	if err != nil {
		return "", err
	}
	if raw != nil && *raw != "" {
		return *raw, nil
	}
	value, err := create()
	if err != nil {
		return "", err
	}
	if err := store.SetOIDCStorageValue(key, value); err != nil {
		return "", err
	}
	return value, nil
}

// loadOrCreateSigningKey returns the RSA key the ID tokens are signed with.
// The key format is the same as with fosite, so existing keys are kept.
func loadOrCreateSigningKey(store db.DataStore) (*rsa.PrivateKey, error) {
	raw, err := loadOrCreate(store, signingKeyStorageKey, func() (string, error) {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return "", err
		}
		block := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}
		return string(pem.EncodeToMemory(block)), nil
	})
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode([]byte(raw))
	if block == nil {
		return nil, errors.New("invalid PEM signing key")
	}
	return x509.ParsePKCS1PrivateKey(block.Bytes)
}

// loadOrCreateCryptoKey returns the AES key zitadel/oidc encrypts opaque
// access tokens and auth codes with.
func loadOrCreateCryptoKey(store db.DataStore) ([32]byte, error) {
	var key [32]byte
	raw, err := loadOrCreate(store, cryptoKeyStorageKey, func() (string, error) {
		if _, err := rand.Read(key[:]); err != nil {
			return "", err
		}
		return base64.StdEncoding.EncodeToString(key[:]), nil
	})
	if err != nil {
		return key, err
	}
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil || len(decoded) != len(key) {
		return key, fmt.Errorf("invalid token crypto key")
	}
	copy(key[:], decoded)
	return key, nil
}

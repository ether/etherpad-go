package updater

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
)

// SignatureVerifier optionally verifies the release checksums file against a
// trusted ed25519 public key (base64-encoded, 32 bytes). When Require is false
// verification is a no-op.
type SignatureVerifier struct {
	Require   bool
	PublicKey string
}

// Verify checks an ed25519 signature over message. It returns nil when
// signatures are not required.
func (s SignatureVerifier) Verify(message, signature []byte) error {
	if !s.Require {
		return nil
	}
	if s.PublicKey == "" {
		return errors.New("signature required but no trusted public key configured")
	}
	pk, err := base64.StdEncoding.DecodeString(s.PublicKey)
	if err != nil || len(pk) != ed25519.PublicKeySize {
		return errors.New("invalid trusted public key")
	}
	if !ed25519.Verify(ed25519.PublicKey(pk), message, signature) {
		return errors.New("signature verification failed")
	}
	return nil
}

package oidc

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/ether/etherpad-go/lib/settings"
	"github.com/go-jose/go-jose/v3"
	"github.com/ory/fosite/token/jwt"
)

func newTestAuthenticator(t *testing.T) *Authenticator {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate rsa key: %v", err)
	}

	settings := testSettings()
	settings.SSO.Issuer = "https://issuer.example.com"

	return &Authenticator{
		privateKey:        privateKey,
		retrievedSettings: settings,
	}
}

func signToken(
	t *testing.T,
	privateKey *rsa.PrivateKey,
	claims jwt.MapClaims,
) string {
	t.Helper()

	token := jwt.NewWithClaims(jose.RS256, claims)
	signed, err := token.SignedString(privateKey)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	return signed
}

func TestValidateAdminToken(t *testing.T) {
	auth := newTestAuthenticator(t)

	adminClient := &settings.SSOClient{
		ClientId: "admin-client",
	}

	now := time.Now()

	tests := []struct {
		name        string
		tokenString string
		expectOK    bool
		expectErr   bool
	}{
		{
			name:        "invalid jwt",
			tokenString: "not-a-jwt",
			expectOK:    false,
			expectErr:   true,
		},
		{
			name: "expired token",
			tokenString: signToken(t, auth.privateKey, jwt.MapClaims{
				"exp":   now.Add(-time.Hour).Unix(),
				"aud":   []string{"admin-client"},
				"iss":   auth.retrievedSettings.SSO.Issuer,
				"admin": true,
			}),
			expectOK:  false,
			expectErr: true,
		},
		{
			name: "missing aud claim",
			tokenString: signToken(t, auth.privateKey, jwt.MapClaims{
				"exp":   now.Add(time.Hour).Unix(),
				"iss":   auth.retrievedSettings.SSO.Issuer,
				"admin": true,
			}),
			expectOK:  false,
			expectErr: true,
		},
		{
			name: "invalid audience",
			tokenString: signToken(t, auth.privateKey, jwt.MapClaims{
				"exp":   now.Add(time.Hour).Unix(),
				"aud":   []string{"other-client"},
				"iss":   auth.retrievedSettings.SSO.Issuer,
				"admin": true,
			}),
			expectOK:  false,
			expectErr: true,
		},
		{
			name: "invalid issuer",
			tokenString: signToken(t, auth.privateKey, jwt.MapClaims{
				"exp":   now.Add(time.Hour).Unix(),
				"aud":   []string{"admin-client"},
				"iss":   "https://wrong-issuer",
				"admin": true,
			}),
			expectOK:  false,
			expectErr: true,
		},
		{
			name: "missing admin claim",
			tokenString: signToken(t, auth.privateKey, jwt.MapClaims{
				"exp": now.Add(time.Hour).Unix(),
				"aud": []string{"admin-client"},
				"iss": auth.retrievedSettings.SSO.Issuer,
			}),
			expectOK:  false,
			expectErr: true,
		},
		{
			name: "admin false",
			tokenString: signToken(t, auth.privateKey, jwt.MapClaims{
				"exp":   now.Add(time.Hour).Unix(),
				"aud":   []string{"admin-client"},
				"iss":   auth.retrievedSettings.SSO.Issuer,
				"admin": false,
			}),
			expectOK:  false,
			expectErr: true,
		},
		{
			name: "valid admin token",
			tokenString: signToken(t, auth.privateKey, jwt.MapClaims{
				"exp":   now.Add(time.Hour).Unix(),
				"aud":   []string{"admin-client"},
				"iss":   auth.retrievedSettings.SSO.Issuer,
				"admin": true,
			}),
			expectOK:  true,
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, err := auth.ValidateAdminToken(tt.tokenString, adminClient)

			if ok != tt.expectOK {
				t.Fatalf("expected ok=%v, got %v", tt.expectOK, ok)
			}

			if tt.expectErr && err == nil {
				t.Fatalf("expected error, got nil")
			}

			if !tt.expectErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

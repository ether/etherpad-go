package settings

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Port of etherpad-lite's adminSettingsRedact spec.
func TestRedactSettings(t *testing.T) {
	t.Run("a deep copy, the input is untouched", func(t *testing.T) {
		in := map[string]any{"dbSettings": map[string]any{"password": "secret"}}
		out := RedactSettings(in).(map[string]any)
		assert.Equal(t, "secret", in["dbSettings"].(map[string]any)["password"])
		assert.Equal(t, Redacted, out["dbSettings"].(map[string]any)["password"])
	})
	t.Run("users", func(t *testing.T) {
		out := RedactSettings(map[string]any{"users": map[string]any{
			"admin": map[string]any{"password": "p1", "is_admin": true},
			"bob":   map[string]any{"passwordHash": "bcrypt$..."},
			"alice": map[string]any{"hash": "old$..."},
		}}).(map[string]any)["users"].(map[string]any)
		assert.Equal(t, Redacted, out["admin"].(map[string]any)["password"])
		assert.Equal(t, true, out["admin"].(map[string]any)["is_admin"])
		assert.Equal(t, Redacted, out["bob"].(map[string]any)["passwordHash"])
		assert.Equal(t, Redacted, out["alice"].(map[string]any)["hash"])
	})
	t.Run("dbSettings password and user", func(t *testing.T) {
		out := RedactSettings(map[string]any{"dbSettings": map[string]any{
			"host": "localhost", "user": "etherpad", "password": "secret", "filename": "/data/etherpad.db",
		}}).(map[string]any)["dbSettings"].(map[string]any)
		assert.Equal(t, Redacted, out["password"])
		assert.Equal(t, Redacted, out["user"])
		assert.Equal(t, "localhost", out["host"])
		assert.Equal(t, "/data/etherpad.db", out["filename"])
	})
	t.Run("the SMTP password", func(t *testing.T) {
		out := RedactSettings(map[string]any{"mail": map[string]any{
			"host": "smtp.example.com", "auth": map[string]any{"user": "etherpad", "pass": "secret"},
		}}).(map[string]any)["mail"].(map[string]any)
		assert.Equal(t, Redacted, out["auth"].(map[string]any)["pass"])
		assert.Equal(t, "etherpad", out["auth"].(map[string]any)["user"])
		assert.Equal(t, "smtp.example.com", out["host"])
	})
	t.Run("sso client secrets", func(t *testing.T) {
		out := RedactSettings(map[string]any{"sso": map[string]any{"clients": []any{
			map[string]any{"client_id": "app1", "client_secret": "shhh"},
			map[string]any{"client_id": "app2", "secret": "older-style"},
		}}}).(map[string]any)["sso"].(map[string]any)["clients"].([]any)
		assert.Equal(t, Redacted, out[0].(map[string]any)["client_secret"])
		assert.Equal(t, "app1", out[0].(map[string]any)["client_id"])
		assert.Equal(t, Redacted, out[1].(map[string]any)["secret"])
	})
	t.Run("sessionKey, null secrets, other keys", func(t *testing.T) {
		out := RedactSettings(map[string]any{"sessionKey": "k", "title": "Etherpad",
			"dbSettings": map[string]any{"password": nil}}).(map[string]any)
		assert.Equal(t, Redacted, out["sessionKey"])
		assert.Equal(t, "Etherpad", out["title"])
		assert.Equal(t, Redacted, out["dbSettings"].(map[string]any)["password"], "null becomes the sentinel too")
	})
	t.Run("only the exact paths", func(t *testing.T) {
		out := RedactSettings(map[string]any{"ep_plugin": map[string]any{"password": "plugin-owned", "users": map[string]any{"x": map[string]any{"password": "deep"}}}}).(map[string]any)
		p := out["ep_plugin"].(map[string]any)
		assert.Equal(t, "plugin-owned", p["password"])
		assert.Equal(t, "deep", p["users"].(map[string]any)["x"].(map[string]any)["password"])
	})
	t.Run("the live settings", func(t *testing.T) {
		pw := "dbpw"
		s := &Settings{TrustProxy: true}
		s.DBSettings = &DBSettings{Password: pw}
		s.Users = map[string]User{"admin": {Password: &pw}}
		r := s.ResolvedForAdmin()
		assert.Equal(t, true, r["trustProxy"])
		found := false
		for k, v := range r {
			if k == "dbSettings" || k == "DBSettings" {
				found = true
				for kk, vv := range v.(map[string]any) {
					if kk == "Password" || kk == "password" {
						assert.Equal(t, Redacted, vv)
					}
				}
			}
		}
		assert.True(t, found)
		for _, u := range r["users"].(map[string]any) {
			assert.Equal(t, Redacted, u.(map[string]any)["password"])
		}
	})
}

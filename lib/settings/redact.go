package settings

import (
	"encoding/json"
	"strings"
)

// Redacted replaces secret values in settings shown to the admin.
const Redacted = "[REDACTED]"

// RedactSettings returns a deep copy of settings (as decoded JSON) with the
// secrets replaced (etherpad-lite AdminSettingsRedact): users.*.password,
// passwordHash and hash, dbSettings.password and user, mail.auth.pass, sso.clients[*]
// client_secret and secret, and sessionKey. Only those exact paths are
// matched; keys compare case-insensitively since Go's settings struct does
// not use JSON names everywhere.
func RedactSettings(v any) any {
	return redactAt(clone(v), nil)
}

func clone(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[k] = clone(val)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = clone(val)
		}
		return out
	default:
		return v
	}
}

func secretPath(path []string) bool {
	p := make([]string, len(path))
	for i, s := range path {
		p[i] = strings.ToLower(s)
	}
	switch {
	case len(p) == 1 && p[0] == "sessionkey":
		return true
	case len(p) == 3 && p[0] == "users" && (p[2] == "password" || p[2] == "passwordhash" || p[2] == "hash"):
		return true
	case len(p) == 2 && p[0] == "dbsettings" && (p[1] == "password" || p[1] == "user"):
		return true
	case len(p) == 3 && p[0] == "mail" && p[1] == "auth" && p[2] == "pass":
		return true
	case len(p) == 4 && p[0] == "sso" && p[1] == "clients" && (p[3] == "client_secret" || p[3] == "secret" || p[3] == "clientsecret"):
		return true
	}
	return false
}

func redactAt(v any, path []string) any {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			sub := append(append([]string{}, path...), k)
			if secretPath(sub) {
				t[k] = Redacted
				continue
			}
			t[k] = redactAt(val, sub)
		}
		return t
	case []any:
		for i, val := range t {
			t[i] = redactAt(val, append(append([]string{}, path...), "*"))
		}
		return t
	default:
		return v
	}
}

// ResolvedForAdmin is the live settings as the admin page shows them:
// JSON-shaped, with the secrets redacted.
func (s *Settings) ResolvedForAdmin() map[string]any {
	b, err := json.Marshal(s)
	if err != nil {
		return map[string]any{}
	}
	var m map[string]any
	if json.Unmarshal(b, &m) != nil {
		return map[string]any{}
	}
	out, _ := RedactSettings(m).(map[string]any)
	return out
}

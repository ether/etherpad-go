package settings

import (
	"encoding/json"
	"errors"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// StripJSONC turns JSON with comments and trailing commas (etherpad-lite's
// settings.json format) into plain JSON. Strings are copied verbatim, so
// "//", "/*", ",]" and ",}" inside them survive.
func StripJSONC(src []byte) ([]byte, error) {
	out := make([]byte, 0, len(src))
	n := len(src)
	for i := 0; i < n; i++ {
		c := src[i]
		switch {
		case c == '"':
			start := i
			i++
			for i < n && src[i] != '"' {
				if src[i] == '\\' {
					i++
				}
				i++
			}
			if i >= n {
				return nil, errors.New("settings: unterminated string")
			}
			out = append(out, src[start:i+1]...)
		case c == '/' && i+1 < n && src[i+1] == '/':
			for i < n && src[i] != '\n' {
				i++
			}
			if i < n {
				out = append(out, '\n')
			}
		case c == '/' && i+1 < n && src[i+1] == '*':
			end := strings.Index(string(src[i+2:]), "*/")
			if end < 0 {
				return nil, errors.New("settings: unterminated comment")
			}
			i += 2 + end + 1
			out = append(out, ' ')
		case c == ']' || c == '}':
			// Drop a comma (and whitespace) right before the closing bracket.
			j := len(out) - 1
			for j >= 0 && (out[j] == ' ' || out[j] == '\t' || out[j] == '\n' || out[j] == '\r') {
				j--
			}
			if j >= 0 && out[j] == ',' {
				out = append(out[:j], out[j+1:]...)
			}
			out = append(out, c)
		default:
			out = append(out, c)
		}
	}
	return out, nil
}

var envVarValue = regexp.MustCompile(`^\$\{([^:}]+)(?::(.*))?\}$`)

// substituteEnv replaces string values of the form "${VAR}" or
// "${VAR:default}" by the variable's value, as etherpad-lite does (its
// Docker image is configured this way). The result is typed: "true",
// "false", "null" and numbers become those; "undefined" drops the key. An
// unset variable without default is null.
func substituteEnv(v any) (any, bool) {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			nv, keep := substituteEnv(val)
			if !keep {
				delete(t, k)
				continue
			}
			t[k] = nv
		}
		return t, true
	case []any:
		out := t[:0]
		for _, val := range t {
			if nv, keep := substituteEnv(val); keep {
				out = append(out, nv)
			}
		}
		return out, true
	case string:
		m := envVarValue.FindStringSubmatch(t)
		if m == nil {
			return t, true
		}
		raw, set := os.LookupEnv(m[1])
		if !set {
			if !strings.Contains(t, ":") {
				return nil, true
			}
			raw = m[2]
		}
		return coerceEnvValue(raw)
	default:
		return v, true
	}
}

func coerceEnvValue(s string) (any, bool) {
	switch s {
	case "true":
		return true, true
	case "false":
		return false, true
	case "null":
		return nil, true
	case "undefined":
		return nil, false
	}
	if s != "" && strings.TrimSpace(s) == s {
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return f, true
		}
	}
	return s, true
}

// PrepareSettingsJSON is what ReadConfig hands viper for a settings file.
func PrepareSettingsJSON(src []byte) ([]byte, error) {
	plain, err := StripJSONC(src)
	if err != nil {
		return nil, err
	}
	var parsed any
	if err := json.Unmarshal(plain, &parsed); err != nil {
		return nil, err
	}
	parsed, _ = substituteEnv(parsed)
	return json.Marshal(parsed)
}

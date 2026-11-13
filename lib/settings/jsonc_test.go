package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Port of etherpad-lite's settings spec: comments, trailing commas and
// ${VAR:default} values in settings.json.
func TestSettingsJSONC(t *testing.T) {
	parse := func(t *testing.T, src string) map[string]any {
		t.Helper()
		out, err := PrepareSettingsJSON([]byte(src))
		require.NoError(t, err)
		var m map[string]any
		require.NoError(t, json.Unmarshal(out, &m))
		return m
	}
	t.Run("comments and trailing commas", func(t *testing.T) {
		m := parse(t, "// leading line comment\n/* block comment */\n{\n  \"list\": [\n    \"a\",\n    \"b\",\n  ],\n  \"nested\": [\n    [1, 2,],\n    [3, 4,],\n  ],\n  \"obj\": {\n    \"x\": 1,\n    \"y\": 2,\n  },\n}")
		assert.Equal(t, []any{"a", "b"}, m["list"])
		assert.Equal(t, []any{[]any{1.0, 2.0}, []any{3.0, 4.0}}, m["nested"])
		assert.Equal(t, map[string]any{"x": 1.0, "y": 2.0}, m["obj"])
	})
	t.Run("strings survive", func(t *testing.T) {
		m := parse(t, `{"url": "http://example.com/a,]b,}c", "text": "trailing-comma-like ,] and ,} must survive", "c": "/* not a comment */ // nor this", "q": "say \"hi\", ]"}`)
		assert.Equal(t, "http://example.com/a,]b,}c", m["url"])
		assert.Equal(t, "trailing-comma-like ,] and ,} must survive", m["text"])
		assert.Equal(t, "/* not a comment */ // nor this", m["c"])
		assert.Equal(t, `say "hi", ]`, m["q"])
	})
	t.Run("environment variable substitution", func(t *testing.T) {
		for k, v := range map[string]string{"SET_VAR_TRUE": "true", "SET_VAR_FALSE": "false", "SET_VAR_NULL": "null",
			"SET_VAR_UNDEFINED": "undefined", "SET_VAR_NUMBER": "123", "SET_VAR_STRING": "foo", "SET_VAR_EMPTY_STRING": ""} {
			t.Setenv(k, v)
		}
		require.NoError(t, os.Unsetenv("UNSET_VAR"))
		m := parse(t, `{
  "set": {"true": "${SET_VAR_TRUE}", "false": "${SET_VAR_FALSE}", "null": "${SET_VAR_NULL}", "undefined": "${SET_VAR_UNDEFINED}",
    "number": "${SET_VAR_NUMBER}", "string": "${SET_VAR_STRING}", "empty string": "${SET_VAR_EMPTY_STRING}"},
  "unset": {"no default": "${UNSET_VAR}", "true": "${UNSET_VAR:true}", "false": "${UNSET_VAR:false}", "null": "${UNSET_VAR:null}",
    "undefined": "${UNSET_VAR:undefined}", "number": "${UNSET_VAR:123}", "string": "${UNSET_VAR:foo}", "empty string": "${UNSET_VAR:}"}
}`)
		for _, group := range []string{"set", "unset"} {
			obj := m[group].(map[string]any)
			assert.Equal(t, true, obj["true"], group)
			assert.Equal(t, false, obj["false"], group)
			assert.Nil(t, obj["null"], group)
			_, has := obj["undefined"]
			assert.False(t, has, group)
			assert.Equal(t, 123.0, obj["number"], group)
			assert.Equal(t, "foo", obj["string"], group)
			assert.Equal(t, "", obj["empty string"], group)
		}
		v, has := m["unset"].(map[string]any)["no default"]
		assert.True(t, has)
		assert.Nil(t, v)
	})
	t.Run("a Docker-style settings.json is read", func(t *testing.T) {
		t.Setenv("PORT", "9100")
		t.Setenv("TITLE", "From Env")
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "settings.json"), []byte(`/* etherpad-lite style */
{
  "title": "${TITLE:Etherpad}",
  "port": "${PORT:9001}", // the port
  "showRecentPads": "${SHOW_RECENT:false}",
}`), 0o600))
		t.Chdir(dir)
		cfg, err := ReadConfig()
		require.NoError(t, err)
		assert.Equal(t, "From Env", cfg.Title)
		assert.Equal(t, "9100", cfg.Port)
		assert.False(t, cfg.ShowRecentPads)
	})
}

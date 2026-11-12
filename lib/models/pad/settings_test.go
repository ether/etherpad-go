package pad

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// etherpad-lite: Pad.ts, "normalizePadSettings lang (issue #7586)" and
// "normalizePadSettings plugin passthrough (ep_* keys)".

func defaultsForTest(plugins bool, warnings *[]string) PadSettingsDefaults {
	return PadSettingsDefaults{
		ShowChat: true, ShowLineNumbers: true, FadeInactiveAuthorColors: true,
		EnablePluginPadOptions: plugins,
		Warn:                   func(msg string) { *warnings = append(*warnings, msg) },
	}
}

func TestNormalizePadSettingsLang(t *testing.T) {
	var w []string
	d := defaultsForTest(true, &w)

	t.Run("defaults lang to null so the client detects the locale", func(t *testing.T) {
		assert.Nil(t, NormalizePadSettings(map[string]any{}, d)["lang"])
	})
	t.Run("preserves an explicit string lang", func(t *testing.T) {
		assert.Equal(t, "de", NormalizePadSettings(map[string]any{"lang": "de"}, d)["lang"])
	})
	t.Run("drops non-string lang values to null", func(t *testing.T) {
		for _, bogus := range []any{42.0, true, map[string]any{}, []any{}, nil} {
			assert.Nil(t, NormalizePadSettings(map[string]any{"lang": bogus}, d)["lang"], "%v", bogus)
		}
		assert.Nil(t, NormalizePadSettings(map[string]any{}, d)["lang"], "undefined")
	})
}

func TestNormalizePadSettingsPluginKeys(t *testing.T) {
	t.Run("preserves ep_* keys verbatim", func(t *testing.T) {
		var w []string
		ps := NormalizePadSettings(map[string]any{
			"ep_table_of_contents": map[string]any{"enabled": true, "depth": 3.0},
			"ep_font_color":        "red",
		}, defaultsForTest(true, &w))
		assert.Equal(t, map[string]any{"enabled": true, "depth": 3.0}, ps["ep_table_of_contents"])
		assert.Equal(t, "red", ps["ep_font_color"])
	})

	t.Run("drops keys that do not match ep_<lowercase>", func(t *testing.T) {
		var w []string
		ps := NormalizePadSettings(map[string]any{
			"EP_SHOUTY": 1.0, "ep_": 1.0, "ep-dashy": 1.0, "somethingElse": 1.0,
		}, defaultsForTest(true, &w))
		for _, k := range []string{"EP_SHOUTY", "ep_", "ep-dashy", "somethingElse"} {
			assert.NotContains(t, ps, k)
		}
	})

	t.Run("an ep_<core> alias does not overwrite the core key", func(t *testing.T) {
		var w []string
		ps := NormalizePadSettings(map[string]any{"showChat": false, "ep_showchat": "plugin-value"}, defaultsForTest(true, &w))
		assert.Equal(t, false, ps["showChat"])
		assert.Equal(t, "plugin-value", ps["ep_showchat"])
	})

	t.Run("drops a value that is not JSON-serializable, with a warning", func(t *testing.T) {
		var w []string
		ps := NormalizePadSettings(map[string]any{"ep_bad": func() string { return "no" }}, defaultsForTest(true, &w))
		assert.NotContains(t, ps, "ep_bad")
		require.NotEmpty(t, w)
		assert.Contains(t, w[0], "ep_bad")
	})

	t.Run("drops a value larger than the 64 KB per-key cap", func(t *testing.T) {
		var w []string
		ps := NormalizePadSettings(map[string]any{"ep_huge": strings.Repeat("x", 70*1024)}, defaultsForTest(true, &w))
		assert.NotContains(t, ps, "ep_huge")
		require.NotEmpty(t, w)
		assert.Contains(t, w[0], "ep_huge")
		assert.Contains(t, w[0], "per-key cap")
	})

	t.Run("drops keys that would exceed the combined 256 KB cap", func(t *testing.T) {
		var w []string
		big := strings.Repeat("y", 60*1024)
		in := map[string]any{}
		for i := 0; i < 6; i++ {
			in["ep_chunk"+string(rune('0'+i))] = big
		}
		ps := NormalizePadSettings(in, defaultsForTest(true, &w))
		survivors := 0
		for k := range ps {
			if strings.HasPrefix(k, "ep_chunk") {
				survivors++
			}
		}
		assert.Less(t, survivors, 6)
		assert.Contains(t, ps, "ep_chunk0", "the first keys survive")
		require.NotEmpty(t, w)
		assert.Contains(t, w[len(w)-1], "combined ep_* size")
	})

	t.Run("drops every ep_* key when plugin pad options are off", func(t *testing.T) {
		var w []string
		ps := NormalizePadSettings(map[string]any{
			"ep_table_of_contents": map[string]any{"enabled": true}, "ep_font_color": "red",
		}, defaultsForTest(false, &w))
		assert.NotContains(t, ps, "ep_table_of_contents")
		assert.NotContains(t, ps, "ep_font_color")
	})
}

func TestNormalizePadSettingsCoreKeys(t *testing.T) {
	var w []string
	d := defaultsForTest(false, &w)
	ps := NormalizePadSettings(nil, d)
	assert.Equal(t, map[string]any{
		"enforceSettings": false, "showChat": true, "alwaysShowChat": false, "chatAndUsers": false, "lang": nil,
		"view": map[string]any{
			"showAuthorColors": true, "showLineNumbers": true, "rtlIsTrue": false,
			"padFontFamily": "", "fadeInactiveAuthorColors": true,
		},
	}, ps)

	// Server defaults apply only where nothing was set.
	d.ShowChat, d.ShowLineNumbers = false, false
	ps = NormalizePadSettings(map[string]any{"view": map[string]any{"showLineNumbers": true}}, d)
	assert.Equal(t, false, ps["showChat"])
	assert.Equal(t, true, ps["view"].(map[string]any)["showLineNumbers"])

	// JavaScript truthiness, as in upstream's !!value.
	ps = NormalizePadSettings(map[string]any{"enforceSettings": "yes", "alwaysShowChat": 0.0, "view": map[string]any{"padFontFamily": 3.0}}, d)
	assert.Equal(t, true, ps["enforceSettings"])
	assert.Equal(t, false, ps["alwaysShowChat"])
	assert.Equal(t, "", ps["view"].(map[string]any)["padFontFamily"])
}

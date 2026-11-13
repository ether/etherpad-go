package pad

import (
	"encoding/json"
	"fmt"
	"maps"
	"regexp"
	"slices"

	"github.com/ether/etherpad-go/lib/settings"
)

// Pad-wide settings (etherpad-lite's padSettings, #7586, #7698): the pad's
// creator can set chat, view and language options for everyone and enforce
// them. They are stored as JSON in the pad row and travel as plain maps, in
// the same shape etherpad-lite uses on the wire.

var pluginPadOptionKey = regexp.MustCompile(`^ep_[a-z0-9_]+$`)

const (
	// Plugin values are stored with the pad and sent to every client on
	// every change, so they must stay small.
	pluginPadOptionMaxBytes   = 64 * 1024
	pluginPadOptionTotalBytes = 256 * 1024
)

// PadSettingsDefaults are the server settings the normalisation falls back to.
type PadSettingsDefaults struct {
	ShowChat                 bool
	ShowLineNumbers          bool
	FadeInactiveAuthorColors bool
	EnablePluginPadOptions   bool
	// Warn receives a message for every dropped plugin key; may be nil.
	Warn func(msg string)
}

// DefaultsFromSettings builds PadSettingsDefaults from the server settings.
func DefaultsFromSettings(s *settings.Settings) PadSettingsDefaults {
	return PadSettingsDefaults{
		ShowChat:                 s.PadOptions.ShowChat,
		ShowLineNumbers:          s.PadOptions.ShowLineNumbers,
		FadeInactiveAuthorColors: s.PadOptions.FadeInactiveAuthorColors,
		EnablePluginPadOptions:   s.EnablePluginPadOptions,
	}
}

// truthy is JavaScript's !!v for values decoded from JSON.
func truthy(v any) bool {
	switch t := v.(type) {
	case nil:
		return false
	case bool:
		return t
	case float64:
		return t != 0 && t == t // NaN is falsy
	case int:
		return t != 0
	case string:
		return t != ""
	default:
		return true
	}
}

// NormalizePadSettings returns the complete, valid pad settings for raw,
// filling in defaults. It is etherpad-lite's Pad.normalizePadSettings.
func NormalizePadSettings(raw map[string]any, d PadSettingsDefaults) map[string]any {
	if raw == nil {
		raw = map[string]any{}
	}
	rawView, _ := raw["view"].(map[string]any)
	if rawView == nil {
		rawView = map[string]any{}
	}
	orDefault := func(m map[string]any, key string, def bool) bool {
		if v, ok := m[key]; ok && v != nil {
			return truthy(v)
		}
		return def
	}

	// lang stays null unless set, so the client's language detection runs
	// (cookie, browser language, then English).
	var lang any
	if l, ok := raw["lang"].(string); ok {
		lang = l
	}
	font := ""
	if f, ok := rawView["padFontFamily"].(string); ok {
		font = f
	}

	result := map[string]any{
		"enforceSettings": truthy(raw["enforceSettings"]),
		"showChat":        orDefault(raw, "showChat", d.ShowChat),
		"alwaysShowChat":  truthy(raw["alwaysShowChat"]),
		"chatAndUsers":    truthy(raw["chatAndUsers"]),
		"lang":            lang,
		"view": map[string]any{
			"showAuthorColors":         orDefault(rawView, "showAuthorColors", true),
			"showLineNumbers":          orDefault(rawView, "showLineNumbers", d.ShowLineNumbers),
			"rtlIsTrue":                truthy(rawView["rtlIsTrue"]),
			"padFontFamily":            font,
			"fadeInactiveAuthorColors": orDefault(rawView, "fadeInactiveAuthorColors", d.FadeInactiveAuthorColors),
		},
	}

	if !d.EnablePluginPadOptions {
		return result
	}
	warn := d.Warn
	if warn == nil {
		warn = func(string) {}
	}
	// Sorted, so which key the combined cap drops does not depend on map order.
	total := 0
	for _, key := range slices.Sorted(maps.Keys(raw)) {
		if !pluginPadOptionKey.MatchString(key) {
			continue
		}
		encoded, err := json.Marshal(raw[key])
		if err != nil {
			warn(fmt.Sprintf("[normalizePadSettings] dropping %s: value is not JSON-serializable: %v", key, err))
			continue
		}
		if len(encoded) > pluginPadOptionMaxBytes {
			warn(fmt.Sprintf("[normalizePadSettings] dropping %s: serialized size %dB exceeds per-key cap %dB",
				key, len(encoded), pluginPadOptionMaxBytes))
			continue
		}
		if total+len(encoded) > pluginPadOptionTotalBytes {
			warn(fmt.Sprintf("[normalizePadSettings] dropping %s: combined ep_* size would exceed cap %dB",
				key, pluginPadOptionTotalBytes))
			continue
		}
		total += len(encoded)
		result[key] = raw[key]
	}
	return result
}

// storedPadSettings decodes the stored settings; broken JSON counts as none.
func (p *Pad) storedPadSettings() map[string]any {
	if p.settingsJSON == "" {
		return nil
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(p.settingsJSON), &raw); err != nil {
		return nil
	}
	return raw
}

// PadSettingsJSON is the stored pad-wide settings JSON ("" for defaults),
// as exported in .etherpad files.
func (p *Pad) PadSettingsJSON() string { return p.settingsJSON }

// GetPadSettings returns the pad's normalised pad-wide settings.
func (p *Pad) GetPadSettings(d PadSettingsDefaults) map[string]any {
	return NormalizePadSettings(p.storedPadSettings(), d)
}

// SetPadSettings merges change into the current settings (the view object
// key by key), normalises and stores the result. The caller holds the pad's
// write lock. The pad in memory changes only after the store succeeded.
func (p *Pad) SetPadSettings(change map[string]any, d PadSettingsDefaults) (map[string]any, error) {
	current := p.GetPadSettings(d)
	next := maps.Clone(current)
	for k, v := range change {
		next[k] = v
	}
	view := maps.Clone(current["view"].(map[string]any))
	if changeView, ok := change["view"].(map[string]any); ok {
		for k, v := range changeView {
			view[k] = v
		}
	}
	next["view"] = view
	normalized := NormalizePadSettings(next, d)

	encoded, err := json.Marshal(normalized)
	if err != nil {
		return nil, err
	}
	if err := p.db.SetPadSettings(p.Id, string(encoded)); err != nil {
		return nil, err
	}
	p.settingsJSON = string(encoded)
	return normalized, nil
}

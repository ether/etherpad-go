//go:build js && wasm

package main

import (
	"fmt"
	"strings"
	"syscall/js"
)

func loadTranslations() map[string]string {
	result := map[string]string{}
	value := js.Global().Get("__adminTranslations")
	if value.IsUndefined() || value.IsNull() {
		return result
	}
	keys := js.Global().Get("Object").Call("keys", value)
	for i := 0; i < keys.Length(); i++ {
		key := keys.Index(i).String()
		result[key] = value.Get(key).String()
	}
	return result
}

func (a *app) t(key, fallback string) string {
	if a.state.Translations == nil {
		return fallback
	}
	if value, ok := a.state.Translations[key]; ok && value != "" {
		return value
	}
	return fallback
}

func (a *app) tf(key, fallback string, args ...any) string {
	return fmt.Sprintf(a.t(key, fallback), args...)
}

func browserLocale() string {
	lang := js.Global().Get("__adminLocale")
	if lang.IsUndefined() || lang.IsNull() || lang.String() == "" {
		return "en"
	}
	normalized := strings.ToLower(lang.String())
	if idx := strings.IndexByte(normalized, '-'); idx > 0 {
		normalized = normalized[:idx]
	}
	return normalized
}

package utils

import (
	"embed"
	"testing"

	"github.com/ether/etherpad-go/lib/hooks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Fixtures for LoadTranslations, which reads assets/locales/<lang>.json.
//
//go:embed assets/locales
var translationFixtures embed.FS

func TestLoadTranslationsFallsBackToEnglish(t *testing.T) {
	h := hooks.NewHook()
	de, err := LoadTranslations("de", translationFixtures, &h)
	require.NoError(t, err)
	assert.Equal(t, "Einstellungen", de["pad.settings.title"], "the language's own text wins")
	assert.Equal(t, "Only in English", de["pad.only.english"], "a missing key falls back to English")
	assert.NotContains(t, de, "@metadata")

	unknown, err := LoadTranslations("xx", translationFixtures, &h)
	require.NoError(t, err)
	assert.Equal(t, "Settings", unknown["pad.settings.title"])
}

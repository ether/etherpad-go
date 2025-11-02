package main

import "embed"

//go:embed assets/locales
var LocaleEmbed embed.FS

func GetEmbedForLocale() embed.FS {
	return LocaleEmbed
}

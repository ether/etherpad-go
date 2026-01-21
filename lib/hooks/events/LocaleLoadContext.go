package events

type LocaleLoadContext struct {
	RequestedLocale    string
	LoadedTranslations map[string]string
}

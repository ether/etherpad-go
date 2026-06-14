package events

// LoadSettingsContext is passed to loadSettings hooks once settings are loaded
// and plugins have registered (server startup). Settings is exposed as any to
// avoid an events->settings import cycle; plugins type-assert to *settings.Settings.
type LoadSettingsContext struct {
	Settings any
}

// ShutdownContext is passed to shutdown hooks during graceful shutdown. The
// database may be unavailable; callbacks must return quickly.
type ShutdownContext struct{}

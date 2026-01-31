package interfaces

type EpPlugin interface {
	Name() string
	Description() string
	Init(store *EpPluginStore)
	SetEnabled(enabled bool)
	IsEnabled() bool
}

package plugins

type Part struct {
	Name        string            `json:"name"`
	Hooks       map[string]string `json:"hooks"`
	ClientHooks map[string]string `json:"client_hooks"`
	Plugin      *string           `json:"plugin"`
	FullName    *string           `json:"full_name"`
}

type PluginDef struct {
	Parts []Part `json:"parts"`
}

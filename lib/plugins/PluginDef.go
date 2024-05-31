package plugins

type Part struct {
	Name     string            `json:"name"`
	Hooks    map[string]string `json:"hooks"`
	Plugin   *string
	FullName *string
}

type PluginDef struct {
	Parts []Part `json:"parts"`
}

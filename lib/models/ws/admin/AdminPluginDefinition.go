package admin

type InstalledPluginDefinition struct {
	Name         string `json:"name"`
	Version      string `json:"version"`
	Description  string `json:"description"`
	FrontendPath string `json:"frontendPath"`
	BackendPath  string `json:"backendPath"`
	Updatable    bool   `json:"updatable"`
	Enabled      bool   `json:"enabled"`
}

type PluginSearchDefinition struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Downloads   int    `json:"downloads"`
	Official    bool   `json:"official"`
	Time        string `json:"time"`
	Version     string `json:"version"`
}

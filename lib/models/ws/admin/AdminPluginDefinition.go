package admin

type InstalledPluginDefinition struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Path      string `json:"path"`
	RealPath  string `json:"realPath"`
	Updatable bool   `json:"updatable"`
}

type PluginSearchDefinition struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Downloads   int    `json:"downloads"`
	Official    bool   `json:"official"`
	Time        string `json:"time"`
	Version     string `json:"version"`
}

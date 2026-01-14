package plugins

// ToolbarButton definiert einen Toolbar-Button für ein Plugin
type ToolbarButton struct {
	Key       string `json:"key"`        // z.B. "alignLeft"
	Title     string `json:"title"`      // Lokalisierungsschlüssel oder direkter Text
	Icon      string `json:"icon"`       // CSS-Klasse für das Icon
	Group     string `json:"group"`      // "left", "middle", "right" für Gruppierung
	DataAlign string `json:"data_align"` // Optional: data-align Attribut
}

// Part repräsentiert einen Teil eines Plugins
type Part struct {
	Name           string            `json:"name"`
	Hooks          map[string]string `json:"hooks"`
	ClientHooks    map[string]string `json:"client_hooks"`
	ToolbarButtons []ToolbarButton   `json:"toolbar_buttons,omitempty"`
	Plugin         *string           `json:"plugin"`
	FullName       *string           `json:"full_name"`
}

type PluginDef struct {
	Parts []Part `json:"parts"`
}

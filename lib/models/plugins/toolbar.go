package plugins

// ToolbarButton repräsentiert einen einzelnen Toolbar-Button
type ToolbarButton struct {
	Key       string `json:"key"`
	Title     string `json:"title"`
	Icon      string `json:"icon"`
	Group     string `json:"group"` // "left", "middle", "right"
	DataAlign string `json:"dataAlign"`
}

// ToolbarButtonGroup repräsentiert eine Gruppe von Toolbar-Buttons
type ToolbarButtonGroup struct {
	PluginName string
	Buttons    []ToolbarButton
}

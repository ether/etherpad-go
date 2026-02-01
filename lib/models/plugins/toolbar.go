package plugins

// ToolbarButton repräsentiert einen einzelnen Toolbar-Button
type ToolbarButton struct {
	Key         string    `json:"key"`
	Title       string    `json:"title"`
	Icon        string    `json:"icon"`
	Group       string    `json:"group"` // "left", "middle", "right"
	DataPlugin  string    `json:"dataPlugin"`
	Type        string    `json:"type"` // "button", "toggle", "dropdown"
	SelectClass string    `json:"selectClass,omitempty"`
	Options     []Options `json:"options"`
}

type Options struct {
	Title string `json:"title,omitempty"`
	Value string `json:"value,omitempty"`
}

// ToolbarButtonGroup represents a group of toolbar buttons belonging to a plugin
type ToolbarButtonGroup struct {
	PluginName string
	Buttons    []ToolbarButton
}

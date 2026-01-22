package plugins

import (
	pluginTypes "github.com/ether/etherpad-go/lib/models/plugins"
)

// ToolbarButton is an alias to the type in models/plugins
type ToolbarButton = pluginTypes.ToolbarButton

// ToolbarButtonGroup is an alias to the type in models/plugins
type ToolbarButtonGroup = pluginTypes.ToolbarButtonGroup

type SettingsMenuItemGroup = pluginTypes.SettingsMenuItemGroup

type SettingsMenuItem = pluginTypes.SettingsMenuItem

type Part struct {
	Name              string             `json:"name"`
	Hooks             map[string]string  `json:"hooks"`
	ClientHooks       map[string]string  `json:"client_hooks"`
	ToolbarButtons    []ToolbarButton    `json:"toolbar_buttons,omitempty"`
	SettingsMenuItems []SettingsMenuItem `json:"settings_menu_items,omitempty"`
	Plugin            *string            `json:"plugin"`
	FullName          *string            `json:"full_name"`
}

type PluginDef struct {
	Parts []Part `json:"parts"`
}

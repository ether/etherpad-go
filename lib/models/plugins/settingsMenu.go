package plugins

type SettingsMenuItemGroup struct {
	PluginName string
	Items      []SettingsMenuItem
}
type SettingsMenuItem struct {
	Title string `json:"title"`
	Key   string `json:"key"`
	Id    string `json:"id"`
}

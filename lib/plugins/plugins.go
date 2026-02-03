package plugins

import (
	"encoding/json"
	"os"
	"path"

	"github.com/ether/etherpad-go/lib/settings"
	"github.com/gofiber/fiber/v3"
)

type Plugin struct {
	Name     string
	Version  string
	Path     string
	RealPath string
}

const corePluginName = "ep_etherpad-lite"

// GetPackages gibt alle installierten Plugins zurück
func GetPackages() map[string]Plugin {
	var mappedPlugins = make(map[string]Plugin)
	root, _ := os.Getwd()
	mappedPlugins[corePluginName] = Plugin{
		Name:     corePluginName,
		Version:  "1.8.13",
		Path:     "node_modules/" + corePluginName,
		RealPath: path.Join(root, "assets", "ep.json"),
	}

	pluginsDir := path.Join(root, "plugins")
	entries, err := os.ReadDir(pluginsDir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				pluginName := entry.Name()
				pluginPath := path.Join(pluginsDir, pluginName)
				epJsonPath := path.Join(pluginPath, "ep.json")
				if _, err := os.Stat(epJsonPath); err == nil {
					mappedPlugins[pluginName] = Plugin{
						Name:     pluginName,
						Version:  "0.0.1", // Standardversion, falls keine package.json vorhanden
						Path:     "node_modules/" + pluginName,
						RealPath: epJsonPath,
					}
				}
			}
		}
	}

	return mappedPlugins
}

// GetEnabledPackages gibt nur die in den Settings aktivierten Plugins zurück
func GetEnabledPackages() map[string]Plugin {
	allPackages := GetPackages()
	enabledPackages := make(map[string]Plugin)

	for name, plugin := range allPackages {
		// ep_etherpad-lite is always activated
		if name == corePluginName {
			enabledPackages[name] = plugin
			continue
		}

		// Prüfe, ob das Plugin in den Settings aktiviert ist
		if settings.Displayed.IsPluginEnabled(name) {
			enabledPackages[name] = plugin
		}
	}

	return enabledPackages
}

func Update() (map[string]Plugin, map[string]Part, map[string]Plugin) {
	var packages = GetEnabledPackages()
	var parts = make(map[string]Part)
	var plugins = make(map[string]Plugin)

	for _, plugin := range packages {
		LoadPlugin(plugin, plugins, parts)
	}

	return packages, parts, plugins
}

func LoadPlugin(plugin Plugin, plugins map[string]Plugin, parts map[string]Part) {
	var pluginPath = path.Join(plugin.RealPath)

	bytes, err := os.ReadFile(pluginPath)
	if err != nil {
		println("Error reading plugin file")
		return
	}
	var pluginDef PluginDef
	err = json.Unmarshal(bytes, &pluginDef)
	if err != nil {
		panic(err)
	}

	plugins[plugin.Name] = plugin

	for _, part := range pluginDef.Parts {
		part.Plugin = &plugin.Name
		var fullName = plugin.Name + "/" + part.Name
		part.FullName = &fullName
		parts[*part.FullName] = part
	}
}

type ClientPlugin struct {
	Plugins map[string]string `json:"plugins"`
	Parts   []Part            `json:"parts"`
}

func ReturnPluginResponse(c fiber.Ctx) error {
	packages, parts, _ := Update()

	var clientPlugins = ClientPlugin{
		Plugins: map[string]string{},
		Parts:   make([]Part, 0),
	}

	for _, pkg := range packages {
		clientPlugins.Plugins[pkg.Name] = pkg.Version
	}

	for _, part := range parts {
		clientPlugins.Parts = append(clientPlugins.Parts, part)
	}

	var clPlugin, _ = json.Marshal(clientPlugins)
	c.GetRespHeaders()["Content-Type"] = []string{"application/json"}
	return c.Send(clPlugin)
}

// GetToolbarButtons gibt alle Toolbar-Buttons von aktivierten Plugins zurück
func GetToolbarButtons() []ToolbarButton {
	_, parts, _ := Update()
	var buttons []ToolbarButton

	for _, part := range parts {
		if len(part.ToolbarButtons) > 0 {
			buttons = append(buttons, part.ToolbarButtons...)
		}
	}

	return buttons
}

func GetSettingsMenuGroups() []SettingsMenuItemGroup {
	_, parts, _ := Update()
	var groups []SettingsMenuItemGroup

	for _, part := range parts {
		if len(part.SettingsMenuItems) > 0 {
			pluginName := ""
			if part.Plugin != nil {
				pluginName = *part.Plugin
			}
			groups = append(groups, SettingsMenuItemGroup{
				PluginName: pluginName,
				Items:      part.SettingsMenuItems,
			})
		}
	}
	return groups
}

// GetToolbarButtonGroups gibt Toolbar-Buttons gruppiert nach Plugin zurück
func GetToolbarButtonGroups() []ToolbarButtonGroup {
	_, parts, _ := Update()
	var groups []ToolbarButtonGroup

	for _, part := range parts {
		if len(part.ToolbarButtons) > 0 {
			pluginName := ""
			if part.Plugin != nil {
				pluginName = *part.Plugin
			}
			groups = append(groups, ToolbarButtonGroup{
				PluginName: pluginName,
				Buttons:    part.ToolbarButtons,
			})
		}
	}

	return groups
}

// Cached plugin data
var cachedPlugins = map[string]Plugin{}
var cachedParts = map[string]Part{}
var cachedPackages = map[string]Plugin{}

func init() {
	GetCachedPlugins()
}

// GetCachedPlugins returns cached plugins, loading them if necessary
func GetCachedPlugins() map[string]Plugin {
	if len(cachedPlugins) == 0 {
		cachedPackages, cachedParts, cachedPlugins = Update()
	}
	return cachedPlugins
}

// GetCachedParts returns cached parts, loading them if necessary
func GetCachedParts() map[string]Part {
	if cachedParts == nil {
		cachedPackages, cachedParts, cachedPlugins = Update()
	}
	return cachedParts
}

// GetCachedPackages returns cached packages, loading them if necessary
func GetCachedPackages() map[string]Plugin {
	if cachedPackages == nil {
		cachedPackages, cachedParts, cachedPlugins = Update()
	}
	return cachedPackages
}

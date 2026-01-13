package plugins

import (
	"encoding/json"
	"os"
	"path"

	"github.com/ether/etherpad-go/lib/settings"
	"github.com/gofiber/fiber/v2"
)

type Plugin struct {
	Name     string
	Version  string
	Path     string
	RealPath string
}

// GetPackages gibt alle installierten Plugins zurück
func GetPackages() map[string]Plugin {
	var mappedPlugins = make(map[string]Plugin)
	root, _ := os.Getwd()
	mappedPlugins["ep_etherpad-lite"] = Plugin{
		Name:     "ep_etherpad-lite",
		Version:  "1.8.13",
		Path:     "node_modules/ep_etherpad-lite",
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
		// ep_etherpad-lite ist immer aktiviert (Core-Plugin)
		if name == "ep_etherpad-lite" {
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

func ReturnPluginResponse(c *fiber.Ctx) error {
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

package plugins

import (
	"encoding/json"
	"os"
	"path"
)

type Plugin struct {
	Name     string
	Version  string
	Path     string
	RealPath string
}

func GetPackages() map[string]Plugin {
	var mappedPlugins = make(map[string]Plugin)
	root, _ := os.Getwd()
	mappedPlugins["ep_etherpad-lite"] = Plugin{
		Name:     "ep_etherpad-lite",
		Version:  "1.8.13",
		Path:     "node_modules/ep_etherpad-lite",
		RealPath: path.Join(root, "assets", "ep.json"),
	}

	return mappedPlugins
}

func Update() (map[string]Plugin, map[string]Part, map[string]Plugin) {
	var packages = GetPackages()
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
		panic(err)
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

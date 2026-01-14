/**
 * Plugin Build Script
 *
 * Dieses Script scannt das plugins/-Verzeichnis nach Plugin-Definitionen
 * und generiert eine aktualisierte plugin_registry.ts mit allen client_hooks Modulen.
 *
 * Verwendung: node scripts/build-plugins.js
 */

import * as fs from 'node:fs';
import * as path from 'node:path';

const PLUGINS_DIR = '../plugins';
const ASSETS_EP_JSON = '../assets/ep.json';
const REGISTRY_OUTPUT = './src/js/pluginfw/plugin_registry.ts';

interface Part {
  name: string;
  hooks?: Record<string, string>;
  client_hooks?: Record<string, string>;
  plugin?: string;
  full_name?: string;
}

interface PluginDef {
  parts: Part[];
}

interface ClientPluginDef {
  plugins: Record<string, string>;
  parts: Part[];
}

/**
 * Lädt die ep.json eines Plugins
 */
function loadPluginDef(pluginPath: string): PluginDef | null {
  try {
    const content = fs.readFileSync(pluginPath, 'utf-8');
    return JSON.parse(content);
  } catch (err) {
    console.error(`Failed to load plugin definition from ${pluginPath}:`, err);
    return null;
  }
}

/**
 * Sammelt alle client_hooks Module aus den Plugin-Definitionen
 */
function collectClientHooksModules(parts: Part[]): Map<string, string> {
  const modules = new Map<string, string>();

  for (const part of parts) {
    if (part.client_hooks) {
      for (const [, hookPath] of Object.entries(part.client_hooks)) {
        const modulePath = hookPath.split(':')[0];
        modules.set(modulePath, modulePath);
      }
    }
  }

  return modules;
}

/**
 * Konvertiert einen Plugin-Modul-Pfad zu einem relativen Require-Pfad
 */
function modulePathToRequirePath(modulePath: string): string {
  // ep_etherpad-lite/static/js/messageHandler -> ../messageHandler
  if (modulePath.startsWith('ep_etherpad-lite/static/js/')) {
    const localPath = modulePath.replace('ep_etherpad-lite/static/js/', '');
    return `../${localPath}`;
  }

  // Externe Plugins: ep_example/static/js/index -> ../../plugins/ep_example/static/js/index
  // Diese werden dynamisch geladen, nicht gebündelt
  return null;
}

/**
 * Generiert die plugin_registry.ts Datei
 */
function generatePluginRegistry(modules: Map<string, string>): string {
  const builtinModules: string[] = [];

  Array.from(modules.entries()).forEach(([modulePath]) => {
    const requirePath = modulePathToRequirePath(modulePath);
    if (requirePath) {
      builtinModules.push(`  '${modulePath}': require('${requirePath}'),`);
    }
  });

  return `// @ts-nocheck
'use strict';

/**
 * Plugin Registry - Registriert alle client_hooks Module zur Build-Zeit
 *
 * AUTOMATISCH GENERIERT - NICHT MANUELL BEARBEITEN
 * Generiert von: scripts/build-plugins.js
 */

const pluginUtils = require('./shared');

// Mapping von Modul-Pfaden zu ihren Implementierungen
// Dieser Block wird zur Build-Zeit aufgelöst
const builtinModules = {
${builtinModules.join('\n')}
};

/**
 * Registriert alle eingebauten Plugin-Module
 */
const registerBuiltinPlugins = () => {
  for (const [path, module] of Object.entries(builtinModules)) {
    pluginUtils.registerPluginModule(path, module);
  }
};

/**
 * Gibt eine Map aller verfügbaren Module zurück
 */
const getModuleMap = () => {
  const map = new Map();
  for (const [path, module] of Object.entries(builtinModules)) {
    map.set(path, module);
  }
  return map;
};

// Automatisch beim Import registrieren
registerBuiltinPlugins();

exports.registerBuiltinPlugins = registerBuiltinPlugins;
exports.getModuleMap = getModuleMap;
exports.builtinModules = builtinModules;
`;
}

/**
 * Generiert die plugin-definitions.json Datei
 */
function generatePluginDefinitions(plugins: Map<string, string>, parts: Part[]): ClientPluginDef {
  return {
    plugins: Object.fromEntries(plugins),
    parts: parts,
  };
}

/**
 * Hauptfunktion
 */
function main() {
  const allParts: Part[] = [];
  const allPlugins = new Map<string, string>();

  // 1. Lade das Core-Plugin (ep_etherpad-lite)
  const corePluginPath = path.resolve(__dirname, ASSETS_EP_JSON);
  const corePlugin = loadPluginDef(corePluginPath);

  if (corePlugin) {
    allPlugins.set('ep_etherpad-lite', '1.8.13');
    for (const part of corePlugin.parts) {
      part.plugin = 'ep_etherpad-lite';
      part.full_name = `ep_etherpad-lite/${part.name}`;
      allParts.push(part);
    }
  }

  // 2. Lade externe Plugins aus dem plugins/-Verzeichnis
  const pluginsDir = path.resolve(__dirname, PLUGINS_DIR);
  if (fs.existsSync(pluginsDir)) {
    const entries = fs.readdirSync(pluginsDir, { withFileTypes: true });

    for (const entry of entries) {
      if (entry.isDirectory()) {
        const pluginName = entry.name;
        const epJsonPath = path.join(pluginsDir, pluginName, 'ep.json');

        if (fs.existsSync(epJsonPath)) {
          const pluginDef = loadPluginDef(epJsonPath);
          if (pluginDef) {
            allPlugins.set(pluginName, '0.0.1');
            for (const part of pluginDef.parts) {
              part.plugin = pluginName;
              part.full_name = `${pluginName}/${part.name}`;
              allParts.push(part);
            }
          }
        }
      }
    }
  }

  // 3. Sammle alle client_hooks Module
  const clientHooksModules = collectClientHooksModules(allParts);

  // 4. Generiere plugin_registry.ts
  const registryContent = generatePluginRegistry(clientHooksModules);
  const registryPath = path.resolve(__dirname, REGISTRY_OUTPUT);
  fs.writeFileSync(registryPath, registryContent);
  console.log(`Generated: ${registryPath}`);

  // 5. Generiere plugin-definitions.json
  const pluginDefs = generatePluginDefinitions(allPlugins, allParts);
  const pluginDefsPath = path.resolve(__dirname, PLUGINS_DIR, 'plugin-definitions.json');
  fs.writeFileSync(pluginDefsPath, JSON.stringify(pluginDefs, null, 2));
  console.log(`Generated: ${pluginDefsPath}`);

  console.log(`\nRegistered ${allPlugins.size} plugin(s) with ${allParts.length} part(s)`);
  console.log(`Found ${clientHooksModules.size} client_hooks module(s)`);
}

main();


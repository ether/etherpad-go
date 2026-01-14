import * as esbuild from 'esbuild';
import * as fs from "node:fs";
import * as path from "node:path";
import {exec, execSync} from "node:child_process";

// ========================================
// Plugin Registry Generator
// ========================================

const PLUGINS_DIR = '../plugins';
const ASSETS_EP_JSON = '../assets/ep.json';
const REGISTRY_OUTPUT = './src/js/pluginfw/plugin_registry.ts';

/**
 * Lädt die ep.json eines Plugins
 */
function loadPluginDef(pluginPath) {
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
function collectClientHooksModules(parts) {
  const modules = new Map();

  for (const part of parts) {
    if (part.client_hooks) {
      for (const [hookName, hookPath] of Object.entries(part.client_hooks)) {
        const modulePath = hookPath.split(':')[0];
        modules.set(modulePath, modulePath);
      }
    }
  }

  return modules;
}

/**
 * Konvertiert einen Plugin-Modul-Pfad zu einem relativen Require-Pfad
 * Der Pfad ist relativ zu src/js/pluginfw/ wo die plugin_registry.ts liegt
 */
function modulePathToRequirePath(modulePath) {
  if (modulePath.startsWith('ep_etherpad-lite/static/js/')) {
    const localPath = modulePath.replace('ep_etherpad-lite/static/js/', '');
    return `../${localPath}`;
  }

  // Externe Plugins: ep_align/static/js/index -> ../../../../plugins/ep_align/static/js/index
  // Von src/js/pluginfw/ aus: ../../../ geht zu ui/, dann ../plugins/
  const pluginMatch = modulePath.match(/^(ep_[^/]+)\/static\/js\/(.+)$/);
  if (pluginMatch) {
    const [, pluginName, jsPath] = pluginMatch;
    return `../../../../plugins/${pluginName}/static/js/${jsPath}`;
  }

  return null;
}

/**
 * Generiert die plugin_registry.ts Datei
 */
function generatePluginRegistry(modules) {
  const builtinModules = [];

  for (const [modulePath] of modules) {
    const requirePath = modulePathToRequirePath(modulePath);
    if (requirePath) {
      builtinModules.push(`  '${modulePath}': require('${requirePath}'),`);
    }
  }

  return `// @ts-nocheck
'use strict';

/**
 * Plugin Registry - Registriert alle client_hooks Module zur Build-Zeit
 *
 * AUTOMATISCH GENERIERT - NICHT MANUELL BEARBEITEN
 * Generiert von: build.js
 */

const pluginUtils = require('./shared');

// Mapping von Modul-Pfaden zu ihren Implementierungen
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
 * Generiert die Plugin-Definitionen und Registry
 */
function buildPlugins() {
  const allParts = [];
  const allPlugins = new Map();
  const absWorkingDir = process.cwd();

  // 1. Lade das Core-Plugin (ep_etherpad-lite)
  const corePluginPath = path.resolve(absWorkingDir, ASSETS_EP_JSON);
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
  const pluginsDir = path.resolve(absWorkingDir, PLUGINS_DIR);
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
  const registryPath = path.resolve(absWorkingDir, REGISTRY_OUTPUT);
  fs.writeFileSync(registryPath, registryContent);
  console.log(`Generated: ${registryPath}`);

  // 5. Generiere plugin-definitions.json
  const pluginDefs = {
    plugins: Object.fromEntries(allPlugins),
    parts: allParts,
  };
  const pluginDefsPath = path.resolve(absWorkingDir, PLUGINS_DIR, 'plugin-definitions.json');
  fs.writeFileSync(pluginDefsPath, JSON.stringify(pluginDefs, null, 2));
  console.log(`Generated: ${pluginDefsPath}`);

  console.log(`Registered ${allPlugins.size} plugin(s) with ${allParts.length} part(s)`);
  console.log(`Found ${clientHooksModules.size} client_hooks module(s)\n`);

  // Rückgabe für Alias-Generierung
  return { allPlugins, clientHooksModules };
}

// Generiere Plugin-Definitionen vor dem Build
console.log('Building plugin definitions...');
const { allPlugins, clientHooksModules } = buildPlugins();

// ========================================
// esbuild Configuration
// ========================================

const relativePath = 'ep_etherpad-lite/static/js';

const moduleResolutionPath = "./src/js"

// Basis-Aliase
const alias = {
    [`${relativePath}/ace2_inner`]: `${moduleResolutionPath}/ace2_inner`,
    [`${relativePath}/ace2_common`]: `${moduleResolutionPath}/ace2_common`,
    [`${relativePath}/pluginfw/client_plugins`]: `${moduleResolutionPath}/pluginfw/client_plugins`,
    [`${relativePath}/pluginfw/plugin_defs`]: `${moduleResolutionPath}/pluginfw/plugin_defs`,
    [`${relativePath}/pluginfw/hooks`]: `${moduleResolutionPath}/pluginfw/hooks`,
    [`${relativePath}/pluginfw/shared`]: `${moduleResolutionPath}/pluginfw/shared`,
    [`${relativePath}/rjquery`]: `${moduleResolutionPath}/rjquery`,
    [`${relativePath}/nice-select`]: `${moduleResolutionPath}/vendors/nice-select`,
    // Client hooks module mappings
    [`${relativePath}/messageHandler`]: `${moduleResolutionPath}/messageHandler`,
    [`${relativePath}/pad`]: `${moduleResolutionPath}/pad`,
    [`${relativePath}/chat`]: `${moduleResolutionPath}/chat`,
    [`${relativePath}/pad_editbar`]: `${moduleResolutionPath}/pad_editbar`,
    [`${relativePath}/pad_impexp`]: `${moduleResolutionPath}/pad_impexp`,
    [`${relativePath}/collab_client`]: `${moduleResolutionPath}/collab_client`,
    [`${relativePath}/broadcast`]: `${moduleResolutionPath}/broadcast`,
    [`${relativePath}/broadcast_slider`]: `${moduleResolutionPath}/broadcast_slider`,
    [`${relativePath}/broadcast_revisions`]: `${moduleResolutionPath}/broadcast_revisions`,
    [`${relativePath}/colorutils`]: `${moduleResolutionPath}/colorutils`,
    [`${relativePath}/cssmanager`]: `${moduleResolutionPath}/cssmanager`,
    [`${relativePath}/pad_utils`]: `${moduleResolutionPath}/pad_utils`,
    [`${relativePath}/pad_cookie`]: `${moduleResolutionPath}/pad_cookie`,
    [`${relativePath}/pad_editor`]: `${moduleResolutionPath}/pad_editor`,
    [`${relativePath}/pad_userlist`]: `${moduleResolutionPath}/pad_userlist`,
    [`${relativePath}/pad_modals`]: `${moduleResolutionPath}/pad_modals`,
    [`${relativePath}/pad_savedrevs`]: `${moduleResolutionPath}/pad_savedrevs`,
    [`${relativePath}/pad_connectionstatus`]: `${moduleResolutionPath}/pad_connectionstatus`,
    [`${relativePath}/pad_automatic_reconnect`]: `${moduleResolutionPath}/pad_automatic_reconnect`,
    [`${relativePath}/scroll`]: `${moduleResolutionPath}/scroll`,
    [`${relativePath}/caretPosition`]: `${moduleResolutionPath}/caretPosition`,
    [`${relativePath}/security`]: `${moduleResolutionPath}/security`,
    [`${relativePath}/Changeset`]: `${moduleResolutionPath}/Changeset`,
    [`${relativePath}/AttributePool`]: `${moduleResolutionPath}/AttributePool`,
    [`${relativePath}/ace`]: `${moduleResolutionPath}/ace`,
    [`${relativePath}/timeslider`]: `${moduleResolutionPath}/timeslider`,
    [`${relativePath}/socketio`]: `${moduleResolutionPath}/socketio`,
    [`${relativePath}/underscore`]: `${moduleResolutionPath}/underscore`,
    [`${relativePath}/skin_variants`]: `${moduleResolutionPath}/skin_variants`,
};

// Dynamisch Aliase für externe Plugins hinzufügen
for (const [modulePath] of clientHooksModules) {
  const pluginMatch = modulePath.match(/^(ep_[^/]+)\/static\/js\/(.+)$/);
  if (pluginMatch && !modulePath.startsWith('ep_etherpad-lite/')) {
    const [, pluginName, jsPath] = pluginMatch;
    alias[modulePath] = `../plugins/${pluginName}/static/js/${jsPath}`;
    console.log(`Added alias: ${modulePath} -> ../plugins/${pluginName}/static/js/${jsPath}`);
  }
}

const absWorkingDir = process.cwd()

const loaders = {
    '.woff': 'base64',
    '.woff2': 'base64',
    '.ttf': 'base64',
    '.eot': 'base64',
    '.svg': 'base64',
    '.png': 'base64',
    '.jpg': 'base64',
    '.gif': 'base64',
    '.otf': 'base64',
}

await esbuild.buildSync({
    entryPoints: ["./src/pad.js"],
    absWorkingDir: absWorkingDir,
    bundle: true,
    write: true,
    minify: true,
    outdir: '../assets/js/pad/assets',
    logLevel: 'info',
    metafile: true,
    target: 'es2020',
    alias,
    loader:loaders,
    sourcemap: 'inline',
});

await esbuild.buildSync({
    entryPoints: ["./src/welcome.js"],
    absWorkingDir: absWorkingDir,
    bundle: true,
    write: true,
    minify: true,
    outdir: '../assets/js/welcome/assets',
    logLevel: 'info',
    metafile: true,
    target: 'es2020',
    alias,
    loader:loaders,
    sourcemap: 'inline',
});

await esbuild.buildSync({
    entryPoints: ["./src/timeslider.js"],
    absWorkingDir: absWorkingDir,
    bundle: true,
    write: true,
    minify: true,
    outdir: '../assets/js/timeslider/assets',
    logLevel: 'info',
    metafile: true,
    target: 'es2020',
    alias,
    loader: loaders,
    sourcemap: 'inline',
});

await esbuild.buildSync({
    entryPoints: ['../assets/css/skin/colibris/pad.css'],
    absWorkingDir: absWorkingDir,
    bundle: true,
    write: true,
    minify: true,
    outdir: '../assets/css/build/skin/colibris',
    logLevel: 'info',
    metafile: true,
    target: 'es2020',
    external: ['*.woff', '*.woff2', '*.ttf', '*.eot', '*.svg', '*.png', '*.jpg', '*.gif'],
    sourcemap: 'inline',
    loader:loaders,
})

execSync("pnpm run build-admin", {
    cwd: '../admin'
})

await esbuild.buildSync({
    entryPoints: ['../assets/css/static/pad.css'],
    absWorkingDir: absWorkingDir,
    bundle: true,
    write: true,
    minify: true,
    outdir: '../assets/css/build/static',
    logLevel: 'info',
    external: ['*.woff', '*.woff2', '*.ttf', '*.eot', '*.svg', '*.png', '*.jpg', '*.gif', '/font/*', 'font/*'],
    loader: loaders,
    metafile: true,
    target: 'es2020',
    sourcemap: 'inline',
})
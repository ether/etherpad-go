// @ts-nocheck
'use strict';

const pluginUtils = require('./shared');
const defs = require('./plugin_defs');

exports.baseURL = '';

exports.ensure = (cb) => !defs.loaded ? exports.update(cb) : cb();

// Lädt ein Plugin-Script dynamisch
const loadPluginScript = (pluginPath) => {
  return new Promise((resolve, reject) => {
    // Konvertiere Plugin-Pfad zu URL
    // z.B. "ep_align/static/js/index" -> "/static/plugins/ep_align/static/js/index.js"
    const url = `${exports.baseURL}static/plugins/${pluginPath}.js?v=${clientVars.randomVersionString}`;

    const script = document.createElement('script');
    script.src = url;
    script.type = 'text/javascript';
    script.onload = () => {
      // Das Plugin sollte sich selbst über window registriert haben
      const moduleName = pluginPath;
      if (window[moduleName]) {
        pluginUtils.registerPluginModule(pluginPath, window[moduleName]);
      }
      resolve();
    };
    script.onerror = (err) => {
      console.warn(`Failed to load plugin script: ${url}`, err);
      resolve(); // Nicht ablehnen, damit andere Plugins geladen werden können
    };
    document.head.appendChild(script);
  });
};

// Extrahiert alle Plugin-Pfade aus den Parts
const getPluginPaths = (parts) => {
  const paths = new Set();
  for (const part of parts) {
    if (part.client_hooks) {
      for (const hookFnPath of Object.values(part.client_hooks)) {
        // Extrahiere den Modul-Pfad (ohne Funktionsnamen nach dem Doppelpunkt)
        const modulePath = hookFnPath.split(':')[0];
        // Ignoriere ep_etherpad-lite Pfade (sind im Build enthalten)
        if (!modulePath.startsWith('ep_etherpad-lite/')) {
          paths.add(modulePath);
        }
      }
    }
  }
  return [...paths];
};

exports.update = async (modules) => {
  const data = await jQuery.getJSON(
    `${exports.baseURL}pluginfw/plugin-definitions.json?v=${clientVars.randomVersionString}`);
  defs.plugins = data.plugins;
  defs.parts = data.parts;

  // Lade dynamisch die Plugin-Scripts
  const pluginPaths = getPluginPaths(data.parts);
  await Promise.all(pluginPaths.map(loadPluginScript));

  defs.hooks = pluginUtils.extractHooks(defs.parts, 'client_hooks', null, modules);
  defs.loaded = true;
};

const adoptPluginsFromAncestorsOf = (frame) => {
  // Bind plugins with parent;
  let parentRequire = null;
  try {
    while ((frame = frame.parent)) {
      if (typeof (frame.require) !== 'undefined') {
        parentRequire = frame.require;
        break;
      }
    }
  } catch (error) {
    // Silence (this can only be a XDomain issue).
    console.error(error);
  }

  if (!parentRequire) throw new Error('Parent plugins could not be found.');

  const ancestorPluginDefs = parentRequire('ep_etherpad-lite/static/js/pluginfw/plugin_defs');
  defs.hooks = ancestorPluginDefs.hooks;
  defs.loaded = ancestorPluginDefs.loaded;
  defs.parts = ancestorPluginDefs.parts;
  defs.plugins = ancestorPluginDefs.plugins;
  const ancestorPlugins = parentRequire('ep_etherpad-lite/static/js/pluginfw/client_plugins');
  exports.baseURL = ancestorPlugins.baseURL;
  exports.ensure = ancestorPlugins.ensure;
  exports.update = ancestorPlugins.update;
};

exports.adoptPluginsFromAncestorsOf = adoptPluginsFromAncestorsOf;

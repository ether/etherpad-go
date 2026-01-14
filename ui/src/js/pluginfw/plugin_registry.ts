// @ts-nocheck
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
  'ep_etherpad-lite/static/js/messageHandler': require('../messageHandler'),
  'ep_align/static/js/index': require('../../../../plugins/ep_align/static/js/index'),
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

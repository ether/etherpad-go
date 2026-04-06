// @ts-nocheck
/**
 * Plugin Registry - Registriert alle client_hooks Module zur Build-Zeit
 *
 * AUTOMATISCH GENERIERT - NICHT MANUELL BEARBEITEN
 * Generiert von: build.js
 */

import * as pluginUtils from './shared';
import * as pluginModule0 from '../messageHandler';
import * as pluginModule1 from '../../../../plugins/ep_align/static/js/index';
import * as pluginModule2 from '../../../../plugins/ep_chat_log_join_leave/static/js/index';
import * as pluginModule3 from '../../../../plugins/ep_heading/static/js/index';
import * as pluginModule4 from '../../../../plugins/ep_heading/static/js/shared';
import * as pluginModule5 from '../../../../plugins/ep_markdown/static/js/markdown';
import * as pluginModule6 from '../../../../plugins/ep_spellcheck/static/js/index';

// Mapping von Modul-Pfaden zu ihren Implementierungen
const builtinModules = {
  'ep_etherpad-lite/static/js/messageHandler': pluginModule0,
  'ep_align/static/js/index': pluginModule1,
  'ep_chat_log_join_leave/static/js/index': pluginModule2,
  'ep_heading/static/js/index': pluginModule3,
  'ep_heading/static/js/shared': pluginModule4,
  'ep_markdown/static/js/markdown': pluginModule5,
  'ep_spellcheck/static/js/index': pluginModule6,
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

export { registerBuiltinPlugins, getModuleMap, builtinModules };

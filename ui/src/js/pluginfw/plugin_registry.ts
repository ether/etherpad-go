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
import * as pluginModule2 from '../../../../plugins/ep_author_hover/static/js/index';
import * as pluginModule3 from '../../../../plugins/ep_chat_log_join_leave/static/js/index';
import * as pluginModule4 from '../../../../plugins/ep_clear_formatting/static/js/index';
import * as pluginModule5 from '../../../../plugins/ep_cursortrace/static/js/index';
import * as pluginModule6 from '../../../../plugins/ep_font_color/static/js/index';
import * as pluginModule7 from '../../../../plugins/ep_font_family/static/js/index';
import * as pluginModule8 from '../../../../plugins/ep_font_size/static/js/index';
import * as pluginModule9 from '../../../../plugins/ep_heading/static/js/index';
import * as pluginModule10 from '../../../../plugins/ep_heading/static/js/shared';
import * as pluginModule11 from '../../../../plugins/ep_markdown/static/js/markdown';
import * as pluginModule12 from '../../../../plugins/ep_print/static/js/index';
import * as pluginModule13 from '../../../../plugins/ep_spellcheck/static/js/index';
import * as pluginModule14 from '../../../../plugins/ep_table_of_contents/static/js/index';

// Mapping von Modul-Pfaden zu ihren Implementierungen
const builtinModules = {
  'ep_etherpad-lite/static/js/messageHandler': pluginModule0,
  'ep_align/static/js/index': pluginModule1,
  'ep_author_hover/static/js/index': pluginModule2,
  'ep_chat_log_join_leave/static/js/index': pluginModule3,
  'ep_clear_formatting/static/js/index': pluginModule4,
  'ep_cursortrace/static/js/index': pluginModule5,
  'ep_font_color/static/js/index': pluginModule6,
  'ep_font_family/static/js/index': pluginModule7,
  'ep_font_size/static/js/index': pluginModule8,
  'ep_heading/static/js/index': pluginModule9,
  'ep_heading/static/js/shared': pluginModule10,
  'ep_markdown/static/js/markdown': pluginModule11,
  'ep_print/static/js/index': pluginModule12,
  'ep_spellcheck/static/js/index': pluginModule13,
  'ep_table_of_contents/static/js/index': pluginModule14,
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

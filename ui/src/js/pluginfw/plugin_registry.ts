/**
 * Plugin Registry - registriert eingebaute client_hooks Module zur Build-Zeit.
 *
 * AUTOMATISCH GENERIERT - NICHT MANUELL BEARBEITEN
 * Generiert von: build.js
 */

import * as pluginUtils from './shared';

const builtinModules: Record<string, unknown> = {
  'ep_etherpad-lite/static/js/messageHandler': require('../messageHandler'),
  'ep_align/static/js/index': require('../../../../plugins/ep_align/static/js/index'),
  'ep_heading/static/js/index': require('../../../../plugins/ep_heading/static/js/index'),
  'ep_heading/static/js/shared': require('../../../../plugins/ep_heading/static/js/shared'),
  'ep_markdown/static/js/markdown': require('../../../../plugins/ep_markdown/static/js/markdown'),
  'ep_spellcheck/static/js/index': require('../../../../plugins/ep_spellcheck/static/js/index'),
};

export const registerBuiltinPlugins = (): void => {
  for (const [path, module] of Object.entries(builtinModules)) {
    pluginUtils.registerPluginModule(path, module);
  }
};

export const getModuleMap = (): Map<string, unknown> => {
  const map = new Map<string, unknown>();
  for (const [path, module] of Object.entries(builtinModules)) {
    map.set(path, module);
  }
  return map;
};

export {builtinModules};

registerBuiltinPlugins();

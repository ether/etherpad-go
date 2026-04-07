import './js/components';
import './js/core/ComponentBridge';
import './js/basic_error_handler';
import './js/l10n';
import './js/skin_variants';
import {browserFlags} from './js/browser_flags';
import * as chatModule from './js/chat';
import * as padModule from './js/pad';
import * as padEditbarModule from './js/pad_editbar';
import * as pluginClientModule from './js/pluginfw/client_plugins';
import { loadEnabledPlugins } from './js/pluginfw/plugin_registry';

const unwrapModule = (moduleValue) => {
  if (moduleValue && typeof moduleValue === 'object' && 'default' in moduleValue) {
    return moduleValue.default;
  }
  return moduleValue;
};

const waitForDocumentReady = async () => {
  if (document.readyState !== 'loading') return;
  await new Promise((resolve) => {
    document.addEventListener('DOMContentLoaded', resolve, {once: true});
  });
};

const bootstrap = async () => {
  window.clientVars = {
    randomVersionString: Date.now().toString(),
  };

  const basePath = new URL('..', window.location.href).pathname;

  window.browser = browserFlags;
  const pad = unwrapModule(padModule);
  if (typeof pad.setBaseURL === 'function') pad.setBaseURL(basePath);
  else pad.baseURL = basePath;

  window.plugins = unwrapModule(pluginClientModule);

  // These window globals are required because ace2_inner.ts and changesettracker.ts
  // reference them via window.* (ace2_inner runs in a module loaded by the parent
  // window, so window.pad/chat/padeditbar resolve to these assignments).
  window.pad = pad.pad;
  window.chat = unwrapModule(chatModule).chat;
  window.padeditbar = unwrapModule(padEditbarModule).padeditbar;

  window.plugins.setBaseURL(basePath);
  await window.plugins.update();

  // Load only enabled plugins (conditional dynamic imports)
  const enabledPluginNames = window.clientVars?.plugins?.plugins
    ? new Set(Object.keys(window.clientVars.plugins.plugins))
    : null; // null = load all (fallback if clientVars not yet available)
  await loadEnabledPlugins(enabledPluginNames);

  pad.init();
  await waitForDocumentReady();
};

void bootstrap();

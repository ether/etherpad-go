import './js/basic_error_handler';
import './js/l10n';
import './js/skin_variants';
import {browserFlags} from './js/browser_flags';
import * as chatModule from './js/chat';
import * as hooksModule from './js/pluginfw/hooks';
import * as padModule from './js/pad';
import * as padEditbarModule from './js/pad_editbar';
import * as padImpexpModule from './js/pad_impexp';
import * as pluginClientModule from './js/pluginfw/client_plugins';
import * as pluginDefsModule from './js/pluginfw/plugin_defs';
import * as pluginRegistryModule from './js/pluginfw/plugin_registry';

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

  const pluginRegistry = unwrapModule(pluginRegistryModule);
  window.plugins = unwrapModule(pluginClientModule);
  const hooks = unwrapModule(hooksModule);

  window.pad = pad.pad;
  window.chat = unwrapModule(chatModule).chat;
  window.padeditbar = unwrapModule(padEditbarModule).padeditbar;
  window.padimpexp = unwrapModule(padImpexpModule).padimpexp;

  window.plugins.setBaseURL(basePath);
  await window.plugins.update(pluginRegistry.getModuleMap());

  window._postPluginUpdateForTestingDone = false;
  if (window._postPluginUpdateForTesting != null) window._postPluginUpdateForTesting();
  window._postPluginUpdateForTestingDone = true;
  window.pluginDefs = unwrapModule(pluginDefsModule);

  pad.init();
  await waitForDocumentReady();
  await hooks.aCallAll('documentReady');
};

void bootstrap();

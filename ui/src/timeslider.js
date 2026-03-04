import './js/l10n';
import {browserFlags} from './js/browser_flags';
import * as padEditbarModule from './js/pad_editbar';
import './js/pad_impexp';
import * as pluginClientModule from './js/pluginfw/client_plugins';
import * as pluginRegistryModule from './js/pluginfw/plugin_registry';
import * as timeSliderModule from './js/timeslider';

const unwrapModule = (moduleValue) => {
  if (moduleValue && typeof moduleValue === 'object' && 'default' in moduleValue) {
    return moduleValue.default;
  }
  return moduleValue;
};

const bootstrap = async () => {
  window.clientVars = {
    randomVersionString: Date.now().toString(),
  };

  const pathComponents = location.pathname.split('/');
  const baseURL = `${pathComponents.slice(0, pathComponents.length - 3).join('/')}/`;

  window.browser = browserFlags;
  const timeSlider = unwrapModule(timeSliderModule);
  const pluginRegistry = unwrapModule(pluginRegistryModule);
  window.plugins = unwrapModule(pluginClientModule);

  window.plugins.setBaseURL(baseURL);
  await window.plugins.update(pluginRegistry.getModuleMap());

  const padeditbar = unwrapModule(padEditbarModule).padeditbar;
  timeSlider.setBaseURL(baseURL);
  await timeSlider.init();
  window.socket = timeSlider.socket;
  window.BroadcastSlider = timeSlider.BroadcastSlider;
  padeditbar.init();
};

void bootstrap();

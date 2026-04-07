import './js/components';
import './js/l10n';
import {browserFlags} from './js/browser_flags';
import * as padEditbarModule from './js/pad_editbar';
import './js/pad_impexp';
import * as pluginClientModule from './js/pluginfw/client_plugins';
import { loadEnabledPlugins } from './js/pluginfw/plugin_registry';
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
  window.plugins = unwrapModule(pluginClientModule);

  window.plugins.setBaseURL(baseURL);
  await window.plugins.update();
  await loadEnabledPlugins(null); // load all plugins in timeslider

  const padeditbar = unwrapModule(padEditbarModule).padeditbar;
  timeSlider.setBaseURL(baseURL);
  await timeSlider.init();
  padeditbar.init();
};

void bootstrap();

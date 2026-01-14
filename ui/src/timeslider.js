let BroadcastSlider;


window.clientVars = {
    // This is needed to fetch /pluginfw/plugin-definitions.json, which happens before the server
    // sends the CLIENT_VARS message.
    randomVersionString: Date.now().toString(),
};

(function () {
    const timeSlider = require('./js/timeslider')
    const pathComponents = location.pathname.split('/');

    // Strip 'p', the padname and 'timeslider' from the pathname and set as baseURL
    const baseURL = pathComponents.slice(0,pathComponents.length-3).join('/') + '/';
    require('./js/l10n')
    window.$ = window.jQuery = require('./js/rjquery').jQuery; // Expose jQuery #HACK
    require('./js/vendors/gritter')

    window.browser = require('./js/vendors/browser');

    const pluginRegistry = require('./js/pluginfw/plugin_registry');

    window.plugins = require('./js/pluginfw/client_plugins');
    const socket = timeSlider.socket;
    BroadcastSlider = timeSlider.BroadcastSlider;
    plugins.baseURL = baseURL;
    plugins.update(function () {


        /* TODO: These globals shouldn't exist. */

    });
    const padeditbar = require('./js/pad_editbar').padeditbar;
    const padimpexp = require('./js/pad_impexp').padimpexp;
    timeSlider.baseURL = baseURL;
    timeSlider.init();
    padeditbar.init()
})();
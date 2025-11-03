
(async () => {

    require('./js/l10n')

    window.clientVars = {
        // This is needed to fetch /pluginfw/plugin-definitions.json, which happens before the server
        // sends the CLIENT_VARS message.
        randomVersionString: Date.now().toString(),
};

    // Allow other frames to access this frame's modules.
    //window.require.resolveTmp = require.resolve('ep_etherpad-lite/static/js/pad_cookie');

    const basePath = new URL('..', window.location.href).pathname;
    window.$ = window.jQuery = require('./js/rjquery').jQuery;
    window.browser = require('./js/vendors/browser');
    const pad = require('./js/pad');
    pad.baseURL = basePath;
    window.plugins = require('./js/pluginfw/client_plugins');
    const hooks = require('./js/pluginfw/hooks');

    // TODO: These globals shouldn't exist.
    window.pad = pad.pad;
    window.chat = require('./js/chat').chat;
    window.padeditbar = require('./js/pad_editbar').padeditbar;
    window.padimpexp = require('./js/pad_impexp').padimpexp;
    require('./js/skin_variants');
    require('./js/basic_error_handler')

    window.plugins.baseURL = basePath;

    // Mechanism for tests to register hook functions (install fake plugins).
    window._postPluginUpdateForTestingDone = false;
    if (window._postPluginUpdateForTesting != null) window._postPluginUpdateForTesting();
    window._postPluginUpdateForTestingDone = true;
    window.pluginDefs = require('./js/pluginfw/plugin_defs');
    pad.init();
    await new Promise((resolve) => $(resolve));
    await hooks.aCallAll('documentReady');
})();

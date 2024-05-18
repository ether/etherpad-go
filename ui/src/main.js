// @license magnet:?xt=urn:btih:8e4f440f4c65981c5bf93c76d35135ba5064d8b7&dn=apache-2.0.txt
window.clientVars = {
    // This is needed to fetch /pluginfw/plugin-definitions.json, which happens before the
    // server sends the CLIENT_VARS message.
    randomVersionString: "test",
};
(function () {
    const pathComponents = location.pathname.split('/');

    // Strip 'p' and the padname from the pathname and set as baseURL
    const baseURL = pathComponents.slice(0, pathComponents.length - 2).join('/') + '/';

    window.$ = require('./js/rjquery').jQuery; // Expose jQuery #HACK
    window.jQuery = require('./js/rjquery').jQuery;
    window.browser = require('./js/vendors/browser');

    var plugins = require('./js/pluginfw/client_plugins');
    var hooks = require('./js/pluginfw/hooks');

    plugins.baseURL = baseURL;
    plugins.update(function () {
        // Mechanism for tests to register hook functions (install fake plugins).
        window._postPluginUpdateForTestingDone = false;
        if (window._postPluginUpdateForTesting != null) window._postPluginUpdateForTesting();
        window._postPluginUpdateForTestingDone = true;
        // Call documentReady hook
        $(function() {
            hooks.aCallAll('documentReady');
        });

        var pad = require('./js/pad');
        pad.baseURL = baseURL;
        pad.init();
    });

    /* TODO: These globals shouldn't exist. */
    window.pad = require('./js/pad').pad;
    window.chat = require('./js/chat').chat;
    window.padeditbar = require('./js/pad_editbar').padeditbar;
    window.padimpexp = require('./js/pad_impexp').padimpexp;
    window.io = require('socket.io-client');
    require('./js/skin_variants');
}());
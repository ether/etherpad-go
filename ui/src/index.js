(async () => {
    window.$ = window.jQuery = require('./js/rjquery').jQuery;
    require('./js/l10n')
    require('./js/index')
})()
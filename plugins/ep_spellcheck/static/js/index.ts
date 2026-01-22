import type { PostAceInitHook } from '../../../typings/etherpad';
declare var require: any;
const padcookie = require('ep_etherpad-lite/static/js/pad_cookie');



export const postAceInit: PostAceInitHook = (_hookName, context) => {


    const $ = (globalThis as any).jQuery || (globalThis as any).$;

    const $outer = $('iframe[name="ace_outer"]').contents().find('iframe');
    const $inner = $outer.contents().find('#innerdocbody');

    const enableSpellcheck = ()=>{
        $inner.attr('spellcheck', 'true');
        $inner.find('div').each(function () {
            $(this).attr('spellcheck', 'true');
            $(this).find('div').each(function () {
                $(this).attr('spellcheck', 'true');
            });
        });
    }

    const disableSpellcheck = ()=>{
        $inner.attr('spellcheck', 'false');
        $inner.find('div').each(function () {
            $(this).attr('spellcheck', 'false');
            $(this).find('span').each(function () {
                $(this).attr('spellcheck', 'false');
            });
        });
    }

    const padcookieObj = padcookie.padcookie || padcookie;

    if (typeof padcookieObj.getPref !== 'function') {
        console.error('padcookie.getPref is not a function. padcookieObj:', padcookieObj);
    }

    if (padcookieObj.getPref('spellcheck') === false) {
        $('#options-spellcheck').val();
        $('#options-spellcheck').attr('checked', 'unchecked');
        $('#options-spellcheck').attr('checked', false);
    } else {
        $('#options-spellcheck').attr('checked', 'checked');
    }

    if ($('#options-spellcheck').is(':checked')) {
        enableSpellcheck();
    } else {
        disableSpellcheck();
    }

    $('#options-spellcheck').on('click', () => {
        if ($('#options-spellcheck').is(':checked')) {
            padcookieObj.setPref('spellcheck', true);
            enableSpellcheck();
        } else {
            padcookieObj.setPref('spellcheck', false);
            disableSpellcheck();
        }
        window.location.reload()
    });
};

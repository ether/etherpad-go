// @ts-nocheck
/**
 * This code is mostly from the old Etherpad. Please help us to comment this code.
 * This helps other people to understand this code better and helps them to improve it.
 * TL;DR COMMENTS ON THIS FILE ARE HIGHLY APPRECIATED
 */

/**
 * Copyright 2009 Google Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS-IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import padutils,{Cookies} from "./pad_utils";
import {padcookie} from './pad_cookie';
import {Ace2Editor} from './ace';
import {editorBus} from './core/EventBus';
import html10n from './i18n'
import * as skinVariants from './skin_variants';

const q = (selector) => document.querySelector(selector);
const qa = (selector) => Array.from(document.querySelectorAll(selector));

export const padeditor = (() => {
  let pad = undefined;
  let settings = undefined;

  const self = {
    ace: null,
    // this is accessed directly from other files
    viewZoom: 100,
    init: async (initialViewOptions, _pad) => {
      pad = _pad;
      settings = pad.settings;
      self.ace = new Ace2Editor();
      await self.ace.init('editorcontainer', '');
      // EventBus: emit editor:ace:initialized after the ACE editor is created
      editorBus.emit('editor:ace:initialized', {editorInfo: self.ace});
      const editorLoading = q('#editorloadingbox');
      if (editorLoading) editorLoading.style.display = 'none';
      // Listen for clicks on sidediv items
      const outerFrame = q('iframe[name="ace_outer"]');
      const outerDoc = outerFrame?.contentDocument;
      const sideDivInner = outerDoc?.querySelector('#sidedivinner');
      sideDivInner?.addEventListener('click', (event) => {
        const target = event.target;
        if (!(target instanceof HTMLElement) || target.tagName.toLowerCase() !== 'div') return;
        const siblings = Array.from(target.parentElement?.children ?? []);
        const targetLineNumber = siblings.indexOf(target) + 1;
        window.location.hash = `L${targetLineNumber}`;
      });
      focusOnLine(self.ace);
      self.ace.setProperty('wraps', true);
      self.initViewOptions();
      self.setViewOptions(initialViewOptions);
      // view bar
      const viewbar = q('#viewbarcontents');
      if (viewbar) viewbar.style.display = '';
    },
    initViewOptions: () => {
      // Line numbers
      padutils.bindCheckboxChange('#options-linenoscheck', () => {
        pad.changeViewOption('showLineNumbers', padutils.getCheckbox('#options-linenoscheck'));
      });

      // Author colors
      padutils.bindCheckboxChange('#options-colorscheck', () => {
        padcookie.setPref('showAuthorshipColors', padutils.getCheckbox('#options-colorscheck'));
        pad.changeViewOption('showAuthorColors', padutils.getCheckbox('#options-colorscheck'));
      });

      // Right to left
      padutils.bindCheckboxChange('#options-rtlcheck', () => {
        pad.changeViewOption('rtlIsTrue', padutils.getCheckbox('#options-rtlcheck'));
      });
      html10n.bind('localized', () => {
        pad.changeViewOption('rtlIsTrue', ('rtl' === html10n.getDirection()));
        padutils.setCheckbox('#options-rtlcheck', ('rtl' === html10n.getDirection()));
      });



      // font family change
      q('#viewfontmenu')?.addEventListener('change', () => {
        const menu = q('#viewfontmenu');
        pad.changeViewOption('padFontFamily', menu?.value);
      });

      // delete pad
      q('#delete-pad')?.addEventListener('click', () => {
        if (window.confirm(html10n.get('pad.delete.confirm'))) {
          pad.collabClient.sendMessage({type: 'PAD_DELETE', data:{padId: pad.getPadId()}});
          // redirect to home page after deletion
          window.location.href = '/';
        }
      });

      // theme switch
      q('#theme-switcher')?.addEventListener('click', () => {
          if (skinVariants.isDarkMode()) {
            skinVariants.setDarkModeInLocalStorage(false);
            skinVariants.updateSkinVariantsClasses(['super-light-toolbar super-light-editor light-background']);
          } else {
            skinVariants.setDarkModeInLocalStorage(true);
            skinVariants.updateSkinVariantsClasses(['super-dark-editor', 'dark-background', 'super-dark-toolbar']);
          }
      });

      // Language
      html10n.bind('localized', () => {
        const menu = q('#languagemenu');
        if (menu) menu.value = html10n.getLanguage();
        // translate the value of 'unnamed' and 'Enter your name' textboxes in the userlist

        // this does not interfere with html10n's normal value-setting because
        // html10n just ingores <input>s
        // also, a value which has been set by the user will be not overwritten
        // since a user-edited <input> does *not* have the editempty-class
        qa('input[data-l10n-id]').forEach((input) => {
          if (!(input instanceof HTMLInputElement)) return;
          if (input.classList.contains('editempty')) {
            const id = input.getAttribute('data-l10n-id');
            if (id) input.value = html10n.get(id);
          }
        });
      });
      const languageMenu = q('#languagemenu');
      if (languageMenu) languageMenu.value = html10n.getLanguage();
      languageMenu?.addEventListener('change', () => {
        const value = languageMenu.value;
        Cookies.set('language', value, { expires: 36500 });
        location.reload();
        html10n.localize([value, 'en']);
      });
    },
    setViewOptions: (newOptions) => {
      const getOption = (key, defaultValue) => {
        const value = String(newOptions[key]);
        if (value === 'true') return true;
        if (value === 'false') return false;
        return defaultValue;
      };

      let v;

      v = getOption('rtlIsTrue', ('rtl' === html10n.getDirection()));
      self.ace.setProperty('rtlIsTrue', v);
      padutils.setCheckbox('#options-rtlcheck', v);

      v = getOption('showLineNumbers', true);
      self.ace.setProperty('showslinenumbers', v);
      padutils.setCheckbox('#options-linenoscheck', v);

      v = getOption('showAuthorColors', true);
      self.ace.setProperty('showsauthorcolors', v);
      q('#chattext')?.classList.toggle('authorColors', v);
      const sideDivInner = q('iframe[name="ace_outer"]')?.contentDocument?.querySelector('#sidedivinner');
      sideDivInner?.classList.toggle('authorColors', v);
      padutils.setCheckbox('#options-colorscheck', v);

      // Override from parameters if true
      if (settings.noColors !== false) {
        self.ace.setProperty('showsauthorcolors', !settings.noColors);
      }

      self.ace.setProperty('textface', newOptions.padFontFamily || '');
    },
    dispose: () => {
      if (self.ace) {
        self.ace.destroy();
        self.ace = null;
      }
    },
    enable: () => {
      if (self.ace) {
        self.ace.setEditable(true);
      }
    },
    disable: () => {
      if (self.ace) {
        self.ace.setEditable(false);
      }
    },
    restoreRevisionText: (dataFromServer) => {
      pad.addHistoricalAuthors(dataFromServer.historicalAuthorData);
      self.ace.importAText(dataFromServer.atext, dataFromServer.apool, true);
    },
  };
  return self;
})();

export const focusOnLine = (ace) => {
  // If a number is in the URI IE #L124 go to that line number
  const lineNumber = window.location.hash.substr(1);
  if (lineNumber) {
    if (lineNumber[0] === 'L') {
      const lineNumberInt = parseInt(lineNumber.substr(1));
      if (lineNumberInt) {
        const outerFrame = q('iframe[name="ace_outer"]');
        const outerDoc = outerFrame?.contentDocument;
        const outerDocBody = outerDoc?.querySelector('#outerdocbody');
        const innerFrame = outerDoc?.querySelector('iframe');
        const innerDocBody = innerFrame?.contentDocument?.querySelector('#innerdocbody');
        const line = innerDocBody?.querySelector(`div:nth-child(${lineNumberInt})`);
        if (line && outerDocBody && innerDocBody) {
          let offsetTop = line.getBoundingClientRect().top - innerDocBody.getBoundingClientRect().top;
          offsetTop += parseInt(getComputedStyle(outerDocBody).paddingTop.replace('px', ''));
          const hasMobileLayout = window.matchMedia('(max-width: 1000px)').matches;
          if (!hasMobileLayout) {
            offsetTop += parseInt(getComputedStyle(innerDocBody).paddingTop.replace('px', ''));
          }
          (outerDocBody).style.top = `${offsetTop}px`; // Chrome
          outerDoc?.documentElement?.scrollTo({top: offsetTop}); // needed for FF
          const node = line;
          ace.callWithAce((ace) => {
            const selection = {
              startPoint: {
                index: 0,
                focusAtStart: true,
                maxIndex: 1,
                node,
              },
              endPoint: {
                index: 0,
                focusAtStart: true,
                maxIndex: 1,
                node,
              },
            };
            ace.ace_setSelection(selection);
          });
        }
      }
    }
  }
  // End of setSelection / set Y position of editor
};

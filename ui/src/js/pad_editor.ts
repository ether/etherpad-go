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
      // editor:ace:initialized is emitted by ace.ts with the shared info object
      const editorLoading = q('#editorloadingbox');
      if (editorLoading) editorLoading.style.display = 'none';
      // Listen for clicks on sidediv items (now in main document, not iframe)
      const sideDivInner = q('#sidedivinner');
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
      q('#viewfontmenu')?.addEventListener('ep-dropdown-select', ((e: CustomEvent) => {
        const font = e.detail?.value ?? '';
        // Update the trigger button text
        const trigger = q('#viewfontmenu [slot="trigger"]');
        if (trigger) trigger.textContent = font || html10n.get('pad.settings.fontType.normal');
        pad.changeViewOption('padFontFamily', font);
      }) as EventListener);

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
        // Update the language trigger button text
        const lang = html10n.getLanguage();
        const langItem = q(`#languagemenu ep-dropdown-item[value="${lang}"]`);
        const trigger = q('#languagemenu [slot="trigger"]');
        if (trigger && langItem) trigger.textContent = langItem.textContent;

        // translate the value of 'unnamed' and 'Enter your name' textboxes in the userlist
        qa('input[data-l10n-id]').forEach((input) => {
          if (!(input instanceof HTMLInputElement)) return;
          if (input.classList.contains('editempty')) {
            const id = input.getAttribute('data-l10n-id');
            if (id) input.value = html10n.get(id);
          }
        });
      });
      // Set initial language trigger text
      const langTrigger = q('#languagemenu [slot="trigger"]');
      const currentLang = html10n.getLanguage();
      const currentLangItem = q(`#languagemenu ep-dropdown-item[value="${currentLang}"]`);
      if (langTrigger && currentLangItem) langTrigger.textContent = currentLangItem.textContent;

      q('#languagemenu')?.addEventListener('ep-dropdown-select', ((e: CustomEvent) => {
        const value = e.detail?.value ?? '';
        Cookies.set('language', value, { expires: 36500 });
        location.reload();
        html10n.localize([value, 'en']);
      }) as EventListener);
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
      const sideDivInner = q('#sidedivinner');
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
        const innerDocBody = document.getElementById('innerdocbody');
        const line = innerDocBody?.querySelector(`div:nth-child(${lineNumberInt})`);
        if (line && innerDocBody) {
          const offsetTop = line.getBoundingClientRect().top - innerDocBody.getBoundingClientRect().top;
          const editorContainer = document.getElementById('editorcontainer');
          if (editorContainer) {
            editorContainer.scrollTop = offsetTop;
          }
          ace.callWithAce((ace) => {
            const node = line;
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

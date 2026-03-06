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

import {browserFlags as browser} from './browser_flags';
import * as hooks from './pluginfw/hooks';
import padutils from "./pad_utils";
import {padeditor} from './pad_editor';
import * as padsavedrevs from './pad_savedrevs';

const q = (selector) => document.querySelector(selector);
const qa = (selector) => Array.from(document.querySelectorAll(selector));
const debounce = (fn, wait) => {
  let timer = null;
  return (...args) => {
    if (timer != null) window.clearTimeout(timer);
    timer = window.setTimeout(() => fn(...args), wait);
  };
};
const hide = (selector) => {
  const el = q(selector);
  if (el) el.style.display = 'none';
};
const show = (selector, display = 'block') => {
  const el = q(selector);
  if (el) el.style.display = display;
};

class ToolbarItem {
  constructor(element) {
    this.el = element;
  }

  getCommand() {
    return this.el.getAttribute('data-key');
  }

  getValue() {
    if (this.isSelect()) {
      return this.el.querySelector('select')?.value;
    }
  }

  setValue(val) {
    if (this.isSelect()) {
      const select = this.el.querySelector('select');
      if (select) select.value = val;
    }
  }

  getType() {
    return this.el.getAttribute('data-type');
  }

  isSelect() {
    return this.getType() === 'select';
  }

  isButton() {
    return this.getType() === 'button';
  }

  bind(callback) {
    if (this.isButton()) {
      this.el.addEventListener('click', (event) => {
        if (document.activeElement instanceof HTMLElement) document.activeElement.blur();
        callback(this.getCommand(), this);
        event.preventDefault();
      });
    } else if (this.isSelect()) {
      this.el.querySelector('select')?.addEventListener('change', () => {
        callback(this.getCommand(), this);
      });
    }
  }
}

const syncAnimation = (() => {
  const SYNCING = -100;
  const DONE = 100;
  let state = DONE;
  const fps = 25;
  const step = 1 / fps;
  const T_START = -0.5;
  const T_FADE = 1.0;
  const T_GONE = 1.5;
  const animator = padutils.makeAnimationScheduler(() => {
    if (state === SYNCING || state === DONE) {
      return false;
    } else if (state >= T_GONE) {
      state = DONE;
      hide('#syncstatussyncing');
      hide('#syncstatusdone');
      return false;
    } else if (state < 0) {
      state += step;
      if (state >= 0) {
        hide('#syncstatussyncing');
        show('#syncstatusdone');
        const done = q('#syncstatusdone');
        if (done) done.style.opacity = '1';
      }
      return true;
    } else {
      state += step;
      if (state >= T_FADE) {
        const done = q('#syncstatusdone');
        if (done) done.style.opacity = `${(T_GONE - state) / (T_GONE - T_FADE)}`;
      }
      return true;
    }
  }, step * 1000);
  return {
    syncing: () => {
      state = SYNCING;
      show('#syncstatussyncing');
      hide('#syncstatusdone');
    },
    done: () => {
      state = T_START;
      animator.scheduleAnimation();
    },
  };
})();

export const padeditbar = new class {
  constructor() {
    this._editbarPosition = 0;
    this.commands = {};
    this.dropdowns = [];
    this._boundToolbarScrollSync = null;
  }

  init() {
    for (const button of qa('#editbar .editbarbutton')) button.setAttribute('unselectable', 'on');
    this.enable();
    for (const elt of qa('#editbar [data-key]')) {
      new ToolbarItem(elt).bind((command, item) => {
        this.triggerCommand(command, item);
      });
    }

    document.body.addEventListener('keydown', (evt) => {
      this._bodyKeyEvent(evt);
    });

    this.checkAllIconsAreDisplayedInToolbar();
    window.addEventListener('resize', debounce(() => this.checkAllIconsAreDisplayedInToolbar(), 100));

    this._registerDefaultCommands();

    hooks.callAll('postToolbarInit', {
      toolbar: this,
      ace: padeditor.ace,
    });

    /*
     * On safari, the dropdown in the toolbar gets hidden because of toolbar
     * overflow:hidden property. This is a bug from Safari: any children with
     * position:fixed (like the dropdown) should be displayed no matter
     * overflow:hidden on parent
     */
    // When editor is scrolled, we add a class to style the editbar differently
    const frame = q('iframe[name="ace_outer"]');
    const innerDoc = frame?.contentDocument;
    if (innerDoc) {
      innerDoc.addEventListener('scroll', (ev) => {
        const target = ev.target?.documentElement ?? innerDoc.documentElement;
        q('#editbar')?.classList.toggle('editor-scrolled', (target?.scrollTop ?? 0) > 2);
      });
    }
  }
  isEnabled() { return true; }
  disable() {
    const editbar = q('#editbar');
    if (!editbar) return;
    editbar.classList.add('disabledtoolbar');
    editbar.classList.remove('enabledtoolbar');
  }
  enable() {
    const editbar = q('#editbar');
    if (!editbar) return;
    editbar.classList.add('enabledtoolbar');
    editbar.classList.remove('disabledtoolbar');
  }
  registerCommand(cmd, callback) {
    this.commands[cmd] = callback;
    return this;
  }
  registerDropdownCommand(cmd, dropdown) {
    dropdown = dropdown || cmd;
    this.dropdowns.push(dropdown);
    this.registerCommand(cmd, () => {
      this.toggleDropDown(dropdown);
    });
  }
  registerAceCommand(cmd, callback) {
    this.registerCommand(cmd, (cmd, ace, item) => {
      ace.callWithAce((ace) => {
        callback(cmd, ace, item);
      }, cmd, true);
    });
  }
  triggerCommand(cmd, item) {
    if (this.isEnabled() && this.commands[cmd]) {
      this.commands[cmd](cmd, padeditor.ace, item);
    }
    if (padeditor.ace) padeditor.ace.focus();
  }

  // cb is deprecated (this function is synchronous so a callback is unnecessary).
  toggleDropDown(moduleName, cb = null) {
    let cbErr = null;
    try {
      // do nothing if users are sticked
      if (moduleName === 'users' && q('#users')?.classList.contains('stickyUsers')) {
        return;
      }

      for (const el of qa('.toolbar-popup')) el.classList.remove('popup-show');

      // hide all modules and remove highlighting of all buttons
      if (moduleName === 'none') {
        for (const thisModuleName of this.dropdowns) {
          // skip the userlist
          if (thisModuleName === 'users') continue;

          const module = q(`#${thisModuleName}`);

          // skip any "force reconnect" message
          const reconnectButton = module?.querySelector('button#forcereconnect');
          const isAForceReconnectMessage = reconnectButton != null &&
            getComputedStyle(reconnectButton).display !== 'none';
          if (isAForceReconnectMessage) continue;
          if (module?.classList.contains('popup-show')) {
            q(`li[data-key=${thisModuleName}] > a`)?.classList.remove('selected');
            module.classList.remove('popup-show');
          }
        }
      } else {
        // hide all modules that are not selected and remove highlighting
        // respectively add highlighting to the corresponding button
        for (const thisModuleName of this.dropdowns) {
          const module = q(`#${thisModuleName}`);

          if (module?.classList.contains('popup-show')) {
            q(`li[data-key=${thisModuleName}] > a`)?.classList.remove('selected');
            module.classList.remove('popup-show');
          } else if (thisModuleName === moduleName) {
            q(`li[data-key=${thisModuleName}] > a`)?.classList.add('selected');
            module?.classList.add('popup-show');
          }
        }
      }
    } catch (err) {
      cbErr = err || new Error(err);
    } finally {
      if (cb) Promise.resolve().then(() => cb(cbErr));
    }
  }
  setSyncStatus(status) {
    if (status === 'syncing') {
      syncAnimation.syncing();
    } else if (status === 'done') {
      syncAnimation.done();
    }
  }
  setEmbedLinks() {
    const readonlyInput = q('#readonlyinput');
    const embedInput = q('#embedinput');
    const linkInput = q('#linkinput');
    if (!embedInput || !linkInput) return;
    const {link, embed} = this.getShareLinks(Boolean(readonlyInput?.checked));
    embedInput.value = embed;
    linkInput.value = link;
  }

  getShareLinks(isReadonly = false) {
    const padUrl = window.location.href.split('?')[0];
    const params = '?showControls=true&showChat=true&showLineNumbers=true&useMonospaceFont=false';
    const props = 'width="100%" height="600" frameborder="0"';
    if (isReadonly) {
      const urlParts = padUrl.split('/');
      urlParts.pop();
      const readonlyLink = `${urlParts.join('/')}/${clientVars.readOnlyId}`;
      return {
        link: readonlyLink,
        embed: `<iframe name="embed_readonly" src="${readonlyLink}${params}" ${props}></iframe>`,
      };
    }
    return {
      link: padUrl,
      embed: `<iframe name="embed_readwrite" src="${padUrl}${params}" ${props}></iframe>`,
    };
  }

  getQrCodeSrc(isReadonly = false) {
    const qrUrl = new URL(`${window.location.pathname.replace(/\/$/, '')}/qr`, window.location.origin);
    qrUrl.searchParams.set('readonly', isReadonly ? 'true' : 'false');
    return qrUrl.toString();
  }

  async setQrCode() {
    const readonlyInput = q('#qrreadonlyinput');
    const qrImage = q('#qrcodeimg');
    const qrLinkInput = q('#qrcodelinkinput');
    if (!(qrImage instanceof HTMLImageElement) || !(qrLinkInput instanceof HTMLInputElement)) return;
    const {link} = this.getShareLinks(Boolean(readonlyInput instanceof HTMLInputElement && readonlyInput.checked));
    qrLinkInput.value = link;
    qrImage.src = this.getQrCodeSrc(Boolean(readonlyInput instanceof HTMLInputElement && readonlyInput.checked));
  }

  _syncToolbarScrollState() {
    const toolbar = q('.toolbar');
    const menuLeft = q('.toolbar .menu_left');
    if (!(toolbar instanceof HTMLElement) || !(menuLeft instanceof HTMLElement)) return;
    const maxScrollLeft = Math.max(0, menuLeft.scrollWidth - menuLeft.clientWidth);
    const scrollLeft = Math.round(menuLeft.scrollLeft);
    toolbar.classList.toggle('toolbar-can-scroll-left', scrollLeft > 0);
    toolbar.classList.toggle('toolbar-can-scroll-right', scrollLeft < maxScrollLeft - 1);
  }

  _ensureToolbarScrollBehavior(enabled) {
    const menuLeft = q('.toolbar .menu_left');
    if (!(menuLeft instanceof HTMLElement)) return;
    if (!this._boundToolbarScrollSync) {
      this._boundToolbarScrollSync = () => this._syncToolbarScrollState();
      menuLeft.addEventListener('scroll', this._boundToolbarScrollSync, {passive: true});
      menuLeft.addEventListener('wheel', (event) => {
        if (!q('.toolbar')?.classList.contains('toolbar-scrollable')) return;
        if (Math.abs(event.deltaX) <= Math.abs(event.deltaY) && event.deltaY !== 0) {
          menuLeft.scrollLeft += event.deltaY;
          event.preventDefault();
        }
      }, {passive: false});
    }

    if (enabled) {
      this._syncToolbarScrollState();
    } else {
      menuLeft.scrollLeft = 0;
    }
  }
  checkAllIconsAreDisplayedInToolbar() {
    const toolbar = q('.toolbar');
    const menuLeft = q('.toolbar .menu_left');
    const menuRight = q('.toolbar .menu_right');
    if (!(toolbar instanceof HTMLElement) || !(menuLeft instanceof HTMLElement)) return;

    const toolbarWidth = toolbar.clientWidth;
    const menuRightWidth = menuRight instanceof HTMLElement ? menuRight.offsetWidth : 0;
    const isCompactLayout = window.matchMedia('(max-width: 1000px)').matches;
    const availableLeftWidth = isCompactLayout
      ? toolbarWidth
      : Math.max(0, toolbarWidth - menuRightWidth - 10);
    const canUseScrollableToolbar = menuLeft.scrollWidth > availableLeftWidth;
    toolbar.style.setProperty('--toolbar-right-width', `${menuRightWidth}px`);
    toolbar.classList.toggle('toolbar-scrollable', canUseScrollableToolbar);
    toolbar.classList.remove('toolbar-can-scroll-left', 'toolbar-can-scroll-right');
    this._ensureToolbarScrollBehavior(canUseScrollableToolbar);
  }

  _bodyKeyEvent(evt) {
    // If the event is Alt F9 or Escape & we're already in the editbar menu
    // Send the users focus back to the pad
    if ((evt.keyCode === 120 && evt.altKey) || evt.keyCode === 27) {
      const active = document.activeElement;
      if (active && active.closest('.toolbar')) {
        // If we're in the editbar already..
        // Close any dropdowns we have open..
        this.toggleDropDown('none');
        // Shift focus away from any drop downs
        if (active instanceof HTMLElement) active.blur();
        // Check we're on a pad and not on the timeslider
        // Or some other window I haven't thought about!
        if (typeof pad === 'undefined') {
          // Timeslider probably..
          q('#editorcontainerbox')?.focus(); // Focus back onto the pad
        } else {
          padeditor.ace.focus(); // Sends focus back to pad
          // The above focus doesn't always work in FF, you have to hit enter afterwards
          evt.preventDefault();
        }
      } else {
        // Focus on the editbar :)
        const firstEditbarElement = q('#editbar button');
        if (evt.currentTarget instanceof HTMLElement) evt.currentTarget.blur();
        firstEditbarElement?.focus();
        evt.preventDefault();
      }
    }
    // Are we in the toolbar??
    if (document.activeElement && document.activeElement.closest('.toolbar')) {
      // On arrow keys go to next/previous button item in editbar
      if (evt.keyCode !== 39 && evt.keyCode !== 37) return;

      // Get all the focusable items in the editbar
      const focusItems = qa('#editbar button, #editbar select');

      // On left arrow move to next button in editbar
      if (evt.keyCode === 37) {
        // If a dropdown is visible or we're in an input don't move to the next button
        const hasVisiblePopup = qa('.popup').some((el) => getComputedStyle(el).display !== 'none');
        if (hasVisiblePopup || evt.target.localName === 'input') return;

        this._editbarPosition--;
        // Allow focus to shift back to end of row and start of row
        if (this._editbarPosition === -1) this._editbarPosition = focusItems.length - 1;
        focusItems[this._editbarPosition]?.focus();
        focusItems[this._editbarPosition]?.scrollIntoView?.({block: 'nearest', inline: 'nearest'});
      }

      // On right arrow move to next button in editbar
      if (evt.keyCode === 39) {
        // If a dropdown is visible or we're in an input don't move to the next button
        const hasVisiblePopup = qa('.popup').some((el) => getComputedStyle(el).display !== 'none');
        if (hasVisiblePopup || evt.target.localName === 'input') return;

        this._editbarPosition++;
        // Allow focus to shift back to end of row and start of row
        if (this._editbarPosition >= focusItems.length) this._editbarPosition = 0;
        focusItems[this._editbarPosition]?.focus();
        focusItems[this._editbarPosition]?.scrollIntoView?.({block: 'nearest', inline: 'nearest'});
      }
    }
  }

  _registerDefaultCommands() {
    this.registerDropdownCommand('showusers', 'users');
    this.registerDropdownCommand('settings');
    this.registerDropdownCommand('connectivity');
    this.registerDropdownCommand('import_export');
    this.registerDropdownCommand('embed');
    this.registerDropdownCommand('share_qr');
    this.registerCommand('home', ()=>{
      window.location.href = window.location.href + "/../.."
    })

    this.registerCommand('settings', () => {
      this.toggleDropDown('settings');
      q('#options-stickychat')?.focus();
    });

    this.registerCommand('import_export', () => {
      this.toggleDropDown('import_export');
      // If Import file input exists then focus on it..
      if (q('#importfileinput') != null) {
        setTimeout(() => {
          q('#importfileinput')?.focus();
        }, 100);
      } else {
        q('.exportlink')?.focus();
      }
    });

    this.registerCommand('showusers', () => {
      this.toggleDropDown('users');
      q('#myusernameedit')?.focus();
    });
    this.registerCommand('home', ()=>{
      globalThis.location.href = globalThis.location.href + "/../.."
    })

    this.registerCommand('embed', () => {
      this.setEmbedLinks();
      this.toggleDropDown('embed');
      const linkInput = q('#linkinput');
      linkInput?.focus();
      linkInput?.select?.();
    });

    this.registerCommand('share_qr', () => {
      this.toggleDropDown('share_qr');
      void this.setQrCode();
      const qrLinkInput = q('#qrcodelinkinput');
      qrLinkInput?.focus();
      qrLinkInput?.select?.();
    });

    this.registerCommand('savedRevision', () => {
      padsavedrevs.saveNow();
    });

    this.registerCommand('showTimeSlider', () => {
      document.location = `${document.location.pathname}/timeslider`;
    });

    const aceAttributeCommand = (cmd, ace) => {
      ace.ace_toggleAttributeOnSelection(cmd);
    };
    this.registerAceCommand('bold', aceAttributeCommand);
    this.registerAceCommand('italic', aceAttributeCommand);
    this.registerAceCommand('underline', aceAttributeCommand);
    this.registerAceCommand('strikethrough', aceAttributeCommand);

    this.registerAceCommand('undo', (cmd, ace) => {
      ace.ace_doUndoRedo(cmd);
    });

    this.registerAceCommand('redo', (cmd, ace) => {
      ace.ace_doUndoRedo(cmd);
    });

    this.registerAceCommand('insertunorderedlist', (cmd, ace) => {
      ace.ace_doInsertUnorderedList();
    });

    this.registerAceCommand('insertorderedlist', (cmd, ace) => {
      ace.ace_doInsertOrderedList();
    });

    this.registerAceCommand('indent', (cmd, ace) => {
      if (!ace.ace_doIndentOutdent(false)) {
        ace.ace_doInsertUnorderedList();
      }
    });

    this.registerAceCommand('outdent', (cmd, ace) => {
      ace.ace_doIndentOutdent(true);
    });

    this.registerAceCommand('clearauthorship', (cmd, ace) => {
      // If we have the whole document selected IE control A has been hit
      const rep = ace.ace_getRep();
      let doPrompt = false;
      const lastChar = rep.lines.atIndex(rep.lines.length() - 1).width - 1;
      const lastLineIndex = rep.lines.length() - 1;
      if (rep.selStart[0] === 0 && rep.selStart[1] === 0) {
        // nesting intentionally here to make things readable
        if (rep.selEnd[0] === lastLineIndex && rep.selEnd[1] === lastChar) {
          doPrompt = true;
        }
      }
      /*
       * NOTICE: This command isn't fired on Control Shift C.
       * I intentionally didn't create duplicate code because if you are hitting
       * Control Shift C we make the assumption you are a "power user"
       * and as such we assume you don't need the prompt to bug you each time!
       * This does make wonder if it's worth having a checkbox to avoid being
       * prompted again but that's probably overkill for this contribution.
       */

      // if we don't have any text selected, we have a caret or we have already said to prompt
      if ((!(rep.selStart && rep.selEnd)) || ace.ace_isCaret() || doPrompt) {
        if (window.confirm(html10n.get('pad.editbar.clearcolors'))) {
          ace.ace_performDocumentApplyAttributesToCharRange(0, ace.ace_getRep().alltext.length, [
            ['author', ''],
          ]);
        }
      } else {
        ace.ace_setAttributeOnSelection('author', '');
      }
    });

    this.registerCommand('timeslider_returnToPad', (cmd) => {
      if (document.referrer.length > 0 &&
          document.referrer.substring(document.referrer.lastIndexOf('/') - 1,
              document.referrer.lastIndexOf('/')) === 'p') {
        document.location = document.referrer;
      } else {
        document.location = document.location.href
            .substring(0, document.location.href.lastIndexOf('/'));
      }
    });
  }
}();

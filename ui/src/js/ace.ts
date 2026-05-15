/**
 * Ace2Editor — Wrapper around the WebComponent-based AceEditor from etherpad-webcomponents.
 *
 * Replaces the old iframe-based editor (ace2_inner) with a direct contenteditable div.
 * Maintains the same public API so collab_client, pad_editor, and plugins work unchanged.
 *
 * The key pattern: a shared `info` object holds `ace_*` prefixed methods that plugins
 * and callWithAce callbacks use. This mirrors the original ace2_inner architecture.
 *
 * Copyright 2009 Google Inc.
 * Copyright 2025 - Adapted for WebComponent-based editor.
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

import {AceEditor} from 'etherpad-webcomponents';
import {editorBus} from './core/EventBus';
import * as pluginUtils from './pluginfw/shared';

export const Ace2Editor = function (this: any) {
  let editor: AceEditor | null = null;
  let loaded = false;

  // Shared info object — plugins and callWithAce callbacks access ace_* methods on this.
  // This replicates the original editorInfo pattern from ace2_inner.
  const info: Record<string, any> = {editor: this};

  let actionsPendingInit: Array<() => void> = [];

  const pendingInit = (func: (...args: any[]) => any) => function (this: any, ...args: any[]) {
    const action = () => func.apply(this, args);
    if (loaded) return action();
    actionsPendingInit.push(action);
  };

  const doActionsPendingInit = () => {
    for (const fn of actionsPendingInit) fn();
    actionsPendingInit = [];
  };

  /**
   * Populates the info object with ace_* methods that delegate to the AceEditor.
   * Called once after editor.init() completes.
   */
  const populateInfo = () => {
    const e = editor!;

    // --- Core ---
    info.ace_getRep = () => e.rep;
    info.ace_getAuthor = () => (e as any).thisAuthor;
    info.ace_focus = () => e.focus();
    info.ace_setEditable = (val: boolean) => e.setEditable(val);
    info.ace_getDocument = () => document;
    info.ace_dispose = () => e.dispose();

    // --- Text import/export ---
    info.ace_importText = (text: string) => e.setText(text);
    info.ace_importAText = (atext: any, apoolJsonObj: any) => e.setAttributedText(atext, apoolJsonObj);
    info.ace_exportText = () => e.exportText();

    // --- Properties ---
    info.ace_setProperty = (key: string, value: any) => e.setProperty(key, value);

    // --- Formatting ---
    info.ace_toggleAttributeOnSelection = (name: string) => e.toggleAttribute(name);
    info.ace_setAttributeOnSelection = (name: string, value: any) => {
      (e as any).setAttributeOnSelection(name, value);
    };
    info.ace_getAttributeOnSelection = (name: string) => e.getAttribute(name);

    // --- Lists ---
    // Use private methods directly because callWithAce wraps in inCallStack already
    info.ace_doInsertUnorderedList = () => (e as any).doInsertUnorderedList();
    info.ace_doInsertOrderedList = () => (e as any).doInsertOrderedList();
    // doIndentOutdent returns boolean (used by pad_editbar), so call private method
    info.ace_doIndentOutdent = (isOut: boolean) => (e as any).doIndentOutdent(isOut);

    // --- Undo/Redo ---
    info.ace_doUndoRedo = (type: string) => {
      if (type === 'undo') e.undo();
      else if (type === 'redo') e.redo();
    };

    // --- Selection ---
    info.ace_isCaret = () => e.isCaret();
    info.ace_caretLine = () => e.getCaretLine();
    info.ace_caretColumn = () => e.getCaretColumn();
    info.ace_setSelection = (selection: any) => {
      (e as any).performSelectionChange?.(selection);
    };

    // --- Document operations ---
    info.ace_performDocumentApplyAttributesToCharRange = (start: number, end: number, attribs: any[]) => {
      (e as any).performDocumentApplyAttributesToCharRange?.(start, end, attribs);
    };
    info.ace_performDocumentApplyAttributesToRange = (start: any, end: any, attribs: any[]) => {
      if ((e as any).documentAttributeManager) {
        (e as any).documentAttributeManager.setAttributesOnRange(start, end, attribs);
      }
    };
    info.ace_setAttributeOnLine = (lineNum: number, attrName: string, attrValue: any) => {
      if ((e as any).documentAttributeManager) {
        (e as any).documentAttributeManager.setAttributeOnLine(lineNum, attrName, attrValue);
      }
    };
    info.ace_removeAttributeOnLine = (lineNum: number, attrName: string) => {
      if ((e as any).documentAttributeManager) {
        (e as any).documentAttributeManager.removeAttributeOnLine(lineNum, attrName);
      }
    };

    // --- Internal access (used by plugins) ---
    info.ace_fastIncorp = (n: number) => (e as any).fastIncorp(n);
    info.ace_inCallStack = (type: string, fn: () => any) => (e as any).inCallStack(type, fn);
    info.ace_inCallStackIfNecessary = (type: string, fn: () => any) => (e as any).inCallStackIfNecessary(type, fn);
    info.ace_getInInternationalComposition = () => e.getInInternationalComposition();
    info.ace_replaceRange = (start: any, end: any, text: string) => e.replaceRange(start, end, text);
    info.ace_execCommand = (cmd: string, ...args: any[]) => e.execCommand(cmd, ...args);

    // --- Author ---
    info.ace_setAuthorInfo = (author: string, i: any) => e.setAuthorInfo(author, i);
    info.ace_getAuthorInfos = () => (e as any).authorInfos;

    // --- Key handlers ---
    info.ace_setOnKeyPress = (handler: any) => e.setOnKeyPress(handler);
    info.ace_setOnKeyDown = (handler: any) => e.setOnKeyDown(handler);
    info.ace_setNotifyDirty = (handler: any) => e.setNotifyDirty(handler);

    // --- Collaboration ---
    info.ace_setBaseText = (txt: string) => e.setBaseText(txt);
    info.ace_setBaseAttributedText = (atxt: any, apoolJsonObj: any) => e.setBaseAttributedText(atxt, apoolJsonObj);
    info.ace_applyChangesToBase = (c: string, optAuthor?: string, apoolJsonObj?: any) => e.applyChangesToBase(c, optAuthor, apoolJsonObj);
    info.ace_prepareUserChangeset = () => e.prepareUserChangeset();
    info.ace_applyPreparedChangesetToBase = () => e.applyPreparedChangesetToBase();
    info.ace_setUserChangeNotificationCallback = (f: () => void) => e.setUserChangeNotificationCallback(f);

    // --- callWithAce ---
    info.ace_callWithAce = (fn: (aceInfo: any) => any, callStack?: string, normalize?: boolean) => {
      let wrapper = () => fn(info);
      if (normalize !== undefined) {
        const inner = wrapper;
        wrapper = () => {
          info.ace_fastIncorp(9);
          return inner();
        };
      }
      if (callStack !== undefined) {
        return info.ace_inCallStackIfNecessary(callStack, wrapper);
      }
      return wrapper();
    };
  };

  // The following functions are exposed on Ace2Editor but
  // execution is delayed until init is complete
  const aceFunctionsPendingInit = [
    'importText',
    'importAText',
    'focus',
    'setEditable',
    'setOnKeyPress',
    'setOnKeyDown',
    'setNotifyDirty',
    'setProperty',
    'setBaseText',
    'setBaseAttributedText',
    'applyChangesToBase',
    'applyPreparedChangesetToBase',
    'setUserChangeNotificationCallback',
    'setAuthorInfo',
    'callWithAce',
    'execCommand',
    'replaceRange',
  ];

  for (const fnName of aceFunctionsPendingInit) {
    this[fnName] = pendingInit(function (...args: any[]) {
      info[`ace_${fnName}`].apply(info, args);
    });
  }

  // Methods that return values immediately (or fallback if not loaded)
  this.exportText = () => loaded ? info.ace_exportText() : '(awaiting init)\n';
  this.getInInternationalComposition = () => loaded ? info.ace_getInInternationalComposition() : null;
  this.prepareUserChangeset = () => loaded ? info.ace_prepareUserChangeset() : null;

  this.destroy = pendingInit(() => {
    info.ace_dispose();
    const container = document.getElementById('editorcontainer');
    if (container) container.innerHTML = '';
    editor = null;
  });

  this.init = async function (containerId: string, initialCode: string) {
    if (initialCode) {
      this.importText(initialCode);
    }

    const container = document.getElementById(containerId);
    if (!container) throw new Error(`Container #${containerId} not found`);

    const skinVariants = (window as any).clientVars?.skinVariants?.split(' ').filter((x: string) => x !== '') ?? [];

    // iframe_editor.css is loaded statically in pad.templ (no dynamic loading needed).

    // Set up the editor container structure.
    // Original structure: #editorcontainer > iframe(ace_outer) > body#outerdocbody > [sidediv, iframe(ace_inner) > body#innerdocbody]
    // New structure:      #editorcontainer > div#outerdocbody > [sidediv, div#innerdocbody]
    container.innerHTML = '';

    // Apply skin variants to html element. Do NOT add outer-editor/inner-editor classes here —
    // those trigger "background-color: transparent !important" which was meant for iframes only.
    document.documentElement.classList.add(...skinVariants);

    // Create the outerdocbody container (replaces the outer iframe's body)
    const outerBody = document.createElement('div');
    outerBody.id = 'outerdocbody';
    outerBody.classList.add('outerdocbody', ...pluginUtils.clientPluginNames());
    container.appendChild(outerBody);

    // Create sidediv for line numbers
    const sideDiv = document.createElement('div');
    sideDiv.id = 'sidediv';
    sideDiv.classList.add('sidediv');
    const sideDivInner = document.createElement('div');
    sideDivInner.id = 'sidedivinner';
    sideDivInner.classList.add('sidedivinner');
    sideDiv.appendChild(sideDivInner);
    outerBody.appendChild(sideDiv);

    // Create the contenteditable editor body (replaces the inner iframe's body)
    const editorBody = document.createElement('div');
    editorBody.id = 'innerdocbody';
    editorBody.classList.add('innerdocbody');
    editorBody.setAttribute('spellcheck', 'false');
    // flex: 1 replaces the iframe rule "#outerdocbody iframe { flex: 1 auto; width: 100% }"
    editorBody.style.flex = '1 auto';
    editorBody.style.width = '100%';
    // Remove browser focus outline (was invisible when inside an iframe)
    editorBody.style.outline = 'none';
    outerBody.appendChild(editorBody);

    // Load plugin CSS
    const includedCSS: string[] = [];
    editorBus.emit('custom:ace:editor:css', {result: includedCSS, css: includedCSS});

    // Load custom head content from plugins
    const headLines: string[] = [];
    editorBus.emit('custom:ace:init:innerdocbody:head', {iframeHTML: headLines});
    if (headLines.length > 0) {
      document.head.appendChild(
        document.createRange().createContextualFragment(headLines.join('\n')));
    }

    // Create and initialize the AceEditor
    editor = new AceEditor(editorBody);
    await editor.init();

    // Populate the info object with ace_* methods
    populateInfo();

    // Mark container as initialized (removes visibility:hidden from CSS rule
    // #editorcontainerbox #editorcontainer:not(.initialized))
    container.classList.add('initialized');

    // Emit the initialized event with info as editorInfo.
    // Plugins set custom ace_* methods on this object, and callWithAce passes it to callbacks.
    // This replaces the emit that was in ace2_inner.ts in the original code.
    editorBus.emit('editor:ace:initialized', {editorInfo: info});

    loaded = true;
    doActionsPendingInit();
  };
};

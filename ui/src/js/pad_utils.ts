'use strict';

/**
 * This code is mostly from the old Etherpad. Please help us to comment this code.
 * This helps other people to understand this code better and helps them to improve it.
 * TL;DR COMMENTS ON THIS FILE ARE HIGHLY APPRECIATED
 */

import {binarySearch} from "./ace2_common";
import {escapeHtml, escapeHtmlAttribute} from './html_escape';
import notifications from './notifications';

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

/**
 * Generates a random String with the given length. Is needed to generate the Author, Group,
 * readonly, session Ids
 */
export const randomString = (len?: number) => {
  const chars = '0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz';
  let randomstring = '';
  len = len || 20;
  for (let i = 0; i < len; i++) {
    const rnum = Math.floor(Math.random() * chars.length);
    randomstring += chars.substring(rnum, rnum + 1);
  }
  return randomstring;
};

// Set of "letter or digit" chars is based on section 20.5.16 of the original Java Language Spec.
const wordCharRegex = new RegExp(`[${[
  '\u0030-\u0039',
  '\u0041-\u005A',
  '\u0061-\u007A',
  '\u00C0-\u00D6',
  '\u00D8-\u00F6',
  '\u00F8-\u00FF',
  '\u0100-\u1FFF',
  '\u3040-\u9FFF',
  '\uF900-\uFDFF',
  '\uFE70-\uFEFE',
  '\uFF10-\uFF19',
  '\uFF21-\uFF3A',
  '\uFF41-\uFF5A',
  '\uFF66-\uFFDC',
].join('')}]`);

const urlRegex = (() => {
  // TODO: wordCharRegex matches many characters that are not permitted in URIs. Are they included
  // here as an attempt to support IRIs? (See https://tools.ietf.org/html/rfc3987.)
  const urlChar = `[-:@_.,~%+/?=&#!;()\\[\\]$'*${wordCharRegex.source.slice(1, -1)}]`;
  // Matches a single character that should not be considered part of the URL if it is the last
  // character that matches urlChar.
  const postUrlPunct = '[:.,;?!)\\]\'*]';
  // Schemes that must be followed by ://
  const withAuth = `(?:${[
    '(?:x-)?man',
    'afp',
    'file',
    'ftps?',
    'gopher',
    'https?',
    'nfs',
    'sftp',
    'smb',
    'txmt',
  ].join('|')})://`;
  // Schemes that do not need to be followed by ://
  const withoutAuth = `(?:${[
    'about',
    'geo',
    'mailto',
    'tel',
  ].join('|')}):`;
  return new RegExp(
    `(?:${withAuth}|${withoutAuth}|www\\.)${urlChar}*(?!${postUrlPunct})${urlChar}`, 'g');
})();

// https://stackoverflow.com/a/68957976
const base64url = /^(?=(?:.{4})*$)[A-Za-z0-9_-]*(?:[AQgw]==|[AEIMQUYcgkosw048]=)?$/;

const getPadRef = () => (globalThis as any).pad;

class PadUtils {
  public urlRegex: RegExp
  public wordCharRegex: RegExp
  public warnDeprecatedFlags: {
    disabledForTestingOnly: boolean,
    _rl?: {
      prevs: Map<string, number>,
      now: () => number,
      period: number
    }
    logger?: any
  }
  public globalExceptionHandler: null | any = null;


  constructor() {
    this.warnDeprecatedFlags = {
      disabledForTestingOnly: false
    }
    this.wordCharRegex = wordCharRegex
    this.urlRegex = urlRegex
  }

  /**
   * Prints a warning message followed by a stack trace (to make it easier to figure out what code
   * is using the deprecated function).
   *
   * Identical deprecation warnings (as determined by the stack trace, if available) are rate
   * limited to avoid log spam.
   *
   * Most browsers include UI widget to examine the stack at the time of the warning, but this
   * includes the stack in the log message for a couple of reasons:
   *   - This makes it possible to see the stack if the code runs in Node.js.
   *   - Users are more likely to paste the stack in bug reports they might file.
   *
   * @param {...*} args - Passed to `padutils.warnDeprecated.logger.warn` (or `console.warn` if no
   *     logger is set), with a stack trace appended if available.
   */
  warnDeprecated = (...args: any[]) => {
    if (this.warnDeprecatedFlags.disabledForTestingOnly) return;
    const err = new Error();
    if ((Error as any).captureStackTrace) (Error as any).captureStackTrace(err, this.warnDeprecated);
    err.name = '';
    // Rate limit identical deprecation warnings (as determined by the stack) to avoid log spam.
    if (typeof err.stack === 'string') {
      if (this.warnDeprecatedFlags._rl == null) {
        this.warnDeprecatedFlags._rl =
          {prevs: new Map(), now: () => Date.now(), period: 10 * 60 * 1000};
      }
      const rl = this.warnDeprecatedFlags._rl;
      const now = rl.now();
      const prev = rl.prevs.get(err.stack);
      if (prev != null && now - prev < rl.period) return;
      rl.prevs.set(err.stack, now);
    }
    if (err.stack) args.push(err.stack);
    (this.warnDeprecatedFlags.logger || console).warn(...args);
  }
  escapeHtml = (x: string) => escapeHtml(String(x))
  uniqueId = () => {
    const pad = getPadRef();
    // returns string that is exactly 'width' chars, padding with zeros and taking rightmost digits
    const encodeNum =
      (n: number, width: number) => (Array(width + 1).join('0') + Number(n).toString(35)).slice(-width);
    return [
      typeof pad?.getClientIp === 'function' ? pad.getClientIp() : '0.0.0.0',
      encodeNum(+new Date(), 7),
      encodeNum(Math.floor(Math.random() * 1e9), 4),
    ].join('.');
  }

  // e.g. "Thu Jun 18 2009 13:09"
  simpleDateTime = (date: string) => {
    const d = new Date(+date); // accept either number or date
    const dayOfWeek = (['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'])[d.getDay()];
    const month = ([
      'Jan',
      'Feb',
      'Mar',
      'Apr',
      'May',
      'Jun',
      'Jul',
      'Aug',
      'Sep',
      'Oct',
      'Nov',
      'Dec',
    ])[d.getMonth()];
    const dayOfMonth = d.getDate();
    const year = d.getFullYear();
    const hourmin = `${d.getHours()}:${(`0${d.getMinutes()}`).slice(-2)}`;
    return `${dayOfWeek} ${month} ${dayOfMonth} ${year} ${hourmin}`;
  }
  // returns null if no URLs, or [[startIndex1, url1], [startIndex2, url2], ...]
  findURLs = (text: string) => {
    // Copy padutils.urlRegex so that the use of .exec() below (which mutates the RegExp object)
    // does not break other concurrent uses of padutils.urlRegex.
    const urlRegex = new RegExp(this.urlRegex, 'g');
    urlRegex.lastIndex = 0;
    let urls: [number, string][] | null = null;
    let execResult;
    // TODO: Switch to String.prototype.matchAll() after support for Node.js < 12.0.0 is dropped.
    while ((execResult = urlRegex.exec(text))) {
      urls = (urls || []);
      const startIndex = execResult.index;
      const url = execResult[0];
      urls.push([startIndex, url]);
    }
    return urls;
  }
  escapeHtmlWithClickableLinks = (text: string, target: string) => {
    let idx = 0;
    const pieces = [];
    const urls = this.findURLs(text);

    const advanceTo = (i: number) => {
        if (i > idx) {
          pieces.push(escapeHtml(text.substring(idx, i)));
          idx = i;
        }
      }
    ;
    if (urls) {
      for (let j = 0; j < urls.length; j++) {
        const startIndex = urls[j][0];
        const href = urls[j][1];
        advanceTo(startIndex);
        // Using rel="noreferrer" stops leaking the URL/location of the pad when clicking links in
        // the document. Not all browsers understand this attribute, but it's part of the HTML5
        // standard. https://html.spec.whatwg.org/multipage/links.html#link-type-noreferrer
        // Additionally, we do rel="noopener" to ensure a higher level of referrer security.
        // https://html.spec.whatwg.org/multipage/links.html#link-type-noopener
        // https://mathiasbynens.github.io/rel-noopener/
        // https://github.com/ether/etherpad-lite/pull/3636
        pieces.push(
          '<a ',
          (target ? `target="${escapeHtmlAttribute(target)}" ` : ''),
          'href="',
          escapeHtmlAttribute(href),
          '" rel="noreferrer noopener">');
        advanceTo(startIndex + href.length);
        pieces.push('</a>');
      }
    }
    advanceTo(text.length);
    return pieces.join('');
  }
  bindEnterAndEscape = (
      node: HTMLElement | string,
      onEnter: Function,
      onEscape: Function,
  ) => {
    const element = (() => {
      if (typeof node === 'string') return document.querySelector(node);
      if (node instanceof HTMLElement) return node;
      return null;
    })();
    if (element instanceof HTMLElement) {
      if (onEnter) {
        element.addEventListener('keypress', (evt) => {
          if (evt instanceof KeyboardEvent && evt.key === 'Enter') onEnter(evt);
        });
      }
      if (onEscape) {
        element.addEventListener('keydown', (evt) => {
          if (evt instanceof KeyboardEvent && evt.key === 'Escape') onEscape(evt);
        });
      }
    }
  }

  timediff = (d: number) => {
    const pad = getPadRef();
    const format = (n: number, word: string) => {
        n = Math.round(n);
        return (`${n} ${word}${n !== 1 ? 's' : ''} ago`);
      }
    ;
    d = Math.max(0, (+(new Date()) - (+d) - Number(pad?.clientTimeOffset || 0)) / 1000);
    if (d < 60) {
      return format(d, 'second');
    }
    d /= 60;
    if (d < 60) {
      return format(d, 'minute');
    }
    d /= 60;
    if (d < 24) {
      return format(d, 'hour');
    }
    d /= 24;
    return format(d, 'day');
  }
  makeAnimationScheduler =
    (funcToAnimateOneStep: any, stepTime: number, stepsAtOnce?: number) => {
      if (stepsAtOnce === undefined) {
        stepsAtOnce = 1;
      }

      let animationTimer: any = null;

      const scheduleAnimation = () => {
        if (!animationTimer) {
          animationTimer = window.setTimeout(() => {
            animationTimer = null;
            let n = stepsAtOnce;
            let moreToDo = true;
            while (moreToDo && n > 0) {
              moreToDo = funcToAnimateOneStep();
              n--;
            }
            if (moreToDo) {
              // more to do
              scheduleAnimation();
            }
          }, stepTime * stepsAtOnce);
        }
      };
      return {scheduleAnimation};
    }

  makeFieldLabeledWhenEmpty
    =
    (field: HTMLElement | string, labelText: string) => {
      const element = (() => {
        if (typeof field === 'string') return document.querySelector(field);
        if (field instanceof HTMLElement) return field;
        return null;
      })();
      if (element instanceof HTMLInputElement || element instanceof HTMLTextAreaElement) {
        const clear = () => {
          element.classList.add('editempty');
          element.value = labelText;
        };
        element.addEventListener('focus', () => {
          if (element.classList.contains('editempty')) element.value = '';
          element.classList.remove('editempty');
        });
        element.addEventListener('blur', () => {
          if (!element.value) clear();
        });
        return {clear};
      }
      return {clear: () => {}};
    }
  getCheckbox = (node: HTMLElement | string) => {
    const el = typeof node === 'string' ? document.querySelector(node) : node;
    if (!el) return false;
    if (el.tagName === 'EP-CHECKBOX') return (el as any).checked ?? false;
    return el instanceof HTMLInputElement ? el.checked : false;
  }
  setCheckbox =
    (node: HTMLElement | string, value: boolean) => {
      const el = typeof node === 'string' ? document.querySelector(node) : node;
      if (!el) return;
      if (el.tagName === 'EP-CHECKBOX') { (el as any).checked = value; return; }
      if (el instanceof HTMLInputElement) el.checked = value;
    }
  bindCheckboxChange =
    (node: HTMLElement | string, func: Function) => {
      const el = typeof node === 'string' ? document.querySelector(node) : node;
      if (!el) return;
      // ep-checkbox fires 'ep-change', native checkbox fires 'change'
      const event = el.tagName === 'EP-CHECKBOX' ? 'ep-change' : 'change';
      el.addEventListener(event, () => func());
    }
  encodeUserId =
    (userId: string) => userId.replace(/[^a-y0-9]/g, (c) => {
      if (c === '.') return '-';
      return `z${c.charCodeAt(0)}z`;
    })
  decodeUserId =
    (encodedUserId: string) => encodedUserId.replace(/[a-y0-9]+|-|z.+?z/g, (cc) => {
      if (cc === '-') {
        return '.';
      } else if (cc.charAt(0) === 'z') {
        return String.fromCharCode(Number(cc.slice(1, -1)));
      } else {
        return cc;
      }
    })
  /**
   * Returns whether a string has the expected format to be used as a secret token identifying an
   * author. The format is defined as: 't.' followed by a non-empty base64url string (RFC 4648
   * section 5 with padding).
   *
   * Being strict about what constitutes a valid token enables unambiguous extensibility (e.g.,
   * conditional transformation of a token to a database key in a way that does not allow a
   * malicious user to impersonate another user).
   */
  isValidAuthorToken = (t: string | object) => {
    if (typeof t !== 'string' || !t.startsWith('t.')) return false;
    const v = t.slice(2);
    return v.length > 0 && base64url.test(v);
  }


  /**
   * Returns a string that can be used in the `token` cookie as a secret that authenticates a
   * particular author.
   */
  generateAuthorToken = () => `t.${randomString()}`
  setupGlobalExceptionHandler = () => {
    if (this.globalExceptionHandler == null) {
      this.globalExceptionHandler = (e: any) => {
        let type;
        let err;
        let msg, url, linenumber;
        if (e instanceof ErrorEvent) {
          type = 'Uncaught exception';
          err = e.error || {};
          ({message: msg, filename: url, lineno: linenumber} = e);
        } else if (e instanceof PromiseRejectionEvent) {
          type = 'Unhandled Promise rejection';
          err = e.reason || {};
          ({message: msg = 'unknown', fileName: url = 'unknown', lineNumber: linenumber = -1} = err);
        } else {
          throw new Error(`unknown event: ${e.toString()}`);
        }
        if (err.name != null && msg !== err.name && !msg.startsWith(`${err.name}: `)) {
          msg = `${err.name}: ${msg}`;
        }
        const errorId = randomString(20);

        const msgAlreadyVisible = Array.from(document.querySelectorAll('.gritter-item .error-msg'))
            .some((el) => (el.textContent ?? '') === msg);

        if (!msgAlreadyVisible) {
          const errorBox = document.createElement('div');
          const p1 = document.createElement('p');
          const p1b = document.createElement('b');
          p1b.textContent = 'Please press and hold Ctrl and press F5 to reload this page';
          p1.append(p1b);
          const p2 = document.createElement('p');
          p2.textContent = 'If the problem persists, please send this error message to your webmaster:';
          const details = document.createElement('div');
          details.style.textAlign = 'left';
          details.style.fontSize = '.8em';
          details.style.marginTop = '1em';
          const headline = document.createElement('b');
          headline.className = 'error-msg';
          headline.textContent = msg;
          details.append(headline, document.createElement('br'));
          details.append(`at ${url} at line ${linenumber}`, document.createElement('br'));
          details.append(`ErrorId: ${errorId}`, document.createElement('br'));
          details.append(type, document.createElement('br'));
          details.append(`URL: ${window.location.href}`, document.createElement('br'));
          details.append(`UserAgent: ${navigator.userAgent}`, document.createElement('br'));
          errorBox.append(p1, p2, details);

          notifications.add({
            title: 'An error occurred',
            text: errorBox,
            class_name: 'error',
            position: 'bottom',
            sticky: true,
          });
        }

        // send javascript errors to the server
        void fetch('../jserror', {
          method: 'POST',
          headers: {'Content-Type': 'application/x-www-form-urlencoded; charset=UTF-8'},
          body: new URLSearchParams({
            errorInfo: JSON.stringify({
              errorId,
              type,
              msg,
              url: window.location.href,
              source: url,
              linenumber,
              userAgent: navigator.userAgent,
              stack: err.stack,
            }),
          }),
        });
      };
      window.onerror = null; // Clear any pre-existing global error handler.
      window.addEventListener('error', this.globalExceptionHandler);
      window.addEventListener('unhandledrejection', this.globalExceptionHandler);
    }
  }
  binarySearch = binarySearch
}

// https://stackoverflow.com/a/42660748
const inThirdPartyIframe = () => {
  try {
    return (!window.top!.location.hostname);
  } catch (e) {
    return true;
  }
};

type CookieSetOptions = {
  expires?: number;
  sameSite?: 'Lax' | 'None' | 'Strict';
  secure?: boolean;
  path?: string;
};

const defaultCookieOptions: CookieSetOptions = typeof window !== 'undefined' ? {
  // Use `SameSite=Lax`, unless Etherpad is embedded in an iframe from another site in which case
  // use `SameSite=None`. For iframes from another site, only `None` has a chance of working
  // because the cookies are third-party (not same-site). Many browsers/users block third-party
  // cookies, but maybe blocked is better than definitely blocked (which would happen with `Lax`
  // or `Strict`). Note: `None` will not work unless secure is true.
  //
  // `Strict` is not used because it has few security benefits but significant usability drawbacks
  // vs. `Lax`. See https://stackoverflow.com/q/41841880 for discussion.
  sameSite: inThirdPartyIframe() ? 'None' : 'Lax',
  secure: window.location.protocol === 'https:',
  path: '/',
} : {};

export const Cookies = {
  get(name: string): string | undefined {
    if (typeof document === 'undefined') return undefined;
    const needle = `${encodeURIComponent(name)}=`;
    const entries = document.cookie ? document.cookie.split('; ') : [];
    for (const entry of entries) {
      if (!entry.startsWith(needle)) continue;
      return decodeURIComponent(entry.slice(needle.length));
    }
    return undefined;
  },
  set(name: string, value: string, options: CookieSetOptions = {}): void {
    if (typeof document === 'undefined') return;
    const opts = {...defaultCookieOptions, ...options};
    let cookie = `${encodeURIComponent(name)}=${encodeURIComponent(value)}`;
    if (typeof opts.expires === 'number') {
      const expiresAt = new Date(Date.now() + (opts.expires * 24 * 60 * 60 * 1000));
      cookie += `; Expires=${expiresAt.toUTCString()}`;
    }
    if (opts.path) cookie += `; Path=${opts.path}`;
    if (opts.sameSite) cookie += `; SameSite=${opts.sameSite}`;
    if (opts.secure) cookie += '; Secure';
    document.cookie = cookie;
  },
};

export default new PadUtils()

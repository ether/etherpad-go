import ChatMessage from './ChatMessage';
import html10n from './i18n';
import notifications from './notifications';
import {editorBus} from './core/EventBus';
import {padeditor} from './pad_editor';
import 'etherpad-webcomponents/EpChatMessage.js';

// ---------------------------------------------------------------------------
// Inline helpers (replaces padutils + padcookie dependencies)
// ---------------------------------------------------------------------------

/** HTML-escape a string. */
const escapeHtml = (s: string): string =>
  s.replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');

/** HTML-escape for use inside an attribute value. */
const escapeHtmlAttribute = escapeHtml;

/**
 * URL regex ported from pad_utils.ts so we can drop the padutils import.
 * Matches http(s), ftp(s), mailto, tel, geo, etc.
 */
const buildUrlRegex = (): RegExp => {
  const wordCharClass = [
    '\u0030-\u0039', '\u0041-\u005A', '\u0061-\u007A', '\u00C0-\u00D6',
    '\u00D8-\u00F6', '\u00F8-\u00FF', '\u0100-\u1FFF', '\u3040-\u9FFF',
    '\uF900-\uFDFF', '\uFE70-\uFEFE', '\uFF10-\uFF19', '\uFF21-\uFF3A',
    '\uFF41-\uFF5A', '\uFF66-\uFFDC',
  ].join('');
  const urlChar = `[-:@_.,~%+/?=&#!;()\\[\\]$'*${wordCharClass}]`;
  const postUrlPunct = '[:.,;?!)\\]\'*]';
  const withAuth = `(?:(?:x-)?man|afp|file|ftps?|gopher|https?|nfs|sftp|smb|txmt)://`;
  const withoutAuth = `(?:about|geo|mailto|tel):`;
  return new RegExp(
    `(?:${withAuth}|${withoutAuth}|www\\.)${urlChar}*(?!${postUrlPunct})${urlChar}`, 'g');
};

const URL_REGEX = buildUrlRegex();

/** Find all URLs in text. Returns array of [startIndex, url] or null. */
const findURLs = (text: string): [number, string][] | null => {
  const re = new RegExp(URL_REGEX, 'g');
  let urls: [number, string][] | null = null;
  let m: RegExpExecArray | null;
  while ((m = re.exec(text))) {
    urls = urls || [];
    urls.push([m.index, m[0]]);
  }
  return urls;
};

/** Escape HTML and convert URLs into clickable links. */
const escapeHtmlWithClickableLinks = (text: string, target: string): string => {
  let idx = 0;
  const pieces: string[] = [];
  const urls = findURLs(text);
  const advanceTo = (i: number) => {
    if (i > idx) {
      pieces.push(escapeHtml(text.substring(idx, i)));
      idx = i;
    }
  };
  if (urls) {
    for (const [startIndex, href] of urls) {
      advanceTo(startIndex);
      pieces.push(
        '<a ',
        target ? `target="${escapeHtmlAttribute(target)}" ` : '',
        'href="', escapeHtmlAttribute(href),
        '" rel="noreferrer noopener">',
      );
      advanceTo(startIndex + href.length);
      pieces.push('</a>');
    }
  }
  advanceTo(text.length);
  return pieces.join('');
};

// ---------------------------------------------------------------------------
// localStorage helpers (replaces padcookie)
// ---------------------------------------------------------------------------

const PREFS_KEY = 'etherpad_prefs';

const readPrefs = (): Record<string, unknown> => {
  try {
    const raw = localStorage.getItem(PREFS_KEY);
    return raw ? JSON.parse(raw) : {};
  } catch {
    return {};
  }
};

const writePref = (key: string, value: unknown): void => {
  try {
    const prefs = readPrefs();
    prefs[key] = value;
    localStorage.setItem(PREFS_KEY, JSON.stringify(prefs));
  } catch { /* localStorage may be unavailable */ }
};

// ---------------------------------------------------------------------------
// Title badge (chat mention counter in browser tab title)
// ---------------------------------------------------------------------------

const titleBadge = (() => {
  let baseTitle: string | null = null;
  return {
    setBubble(count: number): void {
      if (typeof document === 'undefined') return;
      const current = document.title.replace(/^\(\d+\)\s*/, '');
      if (baseTitle == null) baseTitle = current;
      document.title = count > 0 ? `(${count}) ${baseTitle}` : baseTitle;
    },
  };
})();

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

type CollabClientLike = {
  sendMessage: (message: unknown) => void;
};

type PadLike = {
  clientTimeOffset: number;
  collabClient: CollabClientLike;
  settings?: {
    hideChat?: boolean;
  };
};

type ChatContext = {
  authorName: string;
  author: string;
  text: string;
  message: ChatMessage;
  rendered: unknown;
  sticky: boolean;
  timestamp: number;
  timeStr: string;
  duration: number;
};

// ---------------------------------------------------------------------------
// Utility
// ---------------------------------------------------------------------------

const normalize = (s: string): string =>
  s.normalize('NFD').replace(/[\u0300-\u036f]/g, '').toLowerCase();

const byId = <T extends HTMLElement>(id: string): T | null =>
  document.getElementById(id) as T | null;

const asHtmlElement = (value: EventTarget | null): HTMLElement | null =>
  value instanceof HTMLElement ? value : null;

const parseHtmlFragment = (html: string): DocumentFragment => {
  const template = document.createElement('template');
  template.innerHTML = html;
  return template.content;
};

const getRenderedElement = (rendered: unknown): HTMLElement | null => {
  if (rendered instanceof HTMLElement) return rendered;
  if (typeof rendered === 'string') {
    const fragment = parseHtmlFragment(rendered);
    return fragment.firstElementChild instanceof HTMLElement ? fragment.firstElementChild : null;
  }
  if (rendered != null && typeof rendered === 'object' && 'get' in rendered) {
    const getFn = (rendered as { get: (i: number) => unknown }).get;
    const element = getFn(0);
    return element instanceof HTMLElement ? element : null;
  }
  return null;
};

const authorClass = (authorId: string): string =>
  `author-${authorId.replace(/[^a-y0-9]/g, (c) => c === '.' ? '-' : `z${c.charCodeAt(0)}z`)}`;

// ---------------------------------------------------------------------------
// ChatController
// ---------------------------------------------------------------------------

class ChatController {
  private isStuck = false;
  private userAndChat = false;
  private chatMentions = 0;
  private lastMessage: HTMLElement | null = null;
  private historyPointer = 0;
  private pad!: PadLike;

  // --- DOM accessors -------------------------------------------------------

  private get chatBox(): HTMLElement | null { return byId('chatbox'); }
  private get chatIcon(): HTMLElement | null { return byId('chaticon'); }
  private get chatInput(): HTMLTextAreaElement | HTMLInputElement | null {
    const el = byId('chatinput');
    return el instanceof HTMLTextAreaElement || el instanceof HTMLInputElement ? el : null;
  }
  private get chatText(): HTMLElement | null { return byId('chattext'); }
  private get chatCounter(): HTMLElement | null { return byId('chatcounter'); }

  // --- Public API ----------------------------------------------------------

  show(): void {
    this.chatIcon?.classList.remove('visible');
    this.chatBox?.classList.add('visible');
    this.scrollDown(true);
    this.chatMentions = 0;
    titleBadge.setBubble(0);
    for (const msg of document.querySelectorAll('.chat-gritter-msg[id]')) {
      const id = (msg as HTMLElement).id;
      if (id) notifications.remove(id);
    }
    editorBus.emit('chat:visibility:changed', { visible: true });
  }

  focus = (): void => {
    window.setTimeout(() => this.chatInput?.focus(), 100);
  };

  stickToScreen(fromInitialCall?: boolean): void {
    const stickyOption = byId('options-stickychat') as any;
    if (stickyOption?.checked) stickyOption.checked = false;
    if (this.pad.settings?.hideChat) return;

    this.show();
    this.isStuck = (!this.isStuck || fromInitialCall === true);
    if (this.chatBox != null) this.chatBox.style.display = 'none';

    window.setTimeout(() => {
      for (const el of document.querySelectorAll('#chatbox, .sticky-container')) {
        el.classList.toggle('stickyChat', this.isStuck);
      }
      if (this.chatBox != null) this.chatBox.style.display = 'flex';
    }, 0);

    writePref('chatAlwaysVisible', this.isStuck);
    if (stickyOption != null) stickyOption.checked = this.isStuck;
  }

  chatAndUsers(fromInitialCall?: boolean): void {
    const chatAndUsersOption = byId('options-chatandusers') as any;
    const stickyOption = byId('options-stickychat') as any;
    const toEnable = Boolean(chatAndUsersOption?.checked);

    if (toEnable || !this.userAndChat || fromInitialCall === true) {
      this.stickToScreen(true);
      if (stickyOption != null) {
        stickyOption.checked = true;
        stickyOption.disabled = true;
      }
      if (chatAndUsersOption != null) chatAndUsersOption.checked = true;
      this.userAndChat = true;
    } else {
      if (stickyOption != null) stickyOption.disabled = false;
      this.userAndChat = false;
    }

    writePref('chatAndUsers', this.userAndChat);
    for (const el of document.querySelectorAll('#users, .sticky-container')) {
      el.classList.toggle('chatAndUsers', this.userAndChat);
      el.classList.toggle('popup-show', this.userAndChat);
      el.classList.toggle('stickyUsers', this.userAndChat);
    }
    this.chatBox?.classList.toggle('chatAndUsersChat', this.userAndChat);
  }

  hide(): void {
    const stickyOption = byId('options-stickychat') as any;
    if (stickyOption?.checked) {
      this.stickToScreen();
      stickyOption.checked = false;
      return;
    }
    if (this.chatCounter != null) this.chatCounter.textContent = '0';
    this.chatIcon?.classList.add('visible');
    this.chatBox?.classList.remove('visible');
    editorBus.emit('chat:visibility:changed', { visible: false });
  }

  scrollDown(force?: boolean): void {
    const chatBox = this.chatBox;
    const chatText = this.chatText;
    if (chatBox == null || chatText == null || !chatBox.classList.contains('visible')) return;

    const shouldScroll = (() => {
      if (force) return true;
      if (this.lastMessage == null) return true;
      const top = this.lastMessage.getBoundingClientRect().top - chatText.getBoundingClientRect().top;
      return top < (chatText.clientHeight + 20);
    })();
    if (!shouldScroll) return;

    chatText.scrollTo({ top: chatText.scrollHeight, behavior: 'smooth' });
    const messages = chatText.querySelectorAll('ep-chat-message');
    this.lastMessage = messages.length > 0 ? messages[messages.length - 1] as HTMLElement : null;
  }

  async send(): Promise<void> {
    const input = this.chatInput;
    if (input == null) return;
    const text = input.value;
    if (text.replace(/\s+/, '').length === 0) return;

    const message = new ChatMessage(text);
    // EventBus: emit chat:message:sending before the hook call
    editorBus.emit('chat:message:sending', {message});
    editorBus.emit('chat:message:send', {text, message});
    input.value = '';
    editorBus.emit('chat:message:sent', { text });
  }

  async addMessage(msg: unknown, increment: boolean, isHistoryAdd: boolean): Promise<void> {
    const message = ChatMessage.fromObject(msg as ChatMessage);
    if (message.time == null) message.time = Date.now();
    message.time += this.pad.clientTimeOffset;

    if (!message.authorId) {
      message.authorId = 'unknown';
      console.warn('Missing "authorId" in chat message from server. Replaced with "unknown".');
    }
    if (message.text == null) message.text = '';

    const ctx: ChatContext = {
      authorName: message.displayName ?? html10n.get('pad.userlist.unnamed'),
      author: message.authorId,
      text: escapeHtmlWithClickableLinks(message.text, '_blank'),
      message,
      rendered: null,
      sticky: false,
      timestamp: message.time,
      timeStr: (() => {
        const date = new Date(message.time!);
        const minutes = `${date.getMinutes()}`.padStart(2, '0');
        const hours = `${date.getHours()}`.padStart(2, '0');
        return `${hours}:${minutes}`;
      })(),
      duration: 4000,
    };

    const alreadyFocused = document.activeElement === this.chatInput;
    const chatOpen = this.chatBox?.classList.contains('visible') === true;

    const wasMentioned =
      message.authorId !== String((window as any).clientVars?.userId ?? '') &&
      ctx.authorName !== html10n.get('pad.userlist.unnamed') &&
      normalize(ctx.text).includes(normalize(ctx.authorName));

    if (wasMentioned && !alreadyFocused && !isHistoryAdd && !chatOpen) {
      this.chatMentions++;
      titleBadge.setBubble(this.chatMentions);
      ctx.sticky = true;
    }

    // Notify via EventBus *before* the hook call so listeners can prepare
    editorBus.emit('chat:message:received', {
      authorId: ctx.author,
      text: message.text,
      time: ctx.timestamp,
    });

    // EventBus: emit chat:new:message with mutable context so plugins can
    // modify ctx.rendered, ctx.text, etc. before the DOM render
    editorBus.emit('chat:new:message', ctx);

    // --- Render the message into the DOM -----------------------------------
    const rendered = getRenderedElement(ctx.rendered);
    const chatMsg = rendered ?? document.createElement('ep-chat-message');
    if (rendered == null) {
      const myUserId = String((window as any).clientVars?.userId ?? '');
      chatMsg.setAttribute('data-authorId', ctx.author);
      chatMsg.setAttribute('author', ctx.authorName);
      chatMsg.setAttribute('time', ctx.timeStr);
      if (ctx.author === myUserId) {
        chatMsg.setAttribute('own', '');
      }
      const textContainer = document.createElement('span');
      textContainer.innerHTML = ctx.text;
      chatMsg.append(...Array.from(textContainer.childNodes));
    }

    if (isHistoryAdd) {
      const loadButton = byId('chatloadmessagesbutton');
      loadButton?.insertAdjacentElement('afterend', chatMsg);
    } else {
      this.chatText?.appendChild(chatMsg);
    }
    html10n.translateElement(html10n.translations, chatMsg);

    if (increment && !isHistoryAdd) {
      const currentCount = Number(this.chatCounter?.textContent ?? '0');
      if (this.chatCounter != null) this.chatCounter.textContent = `${currentCount + 1}`;

      if (!chatOpen && ctx.duration > 0) {
        const text = document.createElement('p');
        const authorName = document.createElement('span');
        authorName.classList.add('author-name');
        authorName.textContent = ctx.authorName;
        const textContainer = document.createElement('div');
        textContainer.innerHTML = ctx.text;
        text.append(authorName, ...Array.from(textContainer.childNodes));
        html10n.translateElement(html10n.translations, text);
        notifications.add({
          text,
          sticky: ctx.sticky,
          time: ctx.duration,
          position: 'bottom',
          class_name: 'chat-gritter-msg',
        });
      }
    }
    if (!isHistoryAdd) this.scrollDown();
  }

  init(pad: PadLike): void {
    this.pad = pad;
    const input = this.chatInput;
    const chatCounter = this.chatCounter;
    if (input == null) return;

    // --- Keyboard shortcuts ------------------------------------------------

    input.addEventListener('keydown', (evt: KeyboardEvent) => {
      if ((evt.altKey && evt.key.toLowerCase() === 'c') || evt.key === 'Escape') {
        asHtmlElement(document.activeElement)?.blur();
        (padeditor as any).ace?.focus();
        evt.preventDefault();
      }
    });

    input.addEventListener('click', () => {
      this.chatMentions = 0;
      titleBadge.setBubble(0);
    });

    document.body.addEventListener('keypress', (evt: KeyboardEvent) => {
      if (!(evt.altKey && evt.key.toLowerCase() === 'c')) return;
      asHtmlElement(evt.currentTarget)?.blur();
      this.show();
      this.chatInput?.focus();
      evt.preventDefault();
    });

    input.addEventListener('keypress', (evt: KeyboardEvent) => {
      if (evt.key === 'Enter' && !evt.shiftKey) {
        evt.preventDefault();
        void this.send();
      }
    });

    if (chatCounter != null) chatCounter.textContent = '0';

    // --- Load-more history button ------------------------------------------

    const loadButton = byId<HTMLButtonElement>('chatloadmessagesbutton');
    const loadBall = byId<HTMLElement>('chatloadmessagesball');
    loadButton?.addEventListener('click', () => {
      const start = Math.max(this.historyPointer - 20, 0);
      const end = this.historyPointer;
      if (start === end) return;
      if (loadButton != null) loadButton.style.display = 'none';
      if (loadBall != null) loadBall.style.display = 'block';
      this.pad.collabClient.sendMessage({ type: 'GET_CHAT_MESSAGES', start, end });
      this.historyPointer = start;
    });

    // --- EventBus listener for incoming messages ---------------------------
    // External code can also push messages through the bus instead of calling
    // addMessage directly.
    editorBus.on('chat:message:received', (data) => {
      // If the message was already rendered by addMessage (the normal
      // collab path) we skip to avoid duplicates. The bus emission in
      // addMessage happens *synchronously* before the DOM render, so
      // by the time this handler fires the message is already visible.
      // This listener exists so that *other* modules can react to
      // incoming chat messages without importing the chat module.
    });
  }
}

export const chat = new ChatController();

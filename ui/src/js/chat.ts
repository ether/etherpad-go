import ChatMessage from './ChatMessage';
import {padcookie} from './pad_cookie';
import padutils from './pad_utils';
import html10n from './i18n';
import notifications from './notifications';

import * as hooks from './pluginfw/hooks';
import {padeditor} from './pad_editor';
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

const normalize = (s: string): string => s.normalize('NFD').replace(/[\u0300-\u036f]/g, '').toLowerCase();

const byId = <T extends HTMLElement>(id: string): T | null => document.getElementById(id) as T | null;

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
    const getFn = (rendered as {get: (i: number) => unknown}).get;
    const element = getFn(0);
    return element instanceof HTMLElement ? element : null;
  }
  return null;
};

class ChatController {
  private isStuck = false;
  private userAndChat = false;
  private chatMentions = 0;
  private lastMessage: HTMLElement | null = null;
  private historyPointer = 0;
  private pad!: PadLike;

  private get chatBox(): HTMLElement | null { return byId('chatbox'); }
  private get chatIcon(): HTMLElement | null { return byId('chaticon'); }
  private get chatInput(): HTMLTextAreaElement | HTMLInputElement | null {
    const input = byId('chatinput');
    return input instanceof HTMLTextAreaElement || input instanceof HTMLInputElement ? input : null;
  }
  private get chatText(): HTMLElement | null { return byId('chattext'); }
  private get chatCounter(): HTMLElement | null { return byId('chatcounter'); }

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
  }

  focus = (): void => {
    window.setTimeout(() => this.chatInput?.focus(), 100);
  };

  stickToScreen(fromInitialCall?: boolean): void {
    const stickyOption = byId<HTMLInputElement>('options-stickychat');
    if (stickyOption?.checked) stickyOption.checked = false;
    if (this.pad.settings?.hideChat) return;

    this.show();
    this.isStuck = (!this.isStuck || fromInitialCall === true);
    if (this.chatBox != null) this.chatBox.style.display = 'none';

    window.setTimeout(() => {
      for (const element of document.querySelectorAll('#chatbox, .sticky-container')) {
        element.classList.toggle('stickyChat', this.isStuck);
      }
      if (this.chatBox != null) this.chatBox.style.display = 'flex';
    }, 0);

    padcookie.setPref('chatAlwaysVisible', this.isStuck);
    if (stickyOption != null) stickyOption.checked = this.isStuck;
  }

  chatAndUsers(fromInitialCall?: boolean): void {
    const chatAndUsersOption = byId<HTMLInputElement>('options-chatandusers');
    const stickyOption = byId<HTMLInputElement>('options-stickychat');
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

    padcookie.setPref('chatAndUsers', this.userAndChat);
    for (const element of document.querySelectorAll('#users, .sticky-container')) {
      element.classList.toggle('chatAndUsers', this.userAndChat);
      element.classList.toggle('popup-show', this.userAndChat);
      element.classList.toggle('stickyUsers', this.userAndChat);
    }
    this.chatBox?.classList.toggle('chatAndUsersChat', this.userAndChat);
  }

  hide(): void {
    const stickyOption = byId<HTMLInputElement>('options-stickychat');
    if (stickyOption?.checked) {
      this.stickToScreen();
      stickyOption.checked = false;
      return;
    }
    if (this.chatCounter != null) this.chatCounter.textContent = '0';
    this.chatIcon?.classList.add('visible');
    this.chatBox?.classList.remove('visible');
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

    chatText.scrollTo({top: chatText.scrollHeight, behavior: 'smooth'});
    const paragraphs = chatText.querySelectorAll('p');
    this.lastMessage = paragraphs.length > 0 ? paragraphs[paragraphs.length - 1] as HTMLElement : null;
  }

  async send(): Promise<void> {
    const input = this.chatInput;
    if (input == null) return;
    const text = input.value;
    if (text.replace(/\s+/, '').length === 0) return;

    const message = new ChatMessage(text);
    await hooks.aCallAll('chatSendMessage', Object.freeze({message}));
    this.pad.collabClient.sendMessage({type: 'CHAT_MESSAGE', message});
    input.value = '';
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

    const authorClass = (authorId: string): string => `author-${authorId.replace(/[^a-y0-9]/g, (c) => {
      if (c === '.') return '-';
      return `z${c.charCodeAt(0)}z`;
    })}`;

    const ctx: ChatContext = {
      authorName: message.displayName ?? html10n.get('pad.userlist.unnamed'),
      author: message.authorId,
      text: padutils.escapeHtmlWithClickableLinks(message.text, '_blank'),
      message,
      rendered: null,
      sticky: false,
      timestamp: message.time,
      timeStr: (() => {
        const date = new Date(message.time);
        const minutes = `${date.getMinutes()}`.padStart(2, '0');
        const hours = `${date.getHours()}`.padStart(2, '0');
        return `${hours}:${minutes}`;
      })(),
      duration: 4000,
    };

    const alreadyFocused = document.activeElement === this.chatInput;
    const chatOpen = this.chatBox?.classList.contains('visible') === true;

    const wasMentioned =
      message.authorId !== String(window.clientVars.userId ?? '') &&
      ctx.authorName !== html10n.get('pad.userlist.unnamed') &&
      normalize(ctx.text).includes(normalize(ctx.authorName));

    if (wasMentioned && !alreadyFocused && !isHistoryAdd && !chatOpen) {
      this.chatMentions++;
      titleBadge.setBubble(this.chatMentions);
      ctx.sticky = true;
    }

    await hooks.aCallAll('chatNewMessage', ctx);

    const cls = authorClass(ctx.author);
    const rendered = getRenderedElement(ctx.rendered);
    const chatMsg = rendered ?? document.createElement('p');
    if (rendered == null) {
      chatMsg.setAttribute('data-authorId', ctx.author);
      chatMsg.classList.add(cls);
      const author = document.createElement('b');
      author.textContent = `${ctx.authorName}:`;
      const time = document.createElement('span');
      time.classList.add('time', cls);
      time.innerHTML = ctx.timeStr;
      chatMsg.append(author, time, ' ');
      const textContainer = document.createElement('div');
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

    input.addEventListener('keydown', (evt) => {
      if (!(evt instanceof KeyboardEvent)) return;
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

    document.body.addEventListener('keypress', (evt) => {
      if (!(evt instanceof KeyboardEvent)) return;
      if (!(evt.altKey && evt.key.toLowerCase() === 'c')) return;
      asHtmlElement(evt.currentTarget)?.blur();
      this.show();
      this.chatInput?.focus();
      evt.preventDefault();
    });

    input.addEventListener('keypress', (evt) => {
      if (!(evt instanceof KeyboardEvent)) return;
      if (evt.key === 'Enter' && !evt.shiftKey) {
        evt.preventDefault();
        void this.send();
      }
    });

    if (chatCounter != null) chatCounter.textContent = '0';
    const loadButton = byId<HTMLButtonElement>('chatloadmessagesbutton');
    const loadBall = byId<HTMLElement>('chatloadmessagesball');
    loadButton?.addEventListener('click', () => {
      const start = Math.max(this.historyPointer - 20, 0);
      const end = this.historyPointer;
      if (start === end) return;
      if (loadButton != null) loadButton.style.display = 'none';
      if (loadBall != null) loadBall.style.display = 'block';
      this.pad.collabClient.sendMessage({type: 'GET_CHAT_MESSAGES', start, end});
      this.historyPointer = start;
    });
  }
}

export const chat = new ChatController();

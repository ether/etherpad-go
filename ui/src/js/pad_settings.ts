// Pad-wide settings (etherpad-lite #7586, #7698).
//
// The pad's creator can set chat, view and language options for everyone on
// the pad ("Pad-wide settings" in the settings popup) and enforce them. Other
// users see the pad-wide values as their defaults and may override them with
// their own preferences ("My view"), unless the creator enforces them.
//
// The pure functions below decide what applies; createPadSettings wires them
// to the page.

export type PadViewOptions = {
  showAuthorColors?: boolean | null;
  showLineNumbers?: boolean | null;
  rtlIsTrue?: boolean | null;
  padFontFamily?: string | null;
  fadeInactiveAuthorColors?: boolean | null;
  [key: string]: unknown;
};

export type PadOptions = {
  enforceSettings?: boolean;
  showChat?: boolean | null;
  alwaysShowChat?: boolean | null;
  chatAndUsers?: boolean | null;
  lang?: string | null;
  view?: PadViewOptions;
  [key: string]: unknown;
};

const TOP_LEVEL_KEYS = ['enforceSettings', 'showChat', 'alwaysShowChat', 'chatAndUsers', 'lang'] as const;
const OVERRIDABLE_KEYS = ['showChat', 'alwaysShowChat', 'chatAndUsers', 'lang'] as const;
const PLUGIN_KEY = /^ep_[a-z0-9_]+$/;

const clone = <T>(value: T): T => JSON.parse(JSON.stringify(value ?? {}));

/** Makes the chat options consistent with each other, like upstream. */
export const normalizeChatOptions = (options: PadOptions): PadOptions => {
  if (options.showChat === false) {
    options.alwaysShowChat = false;
    options.chatAndUsers = false;
  }
  if (options.chatAndUsers === true) {
    options.showChat = true;
    options.alwaysShowChat = true;
  } else if (options.alwaysShowChat === true) {
    options.showChat = true;
  }
  return options;
};

/** The user's own preferences; null where the user set nothing. */
export const myViewOverrides = (getPref: (key: string) => unknown, language: string | null): PadOptions => {
  const pref = (key: string) => {
    const value = getPref(key);
    return value === undefined ? null : value as any;
  };
  const overrides: PadOptions = {
    showChat: pref('showChat'),
    alwaysShowChat: pref('chatAlwaysVisible'),
    chatAndUsers: pref('chatAndUsers'),
    view: {
      showAuthorColors: pref('showAuthorshipColors'),
      showLineNumbers: pref('showLineNumbers'),
      rtlIsTrue: pref('rtlIsTrue'),
      padFontFamily: pref('padFontFamily'),
      fadeInactiveAuthorColors: pref('fadeInactiveAuthorColors'),
    },
  };
  if (language != null) overrides.lang = language;
  return overrides;
};

/** True when the creator enforces the pad-wide settings on this user. */
export const isEnforcedForMe = (padOptions: PadOptions | null | undefined, canEdit: boolean): boolean =>
  !!padOptions?.enforceSettings && !canEdit;

/** The options in effect for this user. */
export const effectiveOptions = (padOptions: PadOptions, overrides: PadOptions, enforced: boolean): PadOptions => {
  const effective = clone(padOptions);
  if (!effective.view) effective.view = {};
  if (enforced) return normalizeChatOptions(effective);
  for (const key of OVERRIDABLE_KEYS) {
    if (overrides[key] != null) (effective as any)[key] = overrides[key];
  }
  for (const [key, value] of Object.entries(overrides.view ?? {})) {
    if (value != null) effective.view[key] = value;
  }
  return normalizeChatOptions(effective);
};

/**
 * Merges a change (the full settings from the server, or a partial change)
 * into the pad's options, as upstream's applyPadSettings does.
 */
export const mergePadOptions = (current: PadOptions, change: PadOptions, pluginOptions: boolean): PadOptions => {
  const next = clone(current);
  for (const key of TOP_LEVEL_KEYS) {
    const value = change?.[key];
    if (value == null) continue;
    (next as any)[key] = key === 'lang' ? value : `${value}` === 'true';
  }
  if (change?.view) {
    next.view = {...(next.view ?? {}), ...change.view};
  }
  if (pluginOptions) {
    for (const [key, value] of Object.entries(change ?? {})) {
      if (PLUGIN_KEY.test(key)) next[key] = value;
    }
  }
  return normalizeChatOptions(next);
};

/**
 * What a user opening a new pad proposes as its pad-wide settings: their
 * own preferences, without the unset ones. undefined when there are none.
 */
export const creatorDefaults = (overrides: PadOptions): PadOptions | undefined => {
  const out: PadOptions = {};
  for (const key of OVERRIDABLE_KEYS) {
    if (overrides[key] != null) (out as any)[key] = overrides[key];
  }
  const view = Object.fromEntries(Object.entries(overrides.view ?? {}).filter(([, v]) => v != null));
  if (Object.keys(view).length > 0) out.view = view;
  return Object.keys(out).length > 0 ? out : undefined;
};

// ---------------------------------------------------------------------------
// Page wiring
// ---------------------------------------------------------------------------

type ChatLike = {
  setSticky(want: boolean, persist?: boolean): void;
  setChatAndUsers(want: boolean, persist?: boolean): void;
  setEnabled(enabled: boolean): void;
};

export type PadSettingsDeps = {
  getPadOptions: () => PadOptions;
  setPadOptions: (options: PadOptions) => void;
  canEdit: () => boolean;
  pluginOptions: () => boolean;
  getPref: (key: string) => unknown;
  setPref: (key: string, value: unknown) => void;
  language: () => string | null;
  setViewOptions: (view: PadViewOptions) => void;
  chat: ChatLike;
  applyLanguage: (lang: string) => void;
  send: (options: PadOptions) => void;
};

const byId = (id: string) => document.getElementById(id) as any;

const setChecked = (id: string, value: boolean) => {
  const el = byId(id);
  if (el != null) el.checked = value;
};

const setDisabled = (id: string, value: boolean) => {
  const el = byId(id);
  if (el == null) return;
  el.disabled = value;
  if (value) el.setAttribute('disabled', '');
  else el.removeAttribute('disabled');
};

const MY_VIEW_CONTROLS = [
  'options-disablechat', 'options-stickychat', 'options-chatandusers', 'options-colorscheck',
  'options-fadeauthorcheck', 'options-linenoscheck', 'options-rtlcheck', 'viewfontmenu', 'languagemenu',
];

export const createPadSettings = (deps: PadSettingsDeps) => {
  const enforced = () => isEnforcedForMe(deps.getPadOptions(), deps.canEdit());
  const effective = () =>
    effectiveOptions(deps.getPadOptions() ?? {}, myViewOverrides(deps.getPref, deps.language()), enforced());

  const self = {
    effective,
    enforced,

    /** Applies the options in effect to the editor, the chat and the controls. */
    apply: () => {
      const options = effective();
      const persist = false; // imposed values never become the user's preference
      deps.setViewOptions(options.view ?? {});
      deps.chat.setEnabled(options.showChat !== false);
      if (options.showChat !== false) {
        deps.chat.setChatAndUsers(!!options.chatAndUsers, persist);
        if (!options.chatAndUsers) deps.chat.setSticky(!!options.alwaysShowChat, persist);
        if (enforced() && typeof options.lang === 'string' && options.lang !== deps.language()) {
          deps.applyLanguage(options.lang);
        }
      }
      self.refreshMyViewControls();
      self.refreshPadWideControls();
    },

    /** Merges a change from the server (or made here) and applies it. */
    handleChange: (change: PadOptions) => {
      deps.setPadOptions(mergePadOptions(deps.getPadOptions() ?? {}, change ?? {}, deps.pluginOptions()));
      self.apply();
    },

    refreshMyViewControls: () => {
      const options = effective();
      const locked = enforced();
      setChecked('options-disablechat', options.showChat === false);
      for (const id of MY_VIEW_CONTROLS) setDisabled(id, locked);
      if (!locked) {
        setDisabled('options-stickychat', options.showChat === false || !!options.chatAndUsers);
        setDisabled('options-chatandusers', options.showChat === false);
      }
      const notice = byId('enforce-settings-notice');
      if (notice != null) notice.hidden = !locked;
    },

    refreshPadWideControls: () => {
      const section = byId('pad-settings-section');
      if (section == null) return;
      section.hidden = !deps.canEdit();
      if (!deps.canEdit()) return;
      const options = normalizeChatOptions(clone(deps.getPadOptions() ?? {}));
      const view = options.view ?? {};
      setChecked('padsettings-options-disablechat', options.showChat === false);
      setChecked('padsettings-options-stickychat', !!options.alwaysShowChat);
      setChecked('padsettings-options-chatandusers', !!options.chatAndUsers);
      setChecked('padsettings-options-colorscheck', view.showAuthorColors !== false);
      setChecked('padsettings-options-fadeauthorcheck', view.fadeInactiveAuthorColors !== false);
      setChecked('padsettings-options-linenoscheck', view.showLineNumbers !== false);
      setChecked('padsettings-options-rtlcheck', !!view.rtlIsTrue);
      setChecked('padsettings-enforcecheck', !!options.enforceSettings);
      setDisabled('padsettings-options-stickychat', options.showChat === false);
      setDisabled('padsettings-options-chatandusers', options.showChat === false);
      const font = byId('padsettings-viewfontmenu');
      const fontTrigger = font?.querySelector('[slot="trigger"]');
      const fontItem = font?.querySelector(`ep-dropdown-item[value="${CSS.escape(String(view.padFontFamily ?? ''))}"]`);
      if (fontTrigger && fontItem) fontTrigger.textContent = fontItem.textContent;
      const lang = byId('padsettings-languagemenu');
      const langTrigger = lang?.querySelector('[slot="trigger"]');
      const langItem = lang?.querySelector(`ep-dropdown-item[value="${CSS.escape(String(options.lang ?? deps.language() ?? 'en'))}"]`);
      if (langTrigger && langItem) langTrigger.textContent = langItem.textContent;
    },

    /** A pad-wide change by the creator: applied here, sent to everyone. */
    changePadOption: (key: string, value: unknown) => {
      const change = {[key]: value} as PadOptions;
      self.handleChange(change);
      deps.send(change);
    },

    changePadViewOption: (key: string, value: unknown) => {
      const change: PadOptions = {view: {[key]: value}};
      self.handleChange(change);
      deps.send(change);
      // The creator is never enforced upon, so a stale own preference would
      // hide the value just chosen (upstream #7900): adopt it as the own one.
      deps.setPref(key === 'showAuthorColors' ? 'showAuthorshipColors' : key, value);
      self.apply();
    },

    /** Binds the pad-wide controls; call once after the page is ready. */
    bindControls: () => {
      const onToggle = (id: string, fn: (checked: boolean) => void) => {
        byId(id)?.addEventListener('ep-change', (e: CustomEvent) => {
          fn(e.detail?.checked ?? byId(id)?.checked ?? false);
        });
      };
      onToggle('padsettings-enforcecheck', (v) => self.changePadOption('enforceSettings', v));
      onToggle('padsettings-options-disablechat', (v) => self.changePadOption('showChat', !v));
      onToggle('padsettings-options-stickychat', (v) => self.changePadOption('alwaysShowChat', v));
      onToggle('padsettings-options-chatandusers', (v) => self.changePadOption('chatAndUsers', v));
      onToggle('padsettings-options-colorscheck', (v) => self.changePadViewOption('showAuthorColors', v));
      onToggle('padsettings-options-fadeauthorcheck', (v) => self.changePadViewOption('fadeInactiveAuthorColors', v));
      onToggle('padsettings-options-linenoscheck', (v) => self.changePadViewOption('showLineNumbers', v));
      onToggle('padsettings-options-rtlcheck', (v) => self.changePadViewOption('rtlIsTrue', v));
      byId('padsettings-viewfontmenu')?.addEventListener('ep-dropdown-select', (e: CustomEvent) => {
        self.changePadViewOption('padFontFamily', e.detail?.value ?? '');
      });
      byId('padsettings-languagemenu')?.addEventListener('ep-dropdown-select', (e: CustomEvent) => {
        self.changePadOption('lang', e.detail?.value ?? '');
      });
      // "Disable chat" in My view: the user's own preference.
      onToggle('options-disablechat', (v) => {
        deps.setPref('showChat', !v);
        if (v) {
          deps.setPref('chatAlwaysVisible', false);
          deps.setPref('chatAndUsers', false);
        }
        self.apply();
      });
    },
  };
  return self;
};

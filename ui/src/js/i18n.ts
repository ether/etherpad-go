type Listener = () => void;

type EventName = 'indexed' | 'localized';

class Emitter {
  private listeners = new Map<EventName, Set<Listener>>();

  bind(event: EventName, listener: Listener): void {
    if (!this.listeners.has(event)) this.listeners.set(event, new Set());
    this.listeners.get(event)?.add(listener);
  }

  emit(event: EventName): void {
    this.listeners.get(event)?.forEach((listener) => listener());
  }
}

class I18n {
  public readonly mt = new Emitter();
  public readonly translations = new Map<string, string>();

  private localeIndex: Record<string, string | Record<string, string>> = {};
  private localeIndexBaseUrl = window.location.href;
  private currentLanguage = 'en';
  private direction: 'ltr' | 'rtl' = 'ltr';

  constructor() {
    if (document.readyState === 'loading') {
      document.addEventListener('DOMContentLoaded', () => { void this.index(); }, {once: true});
    } else {
      void this.index();
    }
  }

  bind(event: EventName, listener: Listener): void {
    this.mt.bind(event, listener);
  }

  getLanguage(): string {
    return this.currentLanguage;
  }

  getDirection(): 'ltr' | 'rtl' {
    return this.direction;
  }

  async index(): Promise<void> {
    const link = document.querySelector<HTMLLinkElement>('link[rel="localizations"]');
    if (!link?.href) {
      this.localeIndex = {};
      this.mt.emit('indexed');
      return;
    }

    try {
      const response = await fetch(link.href);
      if (response.ok) {
        this.localeIndex = await response.json() as Record<string, string | Record<string, string>>;
        this.localeIndexBaseUrl = response.url || link.href;
      }
    } catch {
      this.localeIndex = {};
    }

    this.mt.emit('indexed');
  }

  async localize(preferredLanguages: Array<string | undefined>): Promise<void> {
    const selected = preferredLanguages.find((lang) => {
      if (!lang) return false;
      return this.localeIndex[lang] != null || this.localeIndex[lang.toLowerCase()] != null;
    }) ?? 'en';

    const normalized = selected.toLowerCase();
    this.currentLanguage = normalized;

    const en = await this.fetchLocale('en');
    const selectedLocale = normalized === 'en' ? en : await this.fetchLocale(normalized);
    const merged = {...en, ...selectedLocale};

    this.translations.clear();
    Object.entries(merged).forEach(([key, value]) => this.translations.set(key, value));

    this.direction = ['ar', 'dv', 'fa', 'ha', 'he', 'ks', 'ku', 'ps', 'ur', 'yi'].includes(normalized) ? 'rtl' : 'ltr';

    this.translateElement(this.translations, document.body);
    this.mt.emit('localized');
  }

  get(key: string, vars?: Record<string, unknown>): string {
    const value = this.translations.get(key) ?? key;
    if (!vars) return value;
    // Etherpad locales primarily use mustache-style placeholders: {{name}}.
    const replaceToken = (full: string, rawName: string): string => {
      const replacement = vars[rawName.trim()];
      return replacement == null ? full : String(replacement);
    };
    return value
      .replace(/\{\{\s*([^{}]+)\s*\}\}/g, (_full, name: string) => replaceToken(_full, name))
      .replace(/\{([^{}]+)\}/g, (_full, name: string) => replaceToken(_full, name));
  }

  translateElement(_translations: Map<string, string>, root: ParentNode | Element): void {
    const nodes: Element[] = [];
    if (root instanceof Element && root.hasAttribute('data-l10n-id')) nodes.push(root);
    nodes.push(...Array.from(root.querySelectorAll('[data-l10n-id]')));

    nodes.forEach((node) => {
      const key = node.getAttribute('data-l10n-id');
      if (!key) return;
      const value = this.get(key);

      if (node instanceof HTMLInputElement || node instanceof HTMLTextAreaElement) {
        if (node.hasAttribute('placeholder')) node.placeholder = value;
        else node.value = value;
      } else {
        if (node instanceof HTMLButtonElement && node.classList.contains('buttonicon')) {
          // Icon-only buttons should not receive visible text labels.
        } else
        // Do not replace structured content (icons/buttons). Only set text directly
        // if the element has no child elements.
        if (node.children.length === 0) node.textContent = value;
      }

      if (node.hasAttribute('title')) node.setAttribute('title', value);
      if (node.hasAttribute('aria-label')) node.setAttribute('aria-label', value);
    });
  }

  private async fetchLocale(lang: string): Promise<Record<string, string>> {
    const entry = this.localeIndex[lang] ?? this.localeIndex[lang.toLowerCase()];
    if (!entry) return {};

    if (typeof entry === 'object') return entry as Record<string, string>;

    try {
      const resolvedEntry = new URL(entry, this.localeIndexBaseUrl).toString();
      const response = await fetch(resolvedEntry);
      if (!response.ok) return {};
      const data = await response.json() as Record<string, Record<string, string>>;
      return data[lang] ?? data[lang.toLowerCase()] ?? {};
    } catch {
      return {};
    }
  }
}

const i18n = new I18n();
export default i18n;

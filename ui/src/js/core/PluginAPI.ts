/**
 * PluginAPI — A formalized API surface for Etherpad plugins.
 *
 * Instead of reaching into DOM internals or importing random modules,
 * plugins create a PluginAPI instance and interact with the editor through
 * it.  The API wraps the EventBus, tracks registrations for toolbar
 * buttons / selects / settings items, and handles automatic cleanup when a
 * plugin is destroyed.
 *
 * Usage:
 *   import { PluginAPI } from './core/PluginAPI';
 *   import { editorBus }  from './core/EventBus';
 *
 *   const api = new PluginAPI('ep_example', editorBus);
 *   api.registerToolbarButton({ key: 'myBtn', title: 'My Button', icon: 'icon-star', onClick() { ... } });
 *   // later …
 *   api.destroy();
 */

import { EventBus, type EditorEvents } from './EventBus';

// ---------------------------------------------------------------------------
// Configuration types
// ---------------------------------------------------------------------------

export interface ToolbarButtonConfig {
  /** Unique key for this button (doubles as `data-key` attribute). */
  key: string;
  /** i18n key for the tooltip / label. */
  title: string;
  /** CSS class applied to the button icon. */
  icon: string;
  /** Which toolbar group the button belongs to. Defaults to `'left'`. */
  group?: 'left' | 'middle' | 'right';
  /** Click handler. */
  onClick: () => void;
  /** The plugin that registered this button (set automatically). */
  pluginName?: string;
}

export interface ToolbarSelectConfig {
  /** Unique key for this select control. */
  key: string;
  /** i18n key for the tooltip / label. */
  title: string;
  /** Available options. */
  options: { label: string; value: string }[];
  /** Change handler. */
  onChange: (value: string) => void;
  /** The plugin that registered this select (set automatically). */
  pluginName?: string;
}

export interface SettingsItemConfig {
  /** Unique key for this settings entry. */
  key: string;
  /** i18n key for the label. */
  label: string;
  /** Control type. */
  type: 'checkbox' | 'select' | 'text';
  /** Initial value. */
  defaultValue?: any;
  /** Options (only relevant when `type === 'select'`). */
  options?: { label: string; value: string }[];
  /** Change handler. */
  onChange: (value: any) => void;
  /** The plugin that registered this item (set automatically). */
  pluginName?: string;
}

export interface NotificationConfig {
  /** Notification body text. */
  text: string;
  /** Auto-dismiss duration in milliseconds.  `0` = sticky. */
  duration?: number;
  /** Where to display the notification. */
  position?: 'top' | 'bottom';
}

// ---------------------------------------------------------------------------
// Static registries — queried by UI components at render time
// ---------------------------------------------------------------------------

const toolbarButtons: ToolbarButtonConfig[] = [];
const toolbarSelects: ToolbarSelectConfig[] = [];
const settingsItems: SettingsItemConfig[] = [];

// ---------------------------------------------------------------------------
// PluginAPI
// ---------------------------------------------------------------------------

export class PluginAPI {
  /** All unsubscribe functions accumulated by this instance. */
  private unsubs: Array<() => void> = [];
  /** Keys of toolbar buttons registered by *this* plugin. */
  private ownToolbarButtons: string[] = [];
  /** Keys of toolbar selects registered by *this* plugin. */
  private ownToolbarSelects: string[] = [];
  /** Keys of settings items registered by *this* plugin. */
  private ownSettingsItems: string[] = [];

  constructor(
    private pluginName: string,
    private bus: EventBus<EditorEvents>,
  ) {}

  // ------------------------------------------------------------------
  // Static registry accessors
  // ------------------------------------------------------------------

  /** Returns all registered toolbar buttons (across all plugins). */
  static getToolbarButtons(): ToolbarButtonConfig[] {
    return toolbarButtons.slice();
  }

  /** Returns all registered toolbar selects (across all plugins). */
  static getToolbarSelects(): ToolbarSelectConfig[] {
    return toolbarSelects.slice();
  }

  /** Returns all registered settings items (across all plugins). */
  static getSettingsItems(): SettingsItemConfig[] {
    return settingsItems.slice();
  }

  // ------------------------------------------------------------------
  // Toolbar
  // ------------------------------------------------------------------

  /**
   * Register a toolbar button.  The button configuration is stored in a
   * static registry that UI components can query at render time.
   */
  registerToolbarButton(config: Omit<ToolbarButtonConfig, 'pluginName'>): void {
    const entry: ToolbarButtonConfig = { ...config, pluginName: this.pluginName };
    if (!entry.group) entry.group = 'left';
    toolbarButtons.push(entry);
    this.ownToolbarButtons.push(config.key);

    // Also wire up click events from the bus.
    const unsub = this.bus.on('toolbar:button:click', (data) => {
      if (data.key === config.key) {
        try {
          config.onClick();
        } catch (err) {
          this.bus.emit('plugin:error', { name: this.pluginName, error: err as Error });
        }
      }
    });
    this.unsubs.push(unsub);
  }

  /**
   * Register a toolbar select (dropdown) control.
   */
  registerToolbarSelect(config: Omit<ToolbarSelectConfig, 'pluginName'>): void {
    const entry: ToolbarSelectConfig = { ...config, pluginName: this.pluginName };
    toolbarSelects.push(entry);
    this.ownToolbarSelects.push(config.key);

    const unsub = this.bus.on('toolbar:command', (data) => {
      if (data.command === config.key) {
        try {
          config.onChange(data.value as string);
        } catch (err) {
          this.bus.emit('plugin:error', { name: this.pluginName, error: err as Error });
        }
      }
    });
    this.unsubs.push(unsub);
  }

  // ------------------------------------------------------------------
  // Settings
  // ------------------------------------------------------------------

  /**
   * Register a settings panel item.
   */
  registerSettingsItem(config: Omit<SettingsItemConfig, 'pluginName'>): void {
    const entry: SettingsItemConfig = { ...config, pluginName: this.pluginName };
    settingsItems.push(entry);
    this.ownSettingsItems.push(config.key);

    const unsub = this.bus.on('settings:changed', (data) => {
      if (data.key === config.key) {
        try {
          config.onChange(data.value);
        } catch (err) {
          this.bus.emit('plugin:error', { name: this.pluginName, error: err as Error });
        }
      }
    });
    this.unsubs.push(unsub);
  }

  // ------------------------------------------------------------------
  // Editor
  // ------------------------------------------------------------------

  /**
   * Listen to an editor event. Returns an unsubscribe function.
   * The subscription is also automatically cleaned up on `destroy()`.
   */
  onEditorEvent(
    event: string & keyof EditorEvents,
    handler: (data: any) => void,
  ): () => void {
    const unsub = this.bus.on(event, handler as any);
    this.unsubs.push(unsub);
    return unsub;
  }

  /**
   * Run a function that receives the editor instance (the Ace editor / inner
   * document).  The call is deferred until the editor is ready.  If the
   * editor is already ready the function is invoked synchronously.
   *
   * This is intentionally loosely typed (`any`) because the internal editor
   * API is not yet fully typed.
   */
  callWithEditor(fn: (editor: any) => void): void {
    // Emit a request; the editor host listens and calls back.
    this.bus.emit('custom:pluginapi:callWithEditor' as any, {
      pluginName: this.pluginName,
      fn,
    });
  }

  // ------------------------------------------------------------------
  // Chat
  // ------------------------------------------------------------------

  /**
   * Subscribe to incoming chat messages.  Returns an unsubscribe function.
   */
  onChatMessage(
    handler: (msg: { authorId: string; text: string; time: number }) => void,
  ): () => void {
    const unsub = this.bus.on('chat:message:received', handler);
    this.unsubs.push(unsub);
    return unsub;
  }

  // ------------------------------------------------------------------
  // CSS injection
  // ------------------------------------------------------------------

  /**
   * Inject a CSS stylesheet into the outer (pad) document.
   */
  injectCSS(href: string): void {
    if (document.querySelector(`link[href="${href}"]`)) return;
    const link = document.createElement('link');
    link.rel = 'stylesheet';
    link.href = href;
    link.dataset.plugin = this.pluginName;
    document.head.appendChild(link);
  }

  /**
   * Inject a CSS stylesheet into the inner (editor iframe) document.
   * Emits an event so the ace host can pick it up.
   */
  injectEditorCSS(href: string): void {
    this.bus.emit('custom:pluginapi:injectEditorCSS' as any, {
      pluginName: this.pluginName,
      href,
    });
  }

  // ------------------------------------------------------------------
  // Notifications
  // ------------------------------------------------------------------

  /**
   * Show a transient notification using the existing notification system.
   */
  showNotification(config: NotificationConfig): void {
    this.bus.emit('custom:pluginapi:showNotification' as any, {
      pluginName: this.pluginName,
      ...config,
    });
  }

  // ------------------------------------------------------------------
  // Generic events
  // ------------------------------------------------------------------

  /**
   * Subscribe to any bus event. Returns an unsubscribe function.
   * The subscription is cleaned up on `destroy()`.
   */
  on(event: string, handler: (...args: any[]) => void): () => void {
    const unsub = this.bus.on(event as any, handler as any);
    this.unsubs.push(unsub);
    return unsub;
  }

  /**
   * Emit an event on the shared bus.
   */
  emit(event: string, data?: unknown): void {
    this.bus.emit(event as any, data as any);
  }

  // ------------------------------------------------------------------
  // Cleanup
  // ------------------------------------------------------------------

  /**
   * Tear down this plugin API instance.
   *
   * - Removes all event subscriptions created through this instance.
   * - Removes toolbar buttons, selects, and settings items that this
   *   plugin registered.
   * - Removes any `<link>` elements injected via `injectCSS`.
   */
  destroy(): void {
    // Unsubscribe all bus listeners.
    for (const unsub of this.unsubs) {
      try {
        unsub();
      } catch {
        // Ignore — handler may already have been removed.
      }
    }
    this.unsubs.length = 0;

    // Remove toolbar buttons.
    for (const key of this.ownToolbarButtons) {
      const idx = toolbarButtons.findIndex((b) => b.key === key && b.pluginName === this.pluginName);
      if (idx !== -1) toolbarButtons.splice(idx, 1);
    }
    this.ownToolbarButtons.length = 0;

    // Remove toolbar selects.
    for (const key of this.ownToolbarSelects) {
      const idx = toolbarSelects.findIndex((s) => s.key === key && s.pluginName === this.pluginName);
      if (idx !== -1) toolbarSelects.splice(idx, 1);
    }
    this.ownToolbarSelects.length = 0;

    // Remove settings items.
    for (const key of this.ownSettingsItems) {
      const idx = settingsItems.findIndex((s) => s.key === key && s.pluginName === this.pluginName);
      if (idx !== -1) settingsItems.splice(idx, 1);
    }
    this.ownSettingsItems.length = 0;

    // Remove injected stylesheets.
    document.querySelectorAll(`link[data-plugin="${this.pluginName}"]`).forEach((el) => el.remove());
  }
}

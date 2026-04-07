/**
 * UIRegistry — Central registry for plugin UI contributions.
 *
 * Plugins register toolbar buttons, toolbar selects, and settings items here.
 * The Web Components query this registry to render their content.
 *
 * The registry is a singleton — import `uiRegistry` and call its methods.
 */

// ---------------------------------------------------------------------------
// Registration types
// ---------------------------------------------------------------------------

export interface ToolbarButtonRegistration {
  pluginName: string;
  key: string;
  title: string;
  icon: string;
  group: 'left' | 'middle' | 'right';
  onClick: () => void;
}

export interface ToolbarSelectRegistration {
  pluginName: string;
  key: string;
  title: string;
  options: Array<{ label: string; value: string }>;
  onChange: (value: string) => void;
}

export interface SettingsItemRegistration {
  pluginName: string;
  key: string;
  label: string;
  type: 'checkbox' | 'select' | 'text';
  defaultValue?: any;
  options?: Array<{ label: string; value: string }>;
  onChange: (value: any) => void;
}

// ---------------------------------------------------------------------------
// Change listener type
// ---------------------------------------------------------------------------

type ChangeHandler = () => void;

// ---------------------------------------------------------------------------
// UIRegistry
// ---------------------------------------------------------------------------

class UIRegistry {
  private toolbarButtons: ToolbarButtonRegistration[] = [];
  private toolbarSelects: ToolbarSelectRegistration[] = [];
  private settingsItems: SettingsItemRegistration[] = [];
  private changeHandlers = new Set<ChangeHandler>();

  // ---------------------------------------------------------------
  // Registration
  // ---------------------------------------------------------------

  registerToolbarButton(config: ToolbarButtonRegistration): void {
    this.toolbarButtons.push(config);
    this.notifyChange();
  }

  registerToolbarSelect(config: ToolbarSelectRegistration): void {
    this.toolbarSelects.push(config);
    this.notifyChange();
  }

  registerSettingsItem(config: SettingsItemRegistration): void {
    this.settingsItems.push(config);
    this.notifyChange();
  }

  // ---------------------------------------------------------------
  // Unregistration (by plugin name + key)
  // ---------------------------------------------------------------

  unregisterToolbarButton(pluginName: string, key: string): boolean {
    const idx = this.toolbarButtons.findIndex(
      (b) => b.pluginName === pluginName && b.key === key,
    );
    if (idx === -1) return false;
    this.toolbarButtons.splice(idx, 1);
    this.notifyChange();
    return true;
  }

  unregisterToolbarSelect(pluginName: string, key: string): boolean {
    const idx = this.toolbarSelects.findIndex(
      (s) => s.pluginName === pluginName && s.key === key,
    );
    if (idx === -1) return false;
    this.toolbarSelects.splice(idx, 1);
    this.notifyChange();
    return true;
  }

  unregisterSettingsItem(pluginName: string, key: string): boolean {
    const idx = this.settingsItems.findIndex(
      (s) => s.pluginName === pluginName && s.key === key,
    );
    if (idx === -1) return false;
    this.settingsItems.splice(idx, 1);
    this.notifyChange();
    return true;
  }

  // ---------------------------------------------------------------
  // Queries (return defensive copies)
  // ---------------------------------------------------------------

  getToolbarButtons(): ToolbarButtonRegistration[] {
    return this.toolbarButtons.slice();
  }

  getToolbarSelects(): ToolbarSelectRegistration[] {
    return this.toolbarSelects.slice();
  }

  getSettingsItems(): SettingsItemRegistration[] {
    return this.settingsItems.slice();
  }

  /** Returns all toolbar buttons belonging to a specific plugin. */
  getToolbarButtonsByPlugin(pluginName: string): ToolbarButtonRegistration[] {
    return this.toolbarButtons.filter((b) => b.pluginName === pluginName);
  }

  /** Returns all settings items belonging to a specific plugin. */
  getSettingsItemsByPlugin(pluginName: string): SettingsItemRegistration[] {
    return this.settingsItems.filter((s) => s.pluginName === pluginName);
  }

  // ---------------------------------------------------------------
  // Change notification
  // ---------------------------------------------------------------

  /**
   * Register a handler that is called whenever any registration changes.
   * Returns an unsubscribe function.
   */
  onRegistrationChange(handler: ChangeHandler): () => void {
    this.changeHandlers.add(handler);
    return () => {
      this.changeHandlers.delete(handler);
    };
  }

  private notifyChange(): void {
    for (const handler of this.changeHandlers) {
      try {
        handler();
      } catch (err) {
        console.error('[UIRegistry] Error in change handler:', err);
      }
    }
  }

  // ---------------------------------------------------------------
  // Reset (useful for tests)
  // ---------------------------------------------------------------

  clear(): void {
    this.toolbarButtons.length = 0;
    this.toolbarSelects.length = 0;
    this.settingsItems.length = 0;
    this.changeHandlers.clear();
  }
}

// ---------------------------------------------------------------------------
// Singleton
// ---------------------------------------------------------------------------

export const uiRegistry = new UIRegistry();

/**
 * PluginBridge — Bridges the existing Etherpad plugin hook system with the
 * new EventBus-based PluginAPI.
 *
 * Allows plugins to use EITHER the old hooks system OR the new EventBus.
 * When old hooks fire, corresponding EventBus events are emitted.
 * When EventBus events fire, plugins registered via the old hook system
 * still work as before.
 *
 * This module does NOT modify the existing hooks module — it layers on top.
 */

import { editorBus, type EditorEvents } from './EventBus';

// ---------------------------------------------------------------------------
// Hook <-> EventBus mapping
// ---------------------------------------------------------------------------

/**
 * Maps legacy hook names to EventBus event names.
 * Only hooks that have a meaningful EventBus equivalent are listed.
 */
const hookEventMap: Record<string, keyof EditorEvents> = {
  'postAceInit': 'editor:ready',
  'aceEditEvent': 'editor:content:changed',
  'chatNewMessage': 'chat:message:received',
  'postToolbarInit': 'toolbar:ready' as keyof EditorEvents,
  'userJoinOrUpdate': 'user:info:updated',
};

/**
 * Reverse map: EventBus event -> legacy hook name.
 * Built lazily from hookEventMap.
 */
const eventHookMap: Record<string, string> = {};
for (const [hook, event] of Object.entries(hookEventMap)) {
  eventHookMap[event] = hook;
}

// ---------------------------------------------------------------------------
// PluginAPI — lightweight per-plugin facade over the EventBus
// ---------------------------------------------------------------------------

/**
 * A scoped API object handed to each plugin.  Wraps the EventBus so that
 * unsubscriptions can be tracked per-plugin and cleaned up together.
 */
export class PluginBridgeAPI {
  readonly pluginName: string;
  private unsubs: Array<() => void> = [];

  constructor(pluginName: string) {
    this.pluginName = pluginName;
  }

  /** Subscribe to an EventBus event. Returns an unsubscribe function. */
  on<K extends string & keyof EditorEvents>(
    event: K,
    handler: (data: EditorEvents[K]) => void,
  ): () => void {
    const unsub = editorBus.on(event, handler as any);
    this.unsubs.push(unsub);
    return unsub;
  }

  /** Subscribe to an EventBus event once. */
  once<K extends string & keyof EditorEvents>(
    event: K,
    handler: (data: EditorEvents[K]) => void,
  ): () => void {
    const unsub = editorBus.once(event, handler as any);
    this.unsubs.push(unsub);
    return unsub;
  }

  /** Emit an event on the EventBus. */
  emit<K extends string & keyof EditorEvents>(
    event: K,
    ...args: EditorEvents[K] extends void ? [] : [data: EditorEvents[K]]
  ): void {
    (editorBus.emit as Function)(event, ...args);
  }

  /** Emit a custom (plugin-namespaced) event. */
  emitCustom(eventSuffix: string, data?: unknown): void {
    editorBus.emit(`custom:${this.pluginName}:${eventSuffix}` as any, data);
  }

  /** Listen to a custom (plugin-namespaced) event. */
  onCustom(eventSuffix: string, handler: (data: any) => void): () => void {
    const unsub = editorBus.on(
      `custom:${this.pluginName}:${eventSuffix}` as any,
      handler,
    );
    this.unsubs.push(unsub);
    return unsub;
  }

  /**
   * Wait for an event with an optional timeout.
   * Returns a Promise that resolves with the event data.
   */
  waitFor<K extends string & keyof EditorEvents>(
    event: K,
    timeout?: number,
  ): Promise<EditorEvents[K]> {
    return editorBus.waitFor(event, timeout);
  }

  /** Get the mapped EventBus event name for a legacy hook, or null. */
  getEventForHook(hookName: string): string | null {
    return hookEventMap[hookName] ?? null;
  }

  /** Get the mapped legacy hook name for an EventBus event, or null. */
  getHookForEvent(eventName: string): string | null {
    return eventHookMap[eventName] ?? null;
  }

  /** Remove all subscriptions made through this PluginAPI instance. */
  dispose(): void {
    for (const unsub of this.unsubs) {
      try { unsub(); } catch { /* ignore */ }
    }
    this.unsubs.length = 0;
  }
}

// ---------------------------------------------------------------------------
// Per-plugin PluginAPI registry
// ---------------------------------------------------------------------------

const pluginAPIs = new Map<string, PluginBridgeAPI>();

/**
 * Get (or create) a PluginAPI for the given plugin name.
 * Calling this multiple times with the same name returns the same instance.
 */
export function getPluginAPI(pluginName: string): PluginBridgeAPI {
  let api = pluginAPIs.get(pluginName);
  if (api === undefined) {
    api = new PluginBridgeAPI(pluginName);
    pluginAPIs.set(pluginName, api);
  }
  return api;
}

/**
 * Dispose all PluginAPI instances. Useful for tests or full teardown.
 */
export function disposeAllPluginAPIs(): void {
  for (const api of pluginAPIs.values()) {
    api.dispose();
  }
  pluginAPIs.clear();
}

// ---------------------------------------------------------------------------
// Hook event map accessors (for external use)
// ---------------------------------------------------------------------------

export { hookEventMap, eventHookMap };

// ---------------------------------------------------------------------------
// Global exposure — plugins that don't use ES module imports can access
// the API from `window.EtherpadPluginAPI`.
// ---------------------------------------------------------------------------

if (typeof window !== 'undefined') {
  (window as any).EtherpadPluginAPI = {
    getPluginAPI,
    hookEventMap,
    eventHookMap,
    editorBus,
  };
}

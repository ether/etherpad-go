/**
 * EventBus — A typed, singleton event bus for the Etherpad frontend.
 *
 * Replaces direct module coupling with an event-driven architecture.
 * Supports typed events, wildcard listeners, one-shot subscriptions,
 * promise-based waiting, and a debug mode.
 *
 * Usage:
 *   import { editorBus } from './core/EventBus';
 *   const unsub = editorBus.on('editor:ready', () => { ... });
 *   editorBus.emit('editor:ready', undefined);
 *   unsub();
 */

// ---------------------------------------------------------------------------
// Event map
// ---------------------------------------------------------------------------

export interface EditorEvents {
  // Editor lifecycle
  'editor:ready': { ace: any };
  'editor:content:changed': { text: string };
  'editor:cursor:moved': { line: number; column: number };
  'editor:selection:changed': { start: [number, number]; end: [number, number] };

  // Toolbar
  'toolbar:command': { command: string; value?: any };
  'toolbar:button:click': { key: string };
  'toolbar:dropdown:toggle': { name: string; visible: boolean };

  // Chat
  'chat:message:send': { text: string; message: any };
  'chat:message:sent': { text: string };
  'chat:message:received': { authorId: string; text: string; time: number };
  'chat:visibility:changed': { visible: boolean };

  // User
  'user:join': { userId: string; name?: string; colorId?: string };
  'user:leave': { userId: string };
  'user:info:updated': { userId: string; name?: string; colorId?: string };

  // Connection
  'connection:connected': void;
  'connection:disconnected': { reason: string };
  'connection:reconnecting': void;

  // Settings
  'settings:changed': { key: string; value: any };
  'settings:visibility:changed': { visible: boolean };

  // Plugin lifecycle
  'plugin:loaded': { name: string };
  'plugin:error': { name: string; error: Error };

  // Editor hooks (with mutable result arrays for return values)
  'editor:attribs:to:classes': { key: string; value: string; result: string[] };
  'editor:create:dom:line': { cls: string; domline: any; result: any[] };
  'editor:process:line:attribs': { cls: string; domline: any; result: any[]; modifier?: any };
  'editor:register:block:elements': { result: string[] };
  'editor:collect:content:pre': { cc: any; state: any; cls: string };
  'editor:ace:initialized': { editorInfo: any };

  // Chat hooks (mutable context)
  'chat:new:message': any;
  'chat:message:sending': { message: any };

  // Toolbar
  'toolbar:ready': { toolbar: any };
  'toolbar:command:registered': { command: string };
  'toolbar:dropdown:registered': { command: string; dropdown: string };

  // Generic — allows any `custom:*` key for extensibility
  [key: `custom:${string}`]: any;
}

// ---------------------------------------------------------------------------
// Handler types
// ---------------------------------------------------------------------------

/**
 * The payload for a given event key.  When the payload type is `void` the
 * handler receives no arguments; otherwise it receives the payload object.
 */
type EventPayload<
  TMap,
  K extends keyof TMap,
> = TMap[K] extends void ? [] : [data: TMap[K]];

type Handler<T> = T extends void ? () => void : (data: T) => void;
type WildcardHandler = (event: string, data: unknown) => void;

// ---------------------------------------------------------------------------
// EventBus
// ---------------------------------------------------------------------------

export class EventBus<TEvents extends Record<string, any> = EditorEvents> {
  /**
   * When `true`, every `emit` call is logged to `console.debug`.
   */
  static debug = false;

  // Per-event handler lists — lazily created.
  private handlers = new Map<string, Set<Function>>();

  // Wildcard handlers (subscribed via `on('*', …)`).
  private wildcardHandlers = new Set<WildcardHandler>();

  // ------------------------------------------------------------------
  // Subscribe
  // ------------------------------------------------------------------

  /**
   * Subscribe to an event. Returns an unsubscribe function.
   *
   * Pass `'*'` as the event name to subscribe to *all* events (wildcard).
   * The handler then receives `(eventName, data)`.
   */
  on<K extends string & keyof TEvents>(
    event: K,
    handler: Handler<TEvents[K]>,
  ): () => void;
  on(event: '*', handler: WildcardHandler): () => void;
  on(event: string, handler: Function): () => void {
    if (event === '*') {
      this.wildcardHandlers.add(handler as WildcardHandler);
      return () => {
        this.wildcardHandlers.delete(handler as WildcardHandler);
      };
    }

    let set = this.handlers.get(event);
    if (set === undefined) {
      set = new Set();
      this.handlers.set(event, set);
    }
    set.add(handler);

    return () => {
      set!.delete(handler);
      if (set!.size === 0) {
        this.handlers.delete(event);
      }
    };
  }

  // ------------------------------------------------------------------
  // Subscribe once
  // ------------------------------------------------------------------

  /**
   * Subscribe for a single emission, then automatically unsubscribe.
   * Returns an unsubscribe function in case you want to cancel early.
   */
  once<K extends string & keyof TEvents>(
    event: K,
    handler: Handler<TEvents[K]>,
  ): () => void;
  once(event: '*', handler: WildcardHandler): () => void;
  once(event: string, handler: Function): () => void {
    const wrapper = (...args: any[]) => {
      unsub();
      (handler as Function).apply(undefined, args);
    };
    // Tag the wrapper so `off` can still match the original handler.
    (wrapper as any).__original = handler;
    const unsub = this.on(event as any, wrapper as any);
    return unsub;
  }

  // ------------------------------------------------------------------
  // Unsubscribe
  // ------------------------------------------------------------------

  /**
   * Explicitly remove a handler. If the handler was registered via `once`,
   * it can still be removed via `off` using the *original* handler reference.
   */
  off<K extends string & keyof TEvents>(
    event: K,
    handler: Handler<TEvents[K]>,
  ): void;
  off(event: '*', handler: WildcardHandler): void;
  off(event: string, handler: Function): void {
    if (event === '*') {
      this.wildcardHandlers.delete(handler as WildcardHandler);
      return;
    }

    const set = this.handlers.get(event);
    if (set === undefined) return;

    // Direct match.
    if (set.has(handler)) {
      set.delete(handler);
    } else {
      // Search for a `once` wrapper that wraps the given handler.
      for (const fn of set) {
        if ((fn as any).__original === handler) {
          set.delete(fn);
          break;
        }
      }
    }

    if (set.size === 0) {
      this.handlers.delete(event);
    }
  }

  // ------------------------------------------------------------------
  // Emit
  // ------------------------------------------------------------------

  /**
   * Synchronously invoke all handlers for the given event.
   */
  emit<K extends string & keyof TEvents>(
    event: K,
    ...args: EventPayload<TEvents, K>
  ): void;
  emit(event: string, data?: unknown): void {
    if (EventBus.debug) {
      console.debug(`[EventBus] ${event}`, data);
    }

    const set = this.handlers.get(event);
    if (set !== undefined) {
      // Iterate over a snapshot so that handlers that call `off` during
      // emission don't cause issues.
      for (const fn of Array.from(set)) {
        try {
          (fn as Function)(data);
        } catch (err) {
          console.error(`[EventBus] Error in handler for "${event}":`, err);
        }
      }
    }

    // Wildcard handlers always receive the event name as the first argument.
    if (this.wildcardHandlers.size > 0) {
      for (const fn of Array.from(this.wildcardHandlers)) {
        try {
          fn(event, data);
        } catch (err) {
          console.error(`[EventBus] Error in wildcard handler for "${event}":`, err);
        }
      }
    }
  }

  // ------------------------------------------------------------------
  // waitFor
  // ------------------------------------------------------------------

  /**
   * Returns a Promise that resolves the next time `event` is emitted.
   * If `timeout` (ms) is provided and the event is not emitted within that
   * window the Promise rejects with a timeout error.
   */
  waitFor<K extends string & keyof TEvents>(
    event: K,
    timeout?: number,
  ): Promise<TEvents[K]> {
    return new Promise<TEvents[K]>((resolve, reject) => {
      let timer: ReturnType<typeof setTimeout> | undefined;

      const unsub = this.once(event, ((data: TEvents[K]) => {
        if (timer !== undefined) clearTimeout(timer);
        resolve(data);
      }) as any);

      if (timeout !== undefined && timeout > 0) {
        timer = setTimeout(() => {
          unsub();
          reject(new Error(`[EventBus] waitFor("${event}") timed out after ${timeout}ms`));
        }, timeout);
      }
    });
  }

  // ------------------------------------------------------------------
  // Utility
  // ------------------------------------------------------------------

  /**
   * Remove *all* handlers for all events (including wildcards).
   * Primarily useful in tests.
   */
  clear(): void {
    this.handlers.clear();
    this.wildcardHandlers.clear();
  }

  /**
   * Returns the number of handlers currently registered for an event
   * (excluding wildcards).
   */
  listenerCount(event: string): number {
    return this.handlers.get(event)?.size ?? 0;
  }
}

// ---------------------------------------------------------------------------
// Singleton instance for the editor
// ---------------------------------------------------------------------------

// Uses a global reference so that when bundled with etherpad-webcomponents,
// both packages share the exact same EventBus instance. This is critical for
// plugin hooks (editor:attribs:to:classes, editor:process:line:attribs, etc.)
// to work across package boundaries.
const _global = globalThis as any;
if (!_global.__etherpadEditorBus) {
  _global.__etherpadEditorBus = new EventBus<EditorEvents>();
}
export const editorBus: EventBus<EditorEvents> = _global.__etherpadEditorBus;

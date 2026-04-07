/**
 * BaseComponent — A minimal base class for Web Components used in the
 * Etherpad frontend.
 *
 * Provides:
 *  - A pre-attached open ShadowRoot.
 *  - Automatic access to the shared editor EventBus.
 *  - A simple `html` tagged-template helper (no virtual DOM).
 *  - Auto-cleanup of EventBus subscriptions on disconnect.
 *  - Convenience query helpers (`$`, `$$`).
 *
 * Usage:
 *   class MyWidget extends BaseComponent {
 *     static observedAttributes = ['label'];
 *     template() {
 *       return this.html`<span>${this.getAttribute('label')}</span>`;
 *     }
 *   }
 *   customElements.define('my-widget', MyWidget);
 */

import { EventBus, editorBus, type EditorEvents } from './EventBus';

export abstract class BaseComponent extends HTMLElement {
  protected shadow: ShadowRoot;
  protected bus: EventBus<EditorEvents>;

  /** Unsubscribe callbacks accumulated via the `on()` helper. */
  private _unsubs: Array<() => void> = [];

  constructor() {
    super();
    this.shadow = this.attachShadow({ mode: 'open' });
    this.bus = editorBus;
  }

  // ------------------------------------------------------------------
  // Template helpers
  // ------------------------------------------------------------------

  /**
   * Tagged template literal that joins interpolated values into a single
   * HTML string.  Values are **not** auto-escaped — this is intentional
   * so that sub-templates compose naturally.  Always sanitise user input
   * before interpolating.
   */
  protected html(strings: TemplateStringsArray, ...values: any[]): string {
    let result = '';
    for (let i = 0; i < strings.length; i++) {
      result += strings[i];
      if (i < values.length) {
        const v = values[i];
        result += v == null ? '' : String(v);
      }
    }
    return result;
  }

  // ------------------------------------------------------------------
  // Rendering
  // ------------------------------------------------------------------

  /**
   * Subclasses implement this to return the component's HTML.
   */
  protected abstract template(): string;

  /**
   * Re-render the component by replacing the shadow DOM content with the
   * result of `template()`.
   */
  protected render(): void {
    this.shadow.innerHTML = this.template();
  }

  // ------------------------------------------------------------------
  // EventBus helpers
  // ------------------------------------------------------------------

  /**
   * Subscribe to an EventBus event.  The subscription is automatically
   * removed when the element is disconnected from the DOM.
   *
   * Returns an unsubscribe function for early removal.
   */
  protected on<K extends string & keyof EditorEvents>(
    event: K,
    handler: EditorEvents[K] extends void ? () => void : (data: EditorEvents[K]) => void,
  ): () => void {
    const unsub = this.bus.on(event, handler as any);
    this._unsubs.push(unsub);
    return unsub;
  }

  /**
   * Emit an event on the shared bus.
   */
  protected emit<K extends string & keyof EditorEvents>(
    event: K,
    ...args: EditorEvents[K] extends void ? [] : [data: EditorEvents[K]]
  ): void {
    this.bus.emit(event, ...args);
  }

  // ------------------------------------------------------------------
  // Lifecycle
  // ------------------------------------------------------------------

  /**
   * Called when the element is inserted into the DOM.  Performs an initial
   * render.  Subclasses that override this should call `super.connectedCallback()`.
   */
  connectedCallback(): void {
    this.render();
  }

  /**
   * Called when the element is removed from the DOM.  Cleans up all
   * EventBus subscriptions created via the `on()` helper.  Subclasses
   * that override this should call `super.disconnectedCallback()`.
   */
  disconnectedCallback(): void {
    for (const unsub of this._unsubs) {
      try {
        unsub();
      } catch {
        // Ignore — the handler may already have been removed.
      }
    }
    this._unsubs.length = 0;
  }

  // ------------------------------------------------------------------
  // Query helpers
  // ------------------------------------------------------------------

  /**
   * Shorthand for `this.shadow.querySelector`.
   */
  protected $(selector: string): HTMLElement | null {
    return this.shadow.querySelector<HTMLElement>(selector);
  }

  /**
   * Shorthand for `this.shadow.querySelectorAll`, returned as an array.
   */
  protected $$(selector: string): HTMLElement[] {
    return Array.from(this.shadow.querySelectorAll<HTMLElement>(selector));
  }
}

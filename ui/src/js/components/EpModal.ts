/**
 * EpModal — A generic modal/dialog Web Component.
 *
 * Usage:
 *   <ep-modal title="Confirm" open>
 *     <p>Are you sure?</p>
 *     <div slot="actions">
 *       <button data-action="cancel">Cancel</button>
 *       <button data-action="confirm">Confirm</button>
 *     </div>
 *   </ep-modal>
 *
 * Static helpers:
 *   const ok = await EpModal.confirm({ title, message })
 *   const val = await EpModal.prompt({ title, message, placeholder })
 */

const modalStyles = /* css */ `
  :host {
    --ep-modal-bg: #fff;
    --ep-modal-fg: #171717;
    --ep-modal-border: #e5e5e5;
    --ep-modal-overlay: rgba(0, 0, 0, 0.5);
    --ep-modal-radius: 12px;
    --ep-modal-shadow: 0 16px 48px rgba(0, 0, 0, 0.12);
    --ep-modal-font: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto,
      Oxygen, Ubuntu, Cantarell, 'Helvetica Neue', sans-serif;

    position: fixed;
    inset: 0;
    z-index: 10001;
    display: none;
    align-items: center;
    justify-content: center;
    font-family: var(--ep-modal-font);
    font-size: 14px;
    color: var(--ep-modal-fg);
  }

  @media (prefers-color-scheme: dark) {
    :host {
      --ep-modal-bg: #1a1a1a;
      --ep-modal-fg: #e5e5e5;
      --ep-modal-border: #333;
      --ep-modal-overlay: rgba(0, 0, 0, 0.7);
      --ep-modal-shadow: 0 16px 48px rgba(0, 0, 0, 0.4);
    }
  }

  :host([open]) {
    display: flex;
  }

  .overlay {
    position: fixed;
    inset: 0;
    background: var(--ep-modal-overlay);
    animation: ep-modal-fade-in 0.15s ease;
  }

  .dialog {
    position: relative;
    z-index: 1;
    background: var(--ep-modal-bg);
    border-radius: var(--ep-modal-radius);
    box-shadow: var(--ep-modal-shadow);
    max-width: 480px;
    width: calc(100vw - 32px);
    max-height: calc(100vh - 64px);
    overflow: auto;
    animation: ep-modal-scale-in 0.2s ease;
    outline: none;
  }

  .header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 16px 20px 0;
  }

  .title {
    margin: 0;
    font-size: 16px;
    font-weight: 600;
    line-height: 1.4;
  }

  .close-btn {
    background: none;
    border: none;
    cursor: pointer;
    padding: 4px;
    margin: -4px -4px 0 8px;
    color: var(--ep-modal-fg);
    opacity: 0.5;
    transition: opacity 0.15s ease;
    font-size: 18px;
    line-height: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 4px;
  }

  .close-btn:hover,
  .close-btn:focus-visible {
    opacity: 1;
  }

  .close-btn:focus-visible {
    outline: 2px solid currentColor;
    outline-offset: 2px;
  }

  .body {
    padding: 16px 20px;
    line-height: 1.6;
  }

  .actions {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 8px;
    padding: 0 20px 16px;
  }

  .actions ::slotted(button),
  .actions button {
    padding: 8px 16px;
    border-radius: 6px;
    font-size: 14px;
    font-weight: 500;
    cursor: pointer;
    transition: background 0.15s ease, border-color 0.15s ease;
    font-family: inherit;
    line-height: 1;
  }

  /* Built-in button styles for static helpers */
  .btn-cancel {
    background: transparent;
    border: 1px solid var(--ep-modal-border);
    color: var(--ep-modal-fg);
  }

  .btn-cancel:hover {
    background: rgba(128, 128, 128, 0.1);
  }

  .btn-confirm {
    background: #171717;
    border: 1px solid #171717;
    color: #fff;
  }

  .btn-confirm:hover {
    background: #333;
  }

  @media (prefers-color-scheme: dark) {
    .btn-confirm {
      background: #e5e5e5;
      border-color: #e5e5e5;
      color: #000;
    }
    .btn-confirm:hover {
      background: #ccc;
    }
  }

  .prompt-input {
    width: 100%;
    box-sizing: border-box;
    padding: 8px 12px;
    border: 1px solid var(--ep-modal-border);
    border-radius: 6px;
    font-size: 14px;
    font-family: inherit;
    background: var(--ep-modal-bg);
    color: var(--ep-modal-fg);
    margin-top: 12px;
    outline: none;
    transition: border-color 0.15s ease;
  }

  .prompt-input:focus {
    border-color: #666;
  }

  @keyframes ep-modal-fade-in {
    from { opacity: 0; }
    to { opacity: 1; }
  }

  @keyframes ep-modal-scale-in {
    from { opacity: 0; transform: scale(0.96); }
    to { opacity: 1; transform: scale(1); }
  }
`;

interface ConfirmOptions {
  title: string;
  message: string;
  confirmText?: string;
  cancelText?: string;
}

interface PromptOptions {
  title: string;
  message: string;
  placeholder?: string;
}

export class EpModal extends HTMLElement {
  static observedAttributes = ['open', 'title'];

  private _shadow: ShadowRoot;
  private _previousFocus: HTMLElement | null = null;
  private _resolvePromise: ((value: unknown) => void) | null = null;

  /* bound handlers for clean add/remove */
  private _onKeyDown = this._handleKeyDown.bind(this);

  constructor() {
    super();
    this._shadow = this.attachShadow({mode: 'open'});
  }

  /* ── Lifecycle ────────────────────────────────────────────── */

  connectedCallback(): void {
    this._render();
    if (this.hasAttribute('open')) {
      this._onOpen();
    }
  }

  disconnectedCallback(): void {
    document.removeEventListener('keydown', this._onKeyDown);
  }

  attributeChangedCallback(name: string, oldVal: string | null, newVal: string | null): void {
    if (name === 'open') {
      if (newVal != null) {
        this._onOpen();
      } else {
        this._onClose();
      }
    }
    if (name === 'title') {
      const titleEl = this._shadow.querySelector('.title');
      if (titleEl) titleEl.textContent = this.modalTitle;
    }
  }

  /* ── Properties ───────────────────────────────────────────── */

  get modalTitle(): string {
    return this.getAttribute('title') ?? '';
  }

  set modalTitle(v: string) {
    this.setAttribute('title', v);
  }

  get open(): boolean {
    return this.hasAttribute('open');
  }

  set open(v: boolean) {
    if (v) {
      this.setAttribute('open', '');
    } else {
      this.removeAttribute('open');
    }
  }

  /* ── Public ───────────────────────────────────────────────── */

  close(action?: string): void {
    this.dispatchEvent(
      new CustomEvent('ep-modal-close', {bubbles: true, composed: true, detail: {action}}),
    );
    this.open = false;
  }

  /* ── Static helpers ───────────────────────────────────────── */

  static confirm(options: ConfirmOptions): Promise<boolean> {
    return new Promise<boolean>((resolve) => {
      const modal = document.createElement('ep-modal') as EpModal;
      modal.setAttribute('title', options.title);
      modal._resolvePromise = resolve as (v: unknown) => void;

      // Build internal DOM (bypass slots for programmatic usage)
      const bodyContent = document.createElement('p');
      bodyContent.textContent = options.message;
      bodyContent.style.margin = '0';
      modal.appendChild(bodyContent);

      const actionsDiv = document.createElement('div');
      actionsDiv.setAttribute('slot', 'actions');

      const cancelBtn = document.createElement('button');
      cancelBtn.textContent = options.cancelText ?? 'Cancel';
      cancelBtn.setAttribute('data-action', 'cancel');

      const confirmBtn = document.createElement('button');
      confirmBtn.textContent = options.confirmText ?? 'Confirm';
      confirmBtn.setAttribute('data-action', 'confirm');

      actionsDiv.append(cancelBtn, confirmBtn);
      modal.appendChild(actionsDiv);
      document.body.appendChild(modal);

      // Style buttons after they render in the shadow DOM
      requestAnimationFrame(() => {
        const shadowActions = modal._shadow.querySelectorAll('.actions button');
        // No shadow buttons for slotted content; handle via action events
      });

      modal.addEventListener('ep-modal-action', ((e: CustomEvent) => {
        const confirmed = e.detail?.action === 'confirm';
        resolve(confirmed);
        modal.remove();
      }) as EventListener);

      modal.addEventListener('ep-modal-close', () => {
        resolve(false);
        modal.remove();
      });

      modal.open = true;
    });
  }

  static prompt(options: PromptOptions): Promise<string | null> {
    return new Promise<string | null>((resolve) => {
      const modal = document.createElement('ep-modal') as EpModal;
      modal.setAttribute('title', options.title);
      modal._resolvePromise = resolve as (v: unknown) => void;

      const container = document.createElement('div');
      const msg = document.createElement('p');
      msg.textContent = options.message;
      msg.style.margin = '0';

      const input = document.createElement('input');
      input.type = 'text';
      input.className = 'prompt-input';
      input.placeholder = options.placeholder ?? '';

      container.append(msg, input);
      modal.appendChild(container);

      const actionsDiv = document.createElement('div');
      actionsDiv.setAttribute('slot', 'actions');

      const cancelBtn = document.createElement('button');
      cancelBtn.textContent = 'Cancel';
      cancelBtn.setAttribute('data-action', 'cancel');

      const confirmBtn = document.createElement('button');
      confirmBtn.textContent = 'OK';
      confirmBtn.setAttribute('data-action', 'confirm');

      actionsDiv.append(cancelBtn, confirmBtn);
      modal.appendChild(actionsDiv);
      document.body.appendChild(modal);

      modal.addEventListener('ep-modal-action', ((e: CustomEvent) => {
        if (e.detail?.action === 'confirm') {
          // Try shadow DOM input first, then light DOM
          const shadowInput = modal._shadow.querySelector<HTMLInputElement>('.prompt-input');
          const lightInput = modal.querySelector<HTMLInputElement>('input');
          resolve(shadowInput?.value ?? lightInput?.value ?? '');
        } else {
          resolve(null);
        }
        modal.remove();
      }) as EventListener);

      modal.addEventListener('ep-modal-close', () => {
        resolve(null);
        modal.remove();
      });

      modal.open = true;

      // Focus the input once rendered.
      requestAnimationFrame(() => {
        const lightInput = modal.querySelector<HTMLInputElement>('input');
        lightInput?.focus();
      });
    });
  }

  /* ── Private ──────────────────────────────────────────────── */

  private _render(): void {
    this._shadow.innerHTML = `
      <style>${modalStyles}</style>
      <div class="overlay" part="overlay"></div>
      <div class="dialog" role="dialog" aria-modal="true" aria-labelledby="ep-modal-title" tabindex="-1">
        <div class="header">
          <h2 class="title" id="ep-modal-title">${this._escapeHtml(this.modalTitle)}</h2>
          <button class="close-btn" aria-label="Close">&times;</button>
        </div>
        <div class="body">
          <slot></slot>
        </div>
        <div class="actions">
          <slot name="actions"></slot>
        </div>
      </div>
    `;

    this._shadow.querySelector('.overlay')?.addEventListener('click', () => this.close());
    this._shadow.querySelector('.close-btn')?.addEventListener('click', () => this.close());

    // Listen for data-action clicks from slotted content
    this.addEventListener('click', (e: Event) => {
      const target = e.target;
      if (!(target instanceof HTMLElement)) return;
      const action = target.closest<HTMLElement>('[data-action]')?.dataset.action;
      if (action) {
        this.dispatchEvent(
          new CustomEvent('ep-modal-action', {bubbles: true, composed: true, detail: {action}}),
        );
        if (action === 'cancel') this.close(action);
      }
    });
  }

  private _onOpen(): void {
    this._previousFocus = document.activeElement instanceof HTMLElement
      ? document.activeElement
      : null;

    document.addEventListener('keydown', this._onKeyDown);

    // Focus the dialog itself
    requestAnimationFrame(() => {
      const dialog = this._shadow.querySelector<HTMLElement>('.dialog');
      dialog?.focus();
    });
  }

  private _onClose(): void {
    document.removeEventListener('keydown', this._onKeyDown);
    this._previousFocus?.focus();
    this._previousFocus = null;
  }

  private _handleKeyDown(e: KeyboardEvent): void {
    if (e.key === 'Escape') {
      e.preventDefault();
      e.stopPropagation();
      this.close();
      return;
    }

    if (e.key === 'Tab') {
      this._trapFocus(e);
    }
  }

  private _trapFocus(e: KeyboardEvent): void {
    // Collect all focusable elements in both shadow and light DOM.
    const focusable = this._getFocusableElements();
    if (focusable.length === 0) return;

    const first = focusable[0];
    const last = focusable[focusable.length - 1];

    // Determine the currently focused element, checking both shadow and light DOM.
    const active = this._shadow.activeElement ?? document.activeElement;

    if (e.shiftKey) {
      if (active === first || !focusable.includes(active as HTMLElement)) {
        e.preventDefault();
        last.focus();
      }
    } else {
      if (active === last || !focusable.includes(active as HTMLElement)) {
        e.preventDefault();
        first.focus();
      }
    }
  }

  private _getFocusableElements(): HTMLElement[] {
    const selector = 'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])';

    // Shadow DOM focusable elements.
    const shadowEls = Array.from(this._shadow.querySelectorAll<HTMLElement>(selector));
    // Light DOM (slotted) focusable elements.
    const lightEls = Array.from(this.querySelectorAll<HTMLElement>(selector));

    return [...shadowEls, ...lightEls].filter(
      (el) => !el.hasAttribute('disabled') && el.offsetParent !== null,
    );
  }

  private _escapeHtml(text: string): string {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
  }
}

customElements.define('ep-modal', EpModal);

/**
 * EpToastContainer — A lightweight toast notification container.
 *
 * Usage:
 *   <ep-toast-container position="top-right"></ep-toast-container>
 *
 * API:
 *   EpToastContainer.getInstance().addToast({ message, type, duration })
 */

const toastContainerStyles = /* css */ `
  :host {
    --ep-toast-font: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto,
      Oxygen, Ubuntu, Cantarell, 'Helvetica Neue', sans-serif;

    position: fixed;
    z-index: 10002;
    display: flex;
    flex-direction: column;
    gap: 8px;
    pointer-events: none;
    max-width: 380px;
    width: 100%;
    font-family: var(--ep-toast-font);
    font-size: 14px;
  }

  /* Position variants */
  :host([position="top-right"]),
  :host(:not([position])) {
    top: 16px;
    right: 16px;
  }

  :host([position="top-left"]) {
    top: 16px;
    left: 16px;
  }

  :host([position="bottom-right"]) {
    bottom: 16px;
    right: 16px;
    flex-direction: column-reverse;
  }

  :host([position="bottom-left"]) {
    bottom: 16px;
    left: 16px;
    flex-direction: column-reverse;
  }
`;

const toastItemStyles = /* css */ `
  :host {
    --ep-toast-bg: #171717;
    --ep-toast-fg: #fff;
    --ep-toast-radius: 8px;
    --ep-toast-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
    --ep-toast-font: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto,
      Oxygen, Ubuntu, Cantarell, 'Helvetica Neue', sans-serif;

    display: block;
    pointer-events: auto;
    font-family: var(--ep-toast-font);
    font-size: 14px;
    line-height: 1.5;
    opacity: 0;
    transform: translateX(16px);
    transition: opacity 0.25s ease, transform 0.25s ease;
  }

  :host([visible]) {
    opacity: 1;
    transform: translateX(0);
  }

  :host([removing]) {
    opacity: 0;
    transform: translateX(16px);
    transition: opacity 0.2s ease, transform 0.2s ease;
  }

  /* Slide from left for left-positioned containers */
  :host([slide-from="left"]) {
    transform: translateX(-16px);
  }

  :host([slide-from="left"][visible]) {
    transform: translateX(0);
  }

  :host([slide-from="left"][removing]) {
    transform: translateX(-16px);
  }

  @media (prefers-color-scheme: dark) {
    :host {
      --ep-toast-bg: #262626;
      --ep-toast-fg: #e5e5e5;
      --ep-toast-shadow: 0 4px 12px rgba(0, 0, 0, 0.4);
    }
  }

  .toast {
    display: flex;
    align-items: flex-start;
    gap: 10px;
    padding: 12px 16px;
    border-radius: var(--ep-toast-radius);
    box-shadow: var(--ep-toast-shadow);
    background: var(--ep-toast-bg);
    color: var(--ep-toast-fg);
  }

  :host([type="success"]) .toast {
    border-left: 3px solid #22c55e;
  }

  :host([type="error"]) .toast {
    border-left: 3px solid #ef4444;
  }

  :host([type="info"]) .toast {
    border-left: 3px solid #3b82f6;
  }

  .icon {
    flex-shrink: 0;
    width: 16px;
    height: 16px;
    margin-top: 2px;
  }

  .message {
    flex: 1;
    min-width: 0;
    word-wrap: break-word;
  }

  .close {
    flex-shrink: 0;
    background: none;
    border: none;
    color: inherit;
    cursor: pointer;
    padding: 0;
    opacity: 0.5;
    transition: opacity 0.15s ease;
    font-size: 16px;
    line-height: 1;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .close:hover,
  .close:focus-visible {
    opacity: 1;
  }

  .close:focus-visible {
    outline: 2px solid currentColor;
    outline-offset: 2px;
    border-radius: 2px;
  }

  .progress {
    position: absolute;
    bottom: 0;
    left: 0;
    height: 2px;
    background: rgba(255, 255, 255, 0.3);
    border-radius: 0 0 var(--ep-toast-radius) var(--ep-toast-radius);
    transition: width linear;
  }
`;

type ToastPosition = 'top-right' | 'top-left' | 'bottom-right' | 'bottom-left';
type ToastType = 'success' | 'error' | 'info';

interface ToastOptions {
  message: string;
  type?: ToastType;
  duration?: number;
}

const toastIconSvg: Record<string, string> = {
  success: `<svg class="icon" viewBox="0 0 16 16" fill="#22c55e"><path fill-rule="evenodd" d="M13.78 4.22a.75.75 0 010 1.06l-7.25 7.25a.75.75 0 01-1.06 0L2.22 9.28a.75.75 0 011.06-1.06L6 10.94l6.72-6.72a.75.75 0 011.06 0z"/></svg>`,
  error: `<svg class="icon" viewBox="0 0 16 16" fill="#ef4444"><path fill-rule="evenodd" d="M8 15A7 7 0 108 1a7 7 0 000 14zm.75-9.25a.75.75 0 00-1.5 0v4.5a.75.75 0 001.5 0v-4.5zM8 11a1 1 0 100 2 1 1 0 000-2z"/></svg>`,
  info: `<svg class="icon" viewBox="0 0 16 16" fill="#3b82f6"><path fill-rule="evenodd" d="M8 15A7 7 0 108 1a7 7 0 000 14zm.75-9.25a.75.75 0 00-1.5 0v4.5a.75.75 0 001.5 0v-4.5zM8 11a1 1 0 100 2 1 1 0 000-2z"/></svg>`,
};

/* ── Internal Toast Item ───────────────────────────────────── */

class EpToastItem extends HTMLElement {
  private _shadow: ShadowRoot;
  private _dismissTimer: ReturnType<typeof setTimeout> | null = null;

  constructor() {
    super();
    this._shadow = this.attachShadow({mode: 'open'});
  }

  connectedCallback(): void {
    const type = this.getAttribute('type') ?? 'info';
    const message = this.getAttribute('message') ?? '';
    const icon = toastIconSvg[type] ?? toastIconSvg.info;

    this._shadow.innerHTML = `
      <style>${toastItemStyles}</style>
      <div class="toast" role="status" aria-live="polite">
        ${icon}
        <span class="message">${this._escapeHtml(message)}</span>
        <button class="close" aria-label="Dismiss">&times;</button>
      </div>
    `;

    this._shadow.querySelector('.close')?.addEventListener('click', () => this.dismiss());

    // Slide animation entrance.
    requestAnimationFrame(() => this.setAttribute('visible', ''));

    // Auto-dismiss.
    const duration = parseInt(this.getAttribute('duration') ?? '4000', 10);
    if (duration > 0) {
      this._dismissTimer = setTimeout(() => this.dismiss(), duration);
    }
  }

  disconnectedCallback(): void {
    if (this._dismissTimer != null) {
      clearTimeout(this._dismissTimer);
      this._dismissTimer = null;
    }
  }

  dismiss(): void {
    if (this._dismissTimer != null) {
      clearTimeout(this._dismissTimer);
      this._dismissTimer = null;
    }
    this.removeAttribute('visible');
    this.setAttribute('removing', '');

    const cleanup = () => {
      this.removeEventListener('transitionend', cleanup);
      this.remove();
    };
    this.addEventListener('transitionend', cleanup);
    setTimeout(cleanup, 300);
  }

  private _escapeHtml(text: string): string {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
  }
}

// Register the toast item element.
customElements.define('ep-toast-item', EpToastItem);

/* ── Toast Container ───────────────────────────────────────── */

const MAX_VISIBLE = 5;

export class EpToastContainer extends HTMLElement {
  static observedAttributes = ['position'];

  private _shadow: ShadowRoot;

  private static _instance: EpToastContainer | null = null;

  constructor() {
    super();
    this._shadow = this.attachShadow({mode: 'open'});
  }

  /* ── Lifecycle ────────────────────────────────────────────── */

  connectedCallback(): void {
    this._shadow.innerHTML = `
      <style>${toastContainerStyles}</style>
      <slot></slot>
    `;

    // Register as singleton instance.
    EpToastContainer._instance = this;
  }

  disconnectedCallback(): void {
    if (EpToastContainer._instance === this) {
      EpToastContainer._instance = null;
    }
  }

  /* ── Properties ───────────────────────────────────────────── */

  get position(): ToastPosition {
    const val = this.getAttribute('position');
    if (val === 'top-left' || val === 'bottom-right' || val === 'bottom-left') return val;
    return 'top-right';
  }

  set position(v: ToastPosition) {
    this.setAttribute('position', v);
  }

  /* ── Static accessor ──────────────────────────────────────── */

  /**
   * Returns the singleton toast container, creating one if it does not exist.
   */
  static getInstance(): EpToastContainer {
    if (EpToastContainer._instance) return EpToastContainer._instance;

    const container = document.createElement('ep-toast-container') as EpToastContainer;
    container.setAttribute('position', 'top-right');
    document.body.appendChild(container);
    return container;
  }

  /* ── Public API ───────────────────────────────────────────── */

  addToast(options: ToastOptions): EpToastItem {
    // Enforce max visible limit — remove oldest if necessary.
    const existing = this.querySelectorAll('ep-toast-item');
    if (existing.length >= MAX_VISIBLE) {
      const oldest = existing[0] as EpToastItem;
      oldest.dismiss();
    }

    const toast = document.createElement('ep-toast-item') as EpToastItem;
    toast.setAttribute('message', options.message);
    toast.setAttribute('type', options.type ?? 'info');
    toast.setAttribute('duration', String(options.duration ?? 4000));

    // Set slide direction based on container position.
    const isLeft = this.position.includes('left');
    if (isLeft) {
      toast.setAttribute('slide-from', 'left');
    }

    this.appendChild(toast);
    return toast;
  }
}

customElements.define('ep-toast-container', EpToastContainer);

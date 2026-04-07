/**
 * EpNotification — Web Component replacement for the gritter/notification system.
 *
 * Usage:
 *   <ep-notification position="top" duration="3000" type="success">
 *     Message text here
 *   </ep-notification>
 *
 * Static helpers:
 *   EpNotification.show({ text, type, duration, position })
 *   EpNotification.success(text, duration?)
 *   EpNotification.error(text, duration?)
 */

const notificationStyles = /* css */ `
  :host {
    --ep-bg-success: #000;
    --ep-bg-error: #dc2626;
    --ep-bg-info: #171717;
    --ep-fg: #fff;
    --ep-radius: 8px;
    --ep-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
    --ep-font: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen,
      Ubuntu, Cantarell, 'Helvetica Neue', sans-serif;

    display: block;
    pointer-events: auto;
    font-family: var(--ep-font);
    font-size: 14px;
    line-height: 1.5;
    max-width: 420px;
    width: 100%;
    box-sizing: border-box;
    opacity: 0;
    transform: translateY(calc(var(--ep-slide-dir, -1) * 12px));
    transition: opacity 0.25s ease, transform 0.25s ease;
  }

  :host([visible]) {
    opacity: 1;
    transform: translateY(0);
  }

  :host([removing]) {
    opacity: 0;
    transform: translateY(calc(var(--ep-slide-dir, -1) * 12px));
    transition: opacity 0.2s ease, transform 0.2s ease;
  }

  @media (prefers-color-scheme: light) {
    :host {
      --ep-bg-success: #000;
      --ep-bg-error: #dc2626;
      --ep-bg-info: #171717;
      --ep-fg: #fff;
    }
  }

  @media (prefers-color-scheme: dark) {
    :host {
      --ep-bg-success: #22c55e;
      --ep-bg-error: #ef4444;
      --ep-bg-info: #e5e5e5;
      --ep-fg: #000;
    }
  }

  .notification {
    display: flex;
    align-items: flex-start;
    gap: 10px;
    padding: 12px 16px;
    border-radius: var(--ep-radius);
    box-shadow: var(--ep-shadow);
    color: var(--ep-fg);
    background: var(--ep-bg-info);
  }

  :host([type="success"]) .notification {
    background: var(--ep-bg-success);
  }

  :host([type="error"]) .notification {
    background: var(--ep-bg-error);
  }

  .icon {
    flex-shrink: 0;
    width: 18px;
    height: 18px;
    margin-top: 1px;
  }

  .body {
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
    margin: 0;
    opacity: 0.6;
    transition: opacity 0.15s ease;
    line-height: 1;
    font-size: 18px;
    width: 20px;
    height: 20px;
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
`;

const iconSvg: Record<string, string> = {
  success: `<svg class="icon" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd"/></svg>`,
  error: `<svg class="icon" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm1-11a1 1 0 10-2 0v4a1 1 0 102 0V7zm-1 8a1 1 0 100-2 1 1 0 000 2z" clip-rule="evenodd"/></svg>`,
  info: `<svg class="icon" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7-4a1 1 0 11-2 0 1 1 0 012 0zM9 9a1 1 0 000 2v3a1 1 0 001 1h1a1 1 0 100-2v-3a1 1 0 00-1-1H9z" clip-rule="evenodd"/></svg>`,
};

type NotificationPosition = 'top' | 'bottom';
type NotificationType = 'success' | 'error' | 'info';

interface NotificationOptions {
  text: string;
  type?: NotificationType;
  duration?: number;
  position?: NotificationPosition;
}

/**
 * Container element that manages stacking of notifications at a given position.
 */
const ensureContainer = (position: NotificationPosition): HTMLElement => {
  const id = `ep-notification-container-${position}`;
  let container = document.getElementById(id);
  if (container) return container;

  container = document.createElement('div');
  container.id = id;
  Object.assign(container.style, {
    position: 'fixed',
    [position === 'top' ? 'top' : 'bottom']: '16px',
    right: '16px',
    zIndex: '10000',
    display: 'flex',
    flexDirection: position === 'top' ? 'column' : 'column-reverse',
    gap: '8px',
    pointerEvents: 'none',
    maxWidth: '100vw',
    width: '420px',
  } as CSSStyleDeclaration as Record<string, string>);
  document.body.appendChild(container);
  return container;
};

export class EpNotification extends HTMLElement {
  static observedAttributes = ['position', 'duration', 'type'];

  private _dismissTimer: ReturnType<typeof setTimeout> | null = null;
  private _shadow: ShadowRoot;

  constructor() {
    super();
    this._shadow = this.attachShadow({mode: 'open'});
  }

  /* ── Lifecycle ────────────────────────────────────────────── */

  connectedCallback(): void {
    this._render();
    this._startAutoClose();

    // Slide direction: -1 for top (slide down from above), +1 for bottom (slide up from below)
    const dir = this.position === 'bottom' ? '1' : '-1';
    this.style.setProperty('--ep-slide-dir', dir);

    // Trigger enter animation on the next frame.
    requestAnimationFrame(() => this.setAttribute('visible', ''));
  }

  disconnectedCallback(): void {
    this._clearTimer();
  }

  attributeChangedCallback(name: string, _old: string | null, _next: string | null): void {
    if (name === 'duration') {
      this._clearTimer();
      this._startAutoClose();
    }
    if (name === 'type') {
      this._render();
    }
  }

  /* ── Properties ───────────────────────────────────────────── */

  get position(): NotificationPosition {
    const val = this.getAttribute('position');
    return val === 'bottom' ? 'bottom' : 'top';
  }

  set position(v: NotificationPosition) {
    this.setAttribute('position', v);
  }

  get duration(): number {
    const val = this.getAttribute('duration');
    const parsed = val != null ? parseInt(val, 10) : NaN;
    return Number.isFinite(parsed) ? parsed : 3000;
  }

  set duration(v: number) {
    this.setAttribute('duration', String(v));
  }

  get type(): NotificationType {
    const val = this.getAttribute('type');
    if (val === 'success' || val === 'error') return val;
    return 'info';
  }

  set type(v: NotificationType) {
    this.setAttribute('type', v);
  }

  /* ── Public ───────────────────────────────────────────────── */

  dismiss(): void {
    this._clearTimer();
    this.removeAttribute('visible');
    this.setAttribute('removing', '');

    const onDone = () => {
      this.removeEventListener('transitionend', onDone);
      this.remove();
      this._cleanupEmptyContainer();
    };
    this.addEventListener('transitionend', onDone);

    // Safety: remove even if transitionend never fires.
    setTimeout(onDone, 350);
  }

  /* ── Static helpers ───────────────────────────────────────── */

  static show(options: NotificationOptions): EpNotification {
    const el = document.createElement('ep-notification') as EpNotification;
    el.type = options.type ?? 'info';
    el.duration = options.duration ?? 3000;
    el.position = options.position ?? 'top';
    el.textContent = options.text;

    const container = ensureContainer(el.position);
    container.appendChild(el);
    return el;
  }

  static success(text: string, duration?: number): EpNotification {
    return EpNotification.show({text, type: 'success', duration});
  }

  static error(text: string, duration?: number): EpNotification {
    return EpNotification.show({text, type: 'error', duration: duration ?? 5000});
  }

  /* ── Private ──────────────────────────────────────────────── */

  private _render(): void {
    const type = this.type;
    const icon = iconSvg[type] ?? iconSvg.info;

    this._shadow.innerHTML = `
      <style>${notificationStyles}</style>
      <div class="notification" role="alert" aria-live="assertive">
        ${icon}
        <div class="body"><slot></slot></div>
        <button class="close" aria-label="Close notification">&times;</button>
      </div>
    `;

    this._shadow.querySelector('.close')?.addEventListener('click', () => this.dismiss());
  }

  private _startAutoClose(): void {
    const d = this.duration;
    if (d > 0) {
      this._dismissTimer = setTimeout(() => this.dismiss(), d);
    }
  }

  private _clearTimer(): void {
    if (this._dismissTimer != null) {
      clearTimeout(this._dismissTimer);
      this._dismissTimer = null;
    }
  }

  private _cleanupEmptyContainer(): void {
    for (const pos of ['top', 'bottom'] as const) {
      const c = document.getElementById(`ep-notification-container-${pos}`);
      if (c && c.children.length === 0) c.remove();
    }
  }
}

customElements.define('ep-notification', EpNotification);

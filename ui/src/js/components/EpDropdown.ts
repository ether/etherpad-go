/**
 * EpDropdown + EpDropdownItem — Dropdown menu Web Components for toolbar selects.
 *
 * Usage:
 *   <ep-dropdown trigger="click" align="left">
 *     <button slot="trigger">Font Size</button>
 *     <div slot="content">
 *       <ep-dropdown-item value="12">12px</ep-dropdown-item>
 *       <ep-dropdown-item value="14">14px</ep-dropdown-item>
 *     </div>
 *   </ep-dropdown>
 */

/* ── Dropdown Item ─────────────────────────────────────────── */

const dropdownItemStyles = /* css */ `
  :host {
    --ep-item-fg: #485365;
    --ep-item-fg: var(--text-color, #485365);
    --ep-item-hover-bg: #f2f3f4;
    --ep-item-hover-bg: var(--bg-soft-color, #f2f3f4);
    --ep-item-font: var(--main-font-family, Quicksand, Cantarell, "Open Sans", "Helvetica Neue", sans-serif);
    --ep-item-focus: #64d29b;
    --ep-item-focus: var(--primary-color, #64d29b);

    display: block;
    font-family: var(--ep-item-font);
    font-size: 14px;
  }

  .item {
    display: flex;
    align-items: center;
    width: 100%;
    padding: 8px 12px;
    border: none;
    background: none;
    color: var(--ep-item-fg);
    cursor: pointer;
    font: inherit;
    text-align: left;
    white-space: nowrap;
    transition: background 0.1s ease;
    outline: none;
    box-sizing: border-box;
  }

  .item:hover,
  .item[aria-selected="true"],
  :host([focused]) .item {
    background: var(--ep-item-hover-bg);
  }

  .item:focus-visible {
    background: var(--ep-item-hover-bg);
    outline: 2px solid var(--ep-item-focus);
    outline-offset: -2px;
  }

  :host([disabled]) .item {
    opacity: 0.4;
    cursor: not-allowed;
  }
`;

export class EpDropdownItem extends HTMLElement {
  static observedAttributes = ['value', 'disabled'];

  private _shadow: ShadowRoot;

  constructor() {
    super();
    this._shadow = this.attachShadow({mode: 'open'});
  }

  connectedCallback(): void {
    this._render();
    this.setAttribute('role', 'option');
  }

  attributeChangedCallback(): void {
    this._updateState();
  }

  get value(): string {
    return this.getAttribute('value') ?? '';
  }

  set value(v: string) {
    this.setAttribute('value', v);
  }

  get disabled(): boolean {
    return this.hasAttribute('disabled');
  }

  set disabled(v: boolean) {
    if (v) {
      this.setAttribute('disabled', '');
    } else {
      this.removeAttribute('disabled');
    }
  }

  /** Called by the parent dropdown to visually mark this item as focused. */
  setFocused(focused: boolean): void {
    if (focused) {
      this.setAttribute('focused', '');
    } else {
      this.removeAttribute('focused');
    }
  }

  private _render(): void {
    this._shadow.innerHTML = `
      <style>${dropdownItemStyles}</style>
      <div class="item" role="presentation">
        <slot></slot>
      </div>
    `;
  }

  private _updateState(): void {
    const itemEl = this._shadow.querySelector('.item');
    if (itemEl && this.disabled) {
      itemEl.setAttribute('aria-disabled', 'true');
    } else if (itemEl) {
      itemEl.removeAttribute('aria-disabled');
    }
  }
}

customElements.define('ep-dropdown-item', EpDropdownItem);

/* ── Dropdown Container ────────────────────────────────────── */

const dropdownStyles = /* css */ `
  :host {
    --ep-dd-bg: #fff;
    --ep-dd-bg: var(--bg-color, #fff);
    --ep-dd-border: #d2d2d2;
    --ep-dd-border: var(--border-color, #d2d2d2);
    --ep-dd-radius: 8px;
    --ep-dd-shadow: 0 4px 16px rgba(0, 0, 0, 0.12);
    --ep-dd-font: var(--main-font-family, Quicksand, Cantarell, "Open Sans", "Helvetica Neue", sans-serif);

    display: inline-block;
    position: relative;
    font-family: var(--ep-dd-font);
    font-size: 14px;
  }

  .trigger-wrapper {
    display: inline-flex;
  }

  .content-wrapper {
    display: none;
    position: fixed;
    top: 0;
    left: 0;
    min-width: 140px;
    max-height: 280px;
    overflow-y: auto;
    background: var(--ep-dd-bg);
    border: 1px solid var(--ep-dd-border);
    border-radius: var(--ep-dd-radius);
    box-shadow: var(--ep-dd-shadow);
    z-index: 9999;
    padding: 4px 0;
    opacity: 0;
    transform: translateY(-4px);
    transition: opacity 0.15s ease, transform 0.15s ease;
  }

  :host([open]) .content-wrapper {
    display: block;
  }

  :host([open]) .content-wrapper.visible {
    opacity: 1;
    transform: translateY(0);
  }

  :host([align="right"]) .content-wrapper {
    right: 0;
    left: auto;
  }

  :host([align="left"]) .content-wrapper,
  :host(:not([align])) .content-wrapper {
    left: 0;
    right: auto;
  }
`;

type TriggerMode = 'click' | 'hover';

export class EpDropdown extends HTMLElement {
  static observedAttributes = ['trigger', 'align', 'open'];

  private _shadow: ShadowRoot;
  private _focusIndex = -1;
  private _hoverCloseTimer: ReturnType<typeof setTimeout> | null = null;

  /* Bound handlers for clean add/remove */
  private _onDocClick = this._handleOutsideClick.bind(this);
  private _onDocKeydown = this._handleDocKeydown.bind(this);
  private _onViewportChange = this._positionContent.bind(this);

  constructor() {
    super();
    this._shadow = this.attachShadow({mode: 'open'});
  }

  /* ── Lifecycle ────────────────────────────────────────────── */

  connectedCallback(): void {
    this._render();
    this._attachTriggerEvents();

    // Listen for item clicks from slotted content.
    this.addEventListener('click', (e: Event) => {
      const target = e.target;
      if (target instanceof EpDropdownItem && !target.disabled) {
        this._selectItem(target);
      }
    });
  }

  disconnectedCallback(): void {
    document.removeEventListener('click', this._onDocClick, true);
    document.removeEventListener('keydown', this._onDocKeydown);
    window.removeEventListener('resize', this._onViewportChange);
    window.removeEventListener('scroll', this._onViewportChange, true);
    if (this._hoverCloseTimer != null) clearTimeout(this._hoverCloseTimer);
  }

  attributeChangedCallback(name: string, _old: string | null, _next: string | null): void {
    if (name === 'open') {
      if (this.isOpen) {
        this._onOpened();
      } else {
        this._onClosed();
      }
    }
  }

  /* ── Properties ───────────────────────────────────────────── */

  get triggerMode(): TriggerMode {
    return this.getAttribute('trigger') === 'hover' ? 'hover' : 'click';
  }

  set triggerMode(v: TriggerMode) {
    this.setAttribute('trigger', v);
  }

  get align(): 'left' | 'right' {
    return this.getAttribute('align') === 'right' ? 'right' : 'left';
  }

  set align(v: 'left' | 'right') {
    this.setAttribute('align', v);
  }

  get isOpen(): boolean {
    return this.hasAttribute('open');
  }

  set isOpen(v: boolean) {
    if (v) {
      this.setAttribute('open', '');
    } else {
      this.removeAttribute('open');
    }
  }

  /* ── Public ───────────────────────────────────────────────── */

  toggle(): void {
    this.isOpen = !this.isOpen;
  }

  open(): void {
    this.isOpen = true;
  }

  close(): void {
    this.isOpen = false;
  }

  /* ── Private ──────────────────────────────────────────────── */

  private _render(): void {
    this._shadow.innerHTML = `
      <style>${dropdownStyles}</style>
      <div class="trigger-wrapper" part="trigger">
        <slot name="trigger"></slot>
      </div>
      <div class="content-wrapper" role="listbox" part="content">
        <slot name="content"></slot>
      </div>
    `;
  }

  private _attachTriggerEvents(): void {
    const triggerSlot = this._shadow.querySelector('slot[name="trigger"]') as HTMLSlotElement;
    const contentWrapper = this._shadow.querySelector('.content-wrapper') as HTMLElement | null;

    const preserveEditorSelection = (e: MouseEvent) => {
      // Toolbar clicks should not steal focus from the ACE iframe before the command runs.
      e.preventDefault();
    };

    triggerSlot?.addEventListener('mousedown', preserveEditorSelection);
    contentWrapper?.addEventListener('mousedown', preserveEditorSelection);

    if (this.triggerMode === 'click') {
      triggerSlot?.addEventListener('click', (e: Event) => {
        e.stopPropagation();
        this.toggle();
      });
    } else {
      // Hover mode.
      this.addEventListener('mouseenter', () => {
        if (this._hoverCloseTimer != null) {
          clearTimeout(this._hoverCloseTimer);
          this._hoverCloseTimer = null;
        }
        this.open();
      });

      this.addEventListener('mouseleave', () => {
        this._hoverCloseTimer = setTimeout(() => this.close(), 200);
      });

      // Also allow click to toggle in hover mode.
      triggerSlot?.addEventListener('click', (e: Event) => {
        e.stopPropagation();
        this.toggle();
      });
    }
  }

  private _onOpened(): void {
    this._focusIndex = -1;
    this._clearItemFocus();
    this._positionContent();

    // Animate in.
    requestAnimationFrame(() => {
      const content = this._shadow.querySelector('.content-wrapper');
      this._positionContent();
      content?.classList.add('visible');
    });

    document.addEventListener('click', this._onDocClick, true);
    document.addEventListener('keydown', this._onDocKeydown);
    window.addEventListener('resize', this._onViewportChange);
    window.addEventListener('scroll', this._onViewportChange, true);
  }

  private _onClosed(): void {
    const content = this._shadow.querySelector('.content-wrapper');
    content?.classList.remove('visible');
    this._focusIndex = -1;
    this._clearItemFocus();

    document.removeEventListener('click', this._onDocClick, true);
    document.removeEventListener('keydown', this._onDocKeydown);
    window.removeEventListener('resize', this._onViewportChange);
    window.removeEventListener('scroll', this._onViewportChange, true);
  }

  private _positionContent(): void {
    const content = this._shadow.querySelector('.content-wrapper') as HTMLElement | null;
    if (!content || !this.isOpen) return;

    const hostRect = this.getBoundingClientRect();
    const viewportPadding = 8;
    const gap = 4;

    content.style.minWidth = `${Math.max(140, Math.ceil(hostRect.width))}px`;
    content.style.maxWidth = `${Math.max(140, window.innerWidth - (viewportPadding * 2))}px`;

    const contentRect = content.getBoundingClientRect();
    const width = Math.max(contentRect.width, Math.ceil(hostRect.width), 140);
    const height = contentRect.height;

    let left = this.align === 'right' ? hostRect.right - width : hostRect.left;
    left = Math.min(Math.max(viewportPadding, left), Math.max(viewportPadding, window.innerWidth - width - viewportPadding));

    let top = hostRect.bottom + gap;
    if (top + height > window.innerHeight - viewportPadding) {
      top = Math.max(viewportPadding, hostRect.top - height - gap);
    }

    content.style.left = `${Math.round(left)}px`;
    content.style.top = `${Math.round(top)}px`;
  }

  private _handleOutsideClick(e: Event): void {
    if (!this.isOpen) return;
    const path = e.composedPath();
    if (!path.includes(this)) {
      this.close();
    }
  }

  private _handleDocKeydown(e: KeyboardEvent): void {
    if (!this.isOpen) return;

    switch (e.key) {
      case 'Escape':
        e.preventDefault();
        this.close();
        // Return focus to trigger.
        const triggerEl = this.querySelector<HTMLElement>('[slot="trigger"]');
        triggerEl?.focus();
        break;

      case 'ArrowDown':
        e.preventDefault();
        this._moveFocus(1);
        break;

      case 'ArrowUp':
        e.preventDefault();
        this._moveFocus(-1);
        break;

      case 'Home':
        e.preventDefault();
        this._setFocusIndex(0);
        break;

      case 'End': {
        e.preventDefault();
        const items = this._getItems();
        this._setFocusIndex(items.length - 1);
        break;
      }

      case 'Enter':
      case ' ': {
        e.preventDefault();
        const items = this._getItems();
        if (this._focusIndex >= 0 && this._focusIndex < items.length) {
          const item = items[this._focusIndex];
          if (!item.disabled) this._selectItem(item);
        }
        break;
      }
    }
  }

  private _getItems(): EpDropdownItem[] {
    return Array.from(this.querySelectorAll<EpDropdownItem>('ep-dropdown-item'));
  }

  private _moveFocus(direction: number): void {
    const items = this._getItems();
    if (items.length === 0) return;

    let nextIdx = this._focusIndex + direction;
    // Wrap around.
    if (nextIdx < 0) nextIdx = items.length - 1;
    if (nextIdx >= items.length) nextIdx = 0;

    // Skip disabled items.
    const startIdx = nextIdx;
    while (items[nextIdx].disabled) {
      nextIdx += direction;
      if (nextIdx < 0) nextIdx = items.length - 1;
      if (nextIdx >= items.length) nextIdx = 0;
      if (nextIdx === startIdx) return; // All disabled.
    }

    this._setFocusIndex(nextIdx);
  }

  private _setFocusIndex(index: number): void {
    const items = this._getItems();
    this._clearItemFocus();
    this._focusIndex = index;
    if (index >= 0 && index < items.length) {
      items[index].setFocused(true);
      items[index].scrollIntoView({block: 'nearest'});
    }
  }

  private _clearItemFocus(): void {
    for (const item of this._getItems()) {
      item.setFocused(false);
    }
  }

  private _selectItem(item: EpDropdownItem): void {
    this.dispatchEvent(
      new CustomEvent('ep-dropdown-select', {
        bubbles: true,
        composed: true,
        detail: {value: item.value},
      }),
    );
    this.close();
  }
}

customElements.define('ep-dropdown', EpDropdown);

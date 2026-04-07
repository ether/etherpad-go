/**
 * EpColorPicker — A color picker Web Component for plugins like ep_font_color.
 *
 * Usage:
 *   <ep-color-picker colors='["black","red","green","blue","yellow","orange"]'></ep-color-picker>
 */

const colorPickerStyles = /* css */ `
  :host {
    --ep-picker-bg: #fff;
    --ep-picker-border: #e5e5e5;
    --ep-picker-radius: 8px;
    --ep-picker-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
    --ep-picker-font: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto,
      Oxygen, Ubuntu, Cantarell, 'Helvetica Neue', sans-serif;

    display: inline-block;
    font-family: var(--ep-picker-font);
  }

  @media (prefers-color-scheme: dark) {
    :host {
      --ep-picker-bg: #1a1a1a;
      --ep-picker-border: #333;
      --ep-picker-shadow: 0 2px 8px rgba(0, 0, 0, 0.3);
    }
  }

  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, 32px);
    gap: 6px;
    padding: 8px;
    background: var(--ep-picker-bg);
    border: 1px solid var(--ep-picker-border);
    border-radius: var(--ep-picker-radius);
    box-shadow: var(--ep-picker-shadow);
  }

  .swatch {
    width: 32px;
    height: 32px;
    border-radius: 6px;
    border: 2px solid transparent;
    cursor: pointer;
    position: relative;
    transition: border-color 0.15s ease, transform 0.1s ease;
    padding: 0;
    background: none;
    outline: none;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .swatch:hover {
    transform: scale(1.1);
  }

  .swatch:focus-visible {
    outline: 2px solid #3b82f6;
    outline-offset: 2px;
  }

  .swatch[aria-selected="true"] {
    border-color: #3b82f6;
  }

  .swatch-inner {
    width: 100%;
    height: 100%;
    border-radius: 4px;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .check {
    display: none;
    width: 14px;
    height: 14px;
  }

  .swatch[aria-selected="true"] .check {
    display: block;
  }

  /* Screen reader only label */
  .sr-only {
    position: absolute;
    width: 1px;
    height: 1px;
    padding: 0;
    margin: -1px;
    overflow: hidden;
    clip: rect(0, 0, 0, 0);
    white-space: nowrap;
    border: 0;
  }
`;

/**
 * Determine whether a color value is light so we can pick a contrasting
 * check-mark color. Uses a simple canvas trick or falls back to heuristics.
 */
const isLightColor = (color: string): boolean => {
  // Use an off-screen canvas to resolve any CSS color string to RGBA.
  try {
    const ctx = document.createElement('canvas').getContext('2d');
    if (!ctx) return false;
    ctx.fillStyle = color;
    // fillStyle normalises to #rrggbb or rgba(...)
    const resolved = ctx.fillStyle;
    let r = 0, g = 0, b = 0;
    if (resolved.startsWith('#')) {
      const hex = resolved.slice(1);
      r = parseInt(hex.slice(0, 2), 16);
      g = parseInt(hex.slice(2, 4), 16);
      b = parseInt(hex.slice(4, 6), 16);
    } else {
      const match = resolved.match(/\d+/g);
      if (match) {
        r = parseInt(match[0], 10);
        g = parseInt(match[1], 10);
        b = parseInt(match[2], 10);
      }
    }
    // Relative luminance approximation.
    const luminance = (0.299 * r + 0.587 * g + 0.114 * b) / 255;
    return luminance > 0.6;
  } catch {
    return false;
  }
};

const DEFAULT_COLORS = [
  'black', 'red', 'green', 'blue', 'yellow', 'orange',
  'purple', 'pink', 'brown', 'gray', 'white', 'cyan',
];

export class EpColorPicker extends HTMLElement {
  static observedAttributes = ['colors', 'value'];

  private _shadow: ShadowRoot;
  private _value: string = '';

  constructor() {
    super();
    this._shadow = this.attachShadow({mode: 'open'});
  }

  /* ── Lifecycle ────────────────────────────────────────────── */

  connectedCallback(): void {
    this._render();
  }

  attributeChangedCallback(name: string, _old: string | null, _next: string | null): void {
    if (name === 'colors') {
      this._render();
    }
    if (name === 'value') {
      this._value = _next ?? '';
      this._updateSelection();
    }
  }

  /* ── Properties ───────────────────────────────────────────── */

  get colors(): string[] {
    const attr = this.getAttribute('colors');
    if (!attr) return DEFAULT_COLORS;
    try {
      const parsed = JSON.parse(attr);
      return Array.isArray(parsed) ? parsed : DEFAULT_COLORS;
    } catch {
      return DEFAULT_COLORS;
    }
  }

  set colors(v: string[]) {
    this.setAttribute('colors', JSON.stringify(v));
  }

  get value(): string {
    return this._value;
  }

  set value(v: string) {
    this._value = v;
    this.setAttribute('value', v);
    this._updateSelection();
  }

  /* ── Private ──────────────────────────────────────────────── */

  private _render(): void {
    const colors = this.colors;

    this._shadow.innerHTML = `
      <style>${colorPickerStyles}</style>
      <div class="grid" role="listbox" aria-label="Color picker">
        ${colors.map((color, i) => this._renderSwatch(color, i)).join('')}
      </div>
    `;

    // Attach event listeners.
    const swatches = this._shadow.querySelectorAll<HTMLElement>('.swatch');
    swatches.forEach((swatch) => {
      swatch.addEventListener('click', () => {
        const color = swatch.dataset.color!;
        const index = parseInt(swatch.dataset.index!, 10);
        this._selectColor(color, index);
      });

      swatch.addEventListener('keydown', (e: KeyboardEvent) => {
        this._handleSwatchKeydown(e, swatches);
      });
    });
  }

  private _renderSwatch(color: string, index: number): string {
    const isSelected = this._value === color;
    const light = isLightColor(color);
    const checkColor = light ? '#000' : '#fff';

    return `
      <button class="swatch"
              role="option"
              aria-selected="${isSelected}"
              aria-label="${color}"
              data-color="${this._escapeAttr(color)}"
              data-index="${index}"
              tabindex="${index === 0 ? '0' : '-1'}">
        <div class="swatch-inner" style="background:${this._escapeAttr(color)}">
          <svg class="check" viewBox="0 0 14 14" fill="${checkColor}">
            <path d="M11.5 3.5L5.5 10.5L2.5 7.5" stroke="${checkColor}" stroke-width="2"
                  fill="none" stroke-linecap="round" stroke-linejoin="round"/>
          </svg>
        </div>
        <span class="sr-only">${this._escapeHtml(color)}</span>
      </button>
    `;
  }

  private _selectColor(color: string, index: number): void {
    this._value = color;
    this.setAttribute('value', color);
    this._updateSelection();

    this.dispatchEvent(
      new CustomEvent('ep-color-select', {
        bubbles: true,
        composed: true,
        detail: {color, index},
      }),
    );
  }

  private _updateSelection(): void {
    const swatches = this._shadow.querySelectorAll<HTMLElement>('.swatch');
    swatches.forEach((swatch) => {
      const isSelected = swatch.dataset.color === this._value;
      swatch.setAttribute('aria-selected', String(isSelected));
    });
  }

  private _handleSwatchKeydown(e: KeyboardEvent, swatches: NodeListOf<HTMLElement>): void {
    const current = e.currentTarget as HTMLElement;
    const items = Array.from(swatches);
    const idx = items.indexOf(current);
    let nextIdx = -1;

    switch (e.key) {
      case 'ArrowRight':
      case 'ArrowDown':
        nextIdx = (idx + 1) % items.length;
        break;
      case 'ArrowLeft':
      case 'ArrowUp':
        nextIdx = (idx - 1 + items.length) % items.length;
        break;
      case 'Home':
        nextIdx = 0;
        break;
      case 'End':
        nextIdx = items.length - 1;
        break;
      case 'Enter':
      case ' ':
        e.preventDefault();
        current.click();
        return;
      default:
        return;
    }

    e.preventDefault();
    items[idx].setAttribute('tabindex', '-1');
    items[nextIdx].setAttribute('tabindex', '0');
    items[nextIdx].focus();
  }

  private _escapeHtml(text: string): string {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
  }

  private _escapeAttr(text: string): string {
    return text.replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/'/g, '&#39;')
      .replace(/</g, '&lt;').replace(/>/g, '&gt;');
  }
}

customElements.define('ep-color-picker', EpColorPicker);

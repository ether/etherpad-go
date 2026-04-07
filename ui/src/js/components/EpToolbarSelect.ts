type ToolbarSelectOption = {
  label: string;
  value: string;
};

const STYLE_ID = 'ep-toolbar-select-styles';

const ensureStyles = () => {
  if (document.getElementById(STYLE_ID)) return;
  const style = document.createElement('style');
  style.id = STYLE_ID;
  style.textContent = `
    ep-toolbar-select {
      display: flex;
      align-items: center;
      min-width: 0;
    }

    ep-toolbar-select ep-dropdown {
      display: flex;
      align-items: center;
    }

    ep-toolbar-select .ep-toolbar-select__button {
      display: inline-flex;
      align-items: center;
      gap: 6px;
      min-width: 28px;
      height: 28px;
      padding: 0 8px;
      border: none;
      border-radius: 3px;
      background: transparent;
      color: inherit;
      cursor: pointer;
      white-space: nowrap;
      font: inherit;
    }

    ep-toolbar-select .ep-toolbar-select__button:hover {
      background-color: #f2f3f4;
      background-color: var(--bg-soft-color, #f2f3f4);
      color: #485365;
      color: var(--text-color, #485365);
    }

    ep-toolbar-select .ep-toolbar-select__button:focus-visible {
      outline: 2px solid #64d29b;
      outline-offset: 1px;
    }

    ep-toolbar-select .ep-toolbar-select__icon {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      flex: 0 0 auto;
    }

    ep-toolbar-select .ep-toolbar-select__text {
      display: inline-block;
      min-width: 0;
      max-width: 96px;
      overflow: hidden;
      text-overflow: ellipsis;
      font-size: 12px;
      font-weight: 500;
    }

    ep-toolbar-select .ep-toolbar-select__caret {
      flex: 0 0 auto;
      display: inline-block;
      width: 8px;
      height: 8px;
      border-right: 2px solid currentColor;
      border-bottom: 2px solid currentColor;
      transform: translateY(-1px) rotate(45deg);
      opacity: 0.7;
    }
  `;
  document.head.appendChild(style);
};

export class EpToolbarSelect extends HTMLElement {
  private _options: ToolbarSelectOption[] = [];
  private _value = '';
  private _button?: HTMLButtonElement;
  private _label?: HTMLSpanElement;

  connectedCallback(): void {
    ensureStyles();
    this._render();
  }

  get options(): ToolbarSelectOption[] {
    return this._options;
  }

  set options(options: ToolbarSelectOption[]) {
    this._options = Array.isArray(options) ? options : [];
    this._render();
  }

  get value(): string {
    return this._value;
  }

  set value(value: string) {
    this._value = value ?? '';
    this._updateTrigger();
  }

  private _render(): void {
    this.replaceChildren();

    const dropdown = document.createElement('ep-dropdown');
    dropdown.setAttribute('align', 'left');
    dropdown.setAttribute('trigger', 'click');

    const button = document.createElement('button');
    button.type = 'button';
    button.slot = 'trigger';
    button.className = 'ep-toolbar-select__button';

    const iconClass = this.getAttribute('icon-class');
    if (iconClass) {
      const icon = document.createElement('span');
      icon.className = `buttonicon ${iconClass} ep-toolbar-select__icon`;
      button.appendChild(icon);
    }

    const label = document.createElement('span');
    label.className = 'ep-toolbar-select__text';
    button.appendChild(label);

    const caret = document.createElement('span');
    caret.className = 'ep-toolbar-select__caret';
    caret.setAttribute('aria-hidden', 'true');
    button.appendChild(caret);

    const content = document.createElement('div');
    content.slot = 'content';
    for (const option of this._options) {
      const item = document.createElement('ep-dropdown-item');
      item.setAttribute('value', option.value);
      item.textContent = option.label;
      content.appendChild(item);
    }

    dropdown.addEventListener('ep-dropdown-select', ((event: CustomEvent) => {
      this._value = String(event.detail?.value ?? '');
      this._updateTrigger();
      this.dispatchEvent(new CustomEvent('ep-toolbar-select:change', {
        bubbles: true,
        composed: true,
        detail: {value: this._value},
      }));
    }) as EventListener);

    dropdown.append(button, content);
    this.appendChild(dropdown);

    this._button = button;
    this._label = label;
    this._updateTrigger();
  }

  private _updateTrigger(): void {
    if (!this._button || !this._label) return;
    const selected = this._options.find((option) => option.value === this._value);
    const visibleLabel = selected?.label ?? this.getAttribute('placeholder') ?? this.getAttribute('label') ?? '';
    const titlePrefix = this.getAttribute('label') ?? '';

    this._label.textContent = visibleLabel;
    this._button.title = titlePrefix && selected ? `${titlePrefix}: ${selected.label}` : (titlePrefix || visibleLabel);
    this._button.setAttribute('aria-label', this._button.title);
  }
}

customElements.define('ep-toolbar-select', EpToolbarSelect);

// Formula bar: a Name Box (active ref) + an editable input two-way-synced with
// the active cell, plus function-name autocomplete. Commits go through the same
// setCell path as in-cell editing (via onCommit).

import { functionPrefix, filterFunctions } from './autocomplete';

export interface FormulaBarCallbacks {
  onCommit: (raw: string) => void;
  getFunctionNames: () => string[];
  readOnly: boolean;
}

export interface FormulaBarHandle {
  el: HTMLElement;
  setActive: (ref: string, raw: string) => void;
}

const CSS = `
.sheet-formula-bar { display: flex; align-items: stretch; gap: 6px; padding: 3px 4px; border-bottom: 1px solid #d4d8dd; background: #fff; font: 13px system-ui, sans-serif; position: relative; }
.sheet-namebox { min-width: 72px; padding: 2px 6px; border: 1px solid #d4d8dd; background: #fff; text-align: center; align-self: center; border-radius: 3px; }
.sheet-fx-label { align-self: center; font: italic 13px Georgia, 'Times New Roman', serif; color: #8a8f95; pointer-events: none; user-select: none; }
.sheet-fx-input { flex: 1; padding: 2px 6px; border: 1px solid #d4d8dd; background: #fff; font: 13px/1.4 ui-monospace, monospace; outline: none; }
.sheet-fx-input:focus { border-color: #107c41; }
.sheet-fx-ac { position: absolute; top: 100%; left: 104px; z-index: 20; background: #fff; border: 1px solid #bbb; box-shadow: 0 2px 6px rgba(0,0,0,.15); min-width: 180px; max-height: 180px; overflow-y: auto; }
.sheet-fx-ac div { padding: 2px 8px; cursor: pointer; font: 12px/1.5 ui-monospace, monospace; }
.sheet-fx-ac div.hl { background: #cfeede; }
`;

export function createFormulaBar(cb: FormulaBarCallbacks): FormulaBarHandle {
  if (!document.getElementById('sheet-formula-bar-style')) {
    const s = document.createElement('style');
    s.id = 'sheet-formula-bar-style';
    s.textContent = CSS;
    document.head.appendChild(s);
  }
  const bar = document.createElement('div');
  bar.className = 'sheet-formula-bar';

  const nameBox = document.createElement('span');
  nameBox.className = 'sheet-namebox';
  nameBox.textContent = 'A1';

  const fx = document.createElement('span');
  fx.className = 'sheet-fx-label';
  fx.textContent = 'fx';

  const input = document.createElement('input');
  input.className = 'sheet-fx-input';
  input.type = 'text';
  input.disabled = cb.readOnly;

  const ac = document.createElement('div');
  ac.className = 'sheet-fx-ac';
  ac.style.display = 'none';

  bar.append(nameBox, fx, input, ac);

  let lastRaw = '';
  let acItems: string[] = [];
  let acIndex = -1;

  const closeAc = (): void => { ac.style.display = 'none'; acItems = []; acIndex = -1; };

  const renderAc = (): void => {
    const prefix = functionPrefix(input.value, input.selectionStart ?? input.value.length);
    acItems = prefix ? filterFunctions(cb.getFunctionNames(), prefix) : [];
    if (acItems.length === 0) { closeAc(); return; }
    acIndex = 0;
    ac.innerHTML = '';
    acItems.forEach((name, i) => {
      const d = document.createElement('div');
      d.textContent = name;
      if (i === acIndex) d.className = 'hl';
      d.addEventListener('mousedown', (e) => { e.preventDefault(); accept(name); });
      ac.appendChild(d);
    });
    ac.style.display = 'block';
  };

  const highlight = (): void => {
    [...ac.children].forEach((c, i) => (c as HTMLElement).className = i === acIndex ? 'hl' : '');
  };

  const accept = (name: string): void => {
    const caret = input.selectionStart ?? input.value.length;
    const left = input.value.slice(0, caret).replace(/[A-Za-z]+$/, '');
    const right = input.value.slice(caret);
    input.value = `${left}${name}(${right}`;
    const pos = left.length + name.length + 1;
    input.setSelectionRange(pos, pos);
    closeAc();
    input.focus();
  };

  input.addEventListener('input', renderAc);
  input.addEventListener('blur', () => closeAc());
  input.addEventListener('keydown', (e: KeyboardEvent) => {
    if (ac.style.display === 'block' && acItems.length) {
      if (e.key === 'ArrowDown') { e.preventDefault(); acIndex = (acIndex + 1) % acItems.length; return highlight(); }
      if (e.key === 'ArrowUp') { e.preventDefault(); acIndex = (acIndex - 1 + acItems.length) % acItems.length; return highlight(); }
      if (e.key === 'Tab' || e.key === 'Enter') { e.preventDefault(); return accept(acItems[acIndex]); }
      if (e.key === 'Escape') { e.preventDefault(); return closeAc(); }
    }
    if (e.key === 'Enter') { e.preventDefault(); cb.onCommit(input.value); input.blur(); }
    else if (e.key === 'Escape') { e.preventDefault(); input.value = lastRaw; input.blur(); }
  });

  return {
    el: bar,
    setActive(ref: string, raw: string): void {
      nameBox.textContent = ref;
      // Don't stomp the user's in-progress typing — and freeze the Escape
      // revert target (lastRaw) too, so a concurrent remote edit can't
      // silently redefine what Escape restores mid-edit.
      if (document.activeElement !== input) {
        lastRaw = raw;
        input.value = raw;
      }
    },
  };
}

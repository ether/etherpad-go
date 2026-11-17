// A small modal form in the look of the Find dialog, used for Insert Link,
// notes, data validation and conditional-formatting rules.

export interface DialogField {
  key: string;
  label: string;
  type: 'text' | 'textarea' | 'select';
  value?: string;
  placeholder?: string;
  options?: Array<[string, string]>; // [value, label] for select
  // Shown only while another field has one of these values.
  showWhen?: { key: string; values: string[] };
}

export interface DialogOptions {
  title: string;
  fields: DialogField[];
  okLabel?: string;
  // Extra button left of OK (e.g. "Remove Link"); closes the dialog.
  extra?: { label: string; run: () => void };
  // Returns an error message to keep the dialog open, or null to close it.
  onOk: (values: Record<string, string>) => string | null;
}

const CSS = `
.sheet-dialog-back { position: fixed; inset: 0; z-index: 60; background: rgba(0,0,0,0.15); display: flex; align-items: flex-start; justify-content: center; padding-top: 110px; }
.sheet-dialog { width: 380px; background: #fff; border: 1px solid #d4d8dd; box-shadow: 0 6px 18px rgba(0,0,0,0.18); border-radius: 4px; font: 13px system-ui, sans-serif; color: #333; }
.sheet-dialog-head { padding: 8px 10px; background: #107c41; color: #fff; border-radius: 3px 3px 0 0; font-weight: 600; }
.sheet-dialog-body { padding: 10px; display: grid; grid-template-columns: auto 1fr; gap: 8px; align-items: center; }
.sheet-dialog-body label { white-space: nowrap; }
.sheet-dialog-body input, .sheet-dialog-body select, .sheet-dialog-body textarea { width: 100%; box-sizing: border-box; padding: 3px 6px; border: 1px solid #d4d8dd; border-radius: 2px; font: 13px system-ui, sans-serif; }
.sheet-dialog-body textarea { min-height: 90px; resize: vertical; }
.sheet-dialog-error { grid-column: 1 / -1; min-height: 16px; font-size: 12px; color: #c0392b; }
.sheet-dialog-actions { display: flex; gap: 6px; justify-content: flex-end; padding: 0 10px 10px; }
.sheet-dialog-actions button { height: 26px; padding: 0 12px; border: 1px solid #d4d8dd; border-radius: 3px; background: #f5f6f7; font: 13px system-ui, sans-serif; cursor: pointer; }
.sheet-dialog-actions button.primary { background: #107c41; border-color: #107c41; color: #fff; }
.sheet-dialog-actions .sheet-dialog-extra { margin-right: auto; }
`;

export function openDialog(opts: DialogOptions): void {
  if (!document.getElementById('sheet-dialog-style')) {
    const s = document.createElement('style');
    s.id = 'sheet-dialog-style';
    s.textContent = CSS;
    document.head.appendChild(s);
  }
  document.querySelector('.sheet-dialog-back')?.remove();
  const back = document.createElement('div');
  back.className = 'sheet-dialog-back';
  const box = document.createElement('div');
  box.className = 'sheet-dialog';
  box.setAttribute('role', 'dialog');
  box.setAttribute('aria-label', opts.title);
  const head = document.createElement('div');
  head.className = 'sheet-dialog-head';
  head.textContent = opts.title;
  const body = document.createElement('div');
  body.className = 'sheet-dialog-body';

  const inputs = new Map<string, HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement>();
  const rows: Array<{ field: DialogField; els: HTMLElement[] }> = [];
  for (const f of opts.fields) {
    const label = document.createElement('label');
    label.textContent = f.label;
    let input: HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement;
    if (f.type === 'textarea') {
      input = document.createElement('textarea');
    } else if (f.type === 'select') {
      const sel = document.createElement('select');
      for (const [v, l] of f.options ?? []) {
        const o = document.createElement('option');
        o.value = v;
        o.textContent = l;
        sel.appendChild(o);
      }
      input = sel;
    } else {
      input = document.createElement('input');
      (input as HTMLInputElement).type = 'text';
    }
    input.name = f.key;
    if (f.placeholder && !(input instanceof HTMLSelectElement)) input.placeholder = f.placeholder;
    if (f.value !== undefined) input.value = f.value;
    const id = `sheet-dlg-${f.key}`;
    input.id = id;
    label.htmlFor = id;
    inputs.set(f.key, input);
    rows.push({ field: f, els: [label, input] });
    body.append(label, input);
  }
  const error = document.createElement('div');
  error.className = 'sheet-dialog-error';
  body.appendChild(error);

  const sync = (): void => {
    for (const { field, els } of rows) {
      const w = field.showWhen;
      const show = !w || w.values.includes(inputs.get(w.key)?.value ?? '');
      for (const el of els) el.style.display = show ? '' : 'none';
    }
  };
  for (const input of inputs.values()) input.addEventListener('change', sync);
  sync();

  const close = (): void => {
    back.remove();
    document.removeEventListener('keydown', onKey, true);
  };
  const ok = (): void => {
    const values: Record<string, string> = {};
    for (const [k, input] of inputs) values[k] = input.value;
    const err = opts.onOk(values);
    if (err) {
      error.textContent = err;
      return;
    }
    close();
  };
  const onKey = (e: KeyboardEvent): void => {
    // The grid's shortcuts (copy, undo, delete…) must not act while typing here.
    if (box.contains(e.target as Node)) e.stopPropagation();
    if (e.key === 'Escape') {
      e.preventDefault();
      e.stopPropagation();
      close();
    } else if (e.key === 'Enter' && !(e.target instanceof HTMLTextAreaElement)) {
      e.preventDefault();
      e.stopPropagation();
      ok();
    }
  };
  document.addEventListener('keydown', onKey, true);

  const actions = document.createElement('div');
  actions.className = 'sheet-dialog-actions';
  if (opts.extra) {
    const extra = document.createElement('button');
    extra.type = 'button';
    extra.className = 'sheet-dialog-extra';
    extra.textContent = opts.extra.label;
    extra.addEventListener('click', () => {
      close();
      opts.extra?.run();
    });
    actions.appendChild(extra);
  }
  const cancel = document.createElement('button');
  cancel.type = 'button';
  cancel.textContent = 'Cancel';
  cancel.addEventListener('click', close);
  const okBtn = document.createElement('button');
  okBtn.type = 'button';
  okBtn.className = 'primary';
  okBtn.textContent = opts.okLabel ?? 'OK';
  okBtn.addEventListener('click', ok);
  actions.append(cancel, okBtn);

  box.append(head, body, actions);
  back.appendChild(box);
  back.addEventListener('mousedown', (e) => {
    if (e.target === back) close();
  });
  document.body.appendChild(back);
  const first = [...inputs.values()].find((i) => i.offsetParent !== null);
  first?.focus();
  if (first instanceof HTMLInputElement) first.select();
}

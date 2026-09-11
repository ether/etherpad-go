// Excel's Find & Replace dialog. Owns only its DOM and option state; the
// editor supplies the cells and performs the selection/op side effects.
import type { FindOptions } from './findReplace';

export interface FindDialogCallbacks {
  // Selects the next match after the current cell and reports what happened.
  findNext: (query: string, opts: FindOptions) => { found: boolean; total: number };
  // Replaces in the current cell if it matches, then advances.
  replace: (query: string, replacement: string, opts: FindOptions) => { replaced: boolean; total: number };
  replaceAll: (query: string, replacement: string, opts: FindOptions) => number;
  readOnly: boolean;
}

export interface FindDialogHandle {
  el: HTMLElement;
  open: (mode: 'find' | 'replace') => void;
  close: () => void;
  isOpen: () => boolean;
}

const CSS = `
.sheet-find { position: fixed; top: 90px; right: 24px; z-index: 50; width: 320px; background: #fff;
  border: 1px solid #d4d8dd; box-shadow: 0 6px 18px rgba(0,0,0,0.18); border-radius: 4px;
  font: 13px system-ui, sans-serif; color: #333; }
.sheet-find[hidden] { display: none; }
.sheet-find-head { display: flex; align-items: center; justify-content: space-between; padding: 8px 10px;
  background: #107c41; color: #fff; border-radius: 3px 3px 0 0; font-weight: 600; }
.sheet-find-head button { background: none; border: none; color: #fff; font-size: 15px; cursor: pointer; line-height: 1; }
.sheet-find-body { padding: 10px; display: grid; grid-template-columns: auto 1fr; gap: 6px 8px; align-items: center; }
.sheet-find-body input[type=text] { width: 100%; box-sizing: border-box; height: 26px; padding: 0 6px;
  border: 1px solid #d4d8dd; border-radius: 2px; font: 13px system-ui, sans-serif; }
.sheet-find-opts { grid-column: 1 / -1; display: flex; gap: 14px; padding-top: 2px; }
.sheet-find-opts label { display: flex; align-items: center; gap: 4px; font-size: 12px; cursor: pointer; }
.sheet-find-actions { grid-column: 1 / -1; display: flex; gap: 6px; justify-content: flex-end; padding-top: 4px; }
.sheet-find-actions button { height: 26px; padding: 0 10px; border: 1px solid #d4d8dd; border-radius: 3px;
  background: #f5f6f7; font: 13px system-ui, sans-serif; cursor: pointer; }
.sheet-find-actions button:hover:enabled { background: #e6f2ec; border-color: #bcd8c9; }
.sheet-find-actions button:disabled { opacity: 0.5; cursor: default; }
.sheet-find-status { grid-column: 1 / -1; min-height: 16px; font-size: 12px; color: #5f6b7a; }
.sheet-find-status.miss { color: #c0392b; }
`;

export function createFindDialog(cb: FindDialogCallbacks): FindDialogHandle {
  if (!document.getElementById('sheet-find-style')) {
    const s = document.createElement('style');
    s.id = 'sheet-find-style';
    s.textContent = CSS;
    document.head.appendChild(s);
  }

  const el = document.createElement('div');
  el.className = 'sheet-find';
  el.hidden = true;

  const head = document.createElement('div');
  head.className = 'sheet-find-head';
  const title = document.createElement('span');
  title.textContent = 'Find';
  const closeBtn = document.createElement('button');
  closeBtn.type = 'button';
  closeBtn.textContent = '✕';
  closeBtn.title = 'Close';
  head.append(title, closeBtn);

  const body = document.createElement('div');
  body.className = 'sheet-find-body';

  const field = (label: string): HTMLInputElement => {
    const l = document.createElement('label');
    l.textContent = label;
    const input = document.createElement('input');
    input.type = 'text';
    body.append(l, input);
    return input;
  };
  const findInput = field('Find what');
  const replaceInput = field('Replace with');
  const replaceLabel = replaceInput.previousElementSibling as HTMLElement;

  const opts = document.createElement('div');
  opts.className = 'sheet-find-opts';
  const option = (label: string): HTMLInputElement => {
    const wrap = document.createElement('label');
    const box = document.createElement('input');
    box.type = 'checkbox';
    wrap.append(box, label);
    opts.appendChild(wrap);
    return box;
  };
  const matchCase = option('Match case');
  const wholeCell = option('Entire cell');
  body.appendChild(opts);

  const status = document.createElement('div');
  status.className = 'sheet-find-status';
  body.appendChild(status);

  const actions = document.createElement('div');
  actions.className = 'sheet-find-actions';
  const action = (label: string, onClick: () => void): HTMLButtonElement => {
    const b = document.createElement('button');
    b.type = 'button';
    b.textContent = label;
    b.addEventListener('click', onClick);
    actions.appendChild(b);
    return b;
  };

  const options = (): FindOptions => ({ matchCase: matchCase.checked, wholeCell: wholeCell.checked });
  const say = (text: string, miss = false): void => {
    status.textContent = text;
    status.classList.toggle('miss', miss);
  };
  const plural = (n: number, one: string, many: string): string => `${n} ${n === 1 ? one : many}`;

  // Selecting a hit focuses that cell, which would steal the keyboard from the
  // dialog. Excel keeps typing in the box, so take the focus back.
  const keepFocus = (): void => findInput.focus();

  const doFind = (): void => {
    const query = findInput.value;
    if (query === '') return say('');
    const { found, total } = cb.findNext(query, options());
    say(found ? `${plural(total, 'cell', 'cells')} found` : 'No match', !found);
    keepFocus();
  };
  const doReplace = (): void => {
    const query = findInput.value;
    if (query === '') return say('');
    const { replaced, total } = cb.replace(query, replaceInput.value, options());
    say(replaced ? `Replaced, ${plural(total, 'cell', 'cells')} left` : 'No match', !replaced);
    keepFocus();
  };
  const doReplaceAll = (): void => {
    const query = findInput.value;
    if (query === '') return say('');
    const n = cb.replaceAll(query, replaceInput.value, options());
    say(n === 0 ? 'No match' : `Replaced ${plural(n, 'cell', 'cells')}`, n === 0);
  };

  const replaceAllBtn = action('Replace All', doReplaceAll);
  const replaceBtn = action('Replace', doReplace);
  const findBtn = action('Find Next', doFind);
  findBtn.style.fontWeight = '600';
  if (cb.readOnly) {
    replaceBtn.disabled = true;
    replaceAllBtn.disabled = true;
    replaceInput.disabled = true;
  }
  body.appendChild(actions);
  el.append(head, body);

  const close = (): void => {
    el.hidden = true;
  };
  closeBtn.addEventListener('click', close);
  // Enter = Find Next, Escape closes — the two keys the dialog owns while focused.
  el.addEventListener('keydown', (e) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      doFind();
    } else if (e.key === 'Escape') {
      e.preventDefault();
      close();
    }
    e.stopPropagation(); // the grid's global shortcuts must not see dialog typing
  });

  return {
    el,
    open: (mode) => {
      const replacing = mode === 'replace' && !cb.readOnly;
      title.textContent = replacing ? 'Find and Replace' : 'Find';
      replaceLabel.hidden = !replacing;
      replaceInput.hidden = !replacing;
      replaceBtn.hidden = !replacing;
      replaceAllBtn.hidden = !replacing;
      el.hidden = false;
      say('');
      findInput.focus();
      findInput.select();
    },
    close,
    isOpen: () => !el.hidden,
  };
}

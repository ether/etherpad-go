// Excel-style filter list for one column: a search box, "(Select all)" and a
// checkbox per value. OK keeps the rows whose value is checked.

export interface FilterMenuOptions {
  title: string;
  values: string[];
  // Values kept now; null: the column is not filtered.
  shown: string[] | null;
  anyFilter: boolean;
  // null removes the column's filter (everything checked).
  onApply: (values: string[] | null) => void;
  onClearAll: () => void;
}

const CSS = `
.sheet-filter-menu { position: fixed; z-index: 55; width: 240px; background: #fff; border: 1px solid #c8cdd3; box-shadow: 0 6px 16px rgba(0,0,0,0.18); border-radius: 3px; font: 13px system-ui, sans-serif; color: #333; padding: 8px; display: flex; flex-direction: column; gap: 6px; }
.sheet-filter-menu .sheet-filter-title { font-weight: 600; }
.sheet-filter-menu input[type=search] { width: 100%; box-sizing: border-box; padding: 3px 6px; border: 1px solid #d4d8dd; border-radius: 2px; font: inherit; }
.sheet-filter-list { max-height: 220px; overflow-y: auto; border: 1px solid #e1e4e8; padding: 2px 4px; }
.sheet-filter-list label { display: flex; gap: 6px; align-items: center; padding: 1px 0; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; cursor: pointer; }
.sheet-filter-list label.sheet-filter-all { font-style: italic; border-bottom: 1px solid #eee; margin-bottom: 2px; }
.sheet-filter-actions { display: flex; gap: 6px; justify-content: flex-end; flex-wrap: wrap; }
.sheet-filter-actions button { height: 24px; padding: 0 10px; border: 1px solid #d4d8dd; border-radius: 3px; background: #f5f6f7; font: inherit; cursor: pointer; }
.sheet-filter-actions button.primary { background: #107c41; border-color: #107c41; color: #fff; }
.sheet-filter-links { display: flex; flex-direction: column; align-items: flex-start; }
.sheet-filter-links button { border: none; background: none; padding: 2px 0; color: #0563c1; cursor: pointer; font: inherit; }
.sheet-filter-links button:disabled { color: #aaa; cursor: default; }
`;

let current: { close: () => void } | null = null;

export function closeFilterMenu(): void {
  current?.close();
}

export function openFilterMenu(anchor: HTMLElement, o: FilterMenuOptions): void {
  if (!document.getElementById('sheet-filter-style')) {
    const s = document.createElement('style');
    s.id = 'sheet-filter-style';
    s.textContent = CSS;
    document.head.appendChild(s);
  }
  closeFilterMenu();
  const menu = document.createElement('div');
  menu.className = 'sheet-filter-menu';
  menu.setAttribute('role', 'dialog');
  menu.setAttribute('aria-label', o.title);

  const title = document.createElement('div');
  title.className = 'sheet-filter-title';
  title.textContent = o.title;

  const links = document.createElement('div');
  links.className = 'sheet-filter-links';
  const clearCol = document.createElement('button');
  clearCol.type = 'button';
  clearCol.textContent = 'Clear filter from this column';
  clearCol.disabled = o.shown === null;
  const clearAll = document.createElement('button');
  clearAll.type = 'button';
  clearAll.textContent = 'Clear all filters';
  clearAll.disabled = !o.anyFilter;
  links.append(clearCol, clearAll);

  const search = document.createElement('input');
  search.type = 'search';
  search.placeholder = 'Search';

  const list = document.createElement('div');
  list.className = 'sheet-filter-list';
  const shown = o.shown === null ? null : new Set(o.shown);
  // Values kept by the filter but no longer in the column stay listed, so
  // OK does not silently drop them.
  const values = [...o.values];
  for (const v of o.shown ?? []) if (!values.includes(v)) values.push(v);

  const allLabel = document.createElement('label');
  allLabel.className = 'sheet-filter-all';
  const allBox = document.createElement('input');
  allBox.type = 'checkbox';
  allLabel.append(allBox, document.createTextNode('(Select all)'));
  list.appendChild(allLabel);

  const boxes: Array<{ value: string; box: HTMLInputElement; label: HTMLLabelElement }> = [];
  for (const v of values) {
    const label = document.createElement('label');
    const box = document.createElement('input');
    box.type = 'checkbox';
    box.checked = shown === null || shown.has(v);
    label.append(box, document.createTextNode(v === '' ? '(Empty text)' : v));
    label.title = v;
    list.appendChild(label);
    boxes.push({ value: v, box, label });
  }
  const visible = () => boxes.filter((b) => b.label.style.display !== 'none');
  const syncAll = (): void => {
    const vis = visible();
    const on = vis.filter((b) => b.box.checked).length;
    allBox.checked = vis.length > 0 && on === vis.length;
    allBox.indeterminate = on > 0 && on < vis.length;
  };
  allBox.addEventListener('change', () => {
    for (const b of visible()) b.box.checked = allBox.checked;
    syncAll();
  });
  for (const b of boxes) b.box.addEventListener('change', syncAll);
  search.addEventListener('input', () => {
    const q = search.value.trim().toLowerCase();
    for (const b of boxes) b.label.style.display = q === '' || b.value.toLowerCase().includes(q) ? '' : 'none';
    syncAll();
  });
  syncAll();

  const actions = document.createElement('div');
  actions.className = 'sheet-filter-actions';
  const cancel = document.createElement('button');
  cancel.type = 'button';
  cancel.textContent = 'Cancel';
  const ok = document.createElement('button');
  ok.type = 'button';
  ok.className = 'primary';
  ok.textContent = 'OK';
  actions.append(cancel, ok);

  menu.append(title, links, search, list, actions);
  document.body.appendChild(menu);
  const r = anchor.getBoundingClientRect();
  menu.style.left = `${Math.max(4, Math.min(r.left, window.innerWidth - menu.offsetWidth - 4))}px`;
  menu.style.top = `${Math.min(r.bottom + 2, window.innerHeight - menu.offsetHeight - 4)}px`;

  const close = (): void => {
    menu.remove();
    document.removeEventListener('mousedown', outside, true);
    document.removeEventListener('keydown', onKey, true);
    current = null;
  };
  const outside = (e: MouseEvent): void => {
    if (!menu.contains(e.target as Node) && e.target !== anchor) close();
  };
  const apply = (): void => {
    const checked = boxes.filter((b) => b.box.checked).map((b) => b.value);
    if (checked.length === 0) return; // like Excel: pick at least one value
    close();
    // All of the column's values checked: no filter. (A search narrows only
    // what is shown in the list, not what OK keeps.)
    o.onApply(checked.length === boxes.length ? null : checked);
  };
  const onKey = (e: KeyboardEvent): void => {
    if (!menu.contains(e.target as Node)) return;
    e.stopPropagation();
    if (e.key === 'Escape') close();
    else if (e.key === 'Enter') {
      e.preventDefault();
      apply();
    }
  };
  document.addEventListener('mousedown', outside, true);
  document.addEventListener('keydown', onKey, true);
  cancel.addEventListener('click', close);
  ok.addEventListener('click', apply);
  clearCol.addEventListener('click', () => {
    close();
    o.onApply(null);
  });
  clearAll.addEventListener('click', () => {
    close();
    o.onClearAll();
  });
  current = { close };
  search.focus();
}

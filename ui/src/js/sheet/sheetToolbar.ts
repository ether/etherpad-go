// Excel-style ribbon toolbar. Emits style-prop *changes* for the current
// selection; the editor merges them onto each cell's existing props and sends
// setStyle ops. Uses native inputs (no dependency).

export interface ToolbarCallbacks {
  getProps: (row: number, col: number) => Record<string, string>;
  focusCell: () => { row: number; col: number };
  applyToSelection: (change: Record<string, string>) => void;
  readOnly: boolean;
  // M4 structural actions.
  sortSelection?: (asc: boolean) => void;
  toggleFreeze?: (kind: 'row' | 'col') => void;
  frozenState?: () => { rows: number; cols: number };
  // Filter on the focus column: values() fills the dropdown lazily, apply(null) clears.
  filterValues?: () => string[];
  applyFilter?: (value: string | null) => void;
  // Ribbon: row/col structure relative to the selection.
  structural?: (action: 'insRowAbove' | 'insRowBelow' | 'insColLeft' | 'insColRight' | 'delRows' | 'delCols') => void;
  // Ribbon: workbook import/export (server round-trip).
  importXlsx?: (file: File) => void;
  exportXlsx?: () => void;
}

const CSS = `
.sheet-toolbar { border-bottom: 1px solid #d4d8dd; background: #fff; font: 13px system-ui, sans-serif; }
.sheet-toolbar[aria-disabled=true] { opacity: 0.5; pointer-events: none; }
.sheet-ribbon-tabs { display: flex; gap: 2px; padding: 0 6px; background: #fff; border-bottom: 1px solid #e3e6ea; }
.sheet-ribbon-tabs button { border: none; background: none; padding: 5px 12px 3px; font: 13px system-ui, sans-serif; color: #444; cursor: pointer; border-bottom: 2.5px solid transparent; }
.sheet-ribbon-tabs button:hover { color: #107c41; }
.sheet-ribbon-tabs button.on { color: #107c41; border-bottom-color: #107c41; font-weight: 600; }
.sheet-ribbon-body { display: flex; align-items: stretch; min-height: 64px; padding: 4px 6px 2px; background: #f9fafb; overflow-x: auto; }
.sheet-ribbon-group { display: flex; flex-direction: column; justify-content: space-between; padding: 2px 10px; border-right: 1px solid #e3e6ea; }
.sheet-ribbon-group:last-child { border-right: none; }
.sheet-ribbon-row { display: flex; gap: 3px; align-items: center; }
.sheet-ribbon-label { font-size: 10.5px; color: #8a8f95; text-align: center; padding-top: 3px; }
.sheet-ribbon-row button { height: 26px; min-width: 26px; padding: 0 6px; border: 1px solid transparent; border-radius: 3px; background: none; font: 13px system-ui, sans-serif; cursor: pointer; }
.sheet-ribbon-row button:hover { background: #e6f2ec; border-color: #bcd8c9; }
.sheet-ribbon-row button.on { background: #cce5d8; border-color: #9fccb4; }
.sheet-ribbon-row select { height: 26px; border: 1px solid #c8cdd3; border-radius: 3px; background: #fff; font: 12px system-ui, sans-serif; cursor: pointer; }
.sheet-ribbon-row input[type=color] { width: 26px; height: 26px; padding: 1px 2px; border: 1px solid transparent; border-radius: 3px; background: none; cursor: pointer; }
.sheet-ribbon-row input[type=color]:hover { background: #e6f2ec; border-color: #bcd8c9; }
`;

type TabName = 'Home' | 'Data' | 'View';

export function createToolbar(cb: ToolbarCallbacks): HTMLElement {
  if (!document.getElementById('sheet-toolbar-style')) {
    const s = document.createElement('style');
    s.id = 'sheet-toolbar-style';
    s.textContent = CSS;
    document.head.appendChild(s);
  }
  const bar = document.createElement('div');
  bar.className = 'sheet-toolbar';
  if (cb.readOnly) bar.setAttribute('aria-disabled', 'true');

  const tabsEl = document.createElement('div');
  tabsEl.className = 'sheet-ribbon-tabs';
  const body = document.createElement('div');
  body.className = 'sheet-ribbon-body';
  bar.append(tabsEl, body);

  const tabBtns = new Map<TabName, HTMLButtonElement>();
  const groups: { tab: TabName; el: HTMLElement }[] = [];
  const selectTab = (name: TabName) => {
    for (const [n, b] of tabBtns) b.classList.toggle('on', n === name);
    for (const g of groups) g.el.style.display = g.tab === name ? '' : 'none';
  };
  for (const name of ['Home', 'Data', 'View'] as TabName[]) {
    const b = document.createElement('button');
    b.textContent = name;
    b.addEventListener('click', () => selectTab(name));
    tabBtns.set(name, b);
    tabsEl.appendChild(b);
  }

  // Creates a group under a tab; returns the row to put controls into.
  const group = (tab: TabName, label: string): HTMLElement => {
    const g = document.createElement('div');
    g.className = 'sheet-ribbon-group';
    const row = document.createElement('div');
    row.className = 'sheet-ribbon-row';
    const lbl = document.createElement('div');
    lbl.className = 'sheet-ribbon-label';
    lbl.textContent = label;
    g.append(row, lbl);
    body.appendChild(g);
    groups.push({ tab, el: g });
    return row;
  };

  const btn = (row: HTMLElement, label: string, title: string, onClick: () => void): HTMLButtonElement => {
    const b = document.createElement('button');
    b.textContent = label;
    b.title = title;
    b.addEventListener('click', onClick);
    row.appendChild(b);
    return b;
  };

  const curProps = () => { const f = cb.focusCell(); return cb.getProps(f.row, f.col); };

  // --- Home: Font ---
  const font = group('Home', 'Font');
  const toggleBtn = (label: string, key: string) => {
    const b = btn(font, label, key, () => {
      const on = curProps()[key] === '1';
      cb.applyToSelection({ [key]: on ? '' : '1' });
    });
    b.dataset.key = key;
    return b;
  };
  toggleBtn('B', 'bold').style.fontWeight = 'bold';
  toggleBtn('I', 'italic').style.fontStyle = 'italic';
  toggleBtn('U', 'underline').style.textDecoration = 'underline';

  const color = document.createElement('input');
  color.type = 'color'; color.title = 'Font color';
  color.addEventListener('input', () => cb.applyToSelection({ color: color.value }));
  font.appendChild(color);

  const bg = document.createElement('input');
  bg.type = 'color'; bg.title = 'Fill color'; bg.value = '#ffffff';
  bg.addEventListener('input', () => cb.applyToSelection({ bg: bg.value }));
  font.appendChild(bg);

  btn(font, '▦', 'Borders', () => {
    const on = curProps().border === 'all';
    cb.applyToSelection({ border: on ? '' : 'all' });
  });

  // --- Home: Alignment ---
  const alignRow = group('Home', 'Alignment');
  for (const a of ['left', 'center', 'right'] as const) {
    btn(alignRow, { left: '⯇', center: '☰', right: '⯈' }[a], `Align ${a}`, () => cb.applyToSelection({ align: a }));
  }

  // --- Home: Number ---
  const numRow = group('Home', 'Number');
  const numFmt = document.createElement('select');
  numFmt.title = 'Number format';
  for (const [v, label] of [['general', 'General'], ['number:2', 'Number'], ['currency:2', 'Currency'], ['percent:0', 'Percent'], ['date', 'Date'], ['text', 'Text']] as const) {
    const o = document.createElement('option'); o.value = v; o.textContent = label; numFmt.appendChild(o);
  }
  numFmt.addEventListener('change', () => cb.applyToSelection({ numFmt: numFmt.value }));
  numRow.appendChild(numFmt);

  // --- Home: Cells ---
  if (cb.structural) {
    const cells = group('Home', 'Cells');
    const acts = [
      ['insRowAbove', '⤒ Row', 'Insert row above'],
      ['insRowBelow', '⤓ Row', 'Insert row below'],
      ['insColLeft', '⇤ Col', 'Insert column left'],
      ['insColRight', '⇥ Col', 'Insert column right'],
      ['delRows', '✕ Rows', 'Delete selected rows'],
      ['delCols', '✕ Cols', 'Delete selected columns'],
    ] as const;
    for (const [act, label, title] of acts) {
      btn(cells, label, title, () => cb.structural?.(act)).dataset.act = act;
    }
  }

  // --- Data: Sort ---
  const sort = group('Data', 'Sort');
  btn(sort, 'A→Z', 'Sort selection ascending by the focused column', () => cb.sortSelection?.(true));
  btn(sort, 'Z→A', 'Sort selection descending by the focused column', () => cb.sortSelection?.(false));

  // --- Data: Filter ---
  const filterRow = group('Data', 'Filter');
  const filter = document.createElement('select');
  filter.title = 'Filter rows by the focused column';
  const fill = () => {
    filter.innerHTML = '';
    const all = document.createElement('option');
    all.value = '';
    all.textContent = '▼ Filter: (all)';
    filter.appendChild(all);
    for (const v of cb.filterValues?.() ?? []) {
      const o = document.createElement('option');
      o.value = v;
      o.textContent = v;
      filter.appendChild(o);
    }
  };
  fill();
  filter.addEventListener('mousedown', fill); // repopulate lazily on open
  filter.addEventListener('focus', fill); // and for keyboard users
  filter.addEventListener('change', () => cb.applyFilter?.(filter.value === '' ? null : filter.value));
  filterRow.appendChild(filter);

  // --- Data: Workbook ---
  if (cb.importXlsx || cb.exportXlsx) {
    const wb = group('Data', 'Workbook');
    if (cb.importXlsx) {
      const file = document.createElement('input');
      file.type = 'file';
      file.accept = '.xlsx';
      file.style.display = 'none';
      file.addEventListener('change', () => {
        const f = file.files?.[0];
        if (f) cb.importXlsx?.(f);
        file.value = '';
      });
      wb.appendChild(file);
      btn(wb, '⬆ Import', 'Import an .xlsx workbook', () => file.click());
    }
    if (cb.exportXlsx) {
      btn(wb, '⬇ Export', 'Export as .xlsx workbook', () => cb.exportXlsx?.());
    }
  }

  // --- View: Freeze panes ---
  const freeze = group('View', 'Freeze panes');
  const mkFreeze = (label: string, kind: 'row' | 'col', title: string) => {
    const sync = (b: HTMLButtonElement) => {
      const fz = cb.frozenState?.() ?? { rows: 0, cols: 0 };
      b.classList.toggle('on', kind === 'row' ? fz.rows > 0 : fz.cols > 0);
    };
    const b = btn(freeze, label, title, () => {
      cb.toggleFreeze?.(kind);
      sync(b);
    });
    sync(b);
  };
  mkFreeze('❄ Row', 'row', 'Freeze first row');
  mkFreeze('❄ Col', 'col', 'Freeze first column');

  selectTab('Home');
  return bar;
}

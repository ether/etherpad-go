// Classic Microsoft-365-Excel-style ribbon toolbar. Emits style-prop *changes*
// for the current selection; the editor merges them onto each cell's existing
// props and sends setStyle ops. Uses native inputs and inline SVGs (no assets).

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
  // Ribbon: clipboard + quick aggregation (wired by the editor).
  clipboardAction?: (a: 'cut' | 'copy' | 'paste') => void;
  autoSum?: () => void;
}

const CSS = `
.sheet-toolbar { border-bottom: 1px solid #d4d8dd; background: #fff; font: 13px system-ui, sans-serif; user-select: none; }
.sheet-toolbar[aria-disabled=true] { opacity: 0.5; pointer-events: none; }
.sheet-ribbon-tabs { display: flex; align-items: flex-end; gap: 2px; padding: 4px 8px 0; background: #fff; border-bottom: 1px solid #d4d8dd; position: relative; }
.sheet-ribbon-tabs button { border: 1px solid transparent; border-bottom: none; background: none; padding: 5px 16px 6px; font: 13px system-ui, sans-serif; color: #5a5f66; cursor: pointer; border-radius: 4px 4px 0 0; margin-bottom: -1px; }
.sheet-ribbon-tabs button:hover { color: #107c41; background: #e6f2ec; }
.sheet-ribbon-tabs button.on { background: #f5f6f7; border-color: #d4d8dd; border-bottom: 1px solid #f5f6f7; color: #107c41; font-weight: 600; }
.sheet-ribbon-tabs .sheet-file-tab, .sheet-ribbon-tabs .sheet-file-tab:hover { background: #107c41; color: #fff; font-weight: 600; border-radius: 4px 4px 0 0; }
.sheet-ribbon-tabs .sheet-file-tab:hover { background: #0e6a38; }
.sheet-file-wrap { position: relative; display: flex; }
.sheet-file-menu { position: absolute; top: 100%; left: 0; z-index: 30; min-width: 170px; background: #fff; border: 1px solid #d4d8dd; box-shadow: 0 4px 10px rgba(0,0,0,0.15); padding: 4px 0; }
.sheet-file-menu button { display: block; width: 100%; text-align: left; border: none; background: none; padding: 7px 14px; font: 13px system-ui, sans-serif; color: #333; cursor: pointer; }
.sheet-file-menu button:hover { background: #e6f2ec; }
.sheet-ribbon-body { display: flex; align-items: stretch; min-height: 66px; padding: 3px 6px 1px; background: #f5f6f7; overflow-x: auto; }
.sheet-ribbon-group { display: flex; flex-direction: column; padding: 1px 8px; border-right: 1px solid #d9dde1; }
.sheet-ribbon-group:last-child { border-right: none; }
.sheet-ribbon-content { flex: 1; display: flex; gap: 3px; align-items: center; justify-content: center; }
.sheet-ribbon-col { display: flex; flex-direction: column; gap: 2px; justify-content: center; }
.sheet-ribbon-row { display: flex; gap: 3px; align-items: center; }
.sheet-ribbon-label { font-size: 10.5px; color: #8a8f95; text-align: center; padding: 2px 2px 1px; }
.sheet-ribbon-row button, .sheet-ribbon-big { border: 1px solid transparent; border-radius: 3px; background: none; font: 13px system-ui, sans-serif; color: #333; cursor: pointer; }
.sheet-ribbon-row button { height: 24px; min-width: 26px; padding: 0 5px; display: inline-flex; align-items: center; justify-content: center; gap: 3px; }
.sheet-ribbon-row button:hover, .sheet-ribbon-big:hover { background: #e6f2ec; border-color: #bcd8c9; }
.sheet-ribbon-row button.on, .sheet-ribbon-big.on { background: #cce5d8; border-color: #9fccb4; }
.sheet-ribbon-big { height: 62px; min-width: 48px; padding: 3px 6px; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 2px; font-size: 11px; }
.sheet-ribbon-big .sheet-big-glyph { font-size: 22px; line-height: 26px; }
.sheet-ribbon-row select { height: 22px; border: 1px solid #d4d8dd; border-radius: 2px; background: #fff; font: 12px system-ui, sans-serif; color: #333; cursor: pointer; }
.sheet-ribbon-row select:hover { border-color: #bcd8c9; }
.sheet-ribbon-row input[type=color] { width: 26px; height: 24px; padding: 1px 2px; border: 1px solid transparent; border-radius: 3px; background: none; cursor: pointer; }
.sheet-ribbon-row input[type=color]:hover { background: #e6f2ec; border-color: #bcd8c9; }
`;

type TabName = 'Home' | 'Data' | 'View';

// Inline 16x16 SVG icons (stroke = currentColor); scaled up for big buttons.
const svg = (inner: string, size = 16): string =>
  `<svg width="${size}" height="${size}" viewBox="0 0 16 16" fill="none" stroke="currentColor"` +
  ` stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${inner}</svg>`;

const IC = {
  paste: '<rect x="3" y="3" width="10" height="11.5" rx="1"/><path d="M6 3V1.5h4V3"/><path d="M5.5 6.5h5M5.5 9h5M5.5 11.5h3"/>',
  cut: '<circle cx="4" cy="12" r="1.8"/><circle cx="12" cy="12" r="1.8"/><path d="M5.3 10.6 11 2M10.7 10.6 5 2"/>',
  copy: '<rect x="2.5" y="2.5" width="8" height="8" rx="1"/><path d="M5.5 13.5h7a1 1 0 0 0 1-1v-7"/>',
  borders: '<rect x="2.5" y="2.5" width="11" height="11"/><path d="M8 2.5v11M2.5 8h11"/>',
  alignLeft: '<path d="M2.5 4h11M2.5 7h7M2.5 10h11M2.5 13h7"/>',
  alignCenter: '<path d="M2.5 4h11M4.5 7h7M2.5 10h11M4.5 13h7"/>',
  alignRight: '<path d="M2.5 4h11M6.5 7h7M2.5 10h11M6.5 13h7"/>',
  wrap: '<path d="M2.5 4h11M2.5 8h8.5a2.5 2.5 0 0 1 0 5H8M2.5 12h3"/><path d="M9.5 11.5 8 13l1.5 1.5"/>',
  insRowAbove: '<rect x="2.5" y="10.5" width="11" height="3"/><path d="M8 8.5V3M5.5 5.5 8 3l2.5 2.5"/>',
  insRowBelow: '<rect x="2.5" y="2.5" width="11" height="3"/><path d="M8 7.5V13M5.5 10.5 8 13l2.5-2.5"/>',
  insColLeft: '<rect x="10.5" y="2.5" width="3" height="11"/><path d="M8.5 8H3M5.5 5.5 3 8l2.5 2.5"/>',
  insColRight: '<rect x="2.5" y="2.5" width="3" height="11"/><path d="M7.5 8H13M10.5 5.5 13 8l-2.5 2.5"/>',
  delRows: '<rect x="2.5" y="6.5" width="11" height="3"/><path d="M10.5 1.5l3 3M13.5 1.5l-3 3"/>',
  delCols: '<rect x="6.5" y="2.5" width="3" height="11"/><path d="M11.5 6.5l3 3M14.5 6.5l-3 3"/>',
  importFile: '<path d="M2.5 10.5v3h11v-3"/><path d="M8 2v7.5M4.5 6 8 9.5 11.5 6"/>',
  exportFile: '<path d="M2.5 10.5v3h11v-3"/><path d="M8 9.5V2M4.5 5.5 8 2l3.5 3.5"/>',
};

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

  // Shared hidden .xlsx file input (File menu + Data tab use the same one).
  const fileInput = document.createElement('input');
  fileInput.type = 'file';
  fileInput.accept = '.xlsx';
  fileInput.style.display = 'none';
  fileInput.addEventListener('change', () => {
    const f = fileInput.files?.[0];
    if (f) cb.importXlsx?.(f);
    fileInput.value = '';
  });
  bar.appendChild(fileInput);

  // --- File tab: green, opens a dropdown menu instead of switching tabs ---
  const fileWrap = document.createElement('div');
  fileWrap.className = 'sheet-file-wrap';
  const fileBtn = document.createElement('button');
  fileBtn.className = 'sheet-file-tab';
  fileBtn.textContent = 'File';
  const fileMenu = document.createElement('div');
  fileMenu.className = 'sheet-file-menu';
  fileMenu.style.display = 'none';
  const closeMenu = () => {
    fileMenu.style.display = 'none';
    document.removeEventListener('mousedown', onOutside, true);
  };
  const onOutside = (e: MouseEvent) => {
    if (!fileWrap.contains(e.target as Node)) closeMenu();
  };
  fileBtn.addEventListener('click', () => {
    if (fileMenu.style.display === 'none') {
      fileMenu.style.display = '';
      document.addEventListener('mousedown', onOutside, true);
    } else closeMenu();
  });
  const fileMenuItem = (label: string, onClick: () => void) => {
    const b = document.createElement('button');
    b.textContent = label;
    b.addEventListener('click', () => { closeMenu(); onClick(); });
    fileMenu.appendChild(b);
  };
  if (cb.importXlsx) fileMenuItem('Import (.xlsx)', () => fileInput.click());
  if (cb.exportXlsx) fileMenuItem('Export (.xlsx)', () => cb.exportXlsx?.());
  fileWrap.append(fileBtn, fileMenu);
  tabsEl.appendChild(fileWrap);

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

  // Creates a group under a tab; returns its content container.
  const group = (tab: TabName, label: string): HTMLElement => {
    const g = document.createElement('div');
    g.className = 'sheet-ribbon-group';
    const content = document.createElement('div');
    content.className = 'sheet-ribbon-content';
    const lbl = document.createElement('div');
    lbl.className = 'sheet-ribbon-label';
    lbl.textContent = label;
    g.append(content, lbl);
    body.appendChild(g);
    groups.push({ tab, el: g });
    return content;
  };
  const row = (parent: HTMLElement): HTMLElement => {
    const r = document.createElement('div');
    r.className = 'sheet-ribbon-row';
    parent.appendChild(r);
    return r;
  };
  const col = (parent: HTMLElement): HTMLElement => {
    const c = document.createElement('div');
    c.className = 'sheet-ribbon-col';
    parent.appendChild(c);
    return c;
  };

  // Small icon-only (or short-text) button.
  const btn = (parent: HTMLElement, content: { icon?: string; text?: string }, title: string, onClick: () => void): HTMLButtonElement => {
    const b = document.createElement('button');
    if (content.icon) b.innerHTML = svg(content.icon);
    if (content.text) b.append(content.text);
    b.title = title;
    b.addEventListener('click', onClick);
    parent.appendChild(b);
    return b;
  };
  // Big Excel-style button: icon (or glyph) on top, label below.
  const bigBtn = (parent: HTMLElement, icon: string, label: string, title: string, onClick: () => void): HTMLButtonElement => {
    const b = document.createElement('button');
    b.className = 'sheet-ribbon-big';
    b.innerHTML = svg(icon, 28);
    b.append(label);
    b.title = title;
    b.addEventListener('click', onClick);
    parent.appendChild(b);
    return b;
  };

  const curProps = () => { const f = cb.focusCell(); return cb.getProps(f.row, f.col); };
  // Toggle a boolean style prop; `on` is derived from the focus cell's props.
  const toggleBtn = (parent: HTMLElement, content: { icon?: string; text?: string }, title: string, key: string): HTMLButtonElement => {
    const b = btn(parent, content, title, () => {
      const on = curProps()[key] === '1';
      cb.applyToSelection({ [key]: on ? '' : '1' });
    });
    b.dataset.key = key;
    return b;
  };

  // --- Home: Clipboard ---
  if (cb.clipboardAction) {
    const clip = group('Home', 'Clipboard');
    bigBtn(clip, IC.paste, 'Paste', 'Paste', () => cb.clipboardAction?.('paste'));
    const small = col(clip);
    btn(small, { icon: IC.cut }, 'Cut', () => cb.clipboardAction?.('cut'));
    btn(small, { icon: IC.copy }, 'Copy', () => cb.clipboardAction?.('copy'));
  }

  // --- Home: Font ---
  const fontCol = col(group('Home', 'Font'));
  const fontTop = row(fontCol);
  const fontFamily = document.createElement('select');
  fontFamily.className = 'sheet-font-family';
  fontFamily.title = 'Font';
  for (const f of ['Calibri', 'Arial', 'Times New Roman', 'Courier New', 'Georgia', 'Verdana']) {
    const o = document.createElement('option');
    o.value = f;
    o.textContent = f;
    fontFamily.appendChild(o);
  }
  fontFamily.addEventListener('change', () => cb.applyToSelection({ fontFamily: fontFamily.value }));
  fontTop.appendChild(fontFamily);
  const fontSize = document.createElement('select');
  fontSize.className = 'sheet-font-size';
  fontSize.title = 'Font size';
  for (const n of [8, 9, 10, 11, 12, 14, 16, 18, 20, 24, 28, 36, 48]) {
    const o = document.createElement('option');
    o.value = String(n);
    o.textContent = String(n);
    fontSize.appendChild(o);
  }
  fontSize.value = '11';
  fontSize.addEventListener('change', () => cb.applyToSelection({ fontSize: fontSize.value }));
  fontTop.appendChild(fontSize);

  const fontRow = row(fontCol);
  toggleBtn(fontRow, { text: 'B' }, 'bold', 'bold').style.fontWeight = 'bold';
  toggleBtn(fontRow, { text: 'I' }, 'italic', 'italic').style.fontStyle = 'italic';
  toggleBtn(fontRow, { text: 'U' }, 'underline', 'underline').style.textDecoration = 'underline';
  btn(fontRow, { icon: IC.borders }, 'Borders', () => {
    const on = curProps().border === 'all';
    cb.applyToSelection({ border: on ? '' : 'all' });
  });
  const bg = document.createElement('input');
  bg.type = 'color';
  bg.title = 'Fill color';
  bg.value = '#ffffff';
  bg.addEventListener('input', () => cb.applyToSelection({ bg: bg.value }));
  fontRow.appendChild(bg);
  const color = document.createElement('input');
  color.type = 'color';
  color.title = 'Font color';
  color.addEventListener('input', () => cb.applyToSelection({ color: color.value }));
  fontRow.appendChild(color);

  // --- Home: Alignment ---
  const alignRow = group('Home', 'Alignment');
  const alignIcons = { left: IC.alignLeft, center: IC.alignCenter, right: IC.alignRight } as const;
  for (const a of ['left', 'center', 'right'] as const) {
    btn(alignRow, { icon: alignIcons[a] }, `Align ${a}`, () => cb.applyToSelection({ align: a }));
  }
  toggleBtn(alignRow, { icon: IC.wrap }, 'Wrap text', 'wrap');

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
    const cells = col(group('Home', 'Cells'));
    const acts = [
      ['insRowAbove', IC.insRowAbove, 'Insert row above'],
      ['insRowBelow', IC.insRowBelow, 'Insert row below'],
      ['insColLeft', IC.insColLeft, 'Insert column left'],
      ['insColRight', IC.insColRight, 'Insert column right'],
      ['delRows', IC.delRows, 'Delete selected rows'],
      ['delCols', IC.delCols, 'Delete selected columns'],
    ] as const;
    const r1 = row(cells);
    const r2 = row(cells);
    acts.forEach(([act, icon, title], i) => {
      btn(i < 3 ? r1 : r2, { icon }, title, () => cb.structural?.(act)).dataset.act = act;
    });
  }

  // --- Home: Editing ---
  if (cb.autoSum) {
    const editing = group('Home', 'Editing');
    btn(editing, { text: 'Σ' }, 'AutoSum', () => cb.autoSum?.());
  }

  // --- Data: Get & Transform ---
  if (cb.importXlsx || cb.exportXlsx) {
    const gt = group('Data', 'Get & Transform');
    if (cb.importXlsx) {
      bigBtn(gt, IC.importFile, 'Import', 'Import .xlsx (replaces this sheet)', () => fileInput.click());
    }
    if (cb.exportXlsx) {
      bigBtn(gt, IC.exportFile, 'Export', 'Export as .xlsx', () => cb.exportXlsx?.());
    }
  }

  // --- Data: Sort & Filter ---
  const sf = col(group('Data', 'Sort & Filter'));
  const sortRow = row(sf);
  btn(sortRow, { text: 'A→Z' }, 'Sort selection ascending by the focused column', () => cb.sortSelection?.(true));
  btn(sortRow, { text: 'Z→A' }, 'Sort selection descending by the focused column', () => cb.sortSelection?.(false));
  const filterRow = row(sf);
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

  // --- View: Freeze panes ---
  const freeze = group('View', 'Freeze panes');
  const mkFreeze = (label: string, kind: 'row' | 'col', title: string) => {
    const b = document.createElement('button');
    b.className = 'sheet-ribbon-big';
    // Glyph span on top + text node below; textContent stays exactly '❄ Row'.
    const glyph = document.createElement('span');
    glyph.className = 'sheet-big-glyph';
    glyph.textContent = '❄';
    b.append(glyph, ` ${label}`);
    b.title = title;
    const sync = () => {
      const fz = cb.frozenState?.() ?? { rows: 0, cols: 0 };
      b.classList.toggle('on', kind === 'row' ? fz.rows > 0 : fz.cols > 0);
    };
    b.addEventListener('click', () => {
      cb.toggleFreeze?.(kind);
      sync();
    });
    sync();
    freeze.appendChild(b);
  };
  mkFreeze('Row', 'row', 'Freeze first row');
  mkFreeze('Col', 'col', 'Freeze first column');

  selectTab('Home');
  return bar;
}

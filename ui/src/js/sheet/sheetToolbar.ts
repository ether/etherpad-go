// Minimal formatting toolbar. Emits style-prop *changes* for the current
// selection; the editor merges them onto each cell's existing props and sends
// setStyle ops. Uses native inputs (no dependency).

export interface ToolbarCallbacks {
  getProps: (row: number, col: number) => Record<string, string>;
  focusCell: () => { row: number; col: number };
  applyToSelection: (change: Record<string, string>) => void;
  readOnly: boolean;
}

const CSS = `
.sheet-toolbar { display: flex; gap: 4px; align-items: center; padding: 4px; border-bottom: 1px solid #d2d2d2; font: 13px system-ui, sans-serif; flex-wrap: wrap; }
.sheet-toolbar button, .sheet-toolbar select { height: 24px; min-width: 24px; cursor: pointer; }
.sheet-toolbar button.on { background: #cfeede; }
.sheet-toolbar input[type=color] { width: 26px; height: 24px; padding: 0; border: 1px solid #ccc; }
.sheet-toolbar[aria-disabled=true] { opacity: 0.5; pointer-events: none; }
`;

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

  const curProps = () => { const f = cb.focusCell(); return cb.getProps(f.row, f.col); };

  const toggleBtn = (label: string, key: string) => {
    const b = document.createElement('button');
    b.textContent = label;
    b.title = key;
    b.dataset.key = key;
    b.addEventListener('click', () => {
      const on = curProps()[key] === '1';
      cb.applyToSelection({ [key]: on ? '' : '1' });
    });
    bar.appendChild(b);
    return b;
  };
  toggleBtn('B', 'bold').style.fontWeight = 'bold';
  toggleBtn('I', 'italic').style.fontStyle = 'italic';
  toggleBtn('U', 'underline').style.textDecoration = 'underline';

  const color = document.createElement('input');
  color.type = 'color'; color.title = 'Font color';
  color.addEventListener('input', () => cb.applyToSelection({ color: color.value }));
  bar.appendChild(color);

  const bg = document.createElement('input');
  bg.type = 'color'; bg.title = 'Fill color'; bg.value = '#ffffff';
  bg.addEventListener('input', () => cb.applyToSelection({ bg: bg.value }));
  bar.appendChild(bg);

  const align = document.createElement('select');
  align.title = 'Align';
  for (const a of ['left', 'center', 'right']) {
    const o = document.createElement('option'); o.value = a; o.textContent = a; align.appendChild(o);
  }
  align.addEventListener('change', () => cb.applyToSelection({ align: align.value }));
  bar.appendChild(align);

  const border = document.createElement('button');
  border.textContent = '▦'; border.title = 'Borders';
  border.addEventListener('click', () => {
    const on = curProps().border === 'all';
    cb.applyToSelection({ border: on ? '' : 'all' });
  });
  bar.appendChild(border);

  const numFmt = document.createElement('select');
  numFmt.title = 'Number format';
  for (const [v, label] of [['general', 'General'], ['number:2', 'Number'], ['currency:2', 'Currency'], ['percent:0', 'Percent'], ['date', 'Date'], ['text', 'Text']] as const) {
    const o = document.createElement('option'); o.value = v; o.textContent = label; numFmt.appendChild(o);
  }
  numFmt.addEventListener('change', () => cb.applyToSelection({ numFmt: numFmt.value }));
  bar.appendChild(numFmt);

  return bar;
}

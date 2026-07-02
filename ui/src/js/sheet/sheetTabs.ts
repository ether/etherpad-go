// Bottom tabs bar: one tab per sheet. Click switches, double-click renames
// (native prompt), right-click deletes (native confirm), drag reorders,
// '+' adds. Pure DOM, no framework — call refresh() after workbook changes.

export interface TabsCallbacks {
  sheets: () => Array<{ id: string; name: string }>;
  activeId: () => string;
  onSwitch: (id: string) => void;
  onAdd: () => void;
  onRename: (id: string, name: string) => void;
  onDelete: (id: string) => void;
  onMove: (id: string, toIndex: number) => void;
  readOnly?: boolean;
}

const STYLE_ID = 'sheet-tabs-style';
const CSS = `
.sheet-tabs { display: flex; gap: 1px; align-items: center; padding: 0 6px; font: 12px/1.4 system-ui, sans-serif; background: #f5f6f7; border-top: 1px solid #d4d8dd; }
.sheet-tabs button { border: none; border-bottom: 2px solid transparent; background: #fff; color: #5f6b7a; padding: 5px 14px 3px; cursor: pointer; }
.sheet-tabs button:hover { color: #107c41; }
.sheet-tabs button.sheet-tab-active { background: #fff; color: #107c41; font-weight: 600; border-bottom-color: #107c41; }
.sheet-tabs button.sheet-tab-add { width: 22px; height: 22px; padding: 0; margin-left: 4px; border: none; border-radius: 50%; background: none; color: #5f6b7a; font-weight: 700; font-size: 14px; line-height: 1; }
.sheet-tabs button.sheet-tab-add:hover { background: #e6f2ec; color: #107c41; }
`;

export function createSheetTabs(cb: TabsCallbacks): { el: HTMLElement; refresh: () => void } {
  if (!document.getElementById(STYLE_ID)) {
    const style = document.createElement('style');
    style.id = STYLE_ID;
    style.textContent = CSS;
    document.head.appendChild(style);
  }

  const el = document.createElement('div');
  el.className = 'sheet-tabs';

  const refresh = (): void => {
    el.innerHTML = '';
    const sheets = cb.sheets();
    sheets.forEach((s, i) => {
      const tab = document.createElement('button');
      tab.textContent = s.name || s.id;
      tab.dataset.sheetId = s.id;
      tab.classList.toggle('sheet-tab-active', s.id === cb.activeId());
      tab.addEventListener('click', () => cb.onSwitch(s.id));
      if (!cb.readOnly) {
        tab.addEventListener('dblclick', () => {
          const name = prompt('Sheet name', s.name);
          if (name && name !== s.name) cb.onRename(s.id, name);
        });
        tab.addEventListener('contextmenu', (e) => {
          e.preventDefault();
          if (sheets.length <= 1) return; // the last sheet cannot be deleted
          if (confirm(`Delete sheet "${s.name}"?`)) cb.onDelete(s.id);
        });
        tab.draggable = true;
        tab.addEventListener('dragstart', (e) => e.dataTransfer?.setData('text/sheet-id', s.id));
        tab.addEventListener('dragover', (e) => e.preventDefault());
        tab.addEventListener('drop', (e) => {
          e.preventDefault();
          const dragged = e.dataTransfer?.getData('text/sheet-id');
          if (dragged && dragged !== s.id) cb.onMove(dragged, i);
        });
      }
      el.appendChild(tab);
    });
    if (!cb.readOnly) {
      const add = document.createElement('button');
      add.className = 'sheet-tab-add';
      add.textContent = '+';
      add.title = 'Add sheet';
      add.addEventListener('click', () => cb.onAdd());
      el.appendChild(add);
    }
  };

  refresh();
  return { el, refresh };
}

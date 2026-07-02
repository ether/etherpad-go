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
.sheet-tabs { display: flex; gap: 2px; align-items: center; padding: 4px 2px; font: 12px/1.4 system-ui, sans-serif; }
.sheet-tabs button { border: 1px solid #d2d2d2; background: #f2f3f4; color: #485365; padding: 3px 12px; cursor: pointer; border-radius: 0 0 4px 4px; }
.sheet-tabs button.sheet-tab-active { background: #fff; font-weight: 600; border-top-color: #fff; }
.sheet-tabs button.sheet-tab-add { padding: 3px 8px; font-weight: 700; }
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

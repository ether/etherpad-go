// The version history panel: the versions of the workbook on the right, the
// chosen one shown read-only on the left, with "Restore this version".

import { renderChart } from './chartRender';
import { openDialog } from './sheetDialog';
import { fetchHistory, fetchVersion, groupRevisions, restoreVersion, type HistoryResponse, type Version } from './sheetHistory';
import { RENDER_CSS, renderSheetHtml, workbookEngine } from './sheetRender';
import { WorkbookState, type WorkbookSnapshot } from './workbookState';

const CSS = `
.sheet-history { position: fixed; inset: 0; z-index: 50; display: flex; flex-direction: column; background: #fff; font: 13px system-ui, sans-serif; color: #333; }
.sheet-history-head { display: flex; align-items: center; gap: 10px; height: 40px; flex: none; padding: 0 12px; background: #107c41; color: #fff; }
.sheet-history-head .sheet-history-title { font-weight: 600; flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.sheet-history-head button { height: 28px; padding: 0 12px; border: 1px solid #fff; border-radius: 3px; background: #107c41; color: #fff; font: inherit; cursor: pointer; }
.sheet-history-head button.primary { background: #fff; color: #107c41; font-weight: 600; }
.sheet-history-head button[hidden] { display: none; }
.sheet-history-main { flex: 1; display: flex; min-height: 0; }
.sheet-history-preview { flex: 1; display: flex; flex-direction: column; min-width: 0; background: #f5f6f7; }
.sheet-history-tabs { display: flex; gap: 2px; padding: 4px 8px 0; border-bottom: 1px solid #d4d8dd; flex: none; }
.sheet-history-tabs button { border: 1px solid transparent; border-bottom: none; background: none; padding: 4px 12px; font: inherit; color: #5a5f66; cursor: pointer; border-radius: 3px 3px 0 0; }
.sheet-history-tabs button.on { background: #fff; border-color: #d4d8dd; color: #107c41; font-weight: 600; }
.sheet-history-sheet { flex: 1; overflow: auto; padding: 12px; background: #fff; }
.sheet-history-list { width: 290px; flex: none; overflow: auto; border-left: 1px solid #d4d8dd; }
.sheet-history-list button { display: block; width: 100%; text-align: left; border: none; border-bottom: 1px solid #eceef0; background: none; padding: 9px 12px; font: inherit; color: inherit; cursor: pointer; }
.sheet-history-list button:hover { background: #e6f2ec; }
.sheet-history-list button.on { background: #cce5d8; }
.sheet-history-when { font-weight: 600; }
.sheet-history-label { font-size: 11px; color: #107c41; margin-left: 6px; font-weight: 600; }
.sheet-history-who { display: flex; flex-wrap: wrap; gap: 4px 10px; margin-top: 3px; font-size: 12px; color: #555; }
.sheet-history-who span::before { content: ''; display: inline-block; width: 8px; height: 8px; border-radius: 50%; margin-right: 4px; background: var(--c, #999); }
.sheet-history-note { padding: 16px; color: #666; }
`;

export interface HistoryPanelOptions {
  readOnly: boolean;
  title: string;
  // Called after a restore went through (its ops arrive over the socket).
  onRestored?: (when: string) => void;
}

const when = (ts: number): string =>
  new Date(ts).toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' });

export interface HistoryPanel {
  close: () => void;
  // Reads the versions again (the sheet changed): the newest becomes the
  // current one, and if it was shown, the new current one is shown.
  refresh: () => void;
  isOpen: () => boolean;
}

export function openHistoryPanel(opts: HistoryPanelOptions): HistoryPanel {
  if (!document.getElementById('sheet-history-style')) {
    const s = document.createElement('style');
    s.id = 'sheet-history-style';
    s.textContent = CSS + RENDER_CSS;
    document.head.appendChild(s);
  }
  document.querySelector('.sheet-history')?.remove();
  const panel = document.createElement('div');
  panel.className = 'sheet-history';
  panel.setAttribute('role', 'dialog');
  panel.setAttribute('aria-label', 'Version history');
  panel.tabIndex = -1;

  const head = document.createElement('div');
  head.className = 'sheet-history-head';
  const title = document.createElement('div');
  title.className = 'sheet-history-title';
  title.textContent = `Version history — ${opts.title}`;
  const restoreBtn = document.createElement('button');
  restoreBtn.className = 'primary sheet-history-restore';
  restoreBtn.textContent = 'Restore this version';
  restoreBtn.hidden = true;
  const closeBtn = document.createElement('button');
  closeBtn.className = 'sheet-history-close';
  closeBtn.textContent = 'Close';
  head.append(title, restoreBtn, closeBtn);

  const main = document.createElement('div');
  main.className = 'sheet-history-main';
  const preview = document.createElement('div');
  preview.className = 'sheet-history-preview';
  const tabs = document.createElement('div');
  tabs.className = 'sheet-history-tabs';
  const sheetEl = document.createElement('div');
  sheetEl.className = 'sheet-history-sheet';
  preview.append(tabs, sheetEl);
  const list = document.createElement('div');
  list.className = 'sheet-history-list';
  list.setAttribute('role', 'listbox');
  main.append(preview, list);
  panel.append(head, main);
  document.body.appendChild(panel);
  panel.focus();

  const note = (el: HTMLElement, text: string): void => {
    el.textContent = '';
    const p = document.createElement('div');
    p.className = 'sheet-history-note';
    p.textContent = text;
    el.appendChild(p);
  };

  const close = (): void => {
    open = false;
    panel.remove();
    document.removeEventListener('keydown', onKey, true);
  };
  const onKey = (e: KeyboardEvent): void => {
    if (e.key === 'Escape' && !document.querySelector('.sheet-dialog-back')) {
      e.preventDefault();
      e.stopPropagation();
      close();
    }
  };
  document.addEventListener('keydown', onKey, true);
  closeBtn.addEventListener('click', close);

  let shown = 0; // the load that counts, against late answers
  let chosen: { rev: number; label: string; current: boolean } | null = null;
  let open = true;

  const showVersion = (rev: number, label: string, current: boolean): void => {
    if (chosen && chosen.rev === rev && sheetEl.querySelector('.sheet-render')) {
      // Already shown; only whether it is the current one may change.
      chosen = { rev, label, current };
      restoreBtn.hidden = opts.readOnly || current;
      return;
    }
    const load = ++shown;
    chosen = { rev, label, current };
    restoreBtn.hidden = true;
    tabs.textContent = '';
    note(sheetEl, 'Loading…');
    void fetchVersion<WorkbookSnapshot>(rev).then((snap) => {
      if (load !== shown) return;
      const wb = new WorkbookState();
      wb.loadSnapshot(snap);
      const engine = workbookEngine(wb);
      const draw = (sheetId: string): void => {
        for (const b of tabs.querySelectorAll('button')) b.classList.toggle('on', b.dataset.sheet === sheetId);
        sheetEl.innerHTML = renderSheetHtml(wb, sheetId, engine, {
          chartSvg: (chart, data) => renderChart(chart, data, chart.width, chart.height).outerHTML,
        });
      };
      for (const sh of wb.sheets) {
        const b = document.createElement('button');
        b.textContent = sh.name;
        b.dataset.sheet = sh.id;
        b.addEventListener('click', () => draw(sh.id));
        tabs.appendChild(b);
      }
      if (wb.sheets.length > 0) draw(wb.sheets[0].id);
      else note(sheetEl, 'This version has no sheets.');
      restoreBtn.hidden = opts.readOnly || current;
    }).catch((err: Error) => {
      if (load === shown) note(sheetEl, `This version could not be loaded: ${err.message}`);
    });
  };

  restoreBtn.addEventListener('click', () => {
    const target = chosen;
    if (!target) return;
    openDialog({
      title: 'Restore this version?',
      fields: [],
      okLabel: 'Restore',
      onOk: () => {
        restoreBtn.disabled = true;
        void restoreVersion(target.rev).then(() => {
          close();
          opts.onRestored?.(target.label);
        }).catch((err: Error) => {
          restoreBtn.disabled = false;
          note(sheetEl, `Restoring failed: ${err.message}`);
        });
        return null;
      },
    });
  });

  const fill = (h: HistoryResponse): void => {
    list.textContent = '';
    if (!h.available) {
      note(list, 'Earlier versions of this sheet are not known: it was imported before the version history was kept.');
      note(sheetEl, 'Only the current version is available.');
      return;
    }
    const versions: Version[] = groupRevisions(h.revisions);
    // The state the changes start from: created empty or imported.
    versions.push({ rev: 0, from: 0, start: 0, end: 0, authors: [] });
    const buttons: HTMLButtonElement[] = [];
    versions.forEach((v, i) => {
      const b = document.createElement('button');
      b.setAttribute('role', 'option');
      b.dataset.rev = String(v.rev);
      const w = document.createElement('div');
      w.className = 'sheet-history-when';
      const label = v.rev === 0 ? 'Original version' : when(v.end);
      w.textContent = label;
      if (i === 0) {
        const cur = document.createElement('span');
        cur.className = 'sheet-history-label';
        cur.textContent = 'Current version';
        w.appendChild(cur);
      }
      b.appendChild(w);
      if (v.authors.length > 0) {
        const who = document.createElement('div');
        who.className = 'sheet-history-who';
        for (const id of v.authors) {
          const a = h.authors[id];
          const s = document.createElement('span');
          s.textContent = a?.name || 'Anonymous';
          if (a?.color && /^#[0-9a-fA-F]{3,8}$/.test(a.color)) s.style.setProperty('--c', a.color);
          who.appendChild(s);
        }
        b.appendChild(who);
      }
      b.addEventListener('click', () => {
        for (const o of buttons) o.classList.toggle('on', o === b);
        showVersion(v.rev, label, i === 0);
      });
      buttons.push(b);
      list.appendChild(b);
    });
    // Keep showing what was chosen; the current version stays chosen as
    // the current one, also when it moved on.
    const keep = chosen && !chosen.current ? buttons.find((b) => b.dataset.rev === String(chosen?.rev)) : undefined;
    (keep ?? buttons[0])?.click();
  };

  let loads = 0;
  const load = (): void => {
    const n = ++loads;
    void fetchHistory().then((h) => {
      if (n === loads && open) fill(h);
    }).catch((err: Error) => {
      if (n === loads && open) note(list, `The history could not be loaded: ${err.message}`);
    });
  };
  note(list, 'Loading…');
  load();
  return { close, refresh: load, isOpen: () => open };
}

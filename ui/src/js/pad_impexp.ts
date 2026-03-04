import html10n from './i18n';

type ImportResponse = {
  code: number;
  message: string;
  data?: {
    directDatabaseAccess?: boolean;
  };
};

const visible = (el: Element | null): boolean => {
  if (!(el instanceof HTMLElement)) return false;
  return getComputedStyle(el).display !== 'none';
};

const getRequiredElement = <T extends Element>(selector: string): T => {
  const element = document.querySelector(selector);
  if (element == null) throw new Error(`Required element missing: ${selector}`);
  return element as T;
};

const hide = (selector: string): void => {
  const el = document.querySelector(selector);
  if (el instanceof HTMLElement) el.style.display = 'none';
};

const show = (selector: string): void => {
  const el = document.querySelector(selector);
  if (el instanceof HTMLElement) el.style.display = '';
};

const setOpacity = (selector: string, value: string): void => {
  const el = document.querySelector(selector);
  if (el instanceof HTMLElement) el.style.opacity = value;
};

const addImportFrames = (): void => {
  const importContainer = document.querySelector<HTMLElement>('#import');
  if (importContainer == null) return;
  for (const frame of document.querySelectorAll('#import .importframe')) frame.remove();
  const iframe = document.createElement('iframe');
  iframe.style.display = 'none';
  iframe.name = 'importiframe';
  iframe.classList.add('importframe');
  importContainer.appendChild(iframe);
};

const fileInputUpdated = (): void => {
  const importSubmitInput = document.querySelector<HTMLInputElement>('#importsubmitinput');
  if (importSubmitInput == null) return;
  importSubmitInput.classList.add('throbbold');
  document.querySelector<HTMLElement>('#importformfilediv')?.classList.add('importformenabled');
  importSubmitInput.disabled = false;
  hide('#importmessagefail');
};

const toImportResponse = async (response: Response): Promise<ImportResponse> => {
  const fallback: ImportResponse = {code: 2, message: 'Unknown import error'};
  try {
    return await response.json() as ImportResponse;
  } catch {
    return fallback;
  }
};

const postImport = async (form: HTMLFormElement): Promise<ImportResponse> => {
  const controller = new AbortController();
  const timeout = window.setTimeout(() => controller.abort(), 25000);
  try {
    const response = await fetch(`${window.location.href.split('?')[0].split('#')[0]}/import`, {
      method: 'POST',
      body: new FormData(form),
      signal: controller.signal,
    });
    return await toImportResponse(response);
  } catch {
    return {code: 2, message: 'Unknown import error'};
  } finally {
    clearTimeout(timeout);
  }
};

const importErrorMessage = (status: string): void => {
  const known = new Set([
    'convertFailed',
    'uploadFailed',
    'padHasData',
    'maxFileSize',
    'permission',
  ]);
  const msg = html10n.get(`pad.impexp.${known.has(status) ? status : 'copypaste'}`);
  const popup = document.querySelector<HTMLElement>('#importmessagefail');
  if (popup == null) return;
  popup.textContent = '';
  const strong = document.createElement('strong');
  strong.style.color = 'red';
  strong.textContent = `${html10n.get('pad.impexp.importfailed')}: `;
  popup.appendChild(strong);
  popup.appendChild(document.createTextNode(msg));
  popup.style.display = '';
};

const cantExport = (event: Event): boolean => {
  const target = event.currentTarget;
  if (!(target instanceof HTMLElement)) return false;

  let type = 'this file';
  if (target.classList.contains('exporthrefpdf')) {
    type = 'PDF';
  } else if (target.classList.contains('exporthrefdoc')) {
    type = 'Microsoft Word';
  } else if (target.classList.contains('exporthrefodt')) {
    type = 'OpenDocument';
  }
  alert(html10n.get('pad.impexp.exportdisabled', {type}));
  event.preventDefault();
  return false;
};

const fileInputSubmit = async function (this: HTMLFormElement, event: SubmitEvent): Promise<void> {
  event.preventDefault();
  hide('#importmessagefail');
  if (!window.confirm(html10n.get('pad.impexp.confirmimport'))) return;

  const importSubmitInput = getRequiredElement<HTMLInputElement>('#importsubmitinput');
  const importFileInput = getRequiredElement<HTMLInputElement>('#importfileinput');

  importSubmitInput.disabled = true;
  importSubmitInput.value = html10n.get('pad.impexp.importing');
  window.setTimeout(() => {
    importFileInput.disabled = true;
  }, 0);

  hide('#importarrow');
  show('#importstatusball');

  const {code, message, data: {directDatabaseAccess} = {}} = await postImport(this);
  if (code !== 0) {
    importErrorMessage(message);
  } else {
    const importExportPopup = document.getElementById('import_export');
    importExportPopup?.classList.remove('popup-show');
    if (directDatabaseAccess) window.location.reload();
  }

  importSubmitInput.disabled = false;
  importSubmitInput.value = html10n.get('pad.impexp.importbutton');
  window.setTimeout(() => {
    importFileInput.disabled = false;
  }, 0);
  hide('#importstatusball');
  addImportFrames();
};

const setImportButtonLabel = (): void => {
  const importSubmitInput = document.getElementById('importsubmitinput');
  if (importSubmitInput instanceof HTMLInputElement) {
    importSubmitInput.value = html10n.get('pad.impexp.importbutton');
  }
};

export const padimpexp = {
  init: (_pad: unknown): void => {
    void _pad;
    const pathMatch = /.*\/p\/[^/]+/.exec(document.location.pathname);
    const clientVars = window.clientVars as Record<string, unknown>;
    const padRootPath = pathMatch?.[0] ?? String(clientVars.padId ?? '');

    setImportButtonLabel();
    html10n.bind('localized', () => {
      setImportButtonLabel();
    });

    const availableExports = (clientVars.availableExports ?? []) as string[];
    for (const exportOption of availableExports) {
      const exportNode = document.getElementById(`export${exportOption}a`);
      if (!(exportNode instanceof HTMLAnchorElement)) continue;
      exportNode.style.display = '';
      exportNode.href = `${padRootPath}/export/${exportOption}`;
    }

    const importForm = document.querySelector<HTMLFormElement>('#importform');
    const importFileInput = document.querySelector<HTMLInputElement>('#importfileinput');
    if (importForm != null && importFileInput != null) {
      addImportFrames();
      importFileInput.onchange = fileInputUpdated;
      importForm.onsubmit = (event) => {
        void fileInputSubmit.call(importForm, event);
      };
    }

    for (const exportButton of document.querySelectorAll('.disabledexport')) {
      exportButton.removeEventListener('click', cantExport);
      exportButton.addEventListener('click', cantExport);
    }

    const importMessage = document.querySelector('#importexport .importmessage');
    if (visible(importMessage)) {
      hide('#importmessagesuccess');
      hide('#importmessagefail');
    }
  },
  disable: (): void => {
    show('#impexp-disabled-clickcatcher');
    setOpacity('#import', '0.5');
    setOpacity('#impexp-export', '0.5');
  },
  enable: (): void => {
    hide('#impexp-disabled-clickcatcher');
    setOpacity('#import', '1');
    setOpacity('#impexp-export', '1');
  },
};

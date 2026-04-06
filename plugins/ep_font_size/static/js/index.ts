const sizes = ['8', '9', '10', '11', '12', '13', '14', '16', '18', '20', '24', '28', '36', '48', '60'] as const;
const sizeRegex = /(?:^| )font-size:(\d+px)/;

// Inject plugin CSS into the outer page for the toolbar icon
let cssLoaded = false;
const ensureCSS = () => {
  if (cssLoaded) return;
  cssLoaded = true;
  const link = document.createElement('link');
  link.rel = 'stylesheet';
  link.href = '/static/plugins/ep_font_size/static/css/size.css';
  document.head.appendChild(link);
};
ensureCSS();

// --- ACE hooks ---

export const aceEditorCSS = (): string[] => ['ep_font_size/static/css/size.css'];

export const aceAttribsToClasses = (_hookName: string, context: { key: string; value: string }) => {
  if (context.key.includes('font-size:')) {
    const m = sizeRegex.exec(context.key);
    if (m) return [`font-size:${m[1]}`];
  }
  if (context.key === 'font-size') {
    return [`font-size:${context.value}`];
  }
};

export const aceCreateDomLine = (_hookName: string, context: { cls: string }) => {
  const m = sizeRegex.exec(context.cls);
  if (!m) return [];
  const sizeVal = m[1].replace('px', '');
  const idx = sizes.indexOf(sizeVal as typeof sizes[number]);
  if (idx < 0) return [];
  return [{ extraOpenTags: '', extraCloseTags: '', cls: context.cls }];
};

const doInsertSizes = function (this: any, level: number) {
  const rep = this.rep;
  const documentAttributeManager = this.documentAttributeManager;
  if (!(rep.selStart && rep.selEnd) || (level >= 0 && sizes[level] === undefined)) return;

  const newSize: [string, string] = level >= 0 ? ['font-size', sizes[level] + 'px'] : ['font-size', ''];
  documentAttributeManager.setAttributesOnRange(rep.selStart, rep.selEnd, [newSize]);
};

export const aceInitialized = (_hookName: string, context: { editorInfo: any }) => {
  context.editorInfo.ace_doInsertSizes = doInsertSizes.bind(context);
};

// --- Toolbar hooks ---

export const postToolbarInit = (_hookName: string, context: { toolbar: any }) => {
  const editbar = context.toolbar;
  editbar.registerCommand('fontSize', () => {
    const dropdown = document.getElementById('font-size-dropdown');
    if (dropdown) dropdown.style.display = dropdown.style.display === 'none' ? '' : 'none';
  });
};

export const postAceInit = (_hookName: string, context: { ace: any }) => {
  // Create the size dropdown and insert it after the toolbar button
  const btn = document.querySelector('[data-key="fontSize"]') as HTMLElement | null;
  if (btn && !document.getElementById('font-size-dropdown')) {
    const dropdown = document.createElement('div');
    dropdown.id = 'font-size-dropdown';
    dropdown.style.display = 'none';
    dropdown.style.position = 'absolute';
    dropdown.style.zIndex = '1000';
    dropdown.style.background = '#fff';
    dropdown.style.border = '1px solid #ccc';
    dropdown.style.borderRadius = '4px';
    dropdown.style.padding = '4px';
    dropdown.style.boxShadow = '0 2px 8px rgba(0,0,0,0.15)';

    sizes.forEach((size, idx) => {
      const swatch = document.createElement('span');
      swatch.style.display = 'inline-block';
      swatch.style.padding = '2px 8px';
      swatch.style.margin = '2px';
      swatch.style.cursor = 'pointer';
      swatch.style.border = '1px solid #999';
      swatch.style.borderRadius = '3px';
      swatch.textContent = size;
      swatch.title = size + 'px';
      swatch.addEventListener('click', () => {
        context.ace.callWithAce((ace: any) => {
          ace.ace_doInsertSizes(idx);
        }, 'insertSize', true);
        dropdown.style.display = 'none';
      });
      dropdown.appendChild(swatch);
    });

    btn.style.position = 'relative';
    btn.appendChild(dropdown);
  }
};

export const aceEditEvent = (_hookName: string, call: any) => {
  const cs = call.callstack;
  if (!['handleClick', 'handleKeyEvent'].includes(cs.type) && !cs.docTextChanged) return;
  if (cs.type === 'setBaseText' || cs.type === 'setup') return;
};

// --- Content collection ---

export const collectContentPre = (_hookName: string, context: any) => {
  const m = sizeRegex.exec(context.cls);
  if (m?.[1]) {
    context.cc.doAttrib(context.state, `font-size::${m[1]}`);
  }
};

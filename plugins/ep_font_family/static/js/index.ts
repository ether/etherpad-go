const fonts = ['arial', 'avant-garde', 'bookman', 'calibri', 'courier', 'garamond', 'helvetica', 'monospace', 'palatino', 'times-new-roman'] as const;
const fontLabels = ['Arial', 'Avant Garde', 'Bookman', 'Calibri', 'Courier', 'Garamond', 'Helvetica', 'Monospace', 'Palatino', 'Times New Roman'] as const;
const fontRegex = /(?:^| )font([a-z-]+)/;

// Inject plugin CSS into the outer page for the toolbar icon
let cssLoaded = false;
const ensureCSS = () => {
  if (cssLoaded) return;
  cssLoaded = true;
  const link = document.createElement('link');
  link.rel = 'stylesheet';
  link.href = '/static/plugins/ep_font_family/static/css/fonts.css';
  document.head.appendChild(link);
};
ensureCSS();

// --- ACE hooks ---

export const aceEditorCSS = (): string[] => ['ep_font_family/static/css/fonts.css'];

export const aceAttribsToClasses = (_hookName: string, context: { key: string; value: string }) => {
  if (context.key === 'font-family') {
    return ['font' + context.value];
  }
};

export const aceCreateDomLine = (_hookName: string, context: { cls: string }) => {
  const m = fontRegex.exec(context.cls);
  if (!m) return [];
  const idx = fonts.indexOf(m[1] as typeof fonts[number]);
  if (idx < 0) return [];
  return [{ extraOpenTags: '', extraCloseTags: '', cls: context.cls }];
};

const doInsertFonts = function (this: any, level: number) {
  const rep = this.rep;
  const documentAttributeManager = this.documentAttributeManager;
  if (!(rep.selStart && rep.selEnd) || (level >= 0 && fonts[level] === undefined)) return;

  const newFont: [string, string] = level >= 0 ? ['font-family', fonts[level]] : ['font-family', ''];
  documentAttributeManager.setAttributesOnRange(rep.selStart, rep.selEnd, [newFont]);
};

export const aceInitialized = (_hookName: string, context: { editorInfo: any }) => {
  context.editorInfo.ace_doInsertFonts = doInsertFonts.bind(context);
};

// --- Toolbar hooks ---

export const postToolbarInit = (_hookName: string, context: { toolbar: any }) => {
  const editbar = context.toolbar;
  editbar.registerCommand('fontFamily', () => {
    const dropdown = document.getElementById('font-family-dropdown');
    if (dropdown) dropdown.style.display = dropdown.style.display === 'none' ? '' : 'none';
  });
};

export const postAceInit = (_hookName: string, context: { ace: any }) => {
  // Create the font family dropdown and insert it after the toolbar button
  const btn = document.querySelector('[data-key="fontFamily"]') as HTMLElement | null;
  if (btn && !document.getElementById('font-family-dropdown')) {
    const dropdown = document.createElement('div');
    dropdown.id = 'font-family-dropdown';
    dropdown.style.display = 'none';
    dropdown.style.position = 'absolute';
    dropdown.style.zIndex = '1000';
    dropdown.style.background = '#fff';
    dropdown.style.border = '1px solid #ccc';
    dropdown.style.borderRadius = '4px';
    dropdown.style.padding = '4px';
    dropdown.style.boxShadow = '0 2px 8px rgba(0,0,0,0.15)';

    fonts.forEach((font, idx) => {
      const item = document.createElement('div');
      item.style.padding = '4px 8px';
      item.style.cursor = 'pointer';
      item.style.whiteSpace = 'nowrap';
      item.textContent = fontLabels[idx];
      item.title = fontLabels[idx];
      item.addEventListener('mouseenter', () => {
        item.style.backgroundColor = '#f0f0f0';
      });
      item.addEventListener('mouseleave', () => {
        item.style.backgroundColor = '';
      });
      item.addEventListener('click', () => {
        context.ace.callWithAce((ace: any) => {
          ace.ace_doInsertFonts(idx);
        }, 'insertFont', true);
        dropdown.style.display = 'none';
      });
      dropdown.appendChild(item);
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
  const m = fontRegex.exec(context.cls);
  if (m?.[1]) {
    context.cc.doAttrib(context.state, `font-family::${m[1]}`);
  }
};

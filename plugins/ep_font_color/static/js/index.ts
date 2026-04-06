const colors = ['black', 'red', 'green', 'blue', 'yellow', 'orange'] as const;
const colorRegex = /(?:^| )color:([A-Za-z0-9]*)/;

// Inject plugin CSS into the outer page for the toolbar icon
let cssLoaded = false;
const ensureCSS = () => {
  if (cssLoaded) return;
  cssLoaded = true;
  const link = document.createElement('link');
  link.rel = 'stylesheet';
  link.href = '/static/plugins/ep_font_color/static/css/color.css';
  document.head.appendChild(link);
};
ensureCSS();

// --- ACE hooks ---

export const aceEditorCSS = (): string[] => ['ep_font_color/static/css/color.css'];

export const aceAttribsToClasses = (_hookName: string, context: { key: string; value: string }) => {
  if (context.key.indexOf('color:') !== -1) {
    const m = colorRegex.exec(context.key);
    if (m) return [`color:${m[1]}`];
  }
  if (context.key === 'color') {
    return [`color:${context.value}`];
  }
};

export const aceCreateDomLine = (_hookName: string, context: { cls: string }) => {
  const m = colorRegex.exec(context.cls);
  if (!m) return [];
  const idx = colors.indexOf(m[1] as typeof colors[number]);
  if (idx < 0) return [];
  return [{ extraOpenTags: '', extraCloseTags: '', cls: context.cls }];
};

const doInsertColors = function (this: any, level: number) {
  const rep = this.rep;
  const documentAttributeManager = this.documentAttributeManager;
  if (!(rep.selStart && rep.selEnd) || (level >= 0 && colors[level] === undefined)) return;

  const newColor: [string, string] = level >= 0 ? ['color', colors[level]] : ['color', ''];
  documentAttributeManager.setAttributesOnRange(rep.selStart, rep.selEnd, [newColor]);
};

export const aceInitialized = (_hookName: string, context: { editorInfo: any }) => {
  context.editorInfo.ace_doInsertColors = doInsertColors.bind(context);
};

// --- Toolbar hooks ---

export const postToolbarInit = (_hookName: string, context: { toolbar: any }) => {
  const editbar = context.toolbar;
  editbar.registerCommand('fontColor', () => {
    const dropdown = document.getElementById('font-color-dropdown');
    if (dropdown) dropdown.style.display = dropdown.style.display === 'none' ? '' : 'none';
  });
};

export const postAceInit = (_hookName: string, context: { ace: any }) => {
  // Create the color dropdown and insert it after the toolbar button
  const btn = document.querySelector('[data-key="fontColor"]') as HTMLElement | null;
  if (btn && !document.getElementById('font-color-dropdown')) {
    const dropdown = document.createElement('div');
    dropdown.id = 'font-color-dropdown';
    dropdown.style.display = 'none';
    dropdown.style.position = 'absolute';
    dropdown.style.zIndex = '1000';
    dropdown.style.background = '#fff';
    dropdown.style.border = '1px solid #ccc';
    dropdown.style.borderRadius = '4px';
    dropdown.style.padding = '4px';
    dropdown.style.boxShadow = '0 2px 8px rgba(0,0,0,0.15)';

    colors.forEach((color, idx) => {
      const swatch = document.createElement('span');
      swatch.style.display = 'inline-block';
      swatch.style.width = '20px';
      swatch.style.height = '20px';
      swatch.style.margin = '2px';
      swatch.style.cursor = 'pointer';
      swatch.style.backgroundColor = color;
      swatch.style.border = '1px solid #999';
      swatch.style.borderRadius = '3px';
      swatch.title = color;
      swatch.addEventListener('click', () => {
        context.ace.callWithAce((ace: any) => {
          ace.ace_doInsertColors(idx);
        }, 'insertColor', true);
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
  if (['handleClick', 'handleKeyEvent'].indexOf(cs.type) === -1 && !cs.docTextChanged) return;
  if (cs.type === 'setBaseText' || cs.type === 'setup') return;
};

// --- Content collection ---

export const collectContentPre = (_hookName: string, context: any) => {
  const m = colorRegex.exec(context.cls);
  if (m && m[1]) {
    context.cc.doAttrib(context.state, `color::${m[1]}`);
  }
};

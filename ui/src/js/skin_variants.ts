// @ts-nocheck

const containers = ['editor', 'background', 'toolbar'];
const colors = ['super-light', 'light', 'dark', 'super-dark'];

// Toolbar background colors that the colibris skin variants resolve to.
// Mirrors --bg-color in assets/css/skin/colibris/pad-variants.css. Order
// matters: when skinVariants contains multiple *-toolbar tokens the CSS
// cascade picks the rule defined last, so iterate in source order and let
// the last matching token win. Upstream #7606 / #7690.
const TOOLBAR_COLORS_IN_CSS_ORDER: Array<[string, string]> = [
  ['super-light-toolbar', '#ffffff'],
  ['light-toolbar', '#f2f3f4'],
  ['super-dark-toolbar', '#485365'],
  ['dark-toolbar', '#576273'],
];
const COLIBRIS_DEFAULT_TOOLBAR_COLOR = '#ffffff';

const toolbarColorForTokens = (tokens: string[]): string => {
  const set = new Set(tokens);
  let color = COLIBRIS_DEFAULT_TOOLBAR_COLOR;
  for (const [variant, c] of TOOLBAR_COLORS_IN_CSS_ORDER) {
    if (set.has(variant)) color = c;
  }
  return color;
};

// Keep <meta name="theme-color"> in sync with the toolbar the user actually
// sees. The server emits a baseline derived from settings.skinVariants, but
// pad.ts may flip the toolbar to super-dark on first paint (enableDarkMode
// + prefers-color-scheme:dark + no localStorage white-mode override) and
// the user can toggle via #options-darkmode. Without this, dark-mode users
// keep the light meta and see a white address bar above a dark toolbar
// (issue #7606 follow-up, upstream #7690). When no meta is present the
// helper is a no-op.
const updateThemeColorMeta = (newClasses: string[]) => {
  const meta = document.querySelector('meta[name="theme-color"]');
  if (!meta) return;
  meta.setAttribute('content',
      toolbarColorForTokens(newClasses.join(' ').split(/\s+/).filter(Boolean)));
};

const getHtmlTargets = () => {
  const targets = [document.documentElement];
  const outerFrame = document.querySelector('iframe[name="ace_outer"]') as HTMLIFrameElement | null;
  const outerHtml = outerFrame?.contentDocument?.documentElement;
  if (outerHtml) targets.push(outerHtml);
  const innerFrame =
    outerFrame?.contentDocument?.querySelector('iframe[name="ace_inner"]') as HTMLIFrameElement | null;
  const innerHtml = innerFrame?.contentDocument?.documentElement;
  if (innerHtml) targets.push(innerHtml);
  return targets;
};

// add corresponding classes when config change
export const updateSkinVariantsClasses = (newClasses) => {
  const domsToUpdate = getHtmlTargets();

  colors.forEach((color) => {
    containers.forEach((container) => {
      domsToUpdate.forEach((el) => { el.classList.remove(`${color}-${container}`); });
    });
  });

  domsToUpdate.forEach((el) => { el.classList.remove('full-width-editor'); });

  if (newClasses.length > 0) {
    domsToUpdate.forEach((el) => { el.classList.add(...newClasses); });
  }

  updateThemeColorMeta(newClasses);
};


export const isDarkMode = ()=>{
  return document.documentElement.classList.contains('super-dark-editor');
};


export const setDarkModeInLocalStorage = (isDark)=>{
  localStorage.setItem('ep_darkMode', isDark?'true':'false');
};

export const isDarkModeEnabledInLocalStorage = ()=>{
  return localStorage.getItem('ep_darkMode')==='true';
};

export const isWhiteModeEnabledInLocalStorage = ()=>{
  return localStorage.getItem('ep_darkMode')==='false';
};

// Specific hash to display the skin variants builder popup
if (window.location.hash.toLowerCase() === '#skinvariantsbuilder') {
  document.querySelector('#skin-variants')?.classList.add('popup-show');

  const getNewClasses = () => {
    const newClasses = [];
    document.querySelectorAll('select.skin-variant-color').forEach((element) => {
      const select = element as HTMLSelectElement;
      const container = select.dataset.container;
      if (container) newClasses.push(`${select.value}-${container}`);
    });
    const fullWidth = document.querySelector('#skin-variant-full-width') as HTMLInputElement | null;
    if (fullWidth?.checked) newClasses.push('full-width-editor');

    const result = document.querySelector('#skin-variants-result') as HTMLInputElement | null;
    if (result) result.value = `"skinVariants": "${newClasses.join(' ')}",`;

    return newClasses;
  };

  // run on init
  const updateCheckboxFromSkinClasses = () => {
    document.documentElement.className.split(' ').forEach((classItem) => {
      const container = classItem.substring(classItem.lastIndexOf('-') + 1, classItem.length);
      if (containers.indexOf(container) > -1) {
        const color = classItem.substring(0, classItem.lastIndexOf('-'));
        const select = document.querySelector(
            `.skin-variant-color[data-container="${container}"]`) as HTMLSelectElement | null;
        if (select) select.value = color;
      }
    });

    const fullWidth = document.querySelector('#skin-variant-full-width') as HTMLInputElement | null;
    if (fullWidth) fullWidth.checked = document.documentElement.classList.contains('full-width-editor');
  };

  document.querySelectorAll('.skin-variant').forEach((element) => {
    element.addEventListener('change', () => {
      updateSkinVariantsClasses(getNewClasses());
    });
  });

  updateCheckboxFromSkinClasses();
  updateSkinVariantsClasses(getNewClasses());
}

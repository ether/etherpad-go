import type { ToolbarContext } from '../../../typings/etherpad';

// Inject print CSS link on load
const injectPrintCSS = (): void => {
  const link = document.createElement('link');
  link.rel = 'stylesheet';
  link.type = 'text/css';
  link.href = '../static/plugins/ep_print/static/css/print.css';
  link.media = 'print';
  document.head.appendChild(link);
};

injectPrintCSS();

export const postToolbarInit = (_hookName: string, context: ToolbarContext): boolean => {
  const editbar = context.toolbar;

  editbar.registerCommand('print', () => {
    window.print();
  });

  return true;
};

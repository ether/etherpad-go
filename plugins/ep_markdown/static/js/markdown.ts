const getPadRootPath = (): string => {
  const match = /.*\/p\/[^/]+/.exec(document.location.pathname);
  if (match?.[0]) return match[0];
  return window.clientVars?.padId ?? '';
};

const getInnerDocBody = (): HTMLElement | null => {
  const outerFrame = document.querySelector<HTMLIFrameElement>('iframe[name="ace_outer"]');
  const innerFrame = outerFrame?.contentDocument?.querySelector<HTMLIFrameElement>('iframe');
  return innerFrame?.contentDocument?.querySelector<HTMLElement>('#innerdocbody') ?? null;
};

const setMarkdownMode = (enabled: boolean): void => {
  const body = getInnerDocBody();
  if (!body) return;

  body.classList.toggle('markdown', enabled);

  const underlineButton = document.querySelector<HTMLElement>('#underline');
  const strikeButton = document.querySelector<HTMLElement>('#strikethrough');
  if (underlineButton) underlineButton.style.display = enabled ? 'none' : '';
  if (strikeButton) strikeButton.style.display = enabled ? 'none' : '';
};

export const postAceInit = (): void => {
  const exportMarkdown = document.querySelector<HTMLAnchorElement>('#exportmarkdowna');
  if (exportMarkdown) exportMarkdown.href = `${getPadRootPath()}/export/markdown`;

  const markdownCheckbox = document.querySelector<HTMLInputElement>('#options-markdown');
  if (!markdownCheckbox) return;

  setMarkdownMode(markdownCheckbox.checked);
  markdownCheckbox.addEventListener('click', () => {
    setMarkdownMode(markdownCheckbox.checked);
  });
};

export const aceEditorCSS = (): string[] => ['/ep_markdown/static/css/markdown.css'];

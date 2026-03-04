import type {
  AceAttribsToClassesHook,
  AceContext,
  AceDomLineProcessLineAttributesHook,
  AceEditEventHook,
  ToolbarContext,
} from '../../../typings/etherpad';

const tags = ['left', 'center', 'justify', 'right'] as const;

const range = (start: number, end: number): number[] => {
  const length = Math.abs(end - start) + 1;
  return Array.from({length}, (_, index) => start + index);
};

const getAlignValue = (target: EventTarget | null): number | null => {
  if (!(target instanceof Element)) return null;
  const button = target.closest<HTMLElement>('.ep_align');
  if (!button) return null;
  const value = Number.parseInt(button.dataset.align ?? '', 10);
  return Number.isNaN(value) ? null : value;
};

export const aceRegisterBlockElements = (): string[] => [...tags];

export const postAceInit = (_hookName: string, context: AceContext): void => {
  document.body.addEventListener('click', (event) => {
    const alignValue = getAlignValue(event.target);
    if (alignValue === null) return;

    context.ace.callWithAce((ace) => {
      ace.ace_doInsertAlign(alignValue);
    }, 'insertalign', true);
  });
};

export const aceEditEvent: AceEditEventHook = (_hookName, call) => {
  const cs = call.callstack;
  if (cs.type !== 'handleClick' && cs.type !== 'handleKeyEvent' && !cs.docTextChanged) return false;
  if (cs.type === 'setBaseText' || cs.type === 'setup') return false;

  return setTimeout(() => {
    const rep = call.rep;
    if (!rep.selStart || !rep.selEnd) return;

    const attributeManager = call.documentAttributeManager;
    const firstLine = rep.selStart[0];
    const lastLine = Math.max(firstLine, rep.selEnd[0] - (rep.selEnd[1] === 0 ? 1 : 0));
    const activeAttributes: Record<string, number> = {};
    let totalNumberOfLines = 0;

    range(firstLine, lastLine + 1).forEach((line) => {
      totalNumberOfLines += 1;
      const attr = attributeManager.getAttributeOnLine(line, 'align');
      if (!attr) return;
      activeAttributes[attr] = (activeAttributes[attr] ?? 0) + 1;
    });

    Object.entries(activeAttributes).forEach(([key, count]) => {
      if (count === totalNumberOfLines) {
        void key;
      }
    });
  }, 250);
};

export const aceAttribsToClasses: AceAttribsToClassesHook = (_hook, context) => {
  if (context.key === 'align') return [`align:${context.value}`];
  return undefined;
};

export const aceDomLineProcessLineAttributes: AceDomLineProcessLineAttributesHook = (_hookName, context) => {
  const alignType = /(?:^| )align:([A-Za-z0-9]*)/.exec(context.cls);
  if (!alignType) return [];

  const tag = alignType[1];
  if (!tags.includes(tag as (typeof tags)[number])) return [];

  const styles =
    'width:100%;margin:0 auto;list-style-position:inside;display:block;text-align:' + tag;

  return [{
    preHtml: `<${tag} style="${styles}">`,
    postHtml: `</${tag}>`,
    processedMarker: true,
  }];
};

export const aceEditorCSS = (): string[] => ['ep_align/static/css/align.css'];

export const aceInitialized = (_hookName: string, context: AceContext): void => {
  const doInsertAlign = function (this: AceContext, level: number): void {
    const {rep, documentAttributeManager} = this;
    if (!(rep.selStart && rep.selEnd)) return;
    if (level >= 0 && tags[level] === undefined) return;

    const firstLine = rep.selStart[0];
    const lastLine = Math.max(firstLine, rep.selEnd[0] - (rep.selEnd[1] === 0 ? 1 : 0));
    range(firstLine, lastLine).forEach((line) => {
      if (level >= 0) {
        documentAttributeManager.setAttributeOnLine(line, 'align', tags[level]);
      } else {
        documentAttributeManager.removeAttributeOnLine(line, 'align');
      }
    });
  };

  context.editorInfo.ace_doInsertAlign = doInsertAlign.bind(context);
};

const align = (context: ToolbarContext, alignment: number): void => {
  context.ace.callWithAce((ace) => {
    ace.ace_doInsertAlign(alignment);
    ace.ace_focus();
  }, 'insertalign', true);
};

export const postToolbarInit = (_hookName: string, context: ToolbarContext): boolean => {
  const editbar = context.toolbar;
  editbar.registerCommand('alignLeft', () => align(context, 0));
  editbar.registerCommand('alignCenter', () => align(context, 1));
  editbar.registerCommand('alignJustify', () => align(context, 2));
  editbar.registerCommand('alignRight', () => align(context, 3));
  return true;
};

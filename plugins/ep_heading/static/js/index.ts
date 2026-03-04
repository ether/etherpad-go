import type {
  AceAttribsToClassesHook,
  AceContext,
  AceDomLineProcessLineAttributesHook,
  AceEditEventHook,
} from '../../../typings/etherpad';

const cssFiles = ['ep_heading/static/css/editor.css'];
const tags = ['h1', 'h2', 'h3', 'h4', 'code'] as const;

const range = (start: number, end: number): number[] =>
  Array.from({length: Math.abs(end - start) + 1}, (_, index) => start + index);

const updateHeadingSelectUi = (_select: HTMLSelectElement): void => {
  // Native select does not need sync hooks.
};

export const aceRegisterBlockElements = (): string[] => [...tags];

export const postAceInit = (_hookName: string, context: AceContext): void => {
  document.querySelectorAll<HTMLElement>('.toolbar a.ep_heading').forEach((button) => {
    button.addEventListener('click', (event) => {
      event.preventDefault();
      const indexOfHeading = Number.parseInt(button.dataset.plugin ?? '', 10);
      if (Number.isNaN(indexOfHeading)) return;
      context.ace.callWithAce((ace) => {
        ace.ace_doInsertHeading(indexOfHeading);
      }, 'insertheading', true);
    });
  });

  const headingSelection = document.querySelector<HTMLSelectElement>('#heading-selection');
  if (!headingSelection) return;

  headingSelection.addEventListener('change', () => {
    const intValue = Number.parseInt(headingSelection.value, 10);
    if (Number.isNaN(intValue)) return;

    context.ace.callWithAce((ace) => {
      ace.ace_doInsertHeading(intValue);
    }, 'insertheading', true);

    headingSelection.value = 'dummy';
    updateHeadingSelectUi(headingSelection);
  });
};

export const aceEditEvent: AceEditEventHook = (_hookName, call) => {
  const cs = call.callstack;
  if (cs.type !== 'handleClick' && cs.type !== 'handleKeyEvent' && !cs.docTextChanged) return false;
  if (cs.type === 'setBaseText' || cs.type === 'setup') return false;

  return setTimeout(() => {
    const rep = call.rep;
    if (!rep.selStart || !rep.selEnd) return;

    const headingSelection = document.querySelector<HTMLSelectElement>('#heading-selection');
    if (headingSelection) {
      headingSelection.value = 'dummy';
      updateHeadingSelectUi(headingSelection);
    }

    const attributeManager = call.documentAttributeManager;
    const activeAttributes: Record<string, number> = {};
    const firstLine = rep.selStart[0];
    const lastLine = Math.max(firstLine, rep.selEnd[0] - (rep.selEnd[1] === 0 ? 1 : 0));
    let totalNumberOfLines = 0;

    range(firstLine, lastLine).forEach((line) => {
      totalNumberOfLines += 1;
      const attr = attributeManager.getAttributeOnLine(line, 'heading');
      if (!attr) return;
      activeAttributes[attr] = (activeAttributes[attr] ?? 0) + 1;
    });

    Object.entries(activeAttributes).forEach(([key, count]) => {
      if (count !== totalNumberOfLines || !headingSelection) return;
      const index = tags.indexOf(key as (typeof tags)[number]);
      if (index < 0) return;
      headingSelection.value = String(index);
      updateHeadingSelectUi(headingSelection);
    });
  }, 250);
};

export const aceAttribsToClasses: AceAttribsToClassesHook = (_hookName, context) => {
  if (context.key === 'heading') return [`heading:${context.value}`];
  return undefined;
};

export const aceDomLineProcessLineAttributes: AceDomLineProcessLineAttributesHook = (_hookName, context) => {
  const headingType = /(?:^| )heading:([A-Za-z0-9]*)/.exec(context.cls);
  if (!headingType) return [];

  let tag = headingType[1];
  if (tag === 'h5' || tag === 'h6') tag = 'h4';
  if (!tags.includes(tag as (typeof tags)[number])) return [];

  return [{
    preHtml: `<${tag}>`,
    postHtml: `</${tag}>`,
    processedMarker: true,
  }];
};

export const aceInitialized = (_hookName: string, context: AceContext): void => {
  context.editorInfo.ace_doInsertHeading = (level: number): void => {
    const {documentAttributeManager, rep} = context;
    if (!(rep.selStart && rep.selEnd)) return;
    if (level >= 0 && tags[level] === undefined) return;

    const firstLine = rep.selStart[0];
    const lastLine = Math.max(firstLine, rep.selEnd[0] - (rep.selEnd[1] === 0 ? 1 : 0));

    range(firstLine, lastLine).forEach((line) => {
      if (level >= 0) {
        documentAttributeManager.setAttributeOnLine(line, 'heading', tags[level]);
      } else {
        documentAttributeManager.removeAttributeOnLine(line, 'heading');
      }
    });
  };
};

export const aceEditorCSS = (): string[] => cssFiles;

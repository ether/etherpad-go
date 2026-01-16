/**
 * ep_align - Text Alignment Plugin for Etherpad
 *
 * Ermöglicht Text-Ausrichtung: links, zentriert, rechts, Blocksatz
 */

/* eslint-disable @typescript-eslint/no-explicit-any */

// Types
interface AceContext {
  ace: {
    callWithAce: (callback: (ace: AceEditor) => void, action: string, flag: boolean) => void;
  };
  rep: RepState;
  documentAttributeManager: DocumentAttributeManager;
  editorInfo: EditorInfo;
}

interface AceEditor {
  ace_doInsertAlign: (level: number) => void;
  ace_focus: () => void;
}

interface RepState {
  selStart: [number, number] | null;
  selEnd: [number, number] | null;
}

interface DocumentAttributeManager {
  getAttributeOnLine: (line: number, attr: string) => string | null;
  setAttributeOnLine: (line: number, attr: string, value: string) => void;
  removeAttributeOnLine: (line: number, attr: string) => void;
}

interface EditorInfo {
  ace_doInsertAlign?: (level: number) => void;
}

interface CallStack {
  type: string;
  docTextChanged?: boolean;
}

interface EditEventCall {
  callstack: CallStack;
  documentAttributeManager: DocumentAttributeManager;
  rep: RepState;
}

interface ToolbarContext {
  toolbar: {
    registerCommand: (name: string, callback: () => void) => void;
  };
  ace: AceContext['ace'];
}

interface AttribContext {
  key: string;
  value: string;
}

interface LineContext {
  cls: string;
}

// All our tags are block elements, so we just return them.
const tags: string[] = ['left', 'center', 'justify', 'right'];

const range = (start: number, end: number): number[] => {
  const length = Math.abs(end - start) + 1;
  const result: number[] = [];
  for (let i = 0; i < length; i++) {
    result.push(start + i);
  }
  return result;
};

// Returns the block elements for alignment
export const aceRegisterBlockElements = (): string[] => tags;

// Bind the event handler to the toolbar buttons
export const postAceInit = (_hookName: string, context: AceContext): void => {
  const $ = (globalThis as any).jQuery || (globalThis as any).$;

  $('body').on('click', '.ep_align', function (this: HTMLElement) {
    const value = $(this).data('align');
    const intValue = parseInt(value, 10);
    if (!isNaN(intValue)) {
      context.ace.callWithAce((ace: AceEditor) => {
        ace.ace_doInsertAlign(intValue);
      }, 'insertalign', true);
    }
  });
};

// On caret position change show the current align
export const aceEditEvent = (_hook: string, call: EditEventCall): false | ReturnType<typeof setTimeout> => {
  // If it's not a click or a key event and the text hasn't changed then do nothing
  const cs = call.callstack;
  if (cs.type !== 'handleClick' && cs.type !== 'handleKeyEvent' && !cs.docTextChanged) {
    return false;
  }
  // If it's an initial setup event then do nothing..
  if (cs.type === 'setBaseText' || cs.type === 'setup') return false;

  // It looks like we should check to see if this section has this attribute
  return setTimeout(() => {
    const attributeManager = call.documentAttributeManager;
    const rep = call.rep;
    const activeAttributes: Record<string, { count: number }> = {};

    if (!rep.selStart || !rep.selEnd) return;

    const firstLine = rep.selStart[0];
    const lastLine = Math.max(firstLine, rep.selEnd[0] - ((rep.selEnd[1] === 0) ? 1 : 0));
    let totalNumberOfLines = 0;

    range(firstLine, lastLine + 1).forEach((line) => {
      totalNumberOfLines++;
      const attr = attributeManager.getAttributeOnLine(line, 'align');
      if (attr) {
        if (activeAttributes[attr]) {
          activeAttributes[attr].count++;
        } else {
          activeAttributes[attr] = { count: 1 };
        }
      }
    });

    // Check which alignment is active on all lines
    for (const key in activeAttributes) {
      if (Object.prototype.hasOwnProperty.call(activeAttributes, key)) {
        const attr = activeAttributes[key];
        if (attr.count === totalNumberOfLines) {
          // All lines have the same alignment - could be used to highlight button
        }
      }
    }
  }, 250);
};

// Our align attribute will result in a align:left class
export const aceAttribsToClasses = (_hook: string, context: AttribContext): string[] | undefined => {
  if (context.key === 'align') {
    return [`align:${context.value}`];
  }
  return undefined;
};

// Here we convert the class align:left into a tag
export const aceDomLineProcessLineAttributes = (_name: string, context: LineContext): Array<{
  preHtml: string;
  postHtml: string;
  processedMarker: boolean;
}> => {
  const cls = context.cls;
  const alignType = /(?:^| )align:([A-Za-z0-9]*)/.exec(cls);
  let tagIndex: number | undefined;
  if (alignType) tagIndex = tags.indexOf(alignType[1]);
  if (tagIndex !== undefined && tagIndex >= 0) {
    const tag = tags[tagIndex];
    const styles =
      `width:100%;margin:0 auto;list-style-position:inside;display:block;text-align:${tag}`;
    const modifier = {
      preHtml: `<${tag} style="${styles}">`,
      postHtml: `</${tag}>`,
      processedMarker: true,
    };
    return [modifier];
  }
  return [];
};

/**
 * Adds CSS to the editor for alignment styles
 */
export const aceEditorCSS = (): string[] => {
  return ['ep_align/static/css/align.css'];
};

// Once ace is initialized, we set ace_doInsertAlign and bind it to the context
export const aceInitialized = (_hook: string, context: AceContext): void => {
  // Passing a level >= 0 will set a alignment on the selected lines, level < 0
  // will remove it
  const doInsertAlign = function (this: AceContext, level: number): void {
    const rep = this.rep;
    const documentAttributeManager = this.documentAttributeManager;
    if (!(rep.selStart && rep.selEnd) || (level >= 0 && tags[level] === undefined)) {
      return;
    }

    const firstLine = rep.selStart[0];
    const lastLine = Math.max(firstLine, rep.selEnd[0] - ((rep.selEnd[1] === 0) ? 1 : 0));
    range(firstLine, lastLine + 1).forEach((i) => {
      if (level >= 0) {
        documentAttributeManager.setAttributeOnLine(i, 'align', tags[level]);
      } else {
        documentAttributeManager.removeAttributeOnLine(i, 'align');
      }
    });
  };

  const editorInfo = context.editorInfo;
  editorInfo.ace_doInsertAlign = doInsertAlign.bind(context);
};

const align = (context: ToolbarContext, alignment: number): void => {
  context.ace.callWithAce((ace: AceEditor) => {
    ace.ace_doInsertAlign(alignment);
    ace.ace_focus();
  }, 'insertalign', true);
};

export const postToolbarInit = (_hookName: string, context: ToolbarContext): boolean => {
  const editbar = context.toolbar;

  editbar.registerCommand('alignLeft', () => {
    align(context, 0);
  });

  editbar.registerCommand('alignCenter', () => {
    align(context, 1);
  });

  editbar.registerCommand('alignJustify', () => {
    align(context, 2);
  });

  editbar.registerCommand('alignRight', () => {
    align(context, 3);
  });

  return true;
};

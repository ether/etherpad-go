import type { AceContext, ToolbarContext } from '../../../typings/etherpad';

interface AceRep {
  selStart: [number, number];
  selEnd: [number, number];
  apool: {
    attribToNum: Record<string, number>;
  };
}

interface ClearFormattingAce {
  ace_getRep: () => AceRep;
  ace_setAttributeOnSelection: (attr: string, value: false) => void;
  ace_doClearFormatting?: () => void;
}

export const aceInitialized = (_hookName: string, context: AceContext): void => {
  const doClearFormatting = function (this: AceContext): void {
    const { rep } = this;
    if (!rep.selStart || !rep.selEnd) return;

    const isSelection =
      rep.selStart[0] !== rep.selEnd[0] || rep.selStart[1] !== rep.selEnd[1];
    if (!isSelection) return;
  };

  context.editorInfo.ace_doClearFormatting = doClearFormatting.bind(context);
};

export const postAceInit = (_hookName: string, context: AceContext): void => {
  document.body.addEventListener('click', () => {
    // Click handler reserved for direct button clicks if needed
  });
};

export const postToolbarInit = (_hookName: string, context: ToolbarContext): boolean => {
  const editbar = context.toolbar;

  editbar.registerCommand('clearFormatting', () => {
    context.ace.callWithAce((ace) => {
      const editor = ace as unknown as ClearFormattingAce;
      const rep = editor.ace_getRep();

      const isSelection =
        rep.selStart[0] !== rep.selEnd[0] || rep.selStart[1] !== rep.selEnd[1];
      if (!isSelection) return;

      const attrs = rep.apool.attribToNum;
      for (const k of Object.keys(attrs)) {
        const attr = k.split(',')[0];
        if (attr !== 'author') {
          editor.ace_setAttributeOnSelection(attr, false);
        }
      }
    }, 'clearFormatting', true);
  });

  return true;
};

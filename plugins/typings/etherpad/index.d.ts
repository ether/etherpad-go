interface ToolbarApi {
  registerCommand: (name: string, callback: () => void) => void;
}

interface EditorInfo {
  ace_doInsertAlign?: (level: number) => void;
  ace_doInsertHeading?: (level: number) => void;
}

interface DocumentAttributeManager {
  getAttributeOnLine: (line: number, attr: string) => string | null;
  setAttributeOnLine: (line: number, attr: string, value: string) => void;
  removeAttributeOnLine: (line: number, attr: string) => void;
}

interface RepState {
  selStart: [number, number] | null;
  selEnd: [number, number] | null;
}

interface AceEditor {
  ace_doInsertAlign: (level: number) => void;
  ace_doInsertHeading: (level: number) => void;
  ace_focus: () => void;
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

interface AttribContext {
  key: string;
  value: string;
}

interface LineContext {
  cls: string;
}

interface ContentCollectorState {
  lineAttributes: Record<string, string | undefined>;
}

interface ContentCollectorContext {
  tname: string;
  state: ContentCollectorState;
}

export interface AceContext {
  ace: {
    callWithAce: (callback: (ace: AceEditor) => void, action: string, focus: boolean) => void;
  };
  rep: RepState;
  documentAttributeManager: DocumentAttributeManager;
  editorInfo: EditorInfo;
}

export interface ToolbarContext {
  toolbar: ToolbarApi;
  ace: AceContext['ace'];
}

export type PostAceInitHook = (_hookName: string, context: AceContext) => void;
export type AceEditEventHook = (_hookName: string, call: EditEventCall) => false | ReturnType<typeof setTimeout>;
export type AceAttribsToClassesHook = (_hook: string, context: AttribContext) => string[] | undefined;
export type AceDomLineProcessLineAttributesHook = (
  _hook: string,
  context: LineContext,
) => Array<{ preHtml: string; postHtml: string; processedMarker: boolean }>;
export type ContentCollectorHook = (
  _hookName: string,
  context: ContentCollectorContext,
  cb: () => unknown,
) => unknown;

declare global {
  interface Window {
    clientVars?: {
      padId?: string;
    };
  }
}

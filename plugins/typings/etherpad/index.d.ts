interface EditorInfo {
    ace_doInsertAlign?: (level: number) => void;
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

interface CallStack {
    type: string;
    docTextChanged?: boolean;
}

interface EditEventCall {
    callstack: CallStack;
    documentAttributeManager: DocumentAttributeManager;
    rep: RepState;
}

interface AceEditor {
    ace_doInsertAlign: (level: number) => void;
    ace_focus: () => void;
}

export interface AceContext {
    ace: {
        callWithAce: (callback: (ace: AceEditor) => void, action: string, flag: boolean) => void;
    };
    rep: RepState;
    documentAttributeManager: DocumentAttributeManager;
    editorInfo: EditorInfo;
}

export type PostAceInitHook = (_hookName: string, context: AceContext) => void;

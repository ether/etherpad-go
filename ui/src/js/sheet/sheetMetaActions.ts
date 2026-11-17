// Dialog actions for hyperlinks, notes, data validation and conditional
// formatting. The editor supplies the selection and the op side effects.
import { openDialog } from './sheetDialog';
import {
  COND_OPS, COND_PRESETS, MAX_COND_JSON, MAX_COND_RULES, MAX_COND_VALUE, MAX_NOTE_LENGTH, MAX_VALID_LIST_LENGTH, byteLength,
  PROP_COND_FMT, PROP_LINK, PROP_NOTE, PROP_VALID_LIST,
  listValues, normalizeLink, parseCondRules, validLink, type CondOp, type CondRule,
} from './cellMeta';

export interface MetaContext {
  readOnly: () => boolean;
  focus: () => { row: number; col: number };
  cells: () => Array<{ row: number; col: number }>;
  propsOf: (row: number, col: number) => Record<string, string>;
  rawValue: (row: number, col: number) => string;
  // Sets the props of one cell to exactly `props`.
  setProps: (row: number, col: number, props: Record<string, string>) => void;
  setRaw: (row: number, col: number, raw: string) => void;
  // Ends cell editing before ops are sent (keeps typed text from clobbering).
  blur: () => void;
}

const COND_LABELS: Record<CondOp, string> = {
  gt: 'greater than', lt: 'less than', ge: 'greater than or equal to', le: 'less than or equal to',
  eq: 'equal to', ne: 'not equal to', between: 'between', notBetween: 'not between',
  contains: 'containing', notContains: 'not containing', beginsWith: 'beginning with', endsWith: 'ending with',
  blank: 'blank', notBlank: 'not blank',
};

export function createMetaActions(ctx: MetaContext) {
  // update changes one prop on every selected cell (or on `only`).
  const update = (key: string, value: string | ((props: Record<string, string>) => string), only?: { row: number; col: number }): void => {
    ctx.blur();
    for (const { row, col } of only ? [only] : ctx.cells()) {
      const props = { ...ctx.propsOf(row, col) };
      const v = typeof value === 'function' ? value(props) : value;
      if ((props[key] ?? '') === v) continue;
      if (v === '') delete props[key];
      else props[key] = v;
      ctx.setProps(row, col, props);
    }
  };

  const insertLink = (): void => {
    if (ctx.readOnly()) return;
    const f = ctx.focus();
    const props = ctx.propsOf(f.row, f.col);
    const raw = ctx.rawValue(f.row, f.col);
    const isFormula = raw.startsWith('=');
    openDialog({
      title: props[PROP_LINK] ? 'Edit Link' : 'Insert Link',
      fields: [
        ...(isFormula ? [] : [{ key: 'text', label: 'Text to display', type: 'text' as const, value: raw }]),
        { key: 'url', label: 'Address', type: 'text', value: props[PROP_LINK] ?? '', placeholder: 'https://… or name@example.com' },
      ],
      extra: props[PROP_LINK] ? { label: 'Remove Link', run: () => update(PROP_LINK, '') } : undefined,
      onOk: (v) => {
        const url = normalizeLink(v.url ?? '');
        if (!validLink(url)) return 'Enter a web address (https://…) or an e-mail address.';
        update(PROP_LINK, url);
        // An empty display text shows the address, as in Excel.
        const text = (v.text ?? '').trim() === '' ? url : v.text;
        if (!isFormula && text !== undefined && text !== raw) ctx.setRaw(f.row, f.col, text);
        return null;
      },
    });
  };

  const editNote = (): void => {
    if (ctx.readOnly()) return;
    const f = ctx.focus();
    const note = ctx.propsOf(f.row, f.col)[PROP_NOTE] ?? '';
    openDialog({
      title: note ? 'Edit Note' : 'New Note',
      fields: [{ key: 'note', label: 'Note', type: 'textarea', value: note }],
      extra: note ? { label: 'Delete Note', run: () => update(PROP_NOTE, '', f) } : undefined,
      onOk: (v) => {
        const text = (v.note ?? '').replace(/\s+$/, '');
        if (byteLength(text) > MAX_NOTE_LENGTH) return `The note is too long (at most ${MAX_NOTE_LENGTH} bytes).`;
        update(PROP_NOTE, text, f);
        return null;
      },
    });
  };

  const dataValidation = (): void => {
    if (ctx.readOnly()) return;
    const f = ctx.focus();
    const current = ctx.propsOf(f.row, f.col)[PROP_VALID_LIST] ?? '';
    openDialog({
      title: 'Data Validation',
      fields: [
        { key: 'allow', label: 'Allow', type: 'select', value: 'list', options: [['list', 'List'], ['any', 'Any value']] },
        { key: 'source', label: 'Source', type: 'text', value: current, placeholder: 'Yes, No, Maybe', showWhen: { key: 'allow', values: ['list'] } },
      ],
      onOk: (v) => {
        if (v.allow === 'any') {
          update(PROP_VALID_LIST, '');
          return null;
        }
        const values = listValues(v.source);
        if (values.length === 0) return 'Enter the allowed values, separated by commas.';
        const list = values.join(',');
        if (byteLength(list) > MAX_VALID_LIST_LENGTH) return `The list is too long (at most ${MAX_VALID_LIST_LENGTH} bytes, as in Excel).`;
        update(PROP_VALID_LIST, list);
        return null;
      },
    });
  };

  const condFormat = (preset: string): void => {
    if (ctx.readOnly()) return;
    const ops = Object.keys(COND_OPS) as CondOp[];
    openDialog({
      title: 'New Formatting Rule',
      fields: [
        { key: 'op', label: 'Format cells', type: 'select', value: preset || 'gt', options: ops.map((o) => [o, COND_LABELS[o]]) },
        { key: 'v1', label: 'Value', type: 'text', showWhen: { key: 'op', values: ops.filter((o) => COND_OPS[o] >= 1) } },
        { key: 'v2', label: 'and', type: 'text', showWhen: { key: 'op', values: ops.filter((o) => COND_OPS[o] === 2) } },
        { key: 'style', label: 'with', type: 'select', value: '0', options: COND_PRESETS.map((p, i) => [String(i), p.label]) },
      ],
      onOk: (v) => {
        const op = v.op as CondOp;
        const n = COND_OPS[op];
        if (n === undefined) return 'Choose a rule.';
        const v1 = (v.v1 ?? '').trim();
        const v2 = (v.v2 ?? '').trim();
        if ((n >= 1 && v1 === '') || (n === 2 && v2 === '')) return 'Enter a value.';
        if (byteLength(v1) > MAX_COND_VALUE || byteLength(v2) > MAX_COND_VALUE) return `A value can have at most ${MAX_COND_VALUE} bytes.`;
        const p = COND_PRESETS[Number(v.style)] ?? COND_PRESETS[0];
        const rule: CondRule = { op };
        if (n >= 1) rule.v1 = v1;
        if (n === 2) rule.v2 = v2;
        if (p.bg) rule.bg = p.bg;
        if (p.color) rule.color = p.color;
        if (p.bold) rule.bold = p.bold;
        // A cell that already has the most rules keeps them as they are.
        update(PROP_COND_FMT, (props) => {
          const rules = parseCondRules(props[PROP_COND_FMT]);
          const next = JSON.stringify([...rules, rule]);
          return rules.length >= MAX_COND_RULES || byteLength(next) > MAX_COND_JSON ? props[PROP_COND_FMT] ?? '' : next;
        });
        return null;
      },
    });
  };

  return {
    insertLink,
    removeLink: () => !ctx.readOnly() && update(PROP_LINK, ''),
    editNote,
    deleteNote: () => !ctx.readOnly() && update(PROP_NOTE, ''),
    dataValidation,
    condFormat,
    clearCondFormat: () => !ctx.readOnly() && update(PROP_COND_FMT, ''),
  };
}

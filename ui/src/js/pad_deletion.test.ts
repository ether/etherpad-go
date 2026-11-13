// @ts-nocheck
import {afterEach, beforeEach, describe, expect, it} from 'vitest';
import {createPadDeletion, deletionUiState} from './pad_deletion';

describe('deletionUiState', () => {
  it('creator without allowPadDeletionByAllUsers: button, token field and the dialog', () => {
    expect(deletionUiState({padId: 'p', padDeletionToken: 'tok', canDeletePad: true, canDeleteWithoutToken: false}))
        .toEqual({showDeleteButton: true, showTokenField: true, showTokenModal: true});
  });
  it('everyone may delete: no token field, no dialog', () => {
    expect(deletionUiState({padId: 'p', padDeletionToken: null, canDeletePad: true, canDeleteWithoutToken: true}))
        .toEqual({showDeleteButton: true, showTokenField: false, showTokenModal: false});
  });
  it('another user: only the token field', () => {
    expect(deletionUiState({padId: 'p', canDeletePad: false, canDeleteWithoutToken: false}))
        .toEqual({showDeleteButton: false, showTokenField: true, showTokenModal: false});
  });
  it('suppressPadDeletionTokenModal hides the dialog only', () => {
    expect(deletionUiState({padId: 'p', padDeletionToken: 'tok', canDeletePad: true, suppressPadDeletionTokenModal: true}).showTokenModal)
        .toBe(false);
  });
});

describe('createPadDeletion', () => {
  let elements;
  let listeners;
  const el = (id, extra = {}) => (elements[id] = {
    id, hidden: true, value: '', checked: false, textContent: '',
    classList: {add() {}, remove() {}},
    addEventListener: (type, fn) => { (listeners[id + ':' + type] ??= []).push(fn); },
    focus() {}, select() {},
    ...extra,
  });
  const fire = (id, type, detail) => (listeners[id + ':' + type] || []).forEach((fn) => fn({detail}));

  beforeEach(() => {
    elements = {};
    listeners = {};
    for (const id of ['delete-pad', 'delete-pad-with-token', 'delete-pad-token-submit', 'delete-pad-token-input',
      'deletiontoken-modal', 'deletiontoken-value', 'deletiontoken-copy', 'deletiontoken-ack',
      'delete-pad-token-reveal', 'delete-pad-token-toggle', 'delete-pad-token-reveal-panel',
      'delete-pad-token-reveal-value', 'delete-pad-token-reveal-status']) el(id);
    globalThis.document = {getElementById: (id) => elements[id] ?? null, activeElement: null, body: {contains: () => false}};
  });
  afterEach(() => { delete globalThis.document; });

  const setup = (vars, {confirm = true} = {}) => {
    const calls = {sent: [], alerts: [], home: 0};
    const store = new Map();
    const storage = {getItem: (k) => store.get(k) ?? null, setItem: (k, v) => store.set(k, v), removeItem: (k) => store.delete(k)};
    const d = createPadDeletion({
      vars: () => vars,
      t: (k) => `t(${k})`,
      confirm: () => confirm,
      alert: (m) => calls.alerts.push(m),
      send: (m) => calls.sent.push(m),
      navigateHome: () => calls.home++,
      storage: () => storage,
    });
    d.init();
    return {d, calls, store};
  };

  it('shows the dialog with the token to the creator', () => {
    setup({padId: 'p', padDeletionToken: 'secret', canDeletePad: true});
    expect(elements['deletiontoken-modal'].hidden).toBe(false);
    expect(elements['deletiontoken-value'].value).toBe('secret');
    expect(elements['delete-pad'].hidden).toBe(false);
  });

  it('acknowledging the dialog clears the token from the page', () => {
    const vars = {padId: 'p', padDeletionToken: 'secret', canDeletePad: true};
    setup(vars);
    fire('deletiontoken-ack', 'click');
    expect(elements['deletiontoken-modal'].hidden).toBe(true);
    expect(elements['deletiontoken-value'].value).toBe('');
    expect(vars.padDeletionToken).toBeNull();
  });

  it('leaves the page only after the server deleted the pad', () => {
    const {d, calls} = setup({padId: 'p', canDeletePad: true});
    fire('delete-pad', 'click');
    expect(calls.sent).toEqual([{type: 'PAD_DELETE', data: {padId: 'p'}}]);
    expect(calls.home).toBe(0);
    expect(d.handleDeleted()).toBe(true);
    expect(calls.home).toBe(1);
  });

  it('a refused deletion is reported and the page stays', () => {
    const {d, calls} = setup({padId: 'p', canDeletePad: true});
    fire('delete-pad', 'click');
    expect(d.handleShout('pad.deletionToken.notCreator')).toBe(true);
    expect(calls.alerts).toEqual(['t(pad.deletionToken.notCreator)']);
    expect(d.isPending()).toBe(false);
    expect(d.handleDeleted()).toBe(false, 'someone else deleting it later does not count as ours');
    expect(calls.home).toBe(0);
  });

  it('other shouts are not handled here', () => {
    const {d} = setup({padId: 'p'});
    expect(d.handleShout('update.drain.notice')).toBe(false);
    expect(d.handleShout(undefined)).toBe(false);
  });

  it('deletes with the token from the field', () => {
    const {calls} = setup({padId: 'p', canDeletePad: false});
    expect(elements['delete-pad'].hidden).toBe(true);
    expect(elements['delete-pad-with-token'].hidden).toBe(false);
    elements['delete-pad-token-input'].value = '  tok  ';
    fire('delete-pad-token-submit', 'click');
    expect(calls.sent).toEqual([{type: 'PAD_DELETE', data: {padId: 'p', deletionToken: 'tok'}}]);
  });

  it('nothing is sent when the user does not confirm', () => {
    const {calls} = setup({padId: 'p', canDeletePad: true}, {confirm: false});
    fire('delete-pad', 'click');
    expect(calls.sent).toEqual([]);
  });

  it('with suppressPadDeletionTokenModal the token is kept and offered in the settings', () => {
    const {store} = setup({padId: 'p', padDeletionToken: 'secret', canDeletePad: true, suppressPadDeletionTokenModal: true});
    expect(elements['deletiontoken-modal'].hidden).toBe(true);
    expect(store.get('padDeletionToken:p')).toBe('secret');
    expect(elements['delete-pad-token-reveal'].hidden).toBe(false);
  });

  it('a saved token is offered on a later visit, and forgotten when the pad is deleted', () => {
    const vars = {padId: 'p', padDeletionToken: 'secret', canDeletePad: true, suppressPadDeletionTokenModal: true};
    const first = setup(vars);
    // Later visit: the server sends no token any more.
    elements['delete-pad-token-reveal'].hidden = true;
    const later = createPadDeletion({
      vars: () => ({...vars, padDeletionToken: null}), t: (k) => k, confirm: () => true, alert() {},
      send() {}, navigateHome() {},
      storage: () => ({getItem: (k) => first.store.get(k) ?? null, setItem() {}, removeItem: (k) => first.store.delete(k)}),
    });
    later.init();
    expect(elements['delete-pad-token-reveal'].hidden).toBe(false);
    later.handleDeleted();
    expect(first.store.has('padDeletionToken:p')).toBe(false);
  });
});

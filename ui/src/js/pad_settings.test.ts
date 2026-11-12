// @ts-nocheck
import {afterEach, beforeEach, describe, expect, it} from 'vitest';
import {
  createPadSettings, creatorDefaults, effectiveOptions, isEnforcedForMe, mergePadOptions,
  myViewOverrides, normalizeChatOptions,
} from './pad_settings';

const padWide = (over = {}) => ({
  enforceSettings: false, showChat: true, alwaysShowChat: false, chatAndUsers: false, lang: null,
  view: {showAuthorColors: true, showLineNumbers: true, rtlIsTrue: false, padFontFamily: '', fadeInactiveAuthorColors: true},
  ...over,
});

describe('normalizeChatOptions', () => {
  it('turns sticky chat and chat-and-users off with the chat', () => {
    expect(normalizeChatOptions({showChat: false, alwaysShowChat: true, chatAndUsers: true}))
        .toEqual({showChat: false, alwaysShowChat: false, chatAndUsers: false});
  });
  it('chat and users implies a visible, sticky chat', () => {
    expect(normalizeChatOptions({chatAndUsers: true})).toEqual({chatAndUsers: true, showChat: true, alwaysShowChat: true});
  });
  it('sticky chat implies a visible chat', () => {
    expect(normalizeChatOptions({alwaysShowChat: true})).toEqual({alwaysShowChat: true, showChat: true});
  });
});

describe('myViewOverrides', () => {
  it('maps the stored preferences and leaves unset ones null', () => {
    const prefs = {chatAlwaysVisible: true, showAuthorshipColors: false};
    const o = myViewOverrides((k) => prefs[k] ?? null, null);
    expect(o.alwaysShowChat).toBe(true);
    expect(o.showChat).toBeNull();
    expect(o.view.showAuthorColors).toBe(false);
    expect(o.view.showLineNumbers).toBeNull();
    expect('lang' in o).toBe(false);
  });
  it('includes the chosen language', () => {
    expect(myViewOverrides(() => null, 'de').lang).toBe('de');
  });
});

describe('effectiveOptions', () => {
  const overrides = {showChat: null, alwaysShowChat: true, chatAndUsers: null, lang: 'fr',
    view: {showLineNumbers: true, showAuthorColors: null}};

  it('own preferences win over pad-wide values that are not enforced', () => {
    const e = effectiveOptions(padWide({view: {...padWide().view, showLineNumbers: false}}), overrides, false);
    expect(e.view.showLineNumbers).toBe(true);
    expect(e.alwaysShowChat).toBe(true);
    expect(e.lang).toBe('fr');
  });
  it('pad-wide values fill in what the user did not set', () => {
    const e = effectiveOptions(padWide({view: {...padWide().view, showAuthorColors: false}}), overrides, false);
    expect(e.view.showAuthorColors).toBe(false);
  });
  it('enforced pad-wide values ignore the own preferences', () => {
    const e = effectiveOptions(padWide({enforceSettings: true, view: {...padWide().view, showLineNumbers: false}}), overrides, true);
    expect(e.view.showLineNumbers).toBe(false);
    expect(e.alwaysShowChat).toBe(false);
    expect(e.lang).toBeNull();
  });
  it('a disabled chat stays disabled even if the user wants it sticky', () => {
    const e = effectiveOptions(padWide({showChat: false}), {...overrides, showChat: null}, false);
    expect(e.showChat).toBe(false);
    expect(e.alwaysShowChat).toBe(false);
    // The user can turn the chat back on for themselves.
    expect(effectiveOptions(padWide({showChat: false}), {...overrides, showChat: true}, false).showChat).toBe(true);
    const enforced = effectiveOptions(padWide({showChat: false, enforceSettings: true}), overrides, true);
    expect(enforced.showChat).toBe(false);
    expect(enforced.alwaysShowChat).toBe(false);
  });
  it('does not change its input', () => {
    const input = padWide();
    effectiveOptions(input, overrides, false);
    expect(input).toEqual(padWide());
  });
});

describe('isEnforcedForMe', () => {
  it('only when enforced and the user is not the creator', () => {
    expect(isEnforcedForMe({enforceSettings: true}, false)).toBe(true);
    expect(isEnforcedForMe({enforceSettings: true}, true)).toBe(false);
    expect(isEnforcedForMe({enforceSettings: false}, false)).toBe(false);
    expect(isEnforcedForMe(null, false)).toBe(false);
  });
});

describe('mergePadOptions', () => {
  it('merges the view key by key and coerces flags like upstream', () => {
    const next = mergePadOptions(padWide(), {showChat: 'false', view: {rtlIsTrue: true}}, false);
    expect(next.showChat).toBe(false);
    expect(next.view.rtlIsTrue).toBe(true);
    expect(next.view.showLineNumbers).toBe(true);
  });
  it('keeps lang as given', () => {
    expect(mergePadOptions(padWide(), {lang: 'de'}, false).lang).toBe('de');
  });
  it('passes plugin keys only when plugin pad options are on', () => {
    expect(mergePadOptions(padWide(), {ep_toc: {on: true}, other: 1}, true).ep_toc).toEqual({on: true});
    expect(mergePadOptions(padWide(), {ep_toc: {on: true}}, false).ep_toc).toBeUndefined();
    expect(mergePadOptions(padWide(), {other: 1}, true).other).toBeUndefined();
  });
});

describe('creatorDefaults', () => {
  it('sends only what the user set', () => {
    expect(creatorDefaults(myViewOverrides(() => null, null))).toBeUndefined();
    const prefs = {chatAlwaysVisible: true, padFontFamily: 'RobotoMono'};
    expect(creatorDefaults(myViewOverrides((k) => prefs[k] ?? null, 'de')))
        .toEqual({alwaysShowChat: true, lang: 'de', view: {padFontFamily: 'RobotoMono'}});
  });
});

// createPadSettings against a minimal fake page.
describe('createPadSettings', () => {
  let elements;
  const el = (id) => (elements[id] ??= {id, checked: false, disabled: false, hidden: false,
    setAttribute() {}, removeAttribute() {}, querySelector: () => null, addEventListener() {}});

  beforeEach(() => {
    elements = {};
    for (const id of ['options-stickychat', 'options-linenoscheck', 'viewfontmenu', 'enforce-settings-notice',
      'pad-settings-section', 'padsettings-enforcecheck', 'padsettings-options-linenoscheck']) el(id);
    elements['enforce-settings-notice'].hidden = true;
    elements['pad-settings-section'].hidden = true;
    globalThis.document = {getElementById: (id) => elements[id] ?? null};
    globalThis.CSS = {escape: (s) => s};
  });
  afterEach(() => { delete globalThis.document; delete globalThis.CSS; });

  const setup = ({canEdit = false, padOptions = padWide(), prefs = {}} = {}) => {
    const calls = {view: [], sticky: [], chatAndUsers: [], enabled: [], sent: [], lang: []};
    let options = padOptions;
    const ps = createPadSettings({
      getPadOptions: () => options,
      setPadOptions: (o) => { options = o; },
      canEdit: () => canEdit,
      pluginOptions: () => false,
      getPref: (k) => prefs[k] ?? null,
      setPref: (k, v) => { prefs[k] = v; },
      language: () => null,
      setViewOptions: (v) => calls.view.push(v),
      chat: {
        setSticky: (w, p) => calls.sticky.push([w, p]),
        setChatAndUsers: (w, p) => calls.chatAndUsers.push([w, p]),
        setEnabled: (e) => calls.enabled.push(e),
      },
      applyLanguage: (l) => calls.lang.push(l),
      send: (o) => calls.sent.push(o),
    });
    return {ps, calls, prefs, options: () => options};
  };

  it('applies own preferences over pad-wide defaults and never stores imposed values', () => {
    const {ps, calls} = setup({padOptions: padWide({alwaysShowChat: true}), prefs: {showLineNumbers: false}});
    ps.apply();
    expect(calls.view.at(-1).showLineNumbers).toBe(false);
    expect(calls.sticky.at(-1)).toEqual([true, false]);
    expect(calls.enabled.at(-1)).toBe(true);
    expect(elements['enforce-settings-notice'].hidden).toBe(true);
    expect(elements['options-linenoscheck'].disabled).toBe(false);
  });

  it('locks the own controls and shows the notice when enforced', () => {
    const {ps, calls} = setup({
      padOptions: padWide({enforceSettings: true, showChat: false, lang: 'de', view: {...padWide().view, showLineNumbers: false}}),
      prefs: {showLineNumbers: true},
    });
    ps.apply();
    expect(calls.view.at(-1).showLineNumbers).toBe(false);
    expect(calls.enabled.at(-1)).toBe(false);
    expect(elements['enforce-settings-notice'].hidden).toBe(false);
    expect(elements['options-linenoscheck'].disabled).toBe(true);
    expect(elements['viewfontmenu'].disabled).toBe(true);
    expect(elements['pad-settings-section'].hidden).toBe(true, 'not the creator');
  });

  it('applies the chosen language of an enforcing creator', () => {
    const {ps, calls} = setup({padOptions: padWide({enforceSettings: true, lang: 'de'})});
    ps.apply();
    expect(calls.lang).toEqual(['de']);
  });

  it('shows the pad-wide section to the creator, who is never enforced upon', () => {
    const {ps} = setup({canEdit: true, padOptions: padWide({enforceSettings: true, view: {...padWide().view, showLineNumbers: false}})});
    ps.apply();
    expect(elements['pad-settings-section'].hidden).toBe(false);
    expect(elements['padsettings-enforcecheck'].checked).toBe(true);
    expect(elements['padsettings-options-linenoscheck'].checked).toBe(false);
    expect(elements['enforce-settings-notice'].hidden).toBe(true);
  });

  it('a creator change is applied, sent, and adopted as the creator\'s own view preference', () => {
    const {ps, calls, prefs, options} = setup({canEdit: true, prefs: {showLineNumbers: true}});
    ps.changePadViewOption('showLineNumbers', false);
    expect(calls.sent).toEqual([{view: {showLineNumbers: false}}]);
    expect(options().view.showLineNumbers).toBe(false);
    expect(prefs.showLineNumbers).toBe(false);
    expect(calls.view.at(-1).showLineNumbers).toBe(false);
  });

  it('a broadcast change is merged without touching own preferences', () => {
    const {ps, prefs, options} = setup();
    ps.handleChange(padWide({enforceSettings: true, showChat: false}));
    expect(options().enforceSettings).toBe(true);
    expect(options().showChat).toBe(false);
    expect(prefs).toEqual({});
  });
});

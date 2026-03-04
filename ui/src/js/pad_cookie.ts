import html10n from './i18n';
import {Cookies} from './pad_utils';

type PadPrefs = Record<string, unknown>;

class PadCookie {
  private readonly cookieName_: string;

  constructor() {
    this.cookieName_ = window.location.protocol === 'https:' ? 'prefs' : 'prefsHttp';
  }

  init(): void {
    const prefs = this.readPrefs_() ?? {};
    delete prefs.userId;
    delete prefs.name;
    delete prefs.colorId;
    this.writePrefs_(prefs);

    if (this.readPrefs_() == null) {
      const msg = html10n.get('pad.noCookie');
      console.error(msg);
      alert(msg);
    }
  }

  private readPrefs_(): PadPrefs | null {
    try {
      const json = Cookies.get(this.cookieName_);
      if (json == null) return null;
      return JSON.parse(json) as PadPrefs;
    } catch {
      return null;
    }
  }

  private writePrefs_(prefs: PadPrefs): void {
    Cookies.set(this.cookieName_, JSON.stringify(prefs), {expires: 365 * 100});
  }

  getPref(prefName: string): unknown {
    const prefs = this.readPrefs_() ?? {};
    return prefs[prefName];
  }

  setPref(prefName: string, value: unknown): void {
    const prefs = this.readPrefs_() ?? {};
    prefs[prefName] = value;
    this.writePrefs_(prefs);
  }

  clear(): void {
    this.writePrefs_({});
  }
}

export const padcookie = new PadCookie();

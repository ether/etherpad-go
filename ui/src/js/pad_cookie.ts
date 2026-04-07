const STORAGE_KEY = 'etherpad_prefs';

function readAll(): Record<string, unknown> {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    return raw ? JSON.parse(raw) : {};
  } catch { return {}; }
}

function writeAll(prefs: Record<string, unknown>): void {
  try { localStorage.setItem(STORAGE_KEY, JSON.stringify(prefs)); } catch {}
}

export const padcookie = {
  init(): void { /* no-op — localStorage needs no initialization */ },

  getPref(key: string): unknown {
    return readAll()[key] ?? null;
  },

  setPref(key: string, value: unknown): void {
    const prefs = readAll();
    prefs[key] = value;
    writeAll(prefs);
  },

  clear(): void {
    try { localStorage.removeItem(STORAGE_KEY); } catch {}
  },
};

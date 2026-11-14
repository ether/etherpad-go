// Session transfer (etherpad-lite's welcome.ts): moves this browser's author
// identity to another browser or device. The author token is an HttpOnly
// cookie, so the server reads it off the POST itself; the client only adds
// its preferences. Opening the link (or pasting the code) in the other
// browser redeems it once: the server sets the token cookie there, the
// client stores the preferences and reloads.

export const PREFS_KEY = 'etherpad_prefs';

// Go issues 32 hex characters; lite issues UUIDs. Both are accepted so a code
// from either flavour is recognised.
const ID_RE = /^(?:[0-9a-f]{32}|[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})$/i;

export const isValidTransferId = (id: unknown): id is string =>
  typeof id === 'string' && ID_RE.test(id.trim());

export type FetchLike = (url: string, init?: {method?: string; headers?: Record<string, string>; body?: string}) =>
  Promise<{ok: boolean; json: () => Promise<unknown>}>;

export interface StorageLike {
  getItem(key: string): string | null;
  setItem(key: string, value: string): void;
}

const safeJson = async (r: {json: () => Promise<unknown>}): Promise<unknown> => {
  try { return await r.json(); } catch { return null; }
};

// createTransfer asks the server for a transfer id. It resolves to null on
// any failure (no author cookie yet, network error, malformed answer).
export const createTransfer = async (fetchFn: FetchLike, storage: StorageLike): Promise<string | null> => {
  let prefs: string | null = null;
  try { prefs = storage.getItem(PREFS_KEY); } catch { /* storage blocked */ }
  try {
    const r = await fetchFn('./tokenTransfer', {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify({prefs: prefs ?? ''}),
    });
    const data = await safeJson(r) as {id?: unknown} | null;
    if (!r.ok || data == null || typeof data !== 'object') return null;
    return isValidTransferId(data.id) ? data.id : null;
  } catch {
    return null;
  }
};

// redeemTransfer redeems a code. On success the server has set the author
// cookie; the preferences it returns replace this browser's.
export const redeemTransfer = async (code: string, fetchFn: FetchLike, storage: StorageLike): Promise<boolean> => {
  const id = code.trim();
  if (!isValidTransferId(id)) return false;
  try {
    // POST: redeeming replaces this browser's identity, which a plain link
    // must not do (the server refuses GET and requests from other sites).
    const r = await fetchFn(`./tokenTransfer/${encodeURIComponent(id)}`, {method: 'POST'});
    const data = await safeJson(r) as {ok?: unknown; prefs?: unknown} | null;
    if (!r.ok || data == null || typeof data !== 'object' || data.ok !== true) return false;
    if (typeof data.prefs === 'string' && data.prefs !== '') {
      try { storage.setItem(PREFS_KEY, data.prefs); } catch { /* storage blocked */ }
    }
    return true;
  } catch {
    return false;
  }
};

// transferLink is the welcome page URL that pre-fills the code. base is the
// page's base URL (document.baseURI), so a proxy prefix is kept.
export const transferLink = (base: string, id: string): string => {
  const u = new URL(base);
  u.search = '';
  u.hash = '';
  u.searchParams.set('transfer', id);
  return u.toString();
};

// codeFromLocation returns the code a transfer link carries, if any.
export const codeFromLocation = (href: string): string | null => {
  const v = new URL(href).searchParams.get('transfer');
  return isValidTransferId(v) ? v.trim() : null;
};

// Client-local "recently edited" list for the welcome launcher. Pads have no
// per-user ownership server-side, so recents live in localStorage: pad/sheet
// pages record a visit on load, the welcome page renders the list.

export interface RecentDoc {
  name: string;
  type: 'p' | 's'; // pad | sheet
  ts: number;
}

const KEY = 'ep_recentDocs';
const MAX = 20;

export function recordRecent(type: 'p' | 's', name: string): void {
  if (!name) return;
  try {
    const list = listRecent().filter((d) => !(d.type === type && d.name === name));
    list.unshift({ name, type, ts: Date.now() });
    localStorage.setItem(KEY, JSON.stringify(list.slice(0, MAX)));
  } catch {
    // localStorage unavailable (private mode etc.) — recents are best-effort
  }
}

export function listRecent(): RecentDoc[] {
  try {
    const raw = localStorage.getItem(KEY);
    if (!raw) return [];
    const parsed: unknown = JSON.parse(raw);
    if (!Array.isArray(parsed)) return [];
    return parsed.filter(
      (d): d is RecentDoc =>
        !!d && typeof d === 'object' &&
        typeof (d as RecentDoc).name === 'string' &&
        ((d as RecentDoc).type === 'p' || (d as RecentDoc).type === 's') &&
        Number.isFinite((d as RecentDoc).ts),
    );
  } catch {
    return [];
  }
}

// recordCurrentDoc records the doc the current page shows, derived from the
// /p/<name> or /s/<name> URL. Call from the pad/sheet entry points.
export function recordCurrentDoc(): void {
  const m = /\/(p|s)\/([^/?#]+)/.exec(location.pathname);
  if (!m) return;
  let name = m[2];
  try {
    name = decodeURIComponent(name);
  } catch {
    // malformed percent-encoding: keep the raw segment, never block editor boot
  }
  recordRecent(m[1] as 'p' | 's', name);
}

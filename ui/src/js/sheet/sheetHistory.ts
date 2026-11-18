// Version history of a sheet: the stored changes, grouped into the versions
// the history panel lists, and the requests behind it (lib/api/sheetio).

export interface Revision {
  rev: number;
  timestamp: number;
  author?: string;
}

export interface HistoryAuthor {
  name?: string;
  color?: string;
}

export interface HistoryResponse {
  head: number;
  available: boolean;
  revisions: Revision[];
  authors: Record<string, HistoryAuthor>;
}

// A version: consecutive changes with less than GROUP_GAP_MS between them.
// rev is its last change; showing a version shows the sheet after it.
export interface Version {
  rev: number;
  from: number;
  start: number;
  end: number;
  authors: string[];
}

export const GROUP_GAP_MS = 5 * 60 * 1000;

// groupRevisions turns the changes (oldest first) into versions, newest
// first. A change by someone else starts a new version only after a pause,
// like in Google Sheets: who worked on a version is listed with it.
export function groupRevisions(revs: Revision[], gapMs = GROUP_GAP_MS): Version[] {
  const out: Version[] = [];
  let cur: Version | null = null;
  for (const r of revs) {
    if (!cur || r.timestamp - cur.end >= gapMs || r.timestamp < cur.end - gapMs) {
      cur = { rev: r.rev, from: r.rev, start: r.timestamp, end: r.timestamp, authors: [] };
      out.push(cur);
    }
    cur.rev = r.rev;
    cur.end = Math.max(cur.end, r.timestamp);
    if (r.author && !cur.authors.includes(r.author)) cur.authors.push(r.author);
  }
  return out.reverse();
}

const base = (): string => location.pathname;

const asError = async (res: Response): Promise<Error> => {
  const text = (await res.text().catch(() => '')).trim();
  return new Error(text || `${res.status} ${res.statusText}`);
};

export async function fetchHistory(): Promise<HistoryResponse> {
  const res = await fetch(`${base()}/history`, { credentials: 'same-origin' });
  if (!res.ok) throw await asError(res);
  return res.json() as Promise<HistoryResponse>;
}

export async function fetchVersion<T>(rev: number): Promise<T> {
  const res = await fetch(`${base()}/history/${rev}`, { credentials: 'same-origin' });
  if (!res.ok) throw await asError(res);
  return ((await res.json()) as { snapshot: T }).snapshot;
}

export async function restoreVersion(rev: number): Promise<void> {
  const res = await fetch(`${base()}/history/${rev}/restore`, { method: 'POST', credentials: 'same-origin' });
  if (!res.ok) throw await asError(res);
}

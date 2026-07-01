// TS mirror of lib/sheet.StylePool. Dedups styles by content so client-side
// interning of confirmed ops stays in lockstep with the Go server (both start
// from the same snapshot pool and intern the same ordered ops). The canonical
// key need only be injective on props content — it need not byte-match Go's.

export type StyleProps = Record<string, string>;

function canonicalKey(props: StyleProps): string {
  const keys = Object.keys(props).sort();
  if (keys.length === 0) return '';
  return JSON.stringify(keys.map((k) => [k, props[k]]));
}

export class StylePoolMirror {
  private idToStyle = new Map<number, StyleProps>();
  private keyToId = new Map<string, number>([['', 0]]);
  private nextId = 1;

  put(props: StyleProps): number {
    const key = canonicalKey(props);
    const existing = this.keyToId.get(key);
    if (existing !== undefined) return existing;
    const id = this.nextId++;
    this.idToStyle.set(id, { ...props });
    this.keyToId.set(key, id);
    return id;
  }

  // Returns the LIVE pool object (not a copy) — mutating it in place would
  // corrupt dedup/convergence for every other cell sharing this style id.
  get(id: number): StyleProps | undefined {
    if (id === 0) return {};
    return this.idToStyle.get(id);
  }

  // seed loads a serialized Go StylePool ({ idToStyle: {id: {props}}, nextId }).
  seed(snap: unknown): void {
    this.idToStyle = new Map();
    this.keyToId = new Map([['', 0]]);
    this.nextId = 1;
    if (!snap || typeof snap !== 'object') return;
    const s = snap as { idToStyle?: Record<string, { props?: StyleProps }>; nextId?: number };
    for (const [idStr, style] of Object.entries(s.idToStyle ?? {})) {
      const id = Number(idStr);
      const props = style?.props ?? {};
      this.idToStyle.set(id, props);
      this.keyToId.set(canonicalKey(props), id);
    }
    if (typeof s.nextId === 'number' && s.nextId > 0) this.nextId = s.nextId;
  }
}

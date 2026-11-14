// getHomeUrl resolves the start page from a pad URL (etherpad-lite #8111).
// Pad pages live at {prefix}/p/{padId}: one ".." drops the pad id and lands
// on {prefix}/, at the root and under a reverse proxy prefix alike. A query
// or hash on the pad URL does not count, unlike appending "/../.." to the
// href, which ended up in the query of ?userName=... links.
export const getHomeUrl = (fromHref: string): string =>
  new URL('..', fromHref).href;

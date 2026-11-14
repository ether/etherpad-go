// basePath is the path prefix Etherpad is served under, taken from the page
// URL: "/etherpad" for /etherpad/p/foo, /etherpad/p/foo/timeslider or
// /etherpad/s/bar; "" at the root. Pad ids have no "/", so the last /p/ or
// /s/ segment is the page's own.
export function basePath(pathname: string = window.location.pathname): string {
    const m = pathname.match(/^(.*?)\/(?:p|s)\/[^/]+(?:\/timeslider)?\/?$/)
    return m ? m[1] : ''
}

// siteUrl is path ("/static/plugins/...") under the prefix, for plugins
// that load their own files: an absolute "/static/..." would leave a
// reverse proxy's prefix, a relative one breaks on the timeslider.
export function siteUrl(path: string, pathname?: string): string {
    return basePath(pathname) + (path.startsWith('/') ? path : '/' + path)
}

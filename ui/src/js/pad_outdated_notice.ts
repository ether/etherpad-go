// A notice for the first author of a pad that this Etherpad is a minor
// version or more behind (etherpad-lite's pad_outdated_notice). The server
// decides who is told (/api/version-status); a failure never stops the pad.

export interface OutdatedResponse {
  outdated: 'minor' | null
  isFirstAuthor: boolean
}

export interface OutdatedDeps {
  padId: () => string | null
  // The page's own URL: the API sits next to /p/, also under a prefix.
  href: () => string
  fetch: (url: string) => Promise<{ok: boolean, json: () => Promise<unknown>}>
  t: (key: string) => string
  show: (title: string, text: string) => void
}

export const versionStatusUrl = (href: string, padId: string): string =>
  new URL(`../api/version-status?padId=${encodeURIComponent(padId)}`, href).href

export const maybeShowOutdatedNotice = async (d: OutdatedDeps): Promise<boolean> => {
  const padId = d.padId()
  if (!padId) return false
  try {
    const res = await d.fetch(versionStatusUrl(d.href(), padId))
    if (!res.ok) return false
    const data = (await res.json()) as OutdatedResponse
    if (data?.outdated !== 'minor' || data.isFirstAuthor !== true) return false
    d.show(d.t('pad.outdatedNotice.title'), d.t('pad.outdatedNotice.text'))
    return true
  } catch {
    return false
  }
}

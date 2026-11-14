import {describe, expect, it} from 'vitest'
import {maybeShowOutdatedNotice, versionStatusUrl, type OutdatedDeps} from './pad_outdated_notice'

const deps = (body: unknown, opts: Partial<OutdatedDeps> & {ok?: boolean} = {}) => {
  const shown: string[] = []
  const urls: string[] = []
  const d: OutdatedDeps = {
    padId: () => 'my pad',
    href: () => 'https://example.com/sub/p/my%20pad',
    fetch: async (url) => { urls.push(url); return {ok: opts.ok ?? true, json: async () => body} },
    t: (k) => k,
    show: (title, text) => shown.push(`${title}|${text}`),
    ...opts,
  }
  return {d, shown, urls}
}

describe('outdated notice', () => {
  it('asks next to /p/, under a proxy prefix too', () => {
    expect(versionStatusUrl('https://example.com/p/x', 'a b')).toBe('https://example.com/api/version-status?padId=a%20b')
    expect(versionStatusUrl('https://example.com/sub/p/x?userName=y', 'x')).toBe('https://example.com/sub/api/version-status?padId=x')
  })

  it('shows the notice to the first author when the server says outdated', async () => {
    const {d, shown, urls} = deps({outdated: 'minor', isFirstAuthor: true})
    expect(await maybeShowOutdatedNotice(d)).toBe(true)
    expect(shown).toEqual(['pad.outdatedNotice.title|pad.outdatedNotice.text'])
    expect(urls).toEqual(['https://example.com/sub/api/version-status?padId=my%20pad'])
  })

  it('shows nothing otherwise', async () => {
    for (const body of [{outdated: null, isFirstAuthor: false}, {outdated: 'minor', isFirstAuthor: false},
      {outdated: null, isFirstAuthor: true}, null, 'garbage']) {
      const {d, shown} = deps(body)
      expect(await maybeShowOutdatedNotice(d)).toBe(false)
      expect(shown).toEqual([])
    }
    const notOk = deps({outdated: 'minor', isFirstAuthor: true}, {ok: false})
    expect(await maybeShowOutdatedNotice(notOk.d)).toBe(false)
    const noPad = deps({outdated: 'minor', isFirstAuthor: true}, {padId: () => null})
    expect(await maybeShowOutdatedNotice(noPad.d)).toBe(false)
    expect(noPad.urls).toEqual([])
  })

  it('never throws (a 404 when the updater is off, a network error)', async () => {
    const {d} = deps(null, {fetch: async () => { throw new Error('offline') }})
    await expect(maybeShowOutdatedNotice(d)).resolves.toBe(false)
  })
})

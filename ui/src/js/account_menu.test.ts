import {describe, expect, it} from 'vitest'
import {authUrl, back, initialOf, initAccountMenu, loginUrl} from './account_menu'

describe('account menu', () => {
  it('finds /auth next to /p/, under a proxy prefix too', () => {
    expect(authUrl('https://example.com/p/x', 'me')).toBe('https://example.com/auth/me')
    expect(authUrl('https://example.com/sub/p/a%2Fb?x=1', 'logout')).toBe('https://example.com/sub/auth/logout')
  })

  it('comes back to this pad, its encoded id and query kept', () => {
    expect(back('https://example.com/p/team%2Fnotes?x=1#h')).toBe('/p/team%2Fnotes?x=1')
    expect(loginUrl('https://example.com/p/team%2Fnotes?x=1'))
      .toBe('https://example.com/auth/login?redirect=%2Fp%2Fteam%252Fnotes%3Fx%3D1')
  })

  it('shows the first letter of the name', () => {
    expect(initialOf({loginAvailable: true, loggedIn: true, displayName: 'anna B.'})).toBe('A')
    expect(initialOf({loginAvailable: true, loggedIn: true, username: 'élise'})).toBe('É')
    expect(initialOf({loginAvailable: true, loggedIn: true})).toBe('?')
  })

  it('never stops the pad when the server does not answer', async () => {
    const d = {
      href: () => 'https://example.com/p/x',
      fetch: async () => { throw new Error('offline') },
      t: (_k: string, f: string) => f,
      toolbar: () => null,
      document: {} as Document,
    }
    expect(await initAccountMenu(d)).toBe(false)
    expect(await initAccountMenu({...d, fetch: async () => ({ok: false, json: async () => ({})})})).toBe(false)
  })
})

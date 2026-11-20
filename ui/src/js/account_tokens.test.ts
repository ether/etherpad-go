import {describe, expect, it} from 'vitest'
import {errorOf, expiryChoices, graceChoices, tokensUrl} from './account_tokens'

describe('my tokens', () => {
  it('finds /auth/tokens next to /p/, under a proxy prefix too', () => {
    expect(tokensUrl('https://example.com/p/x')).toBe('https://example.com/auth/tokens')
    expect(tokensUrl('https://example.com/sub/p/a%2Fb?x=1', 'ab/c', 'rotate'))
      .toBe('https://example.com/sub/auth/tokens/ab%2Fc/rotate')
  })

  it('offers lifetimes within the policy', () => {
    expect(expiryChoices(false, 0)).toEqual({days: [7, 30, 90, 365, 0], initial: 30})
    expect(expiryChoices(true, 0)).toEqual({days: [7, 30, 90, 365], initial: 30})
    expect(expiryChoices(true, 30)).toEqual({days: [7, 30], initial: 30})
    expect(expiryChoices(false, 14)).toEqual({days: [7, 14], initial: 14})
    expect(expiryChoices(false, 3)).toEqual({days: [3], initial: 3})
    expect(expiryChoices(false, 500)).toEqual({days: [7, 30, 90, 365, 500], initial: 30})
  })

  it('keeps an old key no longer than the token lives', () => {
    const now = 1_000_000
    const day = 24 * 60 * 60 * 1000
    expect(graceChoices(null, now)).toEqual([0, 1, 10, 20])
    expect(graceChoices(now + 15 * day, now)).toEqual([0, 1, 10])
  })

  it('shows the server\'s reason, or a fallback', async () => {
    expect(await errorOf({ok: false, status: 400, json: async () => ({error: 'name too long'})}, 'x')).toBe('name too long')
    expect(await errorOf({ok: false, status: 500, json: async () => { throw new Error('html') }}, 'x')).toBe('x')
  })
})

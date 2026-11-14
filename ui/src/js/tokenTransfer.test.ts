import {describe, expect, it} from 'vitest'
import {codeFromLocation, createTransfer, isValidTransferId, PREFS_KEY, redeemTransfer, transferLink, type FetchLike, type StorageLike} from './tokenTransfer'

const memStorage = (init: Record<string, string> = {}): StorageLike & {data: Record<string, string>} => {
  const data = {...init}
  return {data, getItem: (k) => data[k] ?? null, setItem: (k, v) => { data[k] = v }}
}

const ID = '0123456789abcdef0123456789abcdef'

const fakeFetch = (ok: boolean, body: unknown, calls: Array<{url: string; init?: unknown}> = []): FetchLike =>
  async (url, init) => { calls.push({url, init}); return {ok, json: async () => body} }

describe('token transfer', () => {
  it('accepts Go and lite ids only', () => {
    expect(isValidTransferId(ID)).toBe(true)
    expect(isValidTransferId('123e4567-e89b-12d3-a456-426614174000')).toBe(true)
    expect(isValidTransferId('../admin')).toBe(false)
    expect(isValidTransferId('')).toBe(false)
    expect(isValidTransferId(null)).toBe(false)
  })

  it('sends the prefs and returns the id', async () => {
    const calls: Array<{url: string; init?: any}> = []
    const s = memStorage({[PREFS_KEY]: '{"rtlIsTrue":true}'})
    expect(await createTransfer(fakeFetch(true, {id: ID}, calls), s)).toBe(ID)
    expect(calls[0].url).toBe('./tokenTransfer')
    expect(calls[0].init.method).toBe('POST')
    expect(JSON.parse(calls[0].init.body)).toEqual({prefs: '{"rtlIsTrue":true}'})
  })

  it('returns null on errors and on a malformed id', async () => {
    const s = memStorage()
    expect(await createTransfer(fakeFetch(false, {error: 'No author cookie to transfer'}), s)).toBeNull()
    expect(await createTransfer(fakeFetch(true, {id: '<script>'}), s)).toBeNull()
    expect(await createTransfer(async () => { throw new Error('offline') }, s)).toBeNull()
  })

  it('redeems once and stores the prefs', async () => {
    const calls: Array<{url: string}> = []
    const s = memStorage({[PREFS_KEY]: '{"old":1}'})
    expect(await redeemTransfer(` ${ID} `, fakeFetch(true, {ok: true, prefs: '{"new":2}'}, calls), s)).toBe(true)
    expect(calls[0].url).toBe(`./tokenTransfer/${ID}`)
    expect(s.data[PREFS_KEY]).toBe('{"new":2}')
  })

  it('keeps the prefs when redeeming fails', async () => {
    const s = memStorage({[PREFS_KEY]: '{"old":1}'})
    expect(await redeemTransfer(ID, fakeFetch(false, {error: 'Token expired'}), s)).toBe(false)
    expect(await redeemTransfer('not-a-code', fakeFetch(true, {ok: true}), s)).toBe(false)
    expect(s.data[PREFS_KEY]).toBe('{"old":1}')
  })

  it('builds and reads the link under a proxy prefix', () => {
    const link = transferLink('https://example.com/sub/?x=1#h', ID)
    expect(link).toBe(`https://example.com/sub/?transfer=${ID}`)
    expect(codeFromLocation(link)).toBe(ID)
    expect(codeFromLocation('https://example.com/?transfer=nope')).toBeNull()
  })
})

import {describe, expect, it} from 'vitest'
import {isReadOnlyLink} from './readonly_hint'

describe('read-only hint', () => {
  it('knows a read-only link by its r. id', () => {
    expect(isReadOnlyLink('https://example.com/p/r.0123456789abcdef')).toBe(true)
    expect(isReadOnlyLink('https://example.com/sub/p/r.abc?x=1#L3')).toBe(true)
    expect(isReadOnlyLink('https://example.com/p/r.abc/')).toBe(true)
    expect(isReadOnlyLink('https://example.com/p/test')).toBe(false)
    expect(isReadOnlyLink('https://example.com/p/rabbit')).toBe(false)
    expect(isReadOnlyLink('https://example.com/p/%E0%A4%A')).toBe(false)
  })
})

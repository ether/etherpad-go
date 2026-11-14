import {describe, it, expect} from 'vitest';
import {getHomeUrl} from './getHomeUrl';

// Port of etherpad-lite's getHomeUrl spec.
describe('getHomeUrl', () => {
  it('returns / for a root-deployed pad', () => {
    expect(getHomeUrl('https://example.com/p/testpad')).toBe('https://example.com/');
  });

  it('returns the proxy prefix home for a prefixed pad URL', () => {
    expect(getHomeUrl('https://example.com/etherpad/p/testpad')).toBe('https://example.com/etherpad/');
  });

  it('preserves a deep proxy prefix', () => {
    expect(getHomeUrl('https://example.com/api/hassio_ingress/abc/p/testpad'))
        .toBe('https://example.com/api/hassio_ingress/abc/');
  });

  it('ignores a query and a hash on the pad URL', () => {
    expect(getHomeUrl('https://example.com/p/testpad?userName=Ann&showChat=false')).toBe('https://example.com/');
    expect(getHomeUrl('https://example.com/sub/p/testpad#L3')).toBe('https://example.com/sub/');
  });
});

import {describe, expect, it} from 'vitest';
import {basePath} from './socketIoWrapper';
import {siteUrl} from './js/core/basePath';

describe('basePath', () => {
  it('is empty at the root', () => {
    expect(basePath('/p/foo')).toBe('');
    expect(basePath('/p/foo/timeslider')).toBe('');
    expect(basePath('/s/sheet1')).toBe('');
  });
  it('is the prefix a reverse proxy serves Etherpad under', () => {
    expect(basePath('/etherpad/p/foo')).toBe('/etherpad');
    expect(basePath('/a/b/p/foo/timeslider')).toBe('/a/b');
    expect(basePath('/pads/s/x')).toBe('/pads');
    expect(basePath('/api/hassio_ingress/tok/p/x')).toBe('/api/hassio_ingress/tok');
  });
  it('takes the last /p/ segment, so a prefix may contain one', () => {
    expect(basePath('/p/x/p/foo')).toBe('/p/x');
  });
  it('is empty for other pages', () => {
    expect(basePath('/')).toBe('');
    expect(basePath('/admin/')).toBe('');
  });
});

describe('siteUrl', () => {
  const css = '/static/plugins/ep_x/static/css/x.css';
  it('is the path itself at the root, on the pad and the timeslider', () => {
    expect(siteUrl(css, '/p/foo')).toBe(css);
    expect(siteUrl(css, '/p/foo/timeslider')).toBe(css);
  });
  it('stays under a proxy prefix', () => {
    expect(siteUrl(css, '/sub/p/foo')).toBe('/sub' + css);
    expect(siteUrl(css, '/sub/p/foo/timeslider')).toBe('/sub' + css);
    expect(siteUrl('static/x.css', '/sub/p/foo')).toBe('/sub/static/x.css');
  });
});

// @ts-nocheck -- runs in Node (vitest); the UI tsconfig has no Node types.
// Port of etherpad-lite's cssFontInlining spec (#8268): the pad
// stylesheets keep their fonts as external files.
import {describe, expect, it} from 'vitest';
import * as esbuild from 'esbuild';
import * as fs from 'node:fs';
import * as path from 'node:path';
// @ts-ignore plain JS module shared with build.js
import {cssBundles, cssExternal, rebaseUrls} from '../css-build.js';

const uiDir = path.resolve(__dirname, '..');
const staticRoot = path.resolve(uiDir, '../assets');

const build = async (entry: string, outdir = '/out') => {
  const result = await esbuild.build({
    absWorkingDir: uiDir, bundle: true, minify: true, write: false,
    entryPoints: [entry], outdir, external: cssExternal,
    // The loaders build.js uses: anything not external would be inlined.
    loader: {'.woff': 'base64', '.woff2': 'base64', '.ttf': 'base64', '.eot': 'base64', '.svg': 'base64',
      '.png': 'base64', '.jpg': 'base64', '.gif': 'base64', '.otf': 'base64'},
    logLevel: 'silent',
  });
  return result.outputFiles.find((f) => f.path.endsWith('.css'))!.text;
};

describe('CSS font URLs stay external (#8268)', () => {
  for (const bundle of cssBundles) {
    const entry = bundle.entryPoints[0];
    describe(entry, () => {
      it('does not inline font data as base64', async () => {
        expect(await build(entry)).not.toMatch(/data:(font|application\/(font|vnd\.ms-fontobject|x-font))/);
      });

      it('keeps the bundled stylesheet small', async () => {
        expect((await build(entry)).length).toBeLessThan(200 * 1024);
      });

      it('points every external font URL at a file, as served and under a proxy prefix', async () => {
        // What build.js writes: esbuild's output for the real outdir, rebased
        // to the URL the bundle is served at.
        const css = rebaseUrls(await build(entry, bundle.outdir), bundle, uiDir);
        const urls = [...css.matchAll(/url\(["']?([^"')]+)["']?\)/g)].map((m) => m[1])
            .filter((u) => /\.(woff2?|ttf|otf|eot|svg)([?#]|$)/.test(u) && !u.startsWith('data:'));
        for (const url of urls) {
          expect(url.startsWith('/'), `${url} is relative`).toBe(false);
          const clean = url.split('?')[0].split('#')[0];
          // The browser resolves it against the served URL; under /sub that
          // must stay below /sub.
          const served = path.posix.resolve(bundle.servedAt, clean);
          const underPrefix = path.posix.resolve('/sub' + bundle.servedAt, clean);
          expect(underPrefix, url).toBe('/sub' + served);
          const file = path.join(staticRoot, served);
          expect(fs.existsSync(file), `${url} -> ${file}`).toBe(true);
        }
      });
    });
  }

  it('keeps the icon font with its query-string URLs external', async () => {
    const css = await build('../assets/css/static/pad.css');
    expect(css).toContain('fontawesome-etherpad.woff?2');
    expect(css).toContain('fontawesome-etherpad.eot?2#iefix');
  });
});

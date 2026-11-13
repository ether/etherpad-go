// @ts-nocheck -- runs in Node (vitest); the UI tsconfig has no Node types.
// Port of etherpad-lite's cssFontInlining spec (#8268): the pad
// stylesheets keep their fonts as external files.
import {describe, expect, it} from 'vitest';
import * as esbuild from 'esbuild';
import * as fs from 'node:fs';
import * as path from 'node:path';
// @ts-ignore plain JS module shared with build.js
import {cssBundles, cssExternal} from '../css-build.js';

const uiDir = path.resolve(__dirname, '..');
const staticRoot = path.resolve(uiDir, '../assets');

const build = async (entry: string) => {
  const result = await esbuild.build({
    absWorkingDir: uiDir, bundle: true, minify: true, write: false,
    entryPoints: [entry], outdir: '/out', external: cssExternal,
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

      it('points every external font URL at a file under the static root', async () => {
        const css = await build(entry);
        const outDir = path.resolve(uiDir, bundle.outdir);
        const urls = [...css.matchAll(/url\(["']?([^"')]+)["']?\)/g)].map((m) => m[1])
            .filter((u) => /\.(woff2?|ttf|otf|eot|svg)([?#]|$)/.test(u) && !u.startsWith('data:'));
        for (const url of urls) {
          const resolved = path.resolve(outDir, url.split('?')[0].split('#')[0]);
          expect(resolved.startsWith(staticRoot + path.sep), url).toBe(true);
          expect(fs.existsSync(resolved), `${url} -> ${resolved}`).toBe(true);
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

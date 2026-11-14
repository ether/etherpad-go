import * as path from 'node:path';

// Options of the CSS bundles (used by build.js and its test).
//
// Fonts and images stay external files, served from /static: inlined as
// base64 they made the pad stylesheet half a megabyte that every pad load
// downloads and that the browser cannot cache separately (etherpad-lite
// #8268). esbuild's "*.woff" pattern does not match the icon font's
// "fontawesome-etherpad.woff?2", and *.otf was not listed at all, so the
// font and image directories are listed too (an external pattern may hold
// one wildcard only).
export const cssExternal = [
  '*.woff', '*.woff2', '*.ttf', '*.otf', '*.eot', '*.svg', '*.png', '*.jpg', '*.gif',
  '/font/*', 'font/*', '../../../font/*', '../../../images/*',
];

// servedAt is the URL directory the bundle is served from (lib/api/static:
// /css/static/pad.css answers with assets/css/build/static/pad.css).
export const cssBundles = [
  {entryPoints: ['../assets/css/skin/colibris/pad.css'], outdir: '../assets/css/build/skin/colibris', servedAt: '/css/skin/colibris'},
  {entryPoints: ['../assets/css/static/pad.css'], outdir: '../assets/css/build/static', servedAt: '/css/static'},
];

// esbuild writes the external url()s relative to the bundle's outdir; the
// browser resolves them against servedAt. rebaseUrls rewrites each one so
// it reaches the same file from servedAt (assets/<x> is served at /<x>),
// and stays relative, so a reverse proxy's path prefix is kept: with
// /css/static/pad.css one level shallower than its outdir, the fonts were
// requested above the prefix.
export const rebaseUrls = (css, bundle, uiDir) => {
  const assetsDir = path.resolve(uiDir, '../assets');
  const outDir = path.resolve(uiDir, bundle.outdir);
  return css.replace(/url\((["']?)(\.\.?\/[^"')?#]*)([^"')]*)\1\)/g, (all, q, rel, rest) => {
    const file = path.resolve(outDir, rel);
    if (!file.startsWith(assetsDir + path.sep)) return all;
    const urlPath = '/' + path.relative(assetsDir, file).split(path.sep).join('/');
    let target = path.posix.relative(bundle.servedAt, urlPath);
    if (!target.startsWith('.')) target = './' + target;
    return `url(${q}${target}${rest}${q})`;
  });
};

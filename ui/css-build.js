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

export const cssBundles = [
  {entryPoints: ['../assets/css/skin/colibris/pad.css'], outdir: '../assets/css/build/skin/colibris'},
  {entryPoints: ['../assets/css/static/pad.css'], outdir: '../assets/css/build/static'},
];

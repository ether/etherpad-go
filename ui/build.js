import * as esbuild from 'esbuild';


const relativePath = 'ep_etherpad-lite/static/js';

const moduleResolutionPath = "./src/js"

const alias = {
    [`${relativePath}/ace2_inner`]: `${moduleResolutionPath}/ace2_inner`,
    [`${relativePath}/ace2_common`]: `${moduleResolutionPath}/ace2_common`,
    [`${relativePath}/pluginfw/client_plugins`]: `${moduleResolutionPath}/pluginfw/client_plugins`,
    [`${relativePath}/rjquery`]: `${moduleResolutionPath}/rjquery`,
    [`${relativePath}/nice-select`]: `${moduleResolutionPath}/vendors/nice-select`,
};

const absWorkingDir = process.cwd()

await esbuild.buildSync({
    entryPoints: ["./src/pad.js"],
    absWorkingDir: absWorkingDir,
    bundle: true,
    write: true,
    outdir: '../assets/js/pad/assets',
    logLevel: 'info',
    metafile: true,
    target: 'es2020',
    alias, // should be an object like { react: 'preact/compat' }
    sourcemap: 'inline',
});

await esbuild.buildSync({
    entryPoints: ["./src/welcome.js"],
    absWorkingDir: absWorkingDir,
    bundle: true,
    write: true,
    outdir: '../assets/js/welcome/assets',
    logLevel: 'info',
    metafile: true,
    target: 'es2020',
    alias, // should be an object like { react: 'preact/compat' }
    sourcemap: 'inline',
});
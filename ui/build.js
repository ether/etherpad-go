import * as esbuild from 'esbuild';
import * as fs from "node:fs";
import {exec, execSync} from "node:child_process";


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

const loaders = {
    '.woff': 'base64',
    '.woff2': 'base64',
    '.ttf': 'base64',
    '.eot': 'base64',
    '.svg': 'base64',
    '.png': 'base64',
    '.jpg': 'base64',
    '.gif': 'base64',
    '.otf': 'base64',
}

await esbuild.buildSync({
    entryPoints: ["./src/pad.js"],
    absWorkingDir: absWorkingDir,
    bundle: true,
    write: true,
    minify: true,
    outdir: '../assets/js/pad/assets',
    logLevel: 'info',
    metafile: true,
    target: 'es2020',
    alias,
    loader:loaders,
    sourcemap: 'inline',
});

await esbuild.buildSync({
    entryPoints: ["./src/welcome.js"],
    absWorkingDir: absWorkingDir,
    bundle: true,
    write: true,
    minify: true,
    outdir: '../assets/js/welcome/assets',
    logLevel: 'info',
    metafile: true,
    target: 'es2020',
    alias,
    loader:loaders,
    sourcemap: 'inline',
});

await esbuild.buildSync({
    entryPoints: ["./src/timeslider.js"],
    absWorkingDir: absWorkingDir,
    bundle: true,
    write: true,
    minify: true,
    outdir: '../assets/js/timeslider/assets',
    logLevel: 'info',
    metafile: true,
    target: 'es2020',
    alias,
    loader: loaders,
    sourcemap: 'inline',
});

await esbuild.buildSync({
    entryPoints: ['../assets/css/skin/colibris/pad.css'],
    absWorkingDir: absWorkingDir,
    bundle: true,
    write: true,
    minify: true,
    outdir: '../assets/css/build/skin/colibris',
    logLevel: 'info',
    metafile: true,
    target: 'es2020',
    external: ['*.woff', '*.woff2', '*.ttf', '*.eot', '*.svg', '*.png', '*.jpg', '*.gif'],
    sourcemap: 'inline',
    loader:loaders,
})

execSync("pnpm run build-admin", {
    cwd: '../admin'
})

await esbuild.buildSync({
    entryPoints: ['../assets/css/static/pad.css'],
    absWorkingDir: absWorkingDir,
    bundle: true,
    write: true,
    minify: true,
    outdir: '../assets/css/build/static',
    logLevel: 'info',
    external: ['*.woff', '*.woff2', '*.ttf', '*.eot', '*.svg', '*.png', '*.jpg', '*.gif', '/font/*', 'font/*'],
    loader: loaders,
    metafile: true,
    target: 'es2020',
    sourcemap: 'inline',
})
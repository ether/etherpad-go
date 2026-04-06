import * as path from "node:path";
import {defineConfig} from "vite";
import commonjs from '@rollup/plugin-commonjs';

export default defineConfig(({ mode }) => {
    let entry: Record<string, string> = {};
    let outDir = '';

    if (mode === 'pad') {
        entry = { pad: path.resolve(__dirname, 'src/pad.entry.ts') };
        outDir = '../assets/js/pad';
    } else if (mode === 'welcome') {
        entry = { welcome: path.resolve(__dirname, 'src/welcome.entry.ts') };
        outDir = '../assets/js/welcome';
    } else if (mode === 'timeslider') {
        entry = { timeslider: path.resolve(__dirname, 'src/timeslider.entry.ts') };
        outDir = '../assets/js/timeslider';
    }



    return {
        build: {
            outDir: outDir,
            emptyOutDir: true,
            rollupOptions: {
                input: entry,
                output: {
                    entryFileNames: 'assets/[name].js',
                    chunkFileNames: 'assets/[name].js',
                    assetFileNames: 'assets/[name][extname]',
                },
            },
            commonjsOptions: {
                transformMixedEsModules: true
            },
            minify: false,
        },
        resolve: {
            alias: {
                '@': path.resolve(__dirname, '/src'),
            },
        },
        plugins: [
            commonjs(),
        ],
    };
});

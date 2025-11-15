import * as path from "node:path";
import {defineConfig} from "vite";
import commonjs from '@rollup/plugin-commonjs';

export default defineConfig(({ mode }) => {

    let entry = '';
    let outDir = '';

    if (mode === 'pad') {
        entry = path.resolve(__dirname, 'src/pad.js')
        outDir = '../assets/js/pad';
    } else if (mode === 'welcome') {
        entry = path.resolve(__dirname, 'src/welcome.js')
        outDir = '../assets/js/welcome';
    } else if (mode === 'timeslider') {
        entry = path.resolve(__dirname, 'src/timeslider.js')
        outDir = '../assets/js/timeslider';
    }



    return {
        build: {
            outDir: outDir,
            emptyOutDir: true,
            rollupOptions: {
                input: entry,
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
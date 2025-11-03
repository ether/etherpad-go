import * as path from "node:path";
import {defineConfig} from "vite";
import commonjs from '@rollup/plugin-commonjs';

export default defineConfig(({ mode }) => {
    const isPad = mode === 'pad';
    const entry = isPad ? path.resolve(__dirname, 'src/pad.js') : path.resolve(__dirname, 'src/welcome.js');
    const outDir = isPad ? '../assets/js/pad': '../assets/js/welcome' ;



    return {
        build: {
            outDir: outDir,
            emptyOutDir: true,
            rollupOptions: {
                input: entry,
            },
            minify: false,
        },
        resolve: {
            alias: {
                '@': path.resolve(__dirname, '/src'),
            },
        },
        plugins: [
            commonjs({
                requireReturnsDefault: 'auto',
            }),
        ],
    };
});
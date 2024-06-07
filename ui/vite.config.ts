import * as path from "node:path";
import {defineConfig} from "vite";
import commonjs from '@rollup/plugin-commonjs';

export default defineConfig({
    build:{
      minify: false
    },
    resolve: {
        alias: {
        '@': path.resolve(__dirname, '/src')
        }
    },
        plugins: [
            commonjs({
                requireReturnsDefault: 'auto', // <---- this solves default issue
            }),

            // vite4
            // vitePluginRequire.default()
        ],
    }
    )
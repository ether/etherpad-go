import {defineConfig} from 'vite'
import {viteStaticCopy} from "vite-plugin-static-copy";
import react from '@vitejs/plugin-react';

// https://vitejs.dev/config/
export default defineConfig({
    plugins: [viteStaticCopy({
        targets: [
            {
                src: '../assets/locales',
                dest: ''
            }
        ]
    }), react({
        babel: {
            plugins: ['babel-plugin-react-compiler'],
        }
    })],
    base: '/admin',
    build: {
        outDir: '../src/templates/admin',
        emptyOutDir: true,
    },
    server: {
        proxy: {
            '/admin/ws': {
                target: 'http://localhost:3000',
                changeOrigin: true,
                ws: true,
                configure: (proxy)=> {
                    // @ts-ignore
                    proxy.on('proxyReqWs', (proxyReq: any, req: any, socket: any, options: any, head: any) => {
                        proxyReq.setHeader('origin', 'http://localhost:3000');
                    });
                    // @ts-ignore
                    proxy.on('proxyReq', (proxyReq: any, req: any, res: any, options: any) => {
                        proxyReq.setHeader('origin', 'http://localhost:3000');
                    });
                }
            },
            '/admin-auth/': {
                target: 'http://localhost:3000',
                changeOrigin: true,
            }
        }
    }
})

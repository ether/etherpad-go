import {defineConfig, PluginOption} from 'vite'

function chartingLibrary(): PluginOption {
    return {
        name: 'charting-library',
        enforce: 'pre',
        apply: 'serve',
        transformIndexHtml: async (html, ctx)=>{
            const resp =  await fetch('http://localhost:3000/admin/index.html')
            return await resp.text()
        }
    };
}

export default defineConfig({
    plugins: [chartingLibrary()],
    base: '/admin',
    build: {
        outDir: 'dist',
        emptyOutDir: true,
        assetsDir: 'assets',
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

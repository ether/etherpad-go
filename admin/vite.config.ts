import {defineConfig, PluginOption} from 'vite'

function chartingLibrary(): PluginOption {
    return {
        name: 'charting-library',
        enforce: 'pre',
        apply: 'serve',
        transformIndexHtml: async (html, ctx)=>{
            return html.replace('<div id="loading"></div>', 'div id="loading"></div><span id="config" data-config="{&#34;authority&#34;:&#34;http://localhost:3000/oauth2/&#34;,&#34;clientId&#34;:&#34;admin_client&#34;,&#34;jwksUri&#34;:&#34;http://localhost:3000/oauth2/.well-known/jwks.json&#34;,&#34;redirectUri&#34;:&#34;http://localhost:5173/admin/&#34;,&#34;scope&#34;:[&#34;openid&#34;,&#34;profile&#34;,&#34;email&#34;,&#34;offline&#34;]}"></span>')
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

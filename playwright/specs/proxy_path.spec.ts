import {expect, Page, test} from "@playwright/test";
import http from "node:http";
import net, {AddressInfo} from "node:net";
import {randomUUID} from "node:crypto";

// Etherpad behind a reverse proxy that serves it under /sub, like nginx
// with `location /sub/ { proxy_pass http://etherpad/; proxy_set_header
// x-proxy-path /sub; }`. Every request the browser makes has to stay under
// the prefix: the proxy answers anything else with 404 and records it.

const PREFIX = '/sub';
const UPSTREAM = 9001;

let server: http.Server;
let base = '';
const outside: string[] = [];

const strip = (url: string): string | null =>
    url === PREFIX ? '/' : url.startsWith(PREFIX + '/') ? url.slice(PREFIX.length) : null;

test.beforeAll(async () => {
    server = http.createServer((req, res) => {
        const path = strip(req.url ?? '');
        if (path === null) {
            outside.push(req.url ?? '');
            res.writeHead(404).end('outside the prefix');
            return;
        }
        const up = http.request({
            host: '127.0.0.1', port: UPSTREAM, path, method: req.method,
            headers: {...req.headers, 'x-proxy-path': PREFIX},
        }, (r) => {
            res.writeHead(r.statusCode ?? 502, r.headers);
            r.pipe(res);
        });
        up.on('error', () => res.writeHead(502).end());
        req.pipe(up);
    });
    server.on('upgrade', (req, sock, head) => {
        const path = strip(req.url ?? '');
        if (path === null) {
            outside.push('ws ' + req.url);
            sock.destroy();
            return;
        }
        const up = net.connect(UPSTREAM, '127.0.0.1', () => {
            let s = `${req.method} ${path} HTTP/1.1\r\n`;
            for (let i = 0; i < req.rawHeaders.length; i += 2) s += `${req.rawHeaders[i]}: ${req.rawHeaders[i + 1]}\r\n`;
            up.write(s + `x-proxy-path: ${PREFIX}\r\n\r\n`);
            up.write(head);
            up.pipe(sock);
            sock.pipe(up);
        });
        up.on('error', () => sock.destroy());
        sock.on('error', () => up.destroy());
    });
    await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
    base = `http://127.0.0.1:${(server.address() as AddressInfo).port}${PREFIX}`;
});

test.afterAll(async () => {
    server.closeAllConnections();
    await new Promise((resolve) => server.close(resolve));
});

test.beforeEach(() => {
    outside.length = 0;
});

const failedRequests = (page: Page) => {
    const failed: string[] = [];
    page.on('response', (r) => { if (r.status() >= 400) failed.push(`${r.status()} ${r.url()}`); });
    page.on('requestfailed', (r) => failed.push(`failed ${r.url()}`));
    return failed;
};

const openPad = async (page: Page, padId: string) => {
    await page.goto(`${base}/p/${padId}`);
    await page.locator('#innerdocbody').waitFor({state: 'visible'});
    await expect(page.locator('#editorcontainer')).toHaveClass(/initialized/, {timeout: 15000});
};

test('the start page opens pads under the prefix', async ({page}) => {
    const failed = failedRequests(page);
    await page.goto(`${base}/`);
    await page.locator('#padname').fill('ProxyStart' + randomUUID().slice(0, 8));
    await page.locator('#go2Name button, #go2Name [type=submit]').first().click();
    await page.waitForURL(new RegExp(`${PREFIX}/p/ProxyStart`));
    await expect(page.locator('#editorcontainer')).toHaveClass(/initialized/, {timeout: 15000});
    expect(failed).toEqual([]);
    expect(outside).toEqual([]);
});

test('two clients edit a pad through the prefix', async ({browser}) => {
    const padId = 'ProxyPad' + randomUUID();
    const a = await (await browser.newContext()).newPage();
    const b = await (await browser.newContext()).newPage();
    const failedA = failedRequests(a);
    await openPad(a, padId);
    await openPad(b, padId);

    await a.locator('#innerdocbody').click();
    await a.keyboard.type('hello through the proxy');
    await expect(b.locator('#innerdocbody')).toContainText('hello through the proxy', {timeout: 10000});

    expect(failedA).toEqual([]);
    expect(outside).toEqual([]);
    await a.context().close();
    await b.context().close();
});

test('the timeslider works under the prefix', async ({page}) => {
    const padId = 'ProxyTs' + randomUUID();
    await openPad(page, padId);
    await page.locator('#innerdocbody').click();
    await page.keyboard.type('history');
    await page.waitForTimeout(500);

    const failed = failedRequests(page);
    await page.goto(`${base}/p/${padId}/timeslider`);
    await expect(page.locator('#innerdocbody')).toContainText('history', {timeout: 15000});
    expect(failed).toEqual([]);
    expect(outside).toEqual([]);
});

test('a trailing slash leads back to the pad', async ({page}) => {
    const padId = 'ProxySlash' + randomUUID();
    await page.goto(`${base}/p/${padId}/`);
    await expect(page).toHaveURL(`${base}/p/${padId}`);
    await expect(page.locator('#editorcontainer')).toHaveClass(/initialized/, {timeout: 15000});
    expect(outside).toEqual([]);
});

test('the admin page loads under the prefix', async ({page}) => {
    const failed = failedRequests(page);
    await page.goto(`${base}/admin/`);
    await expect(page.locator('#root')).not.toBeEmpty({timeout: 15000});
    await page.waitForLoadState('networkidle');
    expect(failed.filter((f) => !f.includes('/admin/validate'))).toEqual([]);
    expect(outside).toEqual([]);
});

test('the manifest is prefixed', async ({request}) => {
    const m = await (await request.get(`${base}/manifest.json`)).json();
    expect(m.start_url).toBe(`${PREFIX}/`);
    for (const icon of m.icons) {
        const r = await request.get(`http://${new URL(base).host}${icon.src}`);
        expect(r.status(), icon.src).toBe(200);
    }
});

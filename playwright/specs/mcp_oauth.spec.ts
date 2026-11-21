import {APIRequestContext, expect, test} from "@playwright/test";
import {createHash, randomBytes} from "node:crypto";
import {createServer} from "node:http";
import {AddressInfo} from "node:net";
import {goToNewPad} from "../helper/padHelper";

// An MCP client (as a claude.ai connector does) registers itself, the
// person logs in and allows it on the consent page, the client uses /mcp
// with its token; "My tokens" lists the app and disconnecting it stops
// the token.

const b64url = (b: Buffer) => b.toString('base64').replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');

const callTool = async (request: APIRequestContext, token: string, name: string, args: object) => {
    const resp = await request.post('/mcp', {
        headers: {Authorization: `Bearer ${token}`, 'Content-Type': 'application/json', Accept: 'application/json, text/event-stream'},
        data: {jsonrpc: '2.0', id: 1, method: 'tools/call', params: {name, arguments: args}},
    });
    return {status: resp.status(), body: resp.status() === 200 ? await resp.json() : null};
};

test('an MCP client connects with OAuth and the person can disconnect it', async ({page, request}) => {
    const available = (await (await request.get('/auth/me')).json()).loginAvailable;
    test.skip(!available, 'no SSO client of type "user" on this server');

    // The client's callback: a real server on a free port; the browser gets
    // there by a redirect, which page.route does not see.
    let code = '';
    const callback = createServer((req, res) => {
        const u = new URL(req.url ?? '/', 'http://127.0.0.1');
        if (u.searchParams.get('state') === 'xyz') code = u.searchParams.get('code') ?? '';
        res.end('back at the client');
    });
    await new Promise<void>((resolve) => callback.listen(0, '127.0.0.1', resolve));
    const redirect = `http://127.0.0.1:${(callback.address() as AddressInfo).port}/callback`;

    // Discovery, as a client finds it from the 401.
    const challenge = (await request.post('/mcp', {data: {}})).headers()['www-authenticate'];
    expect(challenge).toContain('resource_metadata=');
    const prm = await (await request.get('/.well-known/oauth-protected-resource/mcp')).json();
    const issuer = prm.authorization_servers[0];
    const as = await (await request.get(new URL('/.well-known/oauth-authorization-server', issuer).toString())).json();
    const appName = `Playwright AI ${randomBytes(4).toString('hex')}`;
    const reg = await request.post(as.registration_endpoint, {data: {client_name: appName, redirect_uris: [redirect]}});
    expect(reg.status()).toBe(201);
    const clientId = (await reg.json()).client_id;

    const padId = await goToNewPad(page);
    const verifier = b64url(randomBytes(32));
    const auth = new URL(as.authorization_endpoint);
    auth.search = new URLSearchParams({
        response_type: 'code', client_id: clientId, redirect_uri: redirect, state: 'xyz',
        scope: 'pad:read pad:write', resource: prm.resource,
        code_challenge: b64url(createHash('sha256').update(verifier).digest()), code_challenge_method: 'S256',
    }).toString();

    try {
    await page.goto(auth.toString());
    await page.fill('input[name="username"]', 'admin');
    await page.fill('input[name="password"]', 'changeme1');
    await page.click('input[type="submit"]');

    // The consent page: the app, the rights it asked for, the pads.
    await expect(page.locator('.consent-app')).toHaveText(appName);
    await expect(page.locator('input[name="scope"][value="pad:read"]')).toBeChecked();
    await expect(page.locator('input[name="scope"][value="pad:write"]')).toBeChecked();
    // Only what the client asked for is offered.
    await expect(page.locator('input[name="scope"][value="chat:write"]')).toHaveCount(0);
    await page.locator('input[name="scope"][value="pad:write"]').uncheck();
    await page.fill('input[name="pads"]', padId);
    await page.click('button[value="allow"]');
    await expect.poll(() => code).not.toBe('');
    } finally {
        callback.close();
    }

    const tok = await request.post(as.token_endpoint, {form: {
        grant_type: 'authorization_code', code, redirect_uri: redirect, client_id: clientId, code_verifier: verifier,
    }});
    expect(tok.status()).toBe(200);
    const {access_token: access, refresh_token: refresh} = await tok.json();
    expect(refresh).toBeTruthy();

    // The token reads the allowed pad, nothing else, and writes nothing.
    const read = await callTool(request, access, 'read_pad', {pad: padId});
    expect(read.status).toBe(200);
    expect(read.body.result.isError).toBeFalsy();
    const other = await callTool(request, access, 'read_pad', {pad: padId + 'x'});
    expect(other.body.result.isError).toBe(true);
    const write = await callTool(request, access, 'append_text', {pad: padId, text: 'x'});
    expect(write.body.result.isError).toBe(true);

    // "My tokens" lists the app; disconnecting it stops the token.
    await page.goto(`/p/${padId}`);
    await page.waitForSelector('#editorcontainer.initialized');
    const account = page.locator('#editbar li[data-key="account"]');
    // Logging in on the pad is its own login.
    await account.getByTestId('account-login').click();
    await page.fill('input[name="username"]', 'admin');
    await page.fill('input[name="password"]', 'changeme1');
    await page.click('input[type="submit"]');
    await page.waitForURL((u) => u.pathname === `/p/${padId}`);
    await page.waitForSelector('#editorcontainer.initialized');
    await account.getByTestId('account-button').click();
    await account.getByTestId('account-tokens-open').click();
    const dialog = page.getByTestId('account-tokens');
    const app = dialog.getByTestId('app').filter({hasText: appName});
    await expect(app).toHaveCount(1);
    await app.getByTestId('app-revoke').click();
    await app.getByTestId('app-revoke').click();
    await expect(dialog.getByTestId('app').filter({hasText: appName})).toHaveCount(0);

    expect((await callTool(request, access, 'read_pad', {pad: padId})).status).toBe(401);
    const again = await request.post(as.token_endpoint, {form: {grant_type: 'refresh_token', refresh_token: refresh, client_id: clientId}});
    expect(again.status()).not.toBe(200);
});

// typescript
// Datei: `admin/src/utils/authUtils.ts`
import {ConfigModel} from "../models/configModel.ts";

const getConfigFromHtmlFile = (): ConfigModel | undefined => {
    const config = document.getElementById('config')
    const dataJson = config?.getAttribute('data-config')
    if (dataJson) return JSON.parse(dataJson)
    return undefined
}

function generateState(byteLength = 32) {
    const randomBytes = new Uint8Array(byteLength);
    crypto.getRandomValues(randomBytes);
    return btoa(String.fromCharCode(...randomBytes))
        .replace(/\+/g, '-')
        .replace(/\//g, '_')
        .replace(/=/g, '');
}

function generateCodeVerifier(length = 128) {
    const array = new Uint8Array(length);
    window.crypto.getRandomValues(array);
    return Array.from(array, b => ('0' + (b % 36).toString(36)).slice(-1)).join('');
}

function base64UrlEncode(str: ArrayBuffer) {
    return btoa(String.fromCharCode(...new Uint8Array(str)))
        .replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

async function generateCodeChallenge(verifier: string) {
    const data = new TextEncoder().encode(verifier);
    const digest = await window.crypto.subtle.digest('SHA-256', data);
    return base64UrlEncode(digest);
}

function base64UrlDecode(input: string) {
    input = input.replace(/-/g, '+').replace(/_/g, '/');
    const pad = input.length % 4;
    if (pad) input += '='.repeat(4 - pad);
    const bin = atob(input);
    try {
        return decodeURIComponent(Array.prototype.map.call(bin, (c: string) =>
            '%' + ('00' + c.charCodeAt(0).toString(16)).slice(-2)
        ).join(''));
    } catch {
        return bin;
    }
}

export function decodeJwt(token: string): Record<string, unknown> | null {
    if (!token || token.split('.').length !== 3) return null;
    const payload = token.split('.')[1];
    try {
        const json = base64UrlDecode(payload);
        return JSON.parse(json);
    } catch {
        return null;
    }
}

export function isExpired(token: string|null, leewaySeconds = 0): boolean {
    if (!token) return true;
    const payload = decodeJwt(token);
    if (!payload) return true;
    const exp = typeof payload.exp === 'number' ? payload.exp : Number.parseInt(String((payload as any).exp || '0'), 10);
    if (!exp) return true;
    return Date.now() / 1000 > (exp + leewaySeconds);
}

const config = getConfigFromHtmlFile();

let refreshIntervalHandle: number | undefined;

function startRefreshInterval() {
    if (refreshIntervalHandle) return;
    refreshIntervalHandle = window.setInterval(async () => {
        const refreshToken = sessionStorage.getItem('refresh_token');
        if (!refreshToken) return;
        try {
            const resp = await fetch(config?.authority + "/../token", {
                method: 'POST',
                headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
                body: new URLSearchParams({
                    grant_type: 'refresh_token',
                    refresh_token: refreshToken,
                    client_id: config?.clientId ?? ''
                })
            });
            if (resp.ok) {
                const refreshData = await resp.json();
                if (refreshData.id_token) {
                    sessionStorage.setItem('refresh_token', refreshData.refresh_token);
                    sessionStorage.setItem('token', refreshData.id_token);
                }
            } else {
                console.error('Failed to refresh token', resp.statusText);
                sessionStorage.removeItem('refresh_token');
                sessionStorage.removeItem('pkce_code_verifier');
                sessionStorage.removeItem('token');
                globalThis.location.search = '';
                globalThis.location.reload();
            }
        } catch (e) {
            console.error('Refresh failed', e);
        }
    }, 60_000);
}

export async function initAuth(): Promise<string> {
    const token = sessionStorage.getItem('token');

    if (!isExpired(token, 60) && token) {
        // Token gültig -> start refresh loop und return
        startRefreshInterval();
        return token;
    }

    // Wenn `code` in URL -> Token mittels PKCE tauschen
    if (globalThis.location.search.includes('code=')) {
        try {
            const codeVerifier = sessionStorage.getItem('pkce_code_verifier') || '';
            const resp = await fetch(config?.authority + "/../token", {
                method: 'POST',
                headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
                body: new URLSearchParams({
                    grant_type: 'authorization_code',
                    code: new URLSearchParams(globalThis.location.search).get('code') || '',
                    redirect_uri: config?.redirectUri ?? '',
                    client_id: config?.clientId ?? '',
                    code_verifier: codeVerifier
                })
            });
            if (resp.ok) {
                const tokenResponse = await resp.json();
                if (tokenResponse.id_token) {
                    sessionStorage.setItem('token', tokenResponse.id_token);
                    sessionStorage.setItem('refresh_token', tokenResponse.refresh_token);
                    // Entferne query params und starte refresh loop
                    const params = new URLSearchParams(globalThis.location.search);
                    params.delete('code');
                    params.delete('state');
                    params.delete('iss');
                    params.delete('session_state');
                    const newSearch = params.toString();
                    const newUrl = globalThis.location.pathname + (newSearch ? '?' + newSearch : '');
                    globalThis.history.replaceState({}, '', newUrl);
                    startRefreshInterval();
                    return tokenResponse.id_token;
                }
            }
            // Fehler beim Token-Austausch
            sessionStorage.removeItem('refresh_token');
            sessionStorage.removeItem('token');
            sessionStorage.removeItem('pkce_code_verifier');
            globalThis.location.search = '';
            globalThis.location.reload();
        } catch (e) {
            console.error('OIDC token exchange failed', e);
            throw e;
        }
        return '';
    }

    // Kein Token und kein Code -> starte OIDC-Redirect (PKCE)
    if (!globalThis.location.href.includes("?error")) {
        const codeVerifier = generateCodeVerifier();
        sessionStorage.setItem('pkce_code_verifier', codeVerifier);
        const codeChallenge = await generateCodeChallenge(codeVerifier);
        const scope = "scope=" + encodeURIComponent((config?.scope ?? []).join(' '));
        const state = generateState();
        const requestUrl = `${config?.authority + "auth"}?client_id=${encodeURIComponent(config?.clientId ?? '')}&redirect_uri=${encodeURIComponent(config?.redirectUri ?? '')}&response_type=code&${scope}&code_challenge=${codeChallenge}&code_challenge_method=S256&state=${state}`;
        globalThis.location.replace(requestUrl);
        // Redirect endet Ausführung hier
    }
    throw new Error('Authentication failed');
}

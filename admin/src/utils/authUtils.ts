import {ConfigModel} from "../models/configModel.ts";

console.error(window.location.href)

const getConfigFromHtmlFile = (): ConfigModel | undefined => {
    const config = document.getElementById('config')

    const dataJson = config?.getAttribute('data-config')


    let configObj: ConfigModel | undefined


    if (dataJson) {
        configObj = JSON.parse(dataJson)
    }
    return configObj
}

function generateState(byteLength = 32) {
    const randomBytes = new Uint8Array(byteLength);
    crypto.getRandomValues(randomBytes);

    // Convert to base64url (no padding, URL-safe)
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


const state = generateState();

const config = getConfigFromHtmlFile();
console.error("Config is", config)


const token = sessionStorage.getItem('token')

function base64UrlDecode(input: string) {
    input = input.replace(/-/g, '+').replace(/_/g, '/');
    // add padding
    const pad = input.length % 4;
    if (pad) input += '='.repeat(4 - pad);
    const bin = atob(input);
    // decode UTF-8
    try {
        return decodeURIComponent(Array.prototype.map.call(bin, (c: string) =>
            '%' + ('00' + c.charCodeAt(0).toString(16)).slice(-2)
        ).join(''));
    } catch {
        return bin;
    }
}

export function isExpired(token: string|null, leewaySeconds = 0): boolean {
    if (!token) return true;
    const payload = decodeJwt(token);
    if (!payload) return true;
    const exp = typeof payload.exp === 'number' ? payload.exp : parseInt(String((payload as any).exp || '0'), 10);
    if (!exp) return true;
    return Date.now() / 1000 > (exp + leewaySeconds);
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


if (!isExpired(token, 60) && token) {
    let refreshToken = sessionStorage.getItem('refresh_token')
    if (!refreshToken) {
        sessionStorage.clear()
        window.location.reload()
        throw new Error('Refresh token not set')
    }
    setInterval(() => {
        refreshToken = sessionStorage.getItem('refresh_token') || refreshToken!
        fetch(config?.authority + "/../token", {
            method: 'POST',
            headers: {
                'Content-Type': 'application/x-www-form-urlencoded'
            },
            body: new URLSearchParams({
                grant_type: 'refresh_token',
                refresh_token: refreshToken,
                client_id: config?.clientId ?? ''
            })
        }).then((refreshResp) => {
            if (refreshResp.ok) {
                refreshResp.json().then((refreshData) => {
                    if (refreshData.id_token) {
                        sessionStorage.setItem('refresh_token', refreshData.refresh_token)
                        sessionStorage.setItem('token', refreshData.id_token)
                    }
                })
            }
        })
    }, 60_000)
} else {
    if (window.location.search.includes('code=')) {
        console.log('Redirecting to', window.location.href, "with client" + config?.clientId)
        try {
            const codeVerifier = sessionStorage.getItem('pkce_code_verifier') || '';
            const resp = await fetch(config?.authority + "/../token", {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/x-www-form-urlencoded'
                },
                body: new URLSearchParams({
                    grant_type: 'authorization_code',
                    code: new URLSearchParams(window.location.search).get('code') || '',
                    redirect_uri: config?.redirectUri ?? '',
                    client_id: config?.clientId ?? '',
                    code_verifier: codeVerifier
                })
            })
            if (resp.ok) {
                let tokenResponse = await resp.json()
                if (tokenResponse.id_token) {
                    sessionStorage.setItem('token', tokenResponse.id_token)
                    const params = new URLSearchParams(window.location.search);
                    params.delete('code');
                    params.delete('state');
                    params.delete('iss');
                    params.delete('session_state');
                    const newSearch = params.toString();
                    const newUrl = window.location.pathname + (newSearch ? '?' + newSearch : '');

                    window.history.replaceState({}, '', newUrl);
                }
                setInterval(() => {
                    console.log('Redirecting to', window.location.href, "with client" + config?.clientId);
                    if (tokenResponse.refresh_token) {
                        console.log(config)
                        fetch(config?.authority + "/../token", {
                            method: 'POST',
                            headers: {
                                'Content-Type': 'application/x-www-form-urlencoded'
                            },
                            body: new URLSearchParams({
                                grant_type: 'refresh_token',
                                refresh_token: tokenResponse.refresh_token,
                                client_id: config?.clientId ?? ''
                            })
                        }).then(async (refreshResp) => {
                            if (refreshResp.ok) {
                                refreshResp.json().then((refreshData) => {
                                    tokenResponse = refreshData;
                                    if (refreshData.id_token) {
                                        sessionStorage.setItem('refresh_token', refreshData.refresh_token)
                                        sessionStorage.setItem('token', refreshData.id_token)
                                    }
                                })
                            }
                        })
                    }
                }, 60_000)
            } else {
                throw new Error('Error during OIDC login: ' + resp.statusText)
            }
        } catch (e) {
            console.error('OIDC login failed', e)
        }
    } else if (!window.location.href.includes("?error")) {
        const codeVerifier = generateCodeVerifier();
        sessionStorage.setItem('pkce_code_verifier', codeVerifier);
        const codeChallenge = await generateCodeChallenge(codeVerifier)
        /*const scope = config?.scope.map(s => {
            return "scope=" + encodeURIComponent(s)
        }).join("&")*/
        const scope = "scope=" + encodeURIComponent((config?.scope ?? []).join(' '))
        console.log("Scopes are", scope)
        const requestUrl = `${config?.authority + "auth"}?client_id=${config?.clientId}&redirect_uri=${encodeURIComponent(config?.redirectUri ?? '')}&response_type=code&${scope}&code_challenge=${codeChallenge}&code_challenge_method=S256&state=${state}`
        window.location.replace(requestUrl)
    }
}


export {};

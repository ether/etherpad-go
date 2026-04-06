import { useCallback, useEffect, useRef, useState } from 'react'

type OidcConfig = {
  authority: string
  clientId: string
  redirectUri: string
  scope: string
} | null

type AuthState = {
  token: string | null
  loading: boolean
  error: string | null
}

// PKCE helpers
function base64url(buf: ArrayBuffer): string {
  return btoa(String.fromCharCode(...new Uint8Array(buf)))
    .replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
}

async function generateCodeVerifier(): Promise<string> {
  const buf = new Uint8Array(64)
  crypto.getRandomValues(buf)
  return base64url(buf.buffer)
}

async function generateCodeChallenge(verifier: string): Promise<string> {
  const hash = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(verifier))
  return base64url(hash)
}

// Parse JWT expiry without a library
function getTokenExpiryMs(token: string): number | null {
  try {
    const payload = JSON.parse(atob(token.split('.')[1]))
    if (typeof payload.exp === 'number') {
      return payload.exp * 1000
    }
  } catch { /* not a JWT or malformed */ }
  return null
}

export function useAuth() {
  const [state, setState] = useState<AuthState>({ token: null, loading: true, error: null })
  const refreshTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const oidcConfigRef = useRef<NonNullable<OidcConfig> | null>(null)

  const scheduleRenewal = useCallback((token: string, refreshToken: string) => {
    if (refreshTimer.current) clearTimeout(refreshTimer.current)

    const expiryMs = getTokenExpiryMs(token)
    if (!expiryMs) return

    // Refresh 60 seconds before expiry
    const renewIn = Math.max(expiryMs - Date.now() - 60_000, 5_000)

    refreshTimer.current = setTimeout(async () => {
      const oidc = oidcConfigRef.current
      if (!oidc || !refreshToken) return

      try {
        const resp = await fetch('/oauth2/token', {
          method: 'POST',
          headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
          body: new URLSearchParams({
            grant_type: 'refresh_token',
            refresh_token: refreshToken,
            client_id: oidc.clientId,
          }),
        })

        if (!resp.ok) throw new Error(`Refresh failed: ${resp.status}`)
        const data = await resp.json()
        const newToken = (data.id_token || data.access_token) as string
        const newRefresh = (data.refresh_token || refreshToken) as string

        sessionStorage.setItem('admin_token', newToken)
        sessionStorage.setItem('admin_refresh_token', newRefresh)
        setState({ token: newToken, loading: false, error: null })

        // Schedule next renewal
        scheduleRenewal(newToken, newRefresh)
      } catch (e) {
        console.error('Token renewal failed, re-authenticating...', e)
        sessionStorage.removeItem('admin_token')
        sessionStorage.removeItem('admin_refresh_token')
        window.location.reload()
      }
    }, renewIn)
  }, [])

  const startAuth = useCallback(async (oidc: NonNullable<OidcConfig>) => {
    const verifier = await generateCodeVerifier()
    const challenge = await generateCodeChallenge(verifier)
    const nonce = base64url(crypto.getRandomValues(new Uint8Array(16)).buffer)
    const authState = base64url(crypto.getRandomValues(new Uint8Array(16)).buffer)

    sessionStorage.setItem('oidc_verifier', verifier)
    sessionStorage.setItem('oidc_state', authState)

    const params = new URLSearchParams({
      response_type: 'code',
      client_id: oidc.clientId,
      redirect_uri: oidc.redirectUri,
      scope: oidc.scope,
      state: authState,
      nonce,
      code_challenge: challenge,
      code_challenge_method: 'S256',
    })

    window.location.href = `/oauth2/auth?${params}`
  }, [])

  const exchangeCode = useCallback(async (code: string, oidc: NonNullable<OidcConfig>) => {
    const verifier = sessionStorage.getItem('oidc_verifier')
    if (!verifier) throw new Error('Missing code verifier')

    const resp = await fetch('/oauth2/token', {
      method: 'POST',
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
      body: new URLSearchParams({
        grant_type: 'authorization_code',
        code,
        redirect_uri: oidc.redirectUri,
        client_id: oidc.clientId,
        code_verifier: verifier,
      }),
    })

    if (!resp.ok) throw new Error(`Token exchange failed: ${resp.status}`)
    const data = await resp.json()
    sessionStorage.removeItem('oidc_verifier')
    sessionStorage.removeItem('oidc_state')

    return {
      token: (data.id_token || data.access_token) as string,
      refreshToken: (data.refresh_token || '') as string,
    }
  }, [])

  useEffect(() => {
    const run = async () => {
      // Check if we already have a token in session
      const existing = sessionStorage.getItem('admin_token')
      const existingRefresh = sessionStorage.getItem('admin_refresh_token') || ''

      if (existing) {
        const resp = await fetch(`/admin/validate?token=${encodeURIComponent(existing)}`)
        if (resp.ok) {
          setState({ token: existing, loading: false, error: null })
          // Fetch OIDC config for refresh
          const configResp = await fetch('/admin/config')
          const config = await configResp.json()
          if (config.oidc) {
            oidcConfigRef.current = config.oidc
            scheduleRenewal(existing, existingRefresh)
          }
          return
        }

        // Token expired — try refresh before re-auth
        if (existingRefresh) {
          const configResp = await fetch('/admin/config')
          const config = await configResp.json()
          if (config.oidc) {
            oidcConfigRef.current = config.oidc
            try {
              const resp = await fetch('/oauth2/token', {
                method: 'POST',
                headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
                body: new URLSearchParams({
                  grant_type: 'refresh_token',
                  refresh_token: existingRefresh,
                  client_id: config.oidc.clientId,
                }),
              })
              if (resp.ok) {
                const data = await resp.json()
                const newToken = (data.id_token || data.access_token) as string
                const newRefresh = (data.refresh_token || existingRefresh) as string
                sessionStorage.setItem('admin_token', newToken)
                sessionStorage.setItem('admin_refresh_token', newRefresh)
                setState({ token: newToken, loading: false, error: null })
                scheduleRenewal(newToken, newRefresh)
                return
              }
            } catch { /* refresh failed, fall through to re-auth */ }
          }
        }

        sessionStorage.removeItem('admin_token')
        sessionStorage.removeItem('admin_refresh_token')
      }

      // Fetch OIDC config
      const configResp = await fetch('/admin/config')
      const config = await configResp.json()
      const oidc: OidcConfig = config.oidc
      oidcConfigRef.current = oidc

      if (!oidc) {
        setState({ token: '', loading: false, error: null })
        return
      }

      // Check for OAuth callback
      const params = new URLSearchParams(window.location.search)
      const code = params.get('code')
      const returnedState = params.get('state')

      if (code && returnedState === sessionStorage.getItem('oidc_state')) {
        try {
          const { token, refreshToken } = await exchangeCode(code, oidc)
          sessionStorage.setItem('admin_token', token)
          if (refreshToken) sessionStorage.setItem('admin_refresh_token', refreshToken)
          window.history.replaceState({}, '', window.location.pathname)
          setState({ token, loading: false, error: null })
          scheduleRenewal(token, refreshToken)
        } catch (e) {
          setState({ token: null, loading: false, error: `Auth failed: ${e}` })
        }
        return
      }

      await startAuth(oidc)
    }

    run().catch(e => setState({ token: null, loading: false, error: String(e) }))

    return () => {
      if (refreshTimer.current) clearTimeout(refreshTimer.current)
    }
  }, [startAuth, exchangeCode, scheduleRenewal])

  return state
}

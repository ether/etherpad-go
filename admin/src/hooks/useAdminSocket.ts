import { createContext, useCallback, useContext, useEffect, useRef } from 'react'
import React from 'react'
import type { ReactNode } from 'react'
import { useAdminStore } from '@/store'
import { adminBase } from '@/base'

// ── Emit context ───────────────────────────────────────────────────────────
// The emit function is shared via context so any component (or hook like
// useAdminActions) can send messages without holding a direct ref to the
// WebSocket.

type EmitFn = (event: string, data?: any) => void

const EmitContext = createContext<EmitFn | null>(null)

export function useEmit(): EmitFn {
  const emit = useContext(EmitContext)
  if (!emit) {
    throw new Error('useEmit must be used within an AdminSocketProvider')
  }
  return emit
}

// checkAdminToken asks the server whether the token is still an admin
// token. Network trouble is "unknown": the caller keeps retrying.
export async function checkAdminToken(token: string): Promise<'ok' | 'rejected' | 'unknown'> {
  try {
    const resp = await fetch(`${adminBase()}/admin/validate?token=${encodeURIComponent(token)}`)
    if (resp.ok) return 'ok'
    if (resp.status === 401 || resp.status === 403) return 'rejected'
    return 'unknown'
  } catch {
    return 'unknown'
  }
}

// ── Provider ───────────────────────────────────────────────────────────────

export function AdminSocketProvider({ children, token }: { children: ReactNode; token: string }) {
  const ws = useRef<WebSocket | null>(null)
  const store = useAdminStore()

  const handleMessageRef = useRef(store.handleMessage)
  handleMessageRef.current = store.handleMessage

  const setConnectedRef = useRef(store.setConnected)
  setConnectedRef.current = store.setConnected

  const emit = useCallback<EmitFn>((event, data = null) => {
    if (ws.current?.readyState === WebSocket.OPEN) {
      ws.current.send(JSON.stringify({ event, data: data ?? {} }))
    }
  }, [])

  const emitRef = useRef(emit)
  emitRef.current = emit

  // The token is renewed while the page is open; a reconnect must use the
  // current one, not the one the first connection was made with.
  const tokenRef = useRef(token)
  tokenRef.current = token

  const showAuthErrorRef = useRef(store.showAuthError)
  showAuthErrorRef.current = store.showAuthError

  const stopped = useRef(false)

  const connect = useCallback(() => {
    if (stopped.current) return
    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const sentToken = tokenRef.current
    const socket = new WebSocket(`${proto}//${window.location.host}${adminBase()}/admin/ws?token=${encodeURIComponent(sentToken)}`)
    let opened = false

    socket.onopen = () => {
      opened = true
      setConnectedRef.current(true)
      // Initial data load
      emitRef.current('load')
      emitRef.current('checkUpdates')
      emitRef.current('getInstalled')
      emitRef.current('getStats')
    }

    socket.onclose = async () => {
      setConnectedRef.current(false)
      if (stopped.current) return
      // A connection that never opened may have been refused because the
      // token is not (or no longer) an admin token. Retrying with it again
      // would fail silently forever, so tell the user instead.
      if (!opened && sentToken === tokenRef.current) {
        const verdict = await checkAdminToken(sentToken)
        if (verdict === 'rejected') {
          stopped.current = true
          showAuthErrorRef.current(
            'The admin connection was refused: you are not signed in as an administrator, ' +
            'or your sign-in has expired. Sign in again.',
          )
          return
        }
      }
      // Reconnect after 2 seconds
      setTimeout(connect, 2000)
    }

    socket.onmessage = (evt: MessageEvent) => {
      try {
        const raw = JSON.parse(evt.data)
        // Protocol: ["eventName", data]
        if (!Array.isArray(raw) || raw.length < 2) return
        const [event, payload] = raw
        handleMessageRef.current(event, payload)
      } catch (e) {
        console.error('Failed to parse admin message', e)
      }
    }

    ws.current = socket
  }, [])

  useEffect(() => {
    stopped.current = false
    connect()
    return () => {
      stopped.current = true
      ws.current?.close()
    }
  }, [connect])

  return React.createElement(EmitContext.Provider, { value: emit }, children)
}

// ── Convenience hook ───────────────────────────────────────────────────────

export function useAdminSocket() {
  const emit = useEmit()
  const { connected } = useAdminStore()
  return { emit, connected }
}

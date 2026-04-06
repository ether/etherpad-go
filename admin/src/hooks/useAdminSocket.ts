import { createContext, useCallback, useContext, useEffect, useRef } from 'react'
import React from 'react'
import type { ReactNode } from 'react'
import { useAdminStore } from '@/store'

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

  const connect = useCallback(() => {
    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const socket = new WebSocket(`${proto}//${window.location.host}/admin/ws?token=${encodeURIComponent(token)}`)

    socket.onopen = () => {
      setConnectedRef.current(true)
      // Initial data load
      emitRef.current('load')
      emitRef.current('checkUpdates')
      emitRef.current('getInstalled')
      emitRef.current('getStats')
    }

    socket.onclose = () => {
      setConnectedRef.current(false)
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
    connect()
    return () => {
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

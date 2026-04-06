import { createContext, useContext, useReducer, useCallback, useMemo, type Dispatch, type ReactNode } from 'react'
import React from 'react'

// ── Types ──────────────────────────────────────────────────────────────────

export interface UpdateCheckResult {
  version?: string
  latestVersion?: string
  needsUpdate?: boolean
}

export interface PadRecord {
  padName: string
  [key: string]: any
}

export interface ShoutMessage {
  message: string
  sticky: boolean
  timestamp: Date
}

export interface PluginRecord {
  name: string
  [key: string]: any
}

export interface Toast {
  kind: 'success' | 'error' | 'info'
  message: string
}

export interface AdminState {
  currentPage: string
  connected: boolean
  loading: boolean
  toast: Toast | null
  update: UpdateCheckResult | null
  pads: PadRecord[]
  padsTotal: number
  padSearch: string
  padSort: string
  padAscending: boolean
  padOffset: number
  padLimit: number
  totalUsers: number
  shoutMessage: string
  shoutSticky: boolean
  shouts: ShoutMessage[]
  settings: string
  plugins: PluginRecord[]
  lastUpdated: Date | null
}

// ── Actions ────────────────────────────────────────────────────────────────

type Action =
  | { type: 'SET_CONNECTED'; payload: boolean }
  | { type: 'SET_LOADING'; payload: boolean }
  | { type: 'SET_TOAST'; payload: Toast | null }
  | { type: 'SET_CURRENT_PAGE'; payload: string }
  | { type: 'SET_SETTINGS'; payload: string }
  | { type: 'SET_UPDATE'; payload: UpdateCheckResult }
  | { type: 'SET_PADS'; payload: { pads: PadRecord[]; total: number } }
  | { type: 'SET_PLUGINS'; payload: PluginRecord[] }
  | { type: 'SET_TOTAL_USERS'; payload: number }
  | { type: 'ADD_SHOUT'; payload: ShoutMessage }
  | { type: 'SET_SHOUT_MESSAGE'; payload: string }
  | { type: 'SET_SHOUT_STICKY'; payload: boolean }
  | { type: 'SET_PAD_SEARCH'; payload: string }
  | { type: 'SET_PAD_SORT'; payload: string }
  | { type: 'SET_PAD_ASCENDING'; payload: boolean }
  | { type: 'SET_PAD_OFFSET'; payload: number }
  | { type: 'SET_PAD_LIMIT'; payload: number }
  | { type: 'SET_LAST_UPDATED'; payload: Date }

// ── Initial state ──────────────────────────────────────────────────────────

const initialState: AdminState = {
  currentPage: 'overview',
  connected: false,
  loading: false,
  toast: null,
  update: null,
  pads: [],
  padsTotal: 0,
  padSearch: '',
  padSort: 'padName',
  padAscending: true,
  padOffset: 0,
  padLimit: 12,
  totalUsers: 0,
  shoutMessage: '',
  shoutSticky: false,
  shouts: [],
  settings: '',
  plugins: [],
  lastUpdated: null,
}

// ── Reducer ────────────────────────────────────────────────────────────────

function adminReducer(state: AdminState, action: Action): AdminState {
  switch (action.type) {
    case 'SET_CONNECTED':
      return { ...state, connected: action.payload }
    case 'SET_LOADING':
      return { ...state, loading: action.payload }
    case 'SET_TOAST':
      return { ...state, toast: action.payload }
    case 'SET_CURRENT_PAGE':
      return { ...state, currentPage: action.payload }
    case 'SET_SETTINGS':
      return { ...state, settings: action.payload }
    case 'SET_UPDATE':
      return { ...state, update: action.payload }
    case 'SET_PADS':
      return { ...state, pads: action.payload.pads, padsTotal: action.payload.total }
    case 'SET_PLUGINS':
      return { ...state, plugins: action.payload }
    case 'SET_TOTAL_USERS':
      return { ...state, totalUsers: action.payload }
    case 'ADD_SHOUT':
      return {
        ...state,
        shouts: [action.payload, ...state.shouts].slice(0, 20),
        shoutMessage: '',
      }
    case 'SET_SHOUT_MESSAGE':
      return { ...state, shoutMessage: action.payload }
    case 'SET_SHOUT_STICKY':
      return { ...state, shoutSticky: action.payload }
    case 'SET_PAD_SEARCH':
      return { ...state, padSearch: action.payload }
    case 'SET_PAD_SORT':
      return { ...state, padSort: action.payload }
    case 'SET_PAD_ASCENDING':
      return { ...state, padAscending: action.payload }
    case 'SET_PAD_OFFSET':
      return { ...state, padOffset: action.payload }
    case 'SET_PAD_LIMIT':
      return { ...state, padLimit: action.payload }
    case 'SET_LAST_UPDATED':
      return { ...state, lastUpdated: action.payload }
    default:
      return state
  }
}

// ── Context ────────────────────────────────────────────────────────────────

interface AdminStore extends AdminState {
  dispatch: Dispatch<Action>
  setConnected: (connected: boolean) => void
  setLoading: (loading: boolean) => void
  setToast: (toast: Toast | null) => void
  setCurrentPage: (page: string) => void
  setShoutMessage: (msg: string) => void
  setShoutSticky: (sticky: boolean) => void
  setPadSearch: (search: string) => void
  setPadSort: (sort: string) => void
  setPadAscending: (asc: boolean) => void
  setPadOffset: (offset: number) => void
  setPadLimit: (limit: number) => void
  handleMessage: (event: string, payload: any) => void
}

const AdminContext = createContext<AdminStore | null>(null)

// ── Provider ───────────────────────────────────────────────────────────────

export function AdminProvider({ children }: { children: ReactNode }) {
  const [state, dispatch] = useReducer(adminReducer, initialState)

  const setConnected = useCallback(
    (connected: boolean) => dispatch({ type: 'SET_CONNECTED', payload: connected }),
    [],
  )
  const setLoading = useCallback(
    (loading: boolean) => dispatch({ type: 'SET_LOADING', payload: loading }),
    [],
  )
  const setToast = useCallback(
    (toast: Toast | null) => dispatch({ type: 'SET_TOAST', payload: toast }),
    [],
  )
  const setCurrentPage = useCallback(
    (page: string) => dispatch({ type: 'SET_CURRENT_PAGE', payload: page }),
    [],
  )
  const setShoutMessage = useCallback(
    (msg: string) => dispatch({ type: 'SET_SHOUT_MESSAGE', payload: msg }),
    [],
  )
  const setShoutSticky = useCallback(
    (sticky: boolean) => dispatch({ type: 'SET_SHOUT_STICKY', payload: sticky }),
    [],
  )
  const setPadSearch = useCallback(
    (search: string) => dispatch({ type: 'SET_PAD_SEARCH', payload: search }),
    [],
  )
  const setPadSort = useCallback(
    (sort: string) => dispatch({ type: 'SET_PAD_SORT', payload: sort }),
    [],
  )
  const setPadAscending = useCallback(
    (asc: boolean) => dispatch({ type: 'SET_PAD_ASCENDING', payload: asc }),
    [],
  )
  const setPadOffset = useCallback(
    (offset: number) => dispatch({ type: 'SET_PAD_OFFSET', payload: offset }),
    [],
  )
  const setPadLimit = useCallback(
    (limit: number) => dispatch({ type: 'SET_PAD_LIMIT', payload: limit }),
    [],
  )

  const handleMessage = useCallback((event: string, payload: any) => {
    switch (event) {
      case 'settings': {
        let formatted: string
        try {
          formatted = typeof payload === 'string'
            ? JSON.stringify(JSON.parse(payload), null, 2)
            : JSON.stringify(payload, null, 2)
        } catch {
          formatted = String(payload)
        }
        dispatch({ type: 'SET_SETTINGS', payload: formatted })
        dispatch({ type: 'SET_LAST_UPDATED', payload: new Date() })
        break
      }
      case 'results:checkUpdates':
        dispatch({ type: 'SET_UPDATE', payload: payload as UpdateCheckResult })
        dispatch({ type: 'SET_LAST_UPDATED', payload: new Date() })
        break
      case 'results:padLoad':
        dispatch({
          type: 'SET_PADS',
          payload: {
            pads: payload.pads ?? payload.results ?? [],
            total: payload.total ?? payload.count ?? 0,
          },
        })
        dispatch({ type: 'SET_LAST_UPDATED', payload: new Date() })
        break
      case 'results:installed': {
        const plugins = (payload as PluginRecord[]).slice().sort((a, b) =>
          (a.name ?? '').localeCompare(b.name ?? ''),
        )
        dispatch({ type: 'SET_PLUGINS', payload: plugins })
        dispatch({ type: 'SET_LAST_UPDATED', payload: new Date() })
        break
      }
      case 'results:stats':
        dispatch({ type: 'SET_TOTAL_USERS', payload: payload.totalUsers ?? payload ?? 0 })
        dispatch({ type: 'SET_LAST_UPDATED', payload: new Date() })
        break
      case 'result:shout':
        dispatch({
          type: 'ADD_SHOUT',
          payload: {
            message: payload.message ?? '',
            sticky: payload.sticky ?? false,
            timestamp: new Date(),
          },
        })
        dispatch({ type: 'SET_TOAST', payload: { kind: 'success', message: 'Broadcast sent successfully' } })
        break
      case 'results:deletePad':
        dispatch({ type: 'SET_TOAST', payload: { kind: 'success', message: 'Pad deleted successfully' } })
        dispatch({ type: 'SET_LAST_UPDATED', payload: new Date() })
        break
      case 'results:createPad': {
        const success = payload.success !== false
        dispatch({
          type: 'SET_TOAST',
          payload: {
            kind: success ? 'success' : 'error',
            message: success ? 'Pad created successfully' : (payload.message ?? 'Failed to create pad'),
          },
        })
        dispatch({ type: 'SET_LAST_UPDATED', payload: new Date() })
        break
      }
      case 'results:cleanupPadRevisions':
        dispatch({ type: 'SET_TOAST', payload: { kind: 'success', message: 'Pad revisions cleaned up successfully' } })
        dispatch({ type: 'SET_LAST_UPDATED', payload: new Date() })
        break
      default:
        console.warn('Unhandled admin event:', event, payload)
    }
  }, [])

  const value = useMemo<AdminStore>(
    () => ({
      ...state,
      dispatch,
      setConnected,
      setLoading,
      setToast,
      setCurrentPage,
      setShoutMessage,
      setShoutSticky,
      setPadSearch,
      setPadSort,
      setPadAscending,
      setPadOffset,
      setPadLimit,
      handleMessage,
    }),
    [
      state,
      setConnected,
      setLoading,
      setToast,
      setCurrentPage,
      setShoutMessage,
      setShoutSticky,
      setPadSearch,
      setPadSort,
      setPadAscending,
      setPadOffset,
      setPadLimit,
      handleMessage,
    ],
  )

  return React.createElement(AdminContext.Provider, { value }, children)
}

// ── Hook ───────────────────────────────────────────────────────────────────

export function useAdminStore(): AdminStore {
  const ctx = useContext(AdminContext)
  if (!ctx) {
    throw new Error('useAdminStore must be used within an AdminProvider')
  }
  return ctx
}

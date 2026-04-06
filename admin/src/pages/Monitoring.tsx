import { useEffect, useState, useCallback } from 'react'
import { RefreshCw, Search, X } from 'lucide-react'
import { useAdminStore } from '@/store'
import { useAdminActions } from '@/hooks/useAdminActions'

// ── Helpers ─────────────────────────────────────────────────────────────────

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  const value = bytes / Math.pow(1024, i)
  return `${value.toFixed(1)} ${units[i]}`
}

// ── Component ───────────────────────────────────────────────────────────────

export function MonitoringPage() {
  const store = useAdminStore()
  const actions = useAdminActions()
  const [searchQuery, setSearchQuery] = useState('')

  // Auto-refresh system info and connections when connected
  useEffect(() => {
    if (!store.connected) return

    actions.getSystemInfo()
    actions.getConnections()

    const interval = setInterval(() => {
      actions.getSystemInfo()
      actions.getConnections()
    }, 15000)

    return () => clearInterval(interval)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [store.connected])

  const handleKick = useCallback(
    (sessionId: string) => {
      if (window.confirm(`Are you sure you want to kick session ${sessionId}?`)) {
        actions.kickUser(sessionId)
      }
    },
    [actions],
  )

  const handleSearch = useCallback(() => {
    if (searchQuery.trim()) {
      actions.searchPadContent(searchQuery.trim())
    }
  }, [actions, searchQuery])

  const handleSearchKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === 'Enter') handleSearch()
    },
    [handleSearch],
  )

  const clearSearch = useCallback(() => {
    setSearchQuery('')
    store.dispatch({ type: 'SET_SEARCH_RESULTS', payload: [] })
  }, [store])

  const sys = store.systemInfo

  return (
    <div className="mx-auto max-w-7xl p-6 lg:p-8">
      {/* Header */}
      <div className="mb-8">
        <h2 className="text-2xl font-semibold text-black dark:text-white">Monitoring</h2>
        <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
          System resources, active connections, and content search
        </p>
      </div>

      {/* ── System Info Cards ──────────────────────────────────────────────── */}
      <div className="mb-8 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-5">
        <MetricCard
          label="Memory Allocated"
          value={sys ? formatBytes(sys.memAlloc) : '--'}
          description="current heap usage"
        />
        <MetricCard
          label="Goroutines"
          value={sys?.numGoroutine ?? '--'}
          description="active goroutines"
        />
        <MetricCard
          label="GC Cycles"
          value={sys?.numGC ?? '--'}
          description="completed cycles"
        />
        <MetricCard
          label="Go Version"
          value={sys?.goVersion ?? '--'}
          description="runtime version"
        />
        <MetricCard
          label="CPUs"
          value={sys?.numCPU ?? '--'}
          description="available processors"
        />
      </div>

      {/* Extra memory details */}
      {sys && (
        <div className="mb-8 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-2">
          <div className="rounded-lg border border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900 p-5">
            <p className="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">
              Total Allocated
            </p>
            <p className="mt-2 text-xl font-bold text-black dark:text-white">
              {formatBytes(sys.memTotalAlloc)}
            </p>
            <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">cumulative bytes allocated</p>
          </div>
          <div className="rounded-lg border border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900 p-5">
            <p className="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">
              System Memory
            </p>
            <p className="mt-2 text-xl font-bold text-black dark:text-white">
              {formatBytes(sys.memSys)}
            </p>
            <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">obtained from OS</p>
          </div>
        </div>
      )}

      {/* ── Active Connections ─────────────────────────────────────────────── */}
      <div className="mb-8 rounded-lg border border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900">
        <div className="flex items-center justify-between border-b border-gray-200 dark:border-gray-800 px-5 py-3.5">
          <h3 className="text-sm font-semibold text-black dark:text-white">
            Active Connections
            <span className="ml-2 text-xs font-normal text-gray-500 dark:text-gray-400">
              ({store.connections.length})
            </span>
          </h3>
          <button
            type="button"
            onClick={() => actions.getConnections()}
            className="flex items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-xs font-medium text-gray-500 dark:text-gray-400 transition-colors hover:bg-gray-100 dark:hover:bg-gray-800 hover:text-black dark:hover:text-white"
          >
            <RefreshCw className="h-3.5 w-3.5" strokeWidth={1.5} />
            Refresh
          </button>
        </div>

        {store.connections.length === 0 ? (
          <p className="px-5 py-10 text-center text-sm text-gray-400 dark:text-gray-500">
            No active connections.
          </p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left text-sm">
              <thead>
                <tr className="border-b border-gray-200 dark:border-gray-800">
                  <th className="whitespace-nowrap px-5 py-3 text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">
                    Type
                  </th>
                  <th className="whitespace-nowrap px-5 py-3 text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">
                    Session ID
                  </th>
                  <th className="whitespace-nowrap px-5 py-3 text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">
                    Pad
                  </th>
                  <th className="whitespace-nowrap px-5 py-3 text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">
                    IP Address
                  </th>
                  <th className="whitespace-nowrap px-5 py-3 text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">
                    Action
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-100 dark:divide-gray-800">
                {store.connections.map((conn: any) => (
                  <tr
                    key={conn.sessionId || Math.random()}
                    className="transition-colors hover:bg-gray-50 dark:hover:bg-gray-800/50"
                  >
                    <td className="whitespace-nowrap px-5 py-3">
                      <span className={`inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${
                        conn.type === 'pad'
                          ? 'bg-blue-50 text-blue-700 dark:bg-blue-950 dark:text-blue-300'
                          : 'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-400'
                      }`}>
                        {conn.type || 'unknown'}
                      </span>
                    </td>
                    <td className="whitespace-nowrap px-5 py-3 font-mono text-xs text-gray-700 dark:text-gray-300">
                      {conn.sessionId || '—'}
                    </td>
                    <td className="whitespace-nowrap px-5 py-3">
                      {conn.padId ? (
                        <a
                          href={`/p/${conn.padId}`}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="text-sm font-medium text-blue-600 dark:text-blue-400 hover:underline"
                        >
                          {conn.padId}
                        </a>
                      ) : (
                        <span className="text-xs text-gray-400 dark:text-gray-500">—</span>
                      )}
                    </td>
                    <td className="whitespace-nowrap px-5 py-3 font-mono text-xs text-gray-700 dark:text-gray-300">
                      {conn.ip || '—'}
                    </td>
                    <td className="whitespace-nowrap px-5 py-3">
                      {conn.type === 'pad' && conn.sessionId && (
                        <button
                          type="button"
                          onClick={() => handleKick(conn.sessionId)}
                          className="rounded-lg border border-red-200 dark:border-red-800 px-3 py-1.5 text-xs font-medium text-red-600 dark:text-red-400 transition-colors hover:bg-red-50 dark:hover:bg-red-950"
                        >
                          Kick
                        </button>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* ── Fulltext Search ────────────────────────────────────────────────── */}
      <div className="rounded-lg border border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900">
        <div className="border-b border-gray-200 dark:border-gray-800 px-5 py-3.5">
          <h3 className="text-sm font-semibold text-black dark:text-white">
            Fulltext Search
          </h3>
        </div>

        <div className="px-5 py-4">
          <div className="flex gap-2">
            <div className="relative flex-1">
              <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-400 dark:text-gray-500" strokeWidth={1.5} />
              <input
                type="text"
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                onKeyDown={handleSearchKeyDown}
                placeholder="Search pad content..."
                className="w-full rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 py-2 pl-10 pr-9 text-sm text-black dark:text-white placeholder-gray-400 dark:placeholder-gray-500 outline-none transition-colors focus:border-gray-400 dark:focus:border-gray-500"
              />
              {searchQuery && (
                <button
                  type="button"
                  onClick={clearSearch}
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
                >
                  <X className="h-4 w-4" strokeWidth={1.5} />
                </button>
              )}
            </div>
            <button
              type="button"
              onClick={handleSearch}
              disabled={!searchQuery.trim()}
              className="rounded-lg bg-black dark:bg-white px-4 py-2 text-sm font-medium text-white dark:text-black transition-opacity hover:opacity-80 disabled:opacity-40"
            >
              Search
            </button>
          </div>
        </div>

        {store.searchResults.length > 0 && (
          <div className="border-t border-gray-200 dark:border-gray-800 divide-y divide-gray-100 dark:divide-gray-800">
            {store.searchResults.map((result, i) => (
              <div
                key={`${result.padId}-${i}`}
                className="px-5 py-4 transition-colors hover:bg-gray-50 dark:hover:bg-gray-800/50"
              >
                <a
                  href={`/p/${result.padId}`}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-sm font-medium text-blue-600 dark:text-blue-400 hover:underline"
                >
                  {result.padId}
                </a>
                <p
                  className="mt-1.5 text-xs leading-relaxed text-gray-600 dark:text-gray-400"
                  dangerouslySetInnerHTML={{
                    __html: highlightMatch(result.snippet, searchQuery),
                  }}
                />
              </div>
            ))}
          </div>
        )}

        {store.searchResults.length === 0 && searchQuery.trim() !== '' && (
          <div className="border-t border-gray-200 dark:border-gray-800 px-5 py-10 text-center text-sm text-gray-400 dark:text-gray-500">
            No results found.
          </div>
        )}
      </div>
    </div>
  )
}

// ── Sub-components ──────────────────────────────────────────────────────────

interface MetricCardProps {
  label: string
  value: string | number
  description: string
}

function MetricCard({ label, value, description }: MetricCardProps) {
  return (
    <div className="rounded-lg border border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900 p-5">
      <p className="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">
        {label}
      </p>
      <p className="mt-2 text-3xl font-bold text-black dark:text-white">{value}</p>
      <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">{description}</p>
    </div>
  )
}

// ── Helpers ─────────────────────────────────────────────────────────────────

function highlightMatch(snippet: string, query: string): string {
  if (!query.trim()) return escapeHtml(snippet)
  const escaped = query.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  const regex = new RegExp(`(${escaped})`, 'gi')
  return escapeHtml(snippet).replace(
    regex,
    '<mark class="bg-yellow-200 dark:bg-yellow-800 text-black dark:text-white rounded px-0.5">$1</mark>',
  )
}

function escapeHtml(str: string): string {
  return str
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
}

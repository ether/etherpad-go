import { useEffect } from 'react'
import { Link } from 'react-router-dom'
import { RefreshCw } from 'lucide-react'
import { useAdminStore } from '@/store'
import { useAdminActions } from '@/hooks/useAdminActions'

export function OverviewPage() {
  const store = useAdminStore()
  const actions = useAdminActions()

  useEffect(() => {
    actions.refreshAll()
    actions.requestPads({
      offset: store.padOffset,
      limit: store.padLimit,
      pattern: store.padSearch,
      sortBy: store.padSort,
      ascending: store.padAscending,
    })
    // Run once on mount
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const activePluginCount = store.plugins.filter((p) => p.enabled).length
  const versionValue = store.update?.version ?? 'n/a'

  return (
    <div className="mx-auto max-w-7xl p-6 lg:p-8">
      {/* Header */}
      <div className="mb-8">
        <h2 className="text-2xl font-bold text-gray-900 dark:text-white">
          Overview
        </h2>
        <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
          System health at a glance
        </p>
      </div>

      {/* Update banner */}
      {store.update?.needsUpdate && (
        <div className="mb-6 rounded-lg bg-yellow-50 border border-yellow-200 px-4 py-3 text-sm text-yellow-800 dark:bg-yellow-900/30 dark:border-yellow-700 dark:text-yellow-200">
          Update available: {store.update.version} &rarr;{' '}
          {store.update.latestVersion}
        </div>
      )}

      {/* Metric cards */}
      <div className="mb-8 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <MetricCard
          label="Live Users"
          value={store.totalUsers}
          description="currently connected"
          color="blue"
        />
        <MetricCard
          label="Pads Indexed"
          value={store.padsTotal}
          description="current search scope"
          color="emerald"
        />
        <MetricCard
          label="Server Version"
          value={versionValue}
          description="running release"
          color="violet"
        />
        <MetricCard
          label="Active Plugins"
          value={activePluginCount}
          description="enabled integrations"
          color="amber"
        />
      </div>

      {/* Panels */}
      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
        {/* Installed Plugins */}
        <div className="rounded-xl border border-gray-200 bg-white shadow-sm dark:border-gray-800 dark:bg-gray-900">
          <div className="flex items-center justify-between border-b border-gray-100 px-6 py-4 dark:border-gray-800">
            <h3 className="text-base font-semibold text-gray-900 dark:text-white">
              Installed Plugins
            </h3>
            <button
              type="button"
              onClick={() => actions.refreshAll()}
              className="flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs font-medium text-blue-600 hover:bg-blue-50 transition-colors dark:text-blue-400 dark:hover:bg-blue-900/30"
            >
              <RefreshCw className="h-3.5 w-3.5" />
              Refresh
            </button>
          </div>
          <div className="divide-y divide-gray-100 dark:divide-gray-800">
            {store.plugins.length === 0 ? (
              <p className="px-6 py-8 text-center text-sm text-gray-400">
                No plugin data loaded yet.
              </p>
            ) : (
              store.plugins.slice(0, 8).map((plugin) => (
                <div
                  key={plugin.name}
                  className="flex items-center justify-between px-6 py-3"
                >
                  <div className="min-w-0 flex-1">
                    <p className="text-sm font-medium text-gray-900 dark:text-white">
                      {plugin.name}
                    </p>
                    <p className="truncate text-xs text-gray-500 dark:text-gray-400">
                      {plugin.description}
                    </p>
                  </div>
                  <div className="ml-4 flex items-center gap-2">
                    <span className="rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-600 dark:bg-gray-800 dark:text-gray-300">
                      {plugin.version}
                    </span>
                    <span
                      className={`rounded-full px-2 py-0.5 text-xs font-medium ${
                        plugin.enabled
                          ? 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-400'
                          : 'bg-gray-100 text-gray-500 dark:bg-gray-800 dark:text-gray-500'
                      }`}
                    >
                      {plugin.enabled ? 'enabled' : 'disabled'}
                    </span>
                  </div>
                </div>
              ))
            )}
          </div>
        </div>

        {/* Recent Broadcasts */}
        <div className="rounded-xl border border-gray-200 bg-white shadow-sm dark:border-gray-800 dark:bg-gray-900">
          <div className="flex items-center justify-between border-b border-gray-100 px-6 py-4 dark:border-gray-800">
            <h3 className="text-base font-semibold text-gray-900 dark:text-white">
              Recent Broadcasts
            </h3>
            <Link
              to="/broadcast"
              className="text-xs font-medium text-blue-600 hover:text-blue-700 dark:text-blue-400"
            >
              View all
            </Link>
          </div>
          <div className="divide-y divide-gray-100 dark:divide-gray-800">
            {store.shouts.length === 0 ? (
              <p className="px-6 py-8 text-center text-sm text-gray-400">
                No broadcast messages yet.
              </p>
            ) : (
              store.shouts.slice(0, 5).map((shout, i) => (
                <div key={i} className="flex items-start justify-between px-6 py-3">
                  <div className="min-w-0 flex-1">
                    <p className="text-sm text-gray-900 dark:text-white">
                      {shout.message}
                    </p>
                    <span className="text-xs text-gray-400">
                      {formatShoutDate(shout.timestamp)}
                    </span>
                  </div>
                  {shout.sticky && (
                    <span className="ml-2 shrink-0 rounded-full bg-blue-100 px-2 py-0.5 text-xs font-medium text-blue-700 dark:bg-blue-900/40 dark:text-blue-400">
                      sticky
                    </span>
                  )}
                </div>
              ))
            )}
          </div>
        </div>
      </div>
    </div>
  )
}

// ── Helpers ──────────────────────────────────────────────────────────────────

interface MetricCardProps {
  label: string
  value: string | number
  description: string
  color: 'blue' | 'emerald' | 'violet' | 'amber'
}

const colorMap: Record<MetricCardProps['color'], string> = {
  blue: 'bg-blue-50 dark:bg-blue-900/20',
  emerald: 'bg-emerald-50 dark:bg-emerald-900/20',
  violet: 'bg-violet-50 dark:bg-violet-900/20',
  amber: 'bg-amber-50 dark:bg-amber-900/20',
}

function MetricCard({ label, value, description, color }: MetricCardProps) {
  return (
    <div
      className={`rounded-xl border border-gray-200 p-5 shadow-sm dark:border-gray-800 ${colorMap[color]}`}
    >
      <p className="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">
        {label}
      </p>
      <p className="mt-2 text-3xl font-bold text-gray-900 dark:text-white">
        {value}
      </p>
      <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">
        {description}
      </p>
    </div>
  )
}

function formatShoutDate(d: Date): string {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${pad(d.getDate())}.${pad(d.getMonth() + 1)}.${d.getFullYear()} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

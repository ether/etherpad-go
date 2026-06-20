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
  const versionValue = store.update?.currentVersion ?? store.updateStatus?.currentVersion ?? 'n/a'

  const execStatus = store.updateStatus?.execution?.status
  const updateBusy =
    execStatus === 'scheduled' ||
    execStatus === 'preflight' ||
    execStatus === 'draining' ||
    execStatus === 'executing' ||
    execStatus === 'pending-verification' ||
    execStatus === 'rolling-back'
  const rollbackFailed = execStatus === 'rollback-failed'
  const canApply = store.updateStatus?.policy?.canManual !== false

  const handleApply = () => {
    actions.applyUpdate()
    // Reflect the new execution state shortly after triggering.
    setTimeout(() => actions.getUpdateStatus(), 1500)
  }

  return (
    <div className="mx-auto max-w-7xl p-6 lg:p-8">
      {/* Header */}
      <div className="mb-8">
        <h2 className="text-2xl font-semibold text-black dark:text-white">Overview</h2>
        <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
          System health at a glance
        </p>
      </div>

      {/* Update banner */}
      {store.update?.updateAvailable && (
        <div className="mb-6 flex flex-wrap items-center justify-between gap-3 rounded-lg border border-amber-300 dark:border-amber-600 bg-amber-50 dark:bg-amber-950 px-4 py-3 text-sm text-amber-800 dark:text-amber-200">
          <span>
            Update available: {store.update.currentVersion} &rarr;{' '}
            {store.update.latestVersion}
          </span>
          {canApply && (
            <button
              type="button"
              onClick={handleApply}
              disabled={updateBusy}
              className="rounded-lg border border-amber-400 dark:border-amber-500 px-3 py-1.5 text-xs font-medium text-amber-900 dark:text-amber-100 transition-colors hover:bg-amber-100 dark:hover:bg-amber-900 disabled:cursor-not-allowed disabled:opacity-50"
            >
              {updateBusy ? 'Updating…' : 'Apply update'}
            </button>
          )}
        </div>
      )}

      {/* Update execution status */}
      {execStatus && execStatus !== 'idle' && execStatus !== 'verified' && (
        <div className="mb-6 flex flex-wrap items-center justify-between gap-3 rounded-lg border border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900 px-4 py-3 text-sm text-gray-700 dark:text-gray-300">
          <span>
            Update status: <span className="font-medium">{execStatus}</span>
            {store.updateStatus?.execution?.reason ? ` — ${store.updateStatus.execution.reason}` : ''}
          </span>
          {rollbackFailed && (
            <button
              type="button"
              onClick={() => {
                actions.acknowledgeUpdate()
                setTimeout(() => actions.getUpdateStatus(), 1000)
              }}
              className="rounded-lg border border-red-300 dark:border-red-700 px-3 py-1.5 text-xs font-medium text-red-700 dark:text-red-300 transition-colors hover:bg-red-50 dark:hover:bg-red-950"
            >
              Acknowledge
            </button>
          )}
        </div>
      )}

      {/* Metric cards */}
      <div className="mb-8 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <MetricCard label="Live Users" value={store.totalUsers} description="currently connected" />
        <MetricCard label="Pads Indexed" value={store.padsTotal} description="current search scope" />
        <MetricCard label="Server Version" value={versionValue} description="running release" />
        <MetricCard label="Active Plugins" value={activePluginCount} description="enabled integrations" />
      </div>

      {/* Panels */}
      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
        {/* Installed Plugins */}
        <div className="rounded-lg border border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900">
          <div className="flex items-center justify-between border-b border-gray-200 dark:border-gray-800 px-5 py-3.5">
            <h3 className="text-sm font-semibold text-black dark:text-white">
              Installed Plugins
            </h3>
            <button
              type="button"
              onClick={() => actions.refreshAll()}
              className="flex items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-xs font-medium text-gray-500 dark:text-gray-400 transition-colors hover:bg-gray-100 dark:hover:bg-gray-800 hover:text-black dark:hover:text-white"
            >
              <RefreshCw className="h-3.5 w-3.5" strokeWidth={1.5} />
              Refresh
            </button>
          </div>
          <div className="divide-y divide-gray-100 dark:divide-gray-800">
            {store.plugins.length === 0 ? (
              <p className="px-5 py-10 text-center text-sm text-gray-400 dark:text-gray-500">
                No plugin data loaded yet.
              </p>
            ) : (
              store.plugins.slice(0, 8).map((plugin) => (
                <div
                  key={plugin.name}
                  className="flex items-center justify-between px-5 py-3"
                >
                  <div className="min-w-0 flex-1">
                    <p className="text-sm font-medium text-black dark:text-white">
                      {plugin.name}
                    </p>
                    <p className="truncate text-xs text-gray-500 dark:text-gray-400">
                      {plugin.description}
                    </p>
                  </div>
                  <div className="ml-4 flex items-center gap-2">
                    <span className="rounded-full border border-gray-200 dark:border-gray-700 px-2 py-0.5 text-xs text-gray-500 dark:text-gray-400">
                      {plugin.version}
                    </span>
                    <span
                      className={`rounded-full px-2 py-0.5 text-xs font-medium ${
                        plugin.enabled
                          ? 'border border-green-200 dark:border-green-800 text-green-700 dark:text-green-400'
                          : 'border border-gray-200 dark:border-gray-700 text-gray-400 dark:text-gray-500'
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
        <div className="rounded-lg border border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900">
          <div className="flex items-center justify-between border-b border-gray-200 dark:border-gray-800 px-5 py-3.5">
            <h3 className="text-sm font-semibold text-black dark:text-white">
              Recent Broadcasts
            </h3>
            <Link
              to="/broadcast"
              className="text-xs font-medium text-gray-500 dark:text-gray-400 transition-colors hover:text-black dark:hover:text-white"
            >
              View all
            </Link>
          </div>
          <div className="divide-y divide-gray-100 dark:divide-gray-800">
            {store.shouts.length === 0 ? (
              <p className="px-5 py-10 text-center text-sm text-gray-400 dark:text-gray-500">
                No broadcast messages yet.
              </p>
            ) : (
              store.shouts.slice(0, 5).map((shout, i) => (
                <div key={i} className="flex items-start justify-between px-5 py-3">
                  <div className="min-w-0 flex-1">
                    <p className="text-sm text-black dark:text-white">{shout.message}</p>
                    <span className="text-xs text-gray-400 dark:text-gray-500">
                      {formatShoutDate(shout.timestamp)}
                    </span>
                  </div>
                  {shout.sticky && (
                    <span className="ml-2 shrink-0 rounded-full border border-gray-200 dark:border-gray-700 px-2 py-0.5 text-xs font-medium text-gray-500 dark:text-gray-400">
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

// -- Helpers ------------------------------------------------------------------

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

function formatShoutDate(d: Date): string {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${pad(d.getDate())}.${pad(d.getMonth() + 1)}.${d.getFullYear()} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

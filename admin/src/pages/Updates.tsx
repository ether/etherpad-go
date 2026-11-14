import { useEffect } from 'react'
import { RefreshCw } from 'lucide-react'
import { useAdminStore } from '@/store'
import { useAdminActions } from '@/hooks/useAdminActions'

const statusText: Record<string, string> = {
  idle: 'Idle',
  scheduled: 'Scheduled',
  preflight: 'Checking preconditions',
  'preflight-failed': 'Preconditions failed',
  draining: 'Warning users before the restart',
  executing: 'Replacing the binary',
  'pending-verification': 'Waiting for a healthy start',
  verified: 'Updated',
  'rolling-back': 'Rolling back',
  'rolled-back': 'Rolled back',
  'rollback-failed': 'Rollback failed – manual intervention required',
}

// UpdatesPage shows the self-updater: what is running, what is available,
// what the updater is doing, and its log.
export function UpdatesPage() {
  const store = useAdminStore()
  const actions = useAdminActions()
  const status = store.updateStatus

  const refresh = () => {
    actions.getUpdateStatus()
    actions.getUpdateLog()
  }

  useEffect(() => {
    if (!store.connected) return
    refresh()
    // The state moves on its own (schedule, drain): keep it current.
    const t = setInterval(refresh, 5000)
    return () => clearInterval(t)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [store.connected])

  const later = (fn: () => void) => {
    fn()
    setTimeout(refresh, 1000)
  }

  const exec = status?.execution
  const lines = store.updateLog ?? []

  return (
    <div className="mx-auto max-w-7xl p-6 lg:p-8">
      <div className="mb-8 flex items-start justify-between gap-4">
        <div>
          <h2 className="text-2xl font-semibold text-black dark:text-white">Updates</h2>
          <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">The built-in updater and its log</p>
        </div>
        <button
          type="button"
          onClick={() => later(actions.checkUpdates)}
          className="flex items-center gap-1.5 rounded-lg border border-gray-200 dark:border-gray-700 px-3 py-1.5 text-xs font-medium text-gray-600 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-800"
        >
          <RefreshCw className="h-3.5 w-3.5" strokeWidth={1.5} />
          Check now
        </button>
      </div>

      <div className="mb-6 rounded-lg border border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900 p-5">
        <dl className="grid grid-cols-1 gap-4 text-sm sm:grid-cols-2 lg:grid-cols-4">
          <Item label="Running" value={status?.currentVersion ?? 'n/a'} />
          <Item label="Latest release" value={status?.latest?.version ?? 'unknown'} href={status?.latest?.htmlUrl} />
          <Item label="Mode" value={status ? `${status.tier} (${status.installMethod})` : 'n/a'} />
          <Item label="Status" value={exec ? statusText[exec.status] ?? exec.status : 'n/a'} testId="update-status" />
        </dl>
        {exec?.reason && <p className="mt-3 text-sm text-gray-600 dark:text-gray-400">Reason: {exec.reason}</p>}
        {status?.policy?.reason && status.policy.reason !== 'ok' && (
          <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">Policy: {status.policy.reason}</p>
        )}
        {status?.lastResult && (
          <p className="mt-1 text-xs text-gray-500 dark:text-gray-400" data-testid="update-last-result">
            Last result: {status.lastResult.outcome} ({status.lastResult.targetTag})
            {status.lastResult.at ? `, ${new Date(status.lastResult.at).toLocaleString()}` : ''}
          </p>
        )}

        <div className="mt-4 flex flex-wrap gap-2">
          {status?.updateAvailable && status.policy?.canManual && !status.canCancel && (
            <Button onClick={() => later(actions.applyUpdate)} label="Apply now" />
          )}
          {status?.canCancel && <Button onClick={() => later(actions.cancelUpdate)} label="Cancel update" testId="update-cancel" />}
          {status?.canAcknowledge && (
            <Button onClick={() => later(actions.acknowledgeUpdate)} label="Acknowledge" danger testId="update-acknowledge" />
          )}
        </div>
      </div>

      <div className="mb-6 flex flex-wrap items-center justify-between gap-3 rounded-lg border border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900 px-5 py-4 text-sm">
        <div>
          <p className="font-medium text-black dark:text-white">E-mail notices</p>
          <p className="mt-0.5 text-gray-600 dark:text-gray-400" data-testid="update-admin-email">
            {status?.adminEmail
              ? `Sent to ${status.adminEmail}: an outdated instance, a scheduled update, a failed one.`
              : 'None: set adminEmail and mail (SMTP) in the settings.'}
          </p>
        </div>
        {status?.adminEmail && <Button onClick={actions.sendTestMail} label="Send test mail" testId="update-test-mail" />}
      </div>

      <div className="rounded-lg border border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900">
        <div className="border-b border-gray-200 dark:border-gray-800 px-5 py-3.5">
          <h3 className="text-sm font-semibold text-black dark:text-white">Update log</h3>
        </div>
        {lines.length === 0 ? (
          <p className="px-5 py-10 text-center text-sm text-gray-400 dark:text-gray-500">No entries yet.</p>
        ) : (
          <pre
            data-testid="update-log"
            className="max-h-96 overflow-auto whitespace-pre-wrap px-5 py-4 font-mono text-xs text-gray-700 dark:text-gray-300"
          >
            {lines.join('\n')}
          </pre>
        )}
      </div>
    </div>
  )
}

function Item({ label, value, href, testId }: { label: string; value: string; href?: string; testId?: string }) {
  return (
    <div>
      <dt className="text-xs text-gray-500 dark:text-gray-400">{label}</dt>
      <dd className="mt-0.5 font-medium text-black dark:text-white" data-testid={testId}>
        {href ? (
          <a href={href} target="_blank" rel="noreferrer" className="underline">
            {value}
          </a>
        ) : (
          value
        )}
      </dd>
    </div>
  )
}

function Button({ onClick, label, danger, testId }: { onClick: () => void; label: string; danger?: boolean; testId?: string }) {
  return (
    <button
      type="button"
      onClick={onClick}
      data-testid={testId}
      className={`rounded-lg border px-3 py-1.5 text-xs font-medium transition-colors ${
        danger
          ? 'border-red-300 dark:border-red-700 text-red-700 dark:text-red-300 hover:bg-red-50 dark:hover:bg-red-950'
          : 'border-gray-300 dark:border-gray-600 text-gray-800 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-800'
      }`}
    >
      {label}
    </button>
  )
}

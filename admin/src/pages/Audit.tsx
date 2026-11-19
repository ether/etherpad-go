import { useEffect, useMemo, useState } from 'react'
import { RefreshCw } from 'lucide-react'
import { useAdminStore } from '@/store'
import { useAdminActions } from '@/hooks/useAdminActions'
import { ErrorBanner, PageHeader, formatTime, inputClass, smallButton } from '@/components/AccountParts'

// AuditPage lists every change to accounts, roles and assignments, newest
// first.
export function AuditPage() {
  const store = useAdminStore()
  const actions = useAdminActions()
  const [filter, setFilter] = useState('')

  useEffect(() => {
    if (store.connected) actions.loadAudit(500)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [store.connected])

  const result = store.audit
  const names = result?.names ?? {}
  const who = (id: string) => names[id] ?? id
  const entries = useMemo(() => {
    const q = filter.trim().toLowerCase()
    return (result?.entries ?? []).filter((e) => {
      if (!q) return true
      const text = `${e.action} ${who(e.actor)} ${who(e.target)} ${JSON.stringify(e.details ?? {})}`.toLowerCase()
      return text.includes(q)
    })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [result, filter])

  return (
    <div className="mx-auto max-w-6xl p-6 lg:p-8">
      <PageHeader title="Audit log">Every change to accounts, roles and assignments, and who made it.</PageHeader>
      <ErrorBanner error={result?.error} testId="audit-error" />
      <div className="mb-3 flex items-center gap-2">
        <input aria-label="Filter audit log" placeholder="Filter…" value={filter} onChange={(e) => setFilter(e.target.value)}
               className={`${inputClass} w-64 py-1.5`} />
        <button type="button" onClick={() => actions.loadAudit(500)} className={`${smallButton} inline-flex items-center gap-1`}>
          <RefreshCw className="h-3 w-3" /> Refresh
        </button>
      </div>
      <div className="overflow-x-auto rounded-lg border border-gray-200 dark:border-gray-800">
        <table className="w-full text-sm" data-testid="audit-table">
          <thead className="bg-gray-50 dark:bg-gray-900 text-left text-xs font-medium uppercase tracking-wide text-gray-500">
            <tr>
              <th className="px-4 py-3">When</th>
              <th className="px-4 py-3">Who</th>
              <th className="px-4 py-3">What</th>
              <th className="px-4 py-3">On</th>
              <th className="px-4 py-3">Details</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100 dark:divide-gray-800">
            {entries.map((e) => (
              <tr key={e.id} data-testid="audit-row">
                <td className="whitespace-nowrap px-4 py-2 text-gray-500">{formatTime(e.at)}</td>
                <td className="px-4 py-2 text-black dark:text-white">{who(e.actor)}</td>
                <td className="px-4 py-2"><code className="text-xs">{e.action}</code></td>
                <td className="px-4 py-2 text-gray-700 dark:text-gray-300">{who(e.target)}</td>
                <td className="px-4 py-2 font-mono text-xs text-gray-500">
                  {e.details && Object.entries(e.details).map(([k, v]) => (
                    <span key={k} className="mr-2">{k}={typeof v === 'string' ? v : JSON.stringify(v)}</span>
                  ))}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {entries.length === 0 && <p className="py-8 text-center text-sm text-gray-400">No entries.</p>}
      </div>
    </div>
  )
}

import { useEffect, useState } from 'react'
import { Globe, Lock, Eye } from 'lucide-react'
import { useAdminStore } from '@/store'
import { useAdminActions } from '@/hooks/useAdminActions'
import { ErrorBanner, PageHeader, RoleChips, smallButton } from '@/components/AccountParts'

const templates = [
  { id: 'open', icon: Globe, title: 'Open', text: 'Visitors read, write and create pads (like etherpad-lite without login).' },
  { id: 'read', icon: Eye, title: 'Read only', text: 'Visitors read pads; writing needs a login.' },
  { id: 'closed', icon: Lock, title: 'Closed', text: 'Visitors see nothing; everyone logged in may edit.' },
] as const

// VisitorsPage sets what people may do who are not logged in, and what
// everyone logged in may do on top of their own roles.
export function VisitorsPage() {
  const store = useAdminStore()
  const actions = useAdminActions()
  const [confirm, setConfirm] = useState<string | null>(null)

  useEffect(() => {
    if (store.connected) actions.loadAccounts()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [store.connected])

  const result = store.accounts
  const roles = result?.roles ?? []
  const anonymous = result?.accounts.find((a) => a.id === 'anonymous')
  const authenticated = result?.accounts.find((a) => a.id === 'authenticated')

  return (
    <div className="mx-auto max-w-5xl p-6 lg:p-8">
      <PageHeader title="Visitors">
        What people may do without logging in, and what everyone logged in may do on top of their own roles.
        Admin scopes are never given this way.
      </PageHeader>
      <ErrorBanner error={result?.error} testId="visitors-error" />

      <h3 className="mb-3 text-sm font-medium text-black dark:text-white">Start from a template</h3>
      <div className="mb-2 grid gap-3 md:grid-cols-3">
        {templates.map(({ id, icon: Icon, title, text }) => (
          <div key={id} data-testid={`template-${id}`} className="rounded-lg border border-gray-200 dark:border-gray-800 p-4">
            <div className="mb-1 flex items-center gap-2 font-medium text-black dark:text-white">
              <Icon className="h-4 w-4" strokeWidth={1.5} /> {title}
            </div>
            <p className="mb-3 text-sm text-gray-500">{text}</p>
            {confirm === id ? (
              <span className="flex items-center gap-2">
                <button type="button" onClick={() => { actions.applyTemplate(id); setConfirm(null) }}
                        className="rounded bg-black px-2 py-1 text-xs text-white dark:bg-white dark:text-black">Replace the roles below</button>
                <button type="button" onClick={() => setConfirm(null)} className="text-xs text-gray-500">Cancel</button>
              </span>
            ) : (
              <button type="button" onClick={() => setConfirm(id)} className={smallButton}>Apply</button>
            )}
          </div>
        ))}
      </div>
      <p className="mb-8 text-xs text-gray-400">A template replaces the roles of visitors and of everyone logged in; the roles of single accounts stay.</p>

      {[{ account: anonymous, title: 'Visitors (not logged in)' }, { account: authenticated, title: 'Everyone logged in' }].map(({ account, title }) =>
        account && (
          <div key={account.id} data-testid={`pseudo-${account.id}`} className="mb-4 rounded-lg border border-gray-200 dark:border-gray-800 p-4">
            <h3 className="mb-2 text-sm font-medium text-black dark:text-white">{title}</h3>
            <RoleChips account={account} roles={roles.filter((r) => !r.scopes.some((s) => s.startsWith('admin:')) && r.id !== 'admin')} />
          </div>
        ),
      )}
    </div>
  )
}

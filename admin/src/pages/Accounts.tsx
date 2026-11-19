import { useEffect, useMemo, useState } from 'react'
import { Bot, Pencil, Plus, ShieldCheck, Trash2, UserRound } from 'lucide-react'
import { useAdminStore, type Account } from '@/store'
import { useAdminActions } from '@/hooks/useAdminActions'
import {
  ErrorBanner, OneTimePassword, PageHeader, RoleChips, formatTime, inputClass, primaryButton, smallButton,
} from '@/components/AccountParts'

function EditProfile({ account, onSave, onDone }: { account: Account; onSave: () => void; onDone: () => void }) {
  const actions = useAdminActions()
  const [username, setUsername] = useState(account.username)
  const [displayName, setDisplayName] = useState(account.displayName)
  const [email, setEmail] = useState(account.email)
  const save = () => {
    onSave()
    actions.updateAccount({ id: account.id, username: username.trim(), displayName, email })
  }
  return (
    <div className="mt-2 flex flex-col gap-2 sm:flex-row" data-testid="account-edit">
      <input aria-label="Username" value={username} onChange={(e) => setUsername(e.target.value)} className={`${inputClass} py-1`} />
      <input aria-label="Display name" value={displayName} onChange={(e) => setDisplayName(e.target.value)} className={`${inputClass} py-1`} />
      <input aria-label="E-mail" type="email" value={email} onChange={(e) => setEmail(e.target.value)} className={`${inputClass} py-1`} />
      <button type="button" onClick={save} disabled={!username.trim()} className={smallButton}>Save</button>
      <button type="button" onClick={onDone} className="text-xs text-gray-500">Cancel</button>
    </div>
  )
}

// AccountsPage manages the people who log in and the service accounts
// that only own tokens. A new person gets a one-time password that must
// be changed at the first login.
export function AccountsPage() {
  const store = useAdminStore()
  const actions = useAdminActions()
  const [username, setUsername] = useState('')
  const [displayName, setDisplayName] = useState('')
  const [kind, setKind] = useState<'human' | 'service'>('human')
  const [role, setRole] = useState('editor')
  const [pads, setPads] = useState('')
  const [filter, setFilter] = useState('')
  const [editing, setEditing] = useState<string | null>(null)
  const [confirmDelete, setConfirmDelete] = useState<string | null>(null)
  // pending is the request whose answer decides whether a form closes:
  // on an error it stays as typed.
  const [pending, setPending] = useState<'create' | 'edit' | null>(null)

  useEffect(() => {
    if (store.connected) actions.loadAccounts()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [store.connected])

  const result = store.accounts
  useEffect(() => {
    if (!pending || !result) return
    setPending(null)
    if (result.error) return
    if (pending === 'create') {
      setUsername('')
      setDisplayName('')
      setPads('')
    } else {
      setEditing(null)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [result])

  // A one-time password is shown once: gone when the page is left.
  const shown = store.accountPassword
  const clearPassword = () => store.dispatch({ type: 'CLEAR_ACCOUNT_PASSWORD' })
  useEffect(() => clearPassword, [])  // eslint-disable-line react-hooks/exhaustive-deps

  const roles = result?.roles ?? []
  const people = useMemo(() => {
    const q = filter.trim().toLowerCase()
    return (result?.accounts ?? [])
      .filter((a) => a.kind !== 'builtin')
      .filter((a) => !q || a.username.toLowerCase().includes(q) || a.displayName.toLowerCase().includes(q) || a.email.toLowerCase().includes(q))
      .sort((a, b) => a.username.localeCompare(b.username))
  }, [result, filter])

  const create = () => {
    const u = username.trim()
    if (!u) return
    setPending('create')
    actions.createAccount({ username: u, displayName: displayName.trim(), kind, role: role || undefined, pads: pads.trim() })
  }

  return (
    <div className="mx-auto max-w-6xl p-6 lg:p-8">
      <PageHeader title="Accounts">
        People who log in, and service accounts that only own tokens. Give a role for all pads, or for a pattern such as <code>team/*</code>.
      </PageHeader>

      <ErrorBanner error={result?.error} testId="accounts-error" />
      {shown && <OneTimePassword shown={shown} onClose={clearPassword} />}

      <div className="mb-8 rounded-lg border border-gray-200 dark:border-gray-800 p-4">
        <h3 className="mb-3 text-sm font-medium text-black dark:text-white">New account</h3>
        <div className="flex flex-col gap-2 lg:flex-row">
          <select aria-label="Kind" value={kind} onChange={(e) => setKind(e.target.value as 'human' | 'service')} className={inputClass}>
            <option value="human">Person</option>
            <option value="service">Service (no login)</option>
          </select>
          <input aria-label="New username" placeholder="Username" value={username} onChange={(e) => setUsername(e.target.value)}
                 onKeyDown={(e) => e.key === 'Enter' && create()} className={`${inputClass} flex-1`} />
          <input aria-label="New display name" placeholder="Display name (optional)" value={displayName}
                 onChange={(e) => setDisplayName(e.target.value)} className={`${inputClass} flex-1`} />
          <select aria-label="New role" value={role} onChange={(e) => setRole(e.target.value)} className={inputClass}>
            <option value="">No role</option>
            {roles.map((r) => <option key={r.id} value={r.id}>{r.name}</option>)}
          </select>
          <input aria-label="New pads" placeholder="all pads, or team/*" value={pads} onChange={(e) => setPads(e.target.value)}
                 className={`${inputClass} lg:w-40`} />
          <button type="button" onClick={create} disabled={!username.trim()} className={primaryButton}>
            <Plus className="h-4 w-4" strokeWidth={1.5} />
            Create
          </button>
        </div>
      </div>

      <div className="mb-3 flex items-center justify-between gap-2">
        <input aria-label="Filter accounts" placeholder="Filter…" value={filter} onChange={(e) => setFilter(e.target.value)}
               className={`${inputClass} w-64 py-1.5`} />
        <span className="text-xs text-gray-400">{people.length} accounts</span>
      </div>

      <div className="overflow-x-auto rounded-lg border border-gray-200 dark:border-gray-800">
        <table className="w-full text-sm" data-testid="accounts-table">
          <thead className="bg-gray-50 dark:bg-gray-900 text-left text-xs font-medium uppercase tracking-wide text-gray-500">
            <tr>
              <th className="px-4 py-3">Account</th>
              <th className="px-4 py-3">Roles</th>
              <th className="px-4 py-3">Last login</th>
              <th className="px-4 py-3">Status</th>
              <th className="px-4 py-3" />
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100 dark:divide-gray-800">
            {people.map((a) => (
              <tr key={a.id} data-testid="account-row" data-username={a.username} className={a.disabled ? 'opacity-60' : ''}>
                <td className="px-4 py-3 align-top">
                  <div className="flex items-center gap-2">
                    {a.kind === 'service' ? <Bot className="h-4 w-4 text-gray-400" /> : <UserRound className="h-4 w-4 text-gray-400" />}
                    <span className="font-medium text-black dark:text-white">{a.username}</span>
                    {a.admin && <span title="Admin"><ShieldCheck className="h-4 w-4 text-blue-600" /></span>}
                    {a.id === result?.me && <span className="text-xs text-gray-400">(you)</span>}
                    <button type="button" aria-label={`Edit ${a.username}`} onClick={() => setEditing(editing === a.id ? null : a.id)}
                            className="text-gray-400 hover:text-black dark:hover:text-white">
                      <Pencil className="h-3.5 w-3.5" />
                    </button>
                  </div>
                  {(a.displayName !== a.username || a.email) && (
                    <div className="mt-0.5 text-xs text-gray-500">
                      {a.displayName !== a.username && a.displayName}{a.email && ` · ${a.email}`}
                    </div>
                  )}
                  {editing === a.id && <EditProfile account={a} onSave={() => setPending('edit')} onDone={() => setEditing(null)} />}
                </td>
                <td className="px-4 py-3 align-top"><RoleChips account={a} roles={roles} /></td>
                <td className="whitespace-nowrap px-4 py-3 align-top text-gray-500">
                  {a.kind === 'service' ? '—' : formatTime(a.lastLoginAt)}
                </td>
                <td className="px-4 py-3 align-top">
                  <div className="flex flex-col items-start gap-1">
                    <button type="button" onClick={() => actions.setAccountDisabled(a.id, !a.disabled)} className={smallButton}>
                      {a.disabled ? 'Disabled – enable' : 'Active – disable'}
                    </button>
                    {a.mustChangePassword && <span className="text-xs text-amber-600">must change password</span>}
                  </div>
                </td>
                <td className="whitespace-nowrap px-4 py-3 text-right align-top">
                  {a.kind === 'human' && (
                    <button type="button" onClick={() => actions.resetPassword(a.id)} className={`${smallButton} mr-2`}>
                      Reset password
                    </button>
                  )}
                  {confirmDelete === a.id ? (
                    <span className="inline-flex items-center gap-2">
                      <button type="button" onClick={() => { actions.deleteAccount(a.id); setConfirmDelete(null) }}
                              className="rounded bg-red-600 px-2 py-1 text-xs text-white">Delete for good</button>
                      <button type="button" onClick={() => setConfirmDelete(null)} className="rounded px-2 py-1 text-xs text-gray-500">Cancel</button>
                    </span>
                  ) : (
                    <button type="button" aria-label={`Delete ${a.username}`} onClick={() => setConfirmDelete(a.id)}
                            className="rounded p-1 text-gray-400 hover:text-red-600">
                      <Trash2 className="h-4 w-4" strokeWidth={1.5} />
                    </button>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}

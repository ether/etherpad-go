import { useState } from 'react'
import { Check, Copy, Plus, X } from 'lucide-react'
import type { Account, Assignment, Role } from '@/store'
import { useAdminActions } from '@/hooks/useAdminActions'

export const inputClass =
  'rounded-lg border border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900 px-3 py-2 text-sm text-black dark:text-white placeholder-gray-400 focus:border-black dark:focus:border-white focus:outline-none'
export const primaryButton =
  'flex items-center justify-center gap-2 rounded-lg bg-black px-4 py-2 text-sm font-medium text-white hover:bg-gray-800 disabled:cursor-not-allowed disabled:opacity-40 dark:bg-white dark:text-black dark:hover:bg-gray-200'
export const smallButton =
  'rounded-full border border-gray-200 dark:border-gray-700 px-3 py-0.5 text-xs font-medium text-gray-600 dark:text-gray-300 hover:border-black dark:hover:border-white disabled:opacity-40'

export function formatTime(ms: number | null | undefined): string {
  if (ms == null) return 'never'
  return new Date(ms).toLocaleString()
}

export function padsLabel(pads: string): string {
  return pads === '' || pads === '*' ? 'all pads' : pads
}

export function PageHeader({ title, children }: { title: string; children?: React.ReactNode }) {
  return (
    <div className="mb-8">
      <h2 className="text-2xl font-semibold text-black dark:text-white">{title}</h2>
      {children && <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">{children}</p>}
    </div>
  )
}

export function ErrorBanner({ error, testId }: { error?: string; testId: string }) {
  if (!error) return null
  return (
    <div role="alert" data-testid={testId}
         className="mb-4 rounded-lg border border-red-200 dark:border-red-900 bg-red-50 dark:bg-red-950 px-4 py-3 text-sm text-red-700 dark:text-red-300">
      {error}
    </div>
  )
}

// OneTimePassword shows a new password once; the server keeps only its hash.
export function OneTimePassword({ shown, onClose }: { shown: { username: string; password: string }; onClose: () => void }) {
  const [copied, setCopied] = useState(false)
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(shown.password)
      setCopied(true)
    } catch {
      setCopied(false)
    }
  }
  return (
    <div data-testid="account-password" className="mb-6 rounded-lg border border-green-200 dark:border-green-900 bg-green-50 dark:bg-green-950 p-4">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <p className="text-sm font-medium text-green-800 dark:text-green-200">
            One-time password for <strong>{shown.username}</strong>. Copy it now, it is not shown again.
            It must be changed at the first login.
          </p>
          <code data-testid="account-password-value"
                className="mt-2 block break-all rounded bg-white dark:bg-gray-900 px-3 py-2 font-mono text-sm text-black dark:text-white">
            {shown.password}
          </code>
        </div>
        <button type="button" aria-label="Close" onClick={onClose} className="rounded p-1 text-gray-500 hover:text-black dark:hover:text-white">
          <X className="h-4 w-4" />
        </button>
      </div>
      <button type="button" onClick={copy}
              className="mt-3 flex items-center gap-2 rounded-lg border border-green-300 dark:border-green-800 px-3 py-1.5 text-sm text-green-800 dark:text-green-200">
        {copied ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
        {copied ? 'Copied' : 'Copy'}
      </button>
    </div>
  )
}

// RoleChips lists an account's roles; each can be taken away, and a role
// can be given for all pads or a pattern such as team/*.
export function RoleChips({ account, roles, editable = true }: { account: Account; roles: Role[]; editable?: boolean }) {
  const actions = useAdminActions()
  const [adding, setAdding] = useState(false)
  const [role, setRole] = useState('')
  const [pads, setPads] = useState('')
  const nameOf = (id: string) => roles.find((r) => r.id === id)?.name ?? id
  const add = () => {
    if (!role) return
    actions.addAssignment(account.id, role, pads.trim())
    setAdding(false)
    setRole('')
    setPads('')
  }
  return (
    <div className="flex flex-wrap items-center gap-1.5" data-testid="account-roles">
      {account.roles.length === 0 && <span className="text-xs text-gray-400">no roles</span>}
      {account.roles.map((a: Assignment) => (
        <span key={`${a.role}|${a.pads}`} data-testid="role-chip"
              className="inline-flex items-center gap-1 rounded-full bg-gray-100 dark:bg-gray-800 px-2.5 py-0.5 text-xs text-gray-700 dark:text-gray-200"
              title={`given by ${a.createdBy} on ${formatTime(a.createdAt)}`}>
          {nameOf(a.role)}
          <span className="text-gray-400">· {padsLabel(a.pads)}</span>
          {editable && (
            <button type="button" aria-label={`Remove ${nameOf(a.role)} for ${padsLabel(a.pads)}`}
                    onClick={() => actions.removeAssignment(account.id, a.role, a.pads)}
                    className="ml-0.5 text-gray-400 hover:text-red-600">
              <X className="h-3 w-3" />
            </button>
          )}
        </span>
      ))}
      {editable && !adding && (
        <button type="button" aria-label={`Add a role to ${account.username}`} onClick={() => setAdding(true)}
                className="inline-flex items-center rounded-full border border-dashed border-gray-300 dark:border-gray-700 px-2 py-0.5 text-xs text-gray-500 hover:border-black dark:hover:border-white">
          <Plus className="h-3 w-3" />
        </button>
      )}
      {editable && adding && (
        <span className="inline-flex flex-wrap items-center gap-1.5">
          <select aria-label="Role" value={role} onChange={(e) => setRole(e.target.value)} className={`${inputClass} py-1 text-xs`}>
            <option value="">Role…</option>
            {roles.map((r) => <option key={r.id} value={r.id}>{r.name}</option>)}
          </select>
          <input aria-label="Pads" placeholder="all pads, or e.g. team/*" value={pads}
                 onChange={(e) => setPads(e.target.value)} onKeyDown={(e) => e.key === 'Enter' && add()}
                 className={`${inputClass} w-40 py-1 text-xs`} />
          <button type="button" onClick={add} disabled={!role} className={smallButton}>Add</button>
          <button type="button" onClick={() => setAdding(false)} className="text-xs text-gray-500">Cancel</button>
        </span>
      )}
    </div>
  )
}

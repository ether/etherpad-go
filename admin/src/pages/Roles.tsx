import { useEffect, useState } from 'react'
import { Lock, Plus, RotateCcw, Trash2, X } from 'lucide-react'
import { useAdminStore, type Role, type ScopeInfo } from '@/store'
import { useAdminActions } from '@/hooks/useAdminActions'
import { ErrorBanner, PageHeader, inputClass, primaryButton, smallButton } from '@/components/AccountParts'

interface Draft {
  id?: string
  name: string
  description: string
  scopes: string[]
  includes: string[]
  limits: [string, string][]
}

const emptyDraft: Draft = { name: '', description: '', scopes: [], includes: [], limits: [] }

function draftOf(r: Role): Draft {
  return {
    id: r.id, name: r.name, description: r.description, scopes: [...r.scopes], includes: [...r.includes],
    limits: Object.entries(r.limits).map(([k, v]) => [k, String(v)]),
  }
}

function toggle(list: string[], item: string): string[] {
  return list.includes(item) ? list.filter((x) => x !== item) : [...list, item]
}

// groupOf groups scopes by what comes before the colon (pad, chat, admin…).
function groupOf(scopes: ScopeInfo[]): [string, ScopeInfo[]][] {
  const groups = new Map<string, ScopeInfo[]>()
  for (const s of scopes) {
    const g = s.name.split(':')[0]
    groups.set(g, [...(groups.get(g) ?? []), s])
  }
  return [...groups.entries()]
}

function RoleEditor({ draft, setDraft, roles, scopes, onCancel, onSave }: {
  draft: Draft; setDraft: (d: Draft) => void; roles: Role[]; scopes: ScopeInfo[]; onCancel: () => void; onSave: () => void
}) {
  const actions = useAdminActions()
  const save = () => {
    const limits: Record<string, number> = {}
    for (const [k, v] of draft.limits) {
      const n = Number(v)
      if (k.trim() && Number.isFinite(n)) limits[k.trim()] = Math.trunc(n)
    }
    onSave()
    actions.saveRole({ id: draft.id, name: draft.name.trim(), description: draft.description.trim(), scopes: draft.scopes, includes: draft.includes, limits })
  }
  return (
    <div className="mb-8 rounded-lg border border-gray-200 dark:border-gray-800 p-4" data-testid="role-editor">
      <div className="mb-3 flex items-center justify-between">
        <h3 className="text-sm font-medium text-black dark:text-white">{draft.id ? `Edit ${draft.name}` : 'New role'}</h3>
        <button type="button" aria-label="Close" onClick={onCancel} className="text-gray-400 hover:text-black dark:hover:text-white">
          <X className="h-4 w-4" />
        </button>
      </div>
      <div className="mb-4 flex flex-col gap-2 sm:flex-row">
        <input aria-label="Role name" placeholder="Name" value={draft.name} onChange={(e) => setDraft({ ...draft, name: e.target.value })} className={`${inputClass} sm:w-60`} />
        <input aria-label="Role description" placeholder="What is it for?" value={draft.description}
               onChange={(e) => setDraft({ ...draft, description: e.target.value })} className={`${inputClass} flex-1`} />
      </div>
      <div className="mb-4 grid gap-4 md:grid-cols-2 lg:grid-cols-3">
        {groupOf(scopes).map(([group, list]) => (
          <fieldset key={group}>
            <legend className="mb-1 text-xs font-medium uppercase tracking-wide text-gray-500">{group}</legend>
            {list.map((s) => (
              <label key={s.name} className="flex items-start gap-2 py-0.5 text-sm text-gray-700 dark:text-gray-200" title={s.description}>
                <input type="checkbox" checked={draft.scopes.includes(s.name)} onChange={() => setDraft({ ...draft, scopes: toggle(draft.scopes, s.name) })} className="mt-1" />
                <span><code className="text-xs">{s.name}</code> <span className="text-xs text-gray-500">{s.description}</span></span>
              </label>
            ))}
          </fieldset>
        ))}
      </div>
      <div className="mb-4">
        <div className="mb-1 text-xs font-medium uppercase tracking-wide text-gray-500">Includes the roles</div>
        <div className="flex flex-wrap gap-3">
          {roles.filter((r) => r.id !== draft.id).map((r) => (
            <label key={r.id} className="flex items-center gap-1.5 text-sm text-gray-700 dark:text-gray-200">
              <input type="checkbox" checked={draft.includes.includes(r.id)} onChange={() => setDraft({ ...draft, includes: toggle(draft.includes, r.id) })} />
              {r.name}
            </label>
          ))}
        </div>
      </div>
      <div className="mb-4">
        <div className="mb-1 text-xs font-medium uppercase tracking-wide text-gray-500">Limits (with several roles the highest counts)</div>
        {draft.limits.map(([k, v], i) => (
          <div key={i} className="mb-1 flex gap-2">
            <input aria-label="Limit name" placeholder="e.g. ai.requestsPerDay" value={k}
                   onChange={(e) => setDraft({ ...draft, limits: draft.limits.map((l, j) => (j === i ? [e.target.value, l[1]] : l)) })}
                   className={`${inputClass} flex-1 py-1`} />
            <input aria-label="Limit value" type="number" value={v}
                   onChange={(e) => setDraft({ ...draft, limits: draft.limits.map((l, j) => (j === i ? [l[0], e.target.value] : l)) })}
                   className={`${inputClass} w-32 py-1`} />
            <button type="button" aria-label="Remove limit" onClick={() => setDraft({ ...draft, limits: draft.limits.filter((_, j) => j !== i) })}
                    className="text-gray-400 hover:text-red-600"><X className="h-4 w-4" /></button>
          </div>
        ))}
        <button type="button" onClick={() => setDraft({ ...draft, limits: [...draft.limits, ['', '']] })} className={smallButton}>Add limit</button>
      </div>
      <button type="button" onClick={save} disabled={!draft.name.trim()} className={primaryButton}>Save role</button>
    </div>
  )
}

// RolesPage defines what each role may do. A role holds scopes and can
// include other roles; built-in roles can be changed and reset, the admin
// role is fixed.
export function RolesPage() {
  const store = useAdminStore()
  const actions = useAdminActions()
  const [draft, setDraft] = useState<Draft | null>(null)
  const [confirmDelete, setConfirmDelete] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (store.connected) actions.loadAccounts()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [store.connected])

  const result = store.accounts
  // The editor stays open with what was typed when the server refuses.
  useEffect(() => {
    if (!saving) return
    setSaving(false)
    if (!result?.error) setDraft(null)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [result])
  const roles = result?.roles ?? []
  const scopes = result?.scopes ?? []
  const nameOf = (id: string) => roles.find((r) => r.id === id)?.name ?? id
  const holders = (id: string) => (result?.accounts ?? []).filter((a) => a.roles.some((x) => x.role === id)).length

  return (
    <div className="mx-auto max-w-6xl p-6 lg:p-8">
      <PageHeader title="Roles">
        A role is a set of scopes and can include other roles. Nobody can give rights they do not have themselves.
      </PageHeader>
      <ErrorBanner error={result?.error} testId="roles-error" />

      {draft ? (
        <RoleEditor draft={draft} setDraft={setDraft} roles={roles} scopes={scopes} onCancel={() => setDraft(null)} onSave={() => setSaving(true)} />
      ) : (
        <button type="button" onClick={() => setDraft({ ...emptyDraft })} className={`${primaryButton} mb-6`}>
          <Plus className="h-4 w-4" strokeWidth={1.5} />
          New role
        </button>
      )}

      <div className="grid gap-3">
        {roles.map((r) => (
          <div key={r.id} data-testid="role-card" data-role={r.id} className="rounded-lg border border-gray-200 dark:border-gray-800 p-4">
            <div className="flex flex-wrap items-start justify-between gap-2">
              <div>
                <div className="flex items-center gap-2">
                  <span className="font-medium text-black dark:text-white">{r.name}</span>
                  {r.builtin && <span className="rounded bg-gray-100 dark:bg-gray-800 px-1.5 py-0.5 text-[11px] text-gray-500">built-in</span>}
                  {r.modified && <span className="rounded bg-amber-100 dark:bg-amber-900 px-1.5 py-0.5 text-[11px] text-amber-700 dark:text-amber-200">changed</span>}
                  {r.fixed && <span title="The admin role cannot be changed"><Lock className="h-3.5 w-3.5 text-gray-400" /></span>}
                  <span className="text-xs text-gray-400">{holders(r.id)} accounts</span>
                </div>
                {r.description && <p className="mt-0.5 text-sm text-gray-500">{r.description}</p>}
              </div>
              {!r.fixed && (
                <div className="flex items-center gap-2">
                  <button type="button" onClick={() => setDraft(draftOf(r))} className={smallButton}>Edit</button>
                  {r.builtin && r.modified && (
                    <button type="button" onClick={() => actions.resetRole(r.id)} className={`${smallButton} inline-flex items-center gap-1`}>
                      <RotateCcw className="h-3 w-3" /> Reset
                    </button>
                  )}
                  {!r.builtin && (confirmDelete === r.id ? (
                    <span className="inline-flex items-center gap-2">
                      <button type="button" onClick={() => { actions.deleteRole(r.id); setConfirmDelete(null) }}
                              className="rounded bg-red-600 px-2 py-1 text-xs text-white">Delete for good</button>
                      <button type="button" onClick={() => setConfirmDelete(null)} className="text-xs text-gray-500">Cancel</button>
                    </span>
                  ) : (
                    <button type="button" aria-label={`Delete ${r.name}`} onClick={() => setConfirmDelete(r.id)} className="p-1 text-gray-400 hover:text-red-600">
                      <Trash2 className="h-4 w-4" strokeWidth={1.5} />
                    </button>
                  ))}
                </div>
              )}
            </div>
            <div className="mt-2 flex flex-wrap gap-1">
              {r.scopes.map((s) => <code key={s} className="rounded bg-blue-50 dark:bg-blue-950 px-1.5 py-0.5 text-[11px] text-blue-700 dark:text-blue-300">{s}</code>)}
              {r.includes.map((i) => <span key={i} className="rounded bg-gray-100 dark:bg-gray-800 px-1.5 py-0.5 text-[11px] text-gray-600 dark:text-gray-300">+ {nameOf(i)}</span>)}
              {Object.entries(r.limits).map(([k, v]) => <span key={k} className="rounded bg-gray-100 dark:bg-gray-800 px-1.5 py-0.5 text-[11px] text-gray-600 dark:text-gray-300">{k}: {v}</span>)}
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}

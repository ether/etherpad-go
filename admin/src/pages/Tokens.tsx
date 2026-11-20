import { useEffect, useMemo, useState } from 'react'
import { Bot, Check, Copy, KeyRound, Plus, RotateCw, UserRound, X } from 'lucide-react'
import { useAdminStore, type TokenAccount, type TokenPolicy, type TokenView } from '@/store'
import { useAdminActions } from '@/hooks/useAdminActions'
import { ErrorBanner, PageHeader, formatTime, inputClass, primaryButton, smallButton } from '@/components/AccountParts'

const DAY = 24 * 60 * 60 * 1000
// SOON: tokens that end within it are marked.
const SOON = 14 * DAY

function policyOf(result: { personal: TokenPolicy; service: TokenPolicy } | null, kind: string | undefined): TokenPolicy {
  if (!result) return { requireExpiry: false, maxLifetimeDays: 0 }
  return kind === 'service' ? result.service : result.personal
}

function mayNever(p: TokenPolicy): boolean {
  return !p.requireExpiry && p.maxLifetimeDays === 0
}

// dateInput turns a date field's value (yyyy-mm-dd) into the end of that
// day, local time.
function dateInput(value: string): number | null {
  if (!value) return null
  const t = new Date(`${value}T23:59:59`).getTime()
  return Number.isNaN(t) ? null : t
}

function toDateValue(ms: number): string {
  const d = new Date(ms)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

// ExpiryChoice picks never, a number of days or a date, within the policy.
function ExpiryChoice({ policy, value, onChange, label }: {
  policy: TokenPolicy; value: number | null; onChange: (v: number | null) => void; label: string
}) {
  const [mode, setMode] = useState<string>(mayNever(policy) ? 'never' : '30')
  const [date, setDate] = useState('')
  const max = policy.maxLifetimeDays > 0 ? Date.now() + policy.maxLifetimeDays * DAY : undefined
  const options = [30, 90, 365].filter((d) => !policy.maxLifetimeDays || d <= policy.maxLifetimeDays)
  useEffect(() => {
    if (mode === 'never') onChange(null)
    else if (mode === 'date') onChange(dateInput(date))
    else onChange(Date.now() + Number(mode) * DAY)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [mode, date])
  useEffect(() => {
    // The policy of another account may not allow what was picked.
    if (mode === 'never' && !mayNever(policy)) setMode(options[0] ? String(options[0]) : 'date')
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [policy])
  return (
    <span className="inline-flex items-center gap-2">
      <select aria-label={label} value={mode} onChange={(e) => setMode(e.target.value)} className={inputClass}>
        {mayNever(policy) && <option value="never">Never expires</option>}
        {options.map((d) => <option key={d} value={String(d)}>{d} days</option>)}
        <option value="date">Until a date…</option>
      </select>
      {mode === 'date' && (
        <input aria-label={`${label} date`} type="date" value={date} min={toDateValue(Date.now() + DAY)}
               max={max ? toDateValue(max) : undefined} onChange={(e) => setDate(e.target.value)} className={inputClass} />
      )}
      {value === null && mode === 'date' && <span className="text-xs text-gray-400">pick a date</span>}
    </span>
  )
}

function ShownKey({ created, onClose }: { created: { name: string; key: string }; onClose: () => void }) {
  const [copied, setCopied] = useState(false)
  return (
    <div data-testid="token-key" className="mb-6 rounded-lg border border-green-200 dark:border-green-900 bg-green-50 dark:bg-green-950 p-4">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <p className="text-sm font-medium text-green-800 dark:text-green-200">
            The key of <strong>{created.name}</strong>. Copy it now: it is not shown again, only its start is kept.
          </p>
          <code data-testid="token-key-value" className="mt-2 block break-all rounded bg-white dark:bg-gray-900 px-3 py-2 font-mono text-sm text-black dark:text-white">
            {created.key}
          </code>
        </div>
        <button type="button" aria-label="Close" onClick={onClose} className="rounded p-1 text-gray-500 hover:text-black dark:hover:text-white">
          <X className="h-4 w-4" />
        </button>
      </div>
      <button type="button" onClick={async () => { try { await navigator.clipboard.writeText(created.key); setCopied(true) } catch { setCopied(false) } }}
              className="mt-3 flex items-center gap-2 rounded-lg border border-green-300 dark:border-green-800 px-3 py-1.5 text-sm text-green-800 dark:text-green-200">
        {copied ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
        {copied ? 'Copied' : 'Copy'}
      </button>
    </div>
  )
}

function IssueForm({ accounts, scopes, policyFor, onIssue }: {
  accounts: TokenAccount[]; scopes: { name: string; description: string }[]
  policyFor: (kind?: string) => TokenPolicy
  onIssue: (t: { accountId: string; name: string; scopes: string[]; pads: string; expiresAt: number | null }) => void
}) {
  const usable = accounts.filter((a) => !a.disabled)
  const [accountId, setAccountId] = useState('')
  const [name, setName] = useState('')
  const [pads, setPads] = useState('')
  const [picked, setPicked] = useState<string[]>([])
  const [expiresAt, setExpiresAt] = useState<number | null>(null)
  const account = usable.find((a) => a.id === accountId)
  useEffect(() => {
    // Preselected: everything the account has.
    setPicked(account ? [...account.scopes] : [])
  }, [accountId]) // eslint-disable-line react-hooks/exhaustive-deps
  const policy = policyFor(account?.kind)
  const ready = !!account && name.trim() !== '' && picked.length > 0 && (expiresAt !== null || mayNever(policy))
  return (
    <div className="mb-8 rounded-lg border border-gray-200 dark:border-gray-800 p-4" data-testid="token-issue">
      <h3 className="mb-3 text-sm font-medium text-black dark:text-white">Issue a token</h3>
      <div className="mb-3 flex flex-col gap-2 lg:flex-row">
        <select aria-label="Account" value={accountId} onChange={(e) => setAccountId(e.target.value)} className={inputClass}>
          <option value="">Account…</option>
          {usable.map((a) => <option key={a.id} value={a.id}>{a.username}{a.kind === 'service' ? ' (service)' : ''}</option>)}
        </select>
        <input aria-label="Token name" placeholder="Name, e.g. Moodle or Claude Desktop" value={name} onChange={(e) => setName(e.target.value)}
               className={`${inputClass} flex-1`} />
        <input aria-label="Token pads" placeholder="all pads, or e.g. claude/*" value={pads} onChange={(e) => setPads(e.target.value)}
               className={`${inputClass} lg:w-48`} />
        <ExpiryChoice policy={policy} value={expiresAt} onChange={setExpiresAt} label="Token expiry" />
      </div>
      {account && (
        <div className="mb-3 flex flex-wrap gap-x-4 gap-y-1" data-testid="token-scopes">
          {scopes.map((s) => {
            const has = account.scopes.includes(s.name)
            return (
              <label key={s.name} title={has ? s.description : `${s.description} – the account does not have it`}
                     className={`flex items-center gap-1.5 text-sm ${has ? 'text-gray-700 dark:text-gray-200' : 'text-gray-300 dark:text-gray-600'}`}>
                <input type="checkbox" disabled={!has} checked={picked.includes(s.name)}
                       onChange={() => setPicked(picked.includes(s.name) ? picked.filter((x) => x !== s.name) : [...picked, s.name])} />
                <code className="text-xs">{s.name}</code>
              </label>
            )
          })}
        </div>
      )}
      <p className="mb-3 text-xs text-gray-500">
        A token can never do more than its account: it gets the account's rights of the moment, cut down to these scopes and pads.
      </p>
      <button type="button" disabled={!ready} className={primaryButton}
              onClick={() => onIssue({ accountId, name: name.trim(), scopes: picked, pads: pads.trim(), expiresAt })}>
        <Plus className="h-4 w-4" strokeWidth={1.5} /> Issue
      </button>
    </div>
  )
}

function RotateDialog({ token, onRotate, onCancel }: { token: TokenView; onRotate: (oldUntil: number | null) => void; onCancel: () => void }) {
  const [mode, setMode] = useState('10')
  const [date, setDate] = useState('')
  const until = mode === 'now' ? null : mode === 'date' ? dateInput(date) : Date.now() + Number(mode) * DAY
  return (
    <div className="mt-2 flex flex-wrap items-center gap-2 rounded border border-gray-200 dark:border-gray-700 p-2" data-testid="token-rotate">
      <span className="text-xs text-gray-600 dark:text-gray-300">New key with the same rights. The old one keeps working</span>
      <select aria-label="Old key works" value={mode} onChange={(e) => setMode(e.target.value)} className={`${inputClass} py-1 text-xs`}>
        <option value="now">not at all (ends now)</option>
        <option value="10">10 more days</option>
        <option value="20">20 more days</option>
        <option value="date">until a date…</option>
      </select>
      {mode === 'date' && <input aria-label="Old key until" type="date" value={date} onChange={(e) => setDate(e.target.value)} className={`${inputClass} py-1 text-xs`} />}
      <button type="button" className={smallButton} disabled={mode === 'date' && until === null} onClick={() => onRotate(until)}
              title={token.expiresAt ? 'The old key never lasts longer than it would have' : undefined}>Rotate</button>
      <button type="button" className="text-xs text-gray-500" onClick={onCancel}>Cancel</button>
    </div>
  )
}

function ExpiryEditor({ token, policy, onSave, onCancel }: { token: TokenView; policy: TokenPolicy; onSave: (v: number | null) => void; onCancel: () => void }) {
  const [value, setValue] = useState<number | null>(token.expiresAt)
  return (
    <div className="mt-2 flex flex-wrap items-center gap-2 rounded border border-gray-200 dark:border-gray-700 p-2" data-testid="token-expiry">
      <span className="text-xs text-gray-600 dark:text-gray-300">Ends</span>
      <ExpiryChoice policy={policy} value={value} onChange={setValue} label="New expiry" />
      <button type="button" className={smallButton} disabled={value === null && !mayNever(policy)} onClick={() => onSave(value)}>Save</button>
      <button type="button" className="text-xs text-gray-500" onClick={onCancel}>Cancel</button>
    </div>
  )
}

function ExpiryCell({ t }: { t: TokenView }) {
  if (t.status === 'revoked') return <span className="text-gray-400">revoked {formatTime(t.revokedAt)}</span>
  if (t.expiresAt === null) return <span data-testid="token-never" className="rounded bg-amber-100 dark:bg-amber-900 px-1.5 py-0.5 text-xs text-amber-700 dark:text-amber-200">never expires</span>
  const left = t.expiresAt - Date.now()
  if (left <= 0) return <span className="text-gray-400">expired {formatTime(t.expiresAt)}</span>
  const label = t.status === 'grace' ? 'replaced, ends' : 'ends'
  return (
    <span data-testid={left < SOON ? 'token-soon' : undefined} className={left < SOON ? 'text-red-600 dark:text-red-400' : 'text-gray-600 dark:text-gray-300'}>
      {label} {formatTime(t.expiresAt)}{left < SOON && ` (${Math.max(1, Math.ceil(left / DAY))} days)`}
    </span>
  )
}

// TokensPage lists every token of the instance and issues, rotates and
// revokes them. A token can never do more than its account.
export function TokensPage() {
  const store = useAdminStore()
  const actions = useAdminActions()
  const [filterAccount, setFilterAccount] = useState('')
  const [showEnded, setShowEnded] = useState(false)
  const [rotating, setRotating] = useState<string | null>(null)
  const [editing, setEditing] = useState<string | null>(null)
  const [confirmRevoke, setConfirmRevoke] = useState<string | null>(null)

  useEffect(() => {
    if (store.connected) actions.loadTokens()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [store.connected])
  const clearKey = () => store.dispatch({ type: 'CLEAR_TOKEN_KEY' })
  useEffect(() => clearKey, []) // eslint-disable-line react-hooks/exhaustive-deps

  const result = store.tokens
  const accountsById = useMemo(() => new Map((result?.accounts ?? []).map((a) => [a.id, a])), [result])
  const nameOf = (id: string) => result?.names[id] ?? (id === 'system' || id === 'cli' ? id : `${id} (deleted)`)
  const list = (result?.tokens ?? [])
    .filter((t) => !filterAccount || t.accountId === filterAccount)
    .filter((t) => showEnded || t.status === 'active' || t.status === 'grace')

  return (
    <div className="mx-auto max-w-6xl p-6 lg:p-8">
      <PageHeader title="Tokens">
        Keys for the HTTP API and, later, MCP. Each belongs to an account and can do at most what the account can, within its scopes and pads.
      </PageHeader>
      <ErrorBanner error={result?.error} testId="tokens-error" />
      {store.tokenKey && <ShownKey created={store.tokenKey} onClose={clearKey} />}

      {/* A new key resets the form for the next token. */}
      <IssueForm key={store.tokenKey?.id ?? 'issue'} accounts={result?.accounts ?? []} scopes={result?.scopes ?? []} policyFor={(k) => policyOf(result, k)}
                 onIssue={(t) => actions.issueToken(t)} />

      <div className="mb-3 flex flex-wrap items-center gap-3">
        <select aria-label="Filter by account" value={filterAccount} onChange={(e) => setFilterAccount(e.target.value)} className={`${inputClass} py-1.5`}>
          <option value="">All accounts</option>
          {(result?.accounts ?? []).map((a) => <option key={a.id} value={a.id}>{a.username}</option>)}
        </select>
        <label className="flex items-center gap-1.5 text-sm text-gray-600 dark:text-gray-300">
          <input type="checkbox" checked={showEnded} onChange={(e) => setShowEnded(e.target.checked)} /> Show revoked and expired
        </label>
        <span className="text-xs text-gray-400">{list.length} tokens</span>
      </div>

      <div className="overflow-x-auto rounded-lg border border-gray-200 dark:border-gray-800">
        <table className="w-full text-sm" data-testid="tokens-table">
          <thead className="bg-gray-50 dark:bg-gray-900 text-left text-xs font-medium uppercase tracking-wide text-gray-500">
            <tr>
              <th className="px-4 py-3">Token</th>
              <th className="px-4 py-3">Account</th>
              <th className="px-4 py-3">May</th>
              <th className="px-4 py-3">Expiry</th>
              <th className="px-4 py-3">Last used</th>
              <th className="px-4 py-3" />
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100 dark:divide-gray-800">
            {list.map((t) => {
              const account = accountsById.get(t.accountId)
              const live = t.status === 'active' || t.status === 'grace'
              return (
                <tr key={t.id} data-testid="token-row" data-token={t.name} data-status={t.status} className={live ? '' : 'opacity-60'}>
                  <td className="px-4 py-3 align-top">
                    <div className="flex items-center gap-2 font-medium text-black dark:text-white">
                      <KeyRound className="h-4 w-4 text-gray-400" /> {t.name}
                    </div>
                    <code className="text-xs text-gray-500">{t.prefix}…</code>
                    <div className="text-xs text-gray-400">by {nameOf(t.createdBy)}, {formatTime(t.createdAt)}</div>
                    {rotating === t.id && (
                      <RotateDialog token={t} onCancel={() => setRotating(null)}
                                    onRotate={(until) => { actions.rotateToken(t.id, until); setRotating(null) }} />
                    )}
                    {editing === t.id && (
                      <ExpiryEditor token={t} policy={policyOf(result, account?.kind)} onCancel={() => setEditing(null)}
                                    onSave={(v) => { actions.setTokenExpiry(t.id, v); setEditing(null) }} />
                    )}
                  </td>
                  <td className="px-4 py-3 align-top">
                    <span className="inline-flex items-center gap-1.5">
                      {account?.kind === 'service' ? <Bot className="h-4 w-4 text-gray-400" /> : <UserRound className="h-4 w-4 text-gray-400" />}
                      {nameOf(t.accountId)}
                    </span>
                    {account?.disabled && <div className="text-xs text-red-600">account disabled</div>}
                  </td>
                  <td className="px-4 py-3 align-top">
                    <div className="flex flex-wrap gap-1">
                      {t.scopes.includes('*')
                        ? <span className="rounded bg-blue-50 dark:bg-blue-950 px-1.5 py-0.5 text-[11px] text-blue-700 dark:text-blue-300">all scopes</span>
                        : t.scopes.map((s) => <code key={s} className="rounded bg-blue-50 dark:bg-blue-950 px-1.5 py-0.5 text-[11px] text-blue-700 dark:text-blue-300">{s}</code>)}
                    </div>
                    <div className="mt-1 text-xs text-gray-500">{t.pads === '' || t.pads === '*' ? "all of the account's pads" : `${t.pads} (within the account's)`}</div>
                  </td>
                  <td className="px-4 py-3 align-top"><ExpiryCell t={t} /></td>
                  <td className="whitespace-nowrap px-4 py-3 align-top text-gray-500">
                    {formatTime(t.lastUsedAt)}
                    {t.lastUsedIp && <div className="text-xs text-gray-400">{t.lastUsedIp}</div>}
                  </td>
                  <td className="whitespace-nowrap px-4 py-3 text-right align-top">
                    {live && (
                      <span className="inline-flex flex-wrap justify-end gap-1.5">
                        {t.status === 'active' && (
                          <button type="button" className={`${smallButton} inline-flex items-center gap-1`} onClick={() => { setRotating(t.id); setEditing(null) }}>
                            <RotateCw className="h-3 w-3" /> Rotate
                          </button>
                        )}
                        <button type="button" className={smallButton} onClick={() => { setEditing(t.id); setRotating(null) }}>Expiry</button>
                        {confirmRevoke === t.id ? (
                          <>
                            <button type="button" onClick={() => { actions.revokeToken(t.id); setConfirmRevoke(null) }}
                                    className="rounded bg-red-600 px-2 py-0.5 text-xs text-white">Revoke now</button>
                            <button type="button" onClick={() => setConfirmRevoke(null)} className="text-xs text-gray-500">Cancel</button>
                          </>
                        ) : (
                          <button type="button" className={smallButton} onClick={() => setConfirmRevoke(t.id)}>Revoke</button>
                        )}
                      </span>
                    )}
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>
        {list.length === 0 && <p className="py-8 text-center text-sm text-gray-400">No tokens.</p>}
      </div>
    </div>
  )
}

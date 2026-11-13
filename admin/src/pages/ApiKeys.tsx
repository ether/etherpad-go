import { useEffect, useState } from 'react'
import { Check, Copy, KeyRound, Plus, Trash2, X } from 'lucide-react'
import { useAdminStore } from '@/store'
import { useAdminActions } from '@/hooks/useAdminActions'

const MAX_DESCRIPTION = 200

function formatTime(ms: number | null): string {
  if (ms == null) return 'never'
  return new Date(ms).toLocaleString()
}

// ApiKeysPage manages the keys of the HTTP API (/api): one key per service,
// each can be disabled or deleted without touching the others. A new key
// is shown once; the server keeps only its hash.
export function ApiKeysPage() {
  const store = useAdminStore()
  const actions = useAdminActions()
  const [description, setDescription] = useState('')
  const [confirmDelete, setConfirmDelete] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)
  const [shownKey, setShownKey] = useState<{ id: string; key: string } | null>(null)

  useEffect(() => {
    if (store.connected) actions.loadApiKeys()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [store.connected])

  const result = store.apiKeys
  const keys = result?.keys ?? []

  // A created key arrives once; keep it until the admin closes it.
  useEffect(() => {
    if (result?.created) {
      setShownKey(result.created)
      setCopied(false)
      setDescription('')
    }
  }, [result])

  const create = () => {
    const d = description.trim()
    if (!d) return
    actions.createApiKey(d)
  }

  const copy = async () => {
    if (!shownKey) return
    try {
      await navigator.clipboard.writeText(shownKey.key)
      setCopied(true)
    } catch {
      setCopied(false)
    }
  }

  return (
    <div className="mx-auto max-w-5xl p-6 lg:p-8">
      <div className="mb-8">
        <h2 className="text-2xl font-semibold text-black dark:text-white">API keys</h2>
        <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
          Keys for the HTTP API (<code>/api/…?apikey=…</code>), used by Moodle and other services.
          Give every service its own key: you can then disable or delete one without touching the others.
        </p>
      </div>

      {result?.error && (
        <div role="alert" data-testid="apikey-error"
             className="mb-4 rounded-lg border border-red-200 dark:border-red-900 bg-red-50 dark:bg-red-950 px-4 py-3 text-sm text-red-700 dark:text-red-300">
          {result.error}
        </div>
      )}

      {shownKey && (
        <div data-testid="apikey-created"
             className="mb-6 rounded-lg border border-green-200 dark:border-green-900 bg-green-50 dark:bg-green-950 p-4">
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0">
              <p className="text-sm font-medium text-green-800 dark:text-green-200">
                Copy the key now. It is not shown again.
              </p>
              <code data-testid="apikey-value"
                    className="mt-2 block break-all rounded bg-white dark:bg-gray-900 px-3 py-2 font-mono text-sm text-black dark:text-white">
                {shownKey.key}
              </code>
            </div>
            <button type="button" aria-label="Close" onClick={() => setShownKey(null)}
                    className="rounded p-1 text-gray-500 hover:text-black dark:hover:text-white">
              <X className="h-4 w-4" />
            </button>
          </div>
          <button type="button" onClick={copy}
                  className="mt-3 flex items-center gap-2 rounded-lg border border-green-300 dark:border-green-800 px-3 py-1.5 text-sm text-green-800 dark:text-green-200">
            {copied ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
            {copied ? 'Copied' : 'Copy'}
          </button>
        </div>
      )}

      <div className="mb-8 flex flex-col gap-2 sm:flex-row">
        <input
          type="text"
          aria-label="Description of the new key"
          placeholder="What is the key for? e.g. Moodle"
          value={description}
          maxLength={MAX_DESCRIPTION}
          onChange={(e) => setDescription(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && create()}
          className="flex-1 rounded-lg border border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900 px-3 py-2 text-sm text-black dark:text-white placeholder-gray-400 focus:border-black dark:focus:border-white focus:outline-none"
        />
        <button type="button" onClick={create} disabled={!description.trim()}
                className="flex items-center justify-center gap-2 rounded-lg bg-black px-4 py-2 text-sm font-medium text-white hover:bg-gray-800 disabled:cursor-not-allowed disabled:opacity-40">
          <Plus className="h-4 w-4" strokeWidth={1.5} />
          Create key
        </button>
      </div>

      {keys.length === 0 ? (
        <div className="rounded-lg border border-dashed border-gray-200 dark:border-gray-700 py-12 text-center">
          <KeyRound className="mx-auto mb-2 h-6 w-6 text-gray-300" />
          <p className="text-sm text-gray-400 dark:text-gray-500">No API keys yet.</p>
        </div>
      ) : (
        <div className="overflow-x-auto rounded-lg border border-gray-200 dark:border-gray-800">
          <table className="w-full text-sm" data-testid="apikey-table">
            <thead className="bg-gray-50 dark:bg-gray-900 text-left text-xs font-medium uppercase tracking-wide text-gray-500">
              <tr>
                <th className="px-4 py-3">Description</th>
                <th className="px-4 py-3">Key</th>
                <th className="px-4 py-3">Created</th>
                <th className="px-4 py-3">Last used</th>
                <th className="px-4 py-3">Status</th>
                <th className="px-4 py-3" />
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100 dark:divide-gray-800">
              {keys.map((k) => (
                <tr key={k.id} data-testid="apikey-row" className={k.disabled ? 'opacity-60' : ''}>
                  <td className="px-4 py-3 font-medium text-black dark:text-white">{k.description}</td>
                  <td className="px-4 py-3 font-mono text-gray-500">{k.prefix}…</td>
                  <td className="whitespace-nowrap px-4 py-3 text-gray-500">{formatTime(k.createdAt)}</td>
                  <td className="whitespace-nowrap px-4 py-3 text-gray-500">{formatTime(k.lastUsed)}</td>
                  <td className="px-4 py-3">
                    <button type="button"
                            onClick={() => actions.setApiKeyDisabled(k.id, !k.disabled)}
                            className="rounded-full border border-gray-200 dark:border-gray-700 px-3 py-0.5 text-xs font-medium text-gray-600 dark:text-gray-300 hover:border-black dark:hover:border-white">
                      {k.disabled ? 'Disabled – enable' : 'Active – disable'}
                    </button>
                  </td>
                  <td className="px-4 py-3 text-right">
                    {confirmDelete === k.id ? (
                      <span className="flex items-center justify-end gap-2">
                        <button type="button" onClick={() => { actions.deleteApiKey(k.id); setConfirmDelete(null) }}
                                className="rounded bg-red-600 px-2 py-1 text-xs text-white">Delete for good</button>
                        <button type="button" onClick={() => setConfirmDelete(null)}
                                className="rounded px-2 py-1 text-xs text-gray-500">Cancel</button>
                      </span>
                    ) : (
                      <button type="button" aria-label={`Delete ${k.description}`} onClick={() => setConfirmDelete(k.id)}
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
      )}
    </div>
  )
}

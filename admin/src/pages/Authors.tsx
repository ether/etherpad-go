import { useEffect, useState } from 'react'
import { Users } from 'lucide-react'
import { useAdminStore, type AuthorEntry } from '@/store'
import { useAdminActions } from '@/hooks/useAdminActions'

const PAGE = 12

function formatTime(ms: number | null): string {
  return ms == null ? '–' : new Date(ms).toLocaleString()
}

// AuthorsPage lists the authors and erases one on request (GDPR Art. 17):
// a dry run shows what would be touched, then the admin confirms. It needs
// gdprAuthorErasure.enabled in the settings.
export function AuthorsPage() {
  const store = useAdminStore()
  const actions = useAdminActions()
  const [pattern, setPattern] = useState('')
  const [includeErased, setIncludeErased] = useState(false)
  const [sortBy, setSortBy] = useState<'name' | 'lastSeen'>('name')
  const [ascending, setAscending] = useState(true)
  const [offset, setOffset] = useState(0)
  const [selected, setSelected] = useState<AuthorEntry | null>(null)

  const load = () => actions.loadAuthors({ pattern, offset, limit: PAGE, sortBy, ascending, includeErased })

  useEffect(() => {
    if (store.connected) load()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [store.connected, pattern, includeErased, sortBy, ascending, offset])

  // Reload after an erasure.
  useEffect(() => {
    if (store.erasureDone) {
      setSelected(null)
      load()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [store.erasureDone])

  const result = store.authors
  const disabled = result?.error === 'disabled'
  const rows = result?.results ?? []
  const total = result?.total ?? 0
  const preview = selected && store.erasurePreview?.authorID === selected.authorID ? store.erasurePreview : null

  return (
    <div className="mx-auto max-w-6xl p-6 lg:p-8">
      <div className="mb-6">
        <h2 className="text-2xl font-semibold text-black dark:text-white">Authors</h2>
        <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
          Find an author and erase their identity (GDPR Art. 17): name, colour, the links to their browser and
          external account, and the authorship of their chat messages. The pad text stays.
        </p>
      </div>

      {disabled && (
        <div role="alert" data-testid="authors-disabled"
             className="mb-6 rounded-lg border border-amber-300 dark:border-amber-700 bg-amber-50 dark:bg-amber-950 px-4 py-3 text-sm text-amber-800 dark:text-amber-200">
          Author erasure is off. Set <code>gdprAuthorErasure.enabled</code> to <code>true</code> in the settings to use this page.
        </div>
      )}

      {!disabled && (
        <>
          <div className="mb-4 flex flex-wrap items-center gap-3">
            <input type="search" aria-label="Search authors" placeholder="Name, id or external id"
                   value={pattern} onChange={(e) => { setOffset(0); setPattern(e.target.value) }}
                   className="w-72 rounded-lg border border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900 px-3 py-2 text-sm text-black dark:text-white" />
            <select aria-label="Sort by" value={`${sortBy}:${ascending}`}
                    onChange={(e) => { const [s, a] = e.target.value.split(':'); setSortBy(s as 'name' | 'lastSeen'); setAscending(a === 'true') }}
                    className="rounded-lg border border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900 px-3 py-2 text-sm text-black dark:text-white">
              <option value="name:true">Name A–Z</option>
              <option value="name:false">Name Z–A</option>
              <option value="lastSeen:false">Seen recently</option>
              <option value="lastSeen:true">Seen longest ago</option>
            </select>
            <label className="flex items-center gap-2 text-sm text-gray-600 dark:text-gray-300">
              <input type="checkbox" checked={includeErased} onChange={(e) => { setOffset(0); setIncludeErased(e.target.checked) }} />
              Show erased
            </label>
            <span className="ml-auto text-xs text-gray-500" data-testid="authors-total">
              {total} author{total === 1 ? '' : 's'}{result?.cappedAt ? ` (first ${result.cappedAt}; narrow the search)` : ''}
            </span>
          </div>

          {rows.length === 0 ? (
            <div className="rounded-lg border border-dashed border-gray-200 dark:border-gray-700 py-12 text-center">
              <Users className="mx-auto mb-2 h-6 w-6 text-gray-300" />
              <p className="text-sm text-gray-400 dark:text-gray-500">No authors found.</p>
            </div>
          ) : (
            <div className="overflow-x-auto rounded-lg border border-gray-200 dark:border-gray-800">
              <table className="w-full text-sm" data-testid="authors-table">
                <thead className="bg-gray-50 dark:bg-gray-900 text-left text-xs font-medium uppercase tracking-wide text-gray-500">
                  <tr>
                    <th className="px-4 py-3">Name</th>
                    <th className="px-4 py-3">Author id</th>
                    <th className="px-4 py-3">External id</th>
                    <th className="px-4 py-3">Last seen</th>
                    <th className="px-4 py-3" />
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-100 dark:divide-gray-800">
                  {rows.map((a) => (
                    <tr key={a.authorID} data-testid="author-row" className={a.erased ? 'opacity-60' : ''}>
                      <td className="px-4 py-3 font-medium text-black dark:text-white">{a.name ?? <em className="text-gray-400">no name</em>}</td>
                      <td className="px-4 py-3 font-mono text-xs text-gray-500">{a.authorID}</td>
                      <td className="px-4 py-3 text-gray-500">{a.mapper.join(', ') || '–'}</td>
                      <td className="whitespace-nowrap px-4 py-3 text-gray-500">{formatTime(a.lastSeen)}</td>
                      <td className="px-4 py-3 text-right">
                        {a.erased ? (
                          <span className="text-xs text-gray-500">erased {formatTime(a.erasedAt)}</span>
                        ) : (
                          <button type="button" aria-label={`Erase ${a.name ?? a.authorID}`}
                                  onClick={() => { setSelected(a); actions.previewErasure(a.authorID) }}
                                  className="rounded border border-red-300 dark:border-red-800 px-2 py-1 text-xs text-red-700 dark:text-red-300">
                            Erase…
                          </button>
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}

          {total > PAGE && (
            <div className="mt-3 flex items-center justify-end gap-2 text-sm">
              <button type="button" disabled={offset === 0} onClick={() => setOffset(Math.max(0, offset - PAGE))}
                      className="rounded px-2 py-1 disabled:opacity-40">Previous</button>
              <span className="text-gray-500">{offset + 1}–{Math.min(offset + PAGE, total)} of {total}</span>
              <button type="button" disabled={offset + PAGE >= total} onClick={() => setOffset(offset + PAGE)}
                      className="rounded px-2 py-1 disabled:opacity-40">Next</button>
            </div>
          )}
        </>
      )}

      {selected && (
        <div role="dialog" aria-label="Erase author" data-testid="erasure-dialog"
             className="mt-6 rounded-lg border border-red-200 dark:border-red-900 bg-red-50 dark:bg-red-950 p-4 text-sm">
          <p className="font-medium text-red-800 dark:text-red-200">
            Erase {selected.name ?? selected.authorID}? This cannot be undone.
          </p>
          {preview?.error ? (
            <p className="mt-2 text-red-700 dark:text-red-300">{preview.error}</p>
          ) : preview ? (
            <ul className="mt-2 list-disc pl-5 text-red-700 dark:text-red-300" data-testid="erasure-preview">
              <li>{preview.affectedPads} pad(s) keep their text, without this author's name</li>
              <li>{preview.clearedChatMessages} chat message(s) lose their author</li>
              <li>{preview.removedTokenMappings} browser link(s) and {preview.removedExternalMappings} external id(s) are removed</li>
            </ul>
          ) : (
            <p className="mt-2 text-red-700 dark:text-red-300">Counting…</p>
          )}
          <div className="mt-3 flex gap-2">
            <button type="button" disabled={!preview || !!preview.error}
                    onClick={() => actions.eraseAuthor(selected.authorID)}
                    className="rounded bg-red-600 px-3 py-1.5 text-xs font-medium text-white disabled:opacity-40">
              Erase for good
            </button>
            <button type="button" onClick={() => setSelected(null)} className="rounded px-3 py-1.5 text-xs text-gray-600 dark:text-gray-300">
              Cancel
            </button>
          </div>
        </div>
      )}
    </div>
  )
}

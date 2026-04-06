import { useEffect, useState, useCallback } from 'react'
import {
  Plus,
  ExternalLink,
  Trash2,
  ArrowUpDown,
  ChevronLeft,
  ChevronRight,
  RotateCcw,
  X,
  Eye,
} from 'lucide-react'
import { useAdminStore } from '@/store'
import { useAdminActions } from '@/hooks/useAdminActions'

const PAD_LIMIT = 12

export function PadsPage() {
  const store = useAdminStore()
  const actions = useAdminActions()
  const [showCreateModal, setShowCreateModal] = useState(false)
  const [newPadName, setNewPadName] = useState('')
  const [confirmDelete, setConfirmDelete] = useState<string | null>(null)
  const [confirmClean, setConfirmClean] = useState<string | null>(null)

  // Bulk selection state
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [confirmBulkDelete, setConfirmBulkDelete] = useState(false)

  // Preview state
  const [previewPad, setPreviewPad] = useState<string | null>(null)

  // Fulltext search state
  const [contentQuery, setContentQuery] = useState('')

  const doRequest = useCallback(() => {
    actions.requestPads({
      offset: store.padOffset,
      limit: PAD_LIMIT,
      pattern: store.padSearch,
      sortBy: store.padSort,
      ascending: store.padAscending,
    })
  }, [actions, store.padOffset, store.padSearch, store.padSort, store.padAscending])

  useEffect(() => {
    store.setPadLimit(PAD_LIMIT)
    actions.requestPads({
      offset: 0,
      limit: 12,
      pattern: '',
      sortBy: 'padName',
      ascending: true,
    })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const handleSort = (key: string) => {
    if (store.padSort === key) {
      store.setPadAscending(!store.padAscending)
    } else {
      store.setPadSort(key)
      store.setPadAscending(true)
    }
    setTimeout(() => doRequest(), 0)
  }

  const handleSearch = () => {
    store.setPadOffset(0)
    setTimeout(() => doRequest(), 0)
  }

  const handlePrev = () => {
    if (store.padOffset >= PAD_LIMIT) {
      store.setPadOffset(store.padOffset - PAD_LIMIT)
      setTimeout(() => doRequest(), 0)
    }
  }

  const handleNext = () => {
    if (store.padOffset + PAD_LIMIT < store.padsTotal) {
      store.setPadOffset(store.padOffset + PAD_LIMIT)
      setTimeout(() => doRequest(), 0)
    }
  }

  const handleCreatePad = () => {
    const name = newPadName.trim()
    if (!name) return
    actions.createPad(name)
    setNewPadName('')
    setShowCreateModal(false)
    setTimeout(() => doRequest(), 500)
  }

  const handleDeletePad = (padName: string) => {
    actions.deletePad(padName)
    setConfirmDelete(null)
    setTimeout(() => doRequest(), 500)
  }

  const handleCleanPad = (padName: string) => {
    actions.cleanupPadRevisions(padName)
    setConfirmClean(null)
    setTimeout(() => doRequest(), 500)
  }

  // -- Bulk Actions --
  const toggleSelect = (padName: string) => {
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(padName)) {
        next.delete(padName)
      } else {
        next.add(padName)
      }
      return next
    })
  }

  const toggleSelectAll = () => {
    if (store.pads.length > 0 && store.pads.every((p) => selected.has(p.padName))) {
      // Deselect all visible
      setSelected((prev) => {
        const next = new Set(prev)
        store.pads.forEach((p) => next.delete(p.padName))
        return next
      })
    } else {
      // Select all visible
      setSelected((prev) => {
        const next = new Set(prev)
        store.pads.forEach((p) => next.add(p.padName))
        return next
      })
    }
  }

  const handleBulkDelete = () => {
    actions.bulkDeletePads([...selected])
    setSelected(new Set())
    setConfirmBulkDelete(false)
    setTimeout(() => doRequest(), 500)
  }

  // -- Preview --
  const handlePreview = (padName: string) => {
    setPreviewPad(padName)
    actions.getPadContent(padName)
  }

  const closePreview = () => {
    setPreviewPad(null)
  }

  // -- Fulltext Search --
  const handleContentSearch = () => {
    const q = contentQuery.trim()
    if (!q) return
    actions.searchPadContent(q)
  }

  const clearSearchResults = () => {
    store.searchResults = undefined as any
  }

  const totalPages = store.padsTotal > 0 ? Math.ceil(store.padsTotal / PAD_LIMIT) : 1
  const currentPage = Math.floor(store.padOffset / PAD_LIMIT) + 1
  const allVisibleSelected =
    store.pads.length > 0 && store.pads.every((p) => selected.has(p.padName))

  const sortColumns = [
    { key: 'padName', label: 'Pad Name' },
    { key: 'userCount', label: 'Users' },
    { key: 'lastEdited', label: 'Last Edited' },
    { key: 'revisionNumber', label: 'Revisions' },
  ] as const

  return (
    <div className="mx-auto max-w-7xl p-6 lg:p-8">
      {/* Header */}
      <div className="mb-8">
        <h2 className="text-2xl font-semibold text-black dark:text-white">Pads</h2>
        <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
          Manage and inspect collaborative pads
        </p>
      </div>

      {/* Toolbar */}
      <div className="mb-4 flex flex-col gap-3 sm:flex-row sm:items-center">
        <div className="flex flex-1 gap-2">
          <input
            type="text"
            placeholder="Search pads..."
            value={store.padSearch}
            onChange={(e) => store.setPadSearch(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && handleSearch()}
            className="flex-1 rounded-lg border border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900 px-3 py-2 text-sm text-black dark:text-white placeholder-gray-400 dark:placeholder-gray-500 transition-colors focus:border-black dark:focus:border-white focus:outline-none"
          />
          <button
            type="button"
            onClick={handleSearch}
            className="rounded-lg border border-gray-200 dark:border-gray-700 px-4 py-2 text-sm font-medium text-gray-600 dark:text-gray-300 transition-colors hover:border-black dark:hover:border-white hover:text-black dark:hover:text-white"
          >
            Apply
          </button>
        </div>
        <button
          type="button"
          onClick={() => setShowCreateModal(true)}
          className="flex items-center gap-2 rounded-lg bg-black px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-800"
        >
          <Plus className="h-4 w-4" strokeWidth={1.5} />
          Create Pad
        </button>
      </div>

      {/* Fulltext Search */}
      <div className="mb-6 flex gap-2">
        <input
          type="text"
          placeholder="Search in content..."
          value={contentQuery}
          onChange={(e) => setContentQuery(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && handleContentSearch()}
          className="flex-1 rounded-lg border border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900 px-3 py-2 text-sm text-black dark:text-white placeholder-gray-400 dark:placeholder-gray-500 transition-colors focus:border-black dark:focus:border-white focus:outline-none"
        />
        <button
          type="button"
          onClick={handleContentSearch}
          className="rounded-lg border border-gray-200 dark:border-gray-700 px-4 py-2 text-sm font-medium text-gray-600 dark:text-gray-300 transition-colors hover:border-black dark:hover:border-white hover:text-black dark:hover:text-white"
        >
          Search
        </button>
      </div>

      {/* Fulltext Search Results */}
      {store.searchResults && store.searchResults.length > 0 && (
        <div className="mb-6">
          <div className="mb-2 flex items-center justify-between">
            <h3 className="text-sm font-medium text-black dark:text-white">
              Content Search Results ({store.searchResults.length})
            </h3>
            <button
              type="button"
              onClick={clearSearchResults}
              className="flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-gray-500 dark:text-gray-400 transition-colors hover:text-black dark:hover:text-white"
            >
              <X className="h-3 w-3" strokeWidth={1.5} />
              Clear results
            </button>
          </div>
          <div className="space-y-2">
            {store.searchResults.map((result: any, idx: number) => (
              <div
                key={`${result.padId}-${idx}`}
                className="rounded-lg border border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900 px-4 py-3"
              >
                <button
                  type="button"
                  onClick={() => window.open(`/p/${result.padId}`, '_blank')}
                  className="text-sm font-medium text-black dark:text-white underline decoration-gray-300 dark:decoration-gray-600 underline-offset-2 transition-colors hover:decoration-black dark:hover:decoration-white"
                >
                  {result.padId}
                </button>
                {result.snippet && (
                  <p className="mt-1 text-xs text-gray-500 dark:text-gray-400 line-clamp-2">
                    {result.snippet}
                  </p>
                )}
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Bulk Actions Toolbar */}
      {selected.size > 0 && (
        <div className="mb-4 flex items-center gap-3 rounded-lg border border-gray-200 dark:border-gray-800 bg-gray-50 dark:bg-gray-900 px-4 py-2.5">
          <span className="text-sm font-medium text-black dark:text-white">
            {selected.size} selected
          </span>
          {confirmBulkDelete ? (
            <div className="flex items-center gap-2">
              <span className="text-xs text-gray-500 dark:text-gray-400">
                Delete {selected.size} pad{selected.size > 1 ? 's' : ''}?
              </span>
              <button
                type="button"
                onClick={handleBulkDelete}
                className="rounded-md bg-red-600 px-3 py-1 text-xs font-medium text-white transition-colors hover:bg-red-700"
              >
                Confirm
              </button>
              <button
                type="button"
                onClick={() => setConfirmBulkDelete(false)}
                className="rounded-md p-1 text-gray-400 hover:text-black dark:hover:text-white"
              >
                <X className="h-3.5 w-3.5" strokeWidth={1.5} />
              </button>
            </div>
          ) : (
            <button
              type="button"
              onClick={() => setConfirmBulkDelete(true)}
              className="flex items-center gap-1.5 rounded-md border border-red-200 dark:border-red-800 px-3 py-1 text-xs font-medium text-red-600 transition-colors hover:bg-red-50 dark:hover:bg-red-900/20"
            >
              <Trash2 className="h-3 w-3" strokeWidth={1.5} />
              Delete Selected
            </button>
          )}
        </div>
      )}

      {/* Table */}
      <div className="overflow-hidden rounded-lg border border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900">
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-gray-200 dark:border-gray-800 bg-gray-50 dark:bg-gray-900">
                {/* Checkbox column */}
                <th className="w-10 px-3 py-3">
                  <input
                    type="checkbox"
                    checked={allVisibleSelected}
                    onChange={toggleSelectAll}
                    className="h-4 w-4 rounded border-gray-300 accent-black dark:accent-white"
                  />
                </th>
                {sortColumns.map(({ key, label }) => (
                  <th
                    key={key}
                    className="px-5 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400"
                  >
                    <button
                      type="button"
                      onClick={() => handleSort(key)}
                      className="flex items-center gap-1 transition-colors hover:text-black dark:hover:text-white"
                    >
                      {label}
                      <ArrowUpDown className="h-3 w-3" strokeWidth={1.5} />
                      {store.padSort === key && (
                        <span className="text-black dark:text-white">
                          {store.padAscending ? '\u2191' : '\u2193'}
                        </span>
                      )}
                    </button>
                  </th>
                ))}
                <th className="px-5 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">
                  Actions
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100 dark:divide-gray-800">
              {store.pads.length === 0 ? (
                <tr>
                  <td
                    colSpan={6}
                    className="px-5 py-12 text-center text-sm text-gray-400 dark:text-gray-500"
                  >
                    No pads found.
                  </td>
                </tr>
              ) : (
                store.pads.map((pad) => (
                  <tr
                    key={pad.padName}
                    className="transition-colors hover:bg-gray-50 dark:hover:bg-gray-800"
                  >
                    {/* Checkbox */}
                    <td className="w-10 px-3 py-3">
                      <input
                        type="checkbox"
                        checked={selected.has(pad.padName)}
                        onChange={() => toggleSelect(pad.padName)}
                        className="h-4 w-4 rounded border-gray-300 accent-black dark:accent-white"
                      />
                    </td>
                    <td className="whitespace-nowrap px-5 py-3 font-medium text-black dark:text-white">
                      {pad.padName}
                    </td>
                    <td className="whitespace-nowrap px-5 py-3 text-gray-500 dark:text-gray-400">
                      {pad.userCount ?? 0}
                    </td>
                    <td className="whitespace-nowrap px-5 py-3 text-gray-500 dark:text-gray-400">
                      {formatLastEdited(pad.lastEdited)}
                    </td>
                    <td className="whitespace-nowrap px-5 py-3 text-gray-500 dark:text-gray-400">
                      {pad.revisionNumber ?? 0}
                    </td>
                    <td className="whitespace-nowrap px-5 py-3">
                      <div className="flex items-center gap-1">
                        {/* Preview button */}
                        <button
                          type="button"
                          onClick={() => handlePreview(pad.padName)}
                          title="Preview pad content"
                          className="rounded-md p-1.5 text-gray-400 transition-colors hover:bg-gray-100 dark:hover:bg-gray-800 hover:text-black dark:hover:text-white"
                        >
                          <Eye className="h-4 w-4" strokeWidth={1.5} />
                        </button>
                        <button
                          type="button"
                          onClick={() =>
                            window.open(`/p/${pad.padName}`, '_blank')
                          }
                          title="Open pad"
                          className="rounded-md p-1.5 text-gray-400 transition-colors hover:bg-gray-100 dark:hover:bg-gray-800 hover:text-black dark:hover:text-white"
                        >
                          <ExternalLink className="h-4 w-4" strokeWidth={1.5} />
                        </button>
                        {confirmClean === pad.padName ? (
                          <div className="flex items-center gap-1">
                            <button
                              type="button"
                              onClick={() => handleCleanPad(pad.padName)}
                              className="rounded-md border border-gray-200 dark:border-gray-700 px-2 py-1 text-xs font-medium text-black dark:text-white transition-colors hover:bg-gray-50 dark:hover:bg-gray-800"
                            >
                              Confirm
                            </button>
                            <button
                              type="button"
                              onClick={() => setConfirmClean(null)}
                              className="rounded-md p-1 text-gray-400 hover:text-black dark:hover:text-white"
                            >
                              <X className="h-3.5 w-3.5" strokeWidth={1.5} />
                            </button>
                          </div>
                        ) : (
                          <button
                            type="button"
                            onClick={() => setConfirmClean(pad.padName)}
                            title="Clean revisions"
                            className="rounded-md p-1.5 text-gray-400 transition-colors hover:bg-gray-100 dark:hover:bg-gray-800 hover:text-black dark:hover:text-white"
                          >
                            <RotateCcw className="h-4 w-4" strokeWidth={1.5} />
                          </button>
                        )}
                        {confirmDelete === pad.padName ? (
                          <div className="flex items-center gap-1">
                            <button
                              type="button"
                              onClick={() => handleDeletePad(pad.padName)}
                              className="rounded-md border border-red-200 dark:border-red-800 bg-red-600 px-2 py-1 text-xs font-medium text-white transition-colors hover:bg-red-700"
                            >
                              Confirm
                            </button>
                            <button
                              type="button"
                              onClick={() => setConfirmDelete(null)}
                              className="rounded-md p-1 text-gray-400 hover:text-black dark:hover:text-white"
                            >
                              <X className="h-3.5 w-3.5" strokeWidth={1.5} />
                            </button>
                          </div>
                        ) : (
                          <button
                            type="button"
                            onClick={() => setConfirmDelete(pad.padName)}
                            title="Delete pad"
                            className="rounded-md p-1.5 text-gray-400 transition-colors hover:bg-gray-100 dark:hover:bg-gray-800 hover:text-red-600"
                          >
                            <Trash2 className="h-4 w-4" strokeWidth={1.5} />
                          </button>
                        )}
                      </div>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>

        {/* Pagination */}
        <div className="flex items-center justify-between border-t border-gray-200 dark:border-gray-800 px-5 py-3">
          <button
            type="button"
            disabled={store.padOffset < PAD_LIMIT}
            onClick={handlePrev}
            className="flex items-center gap-1 rounded-lg px-3 py-1.5 text-sm font-medium text-gray-500 dark:text-gray-400 transition-colors hover:bg-gray-100 dark:hover:bg-gray-800 hover:text-black dark:hover:text-white disabled:cursor-not-allowed disabled:opacity-40"
          >
            <ChevronLeft className="h-4 w-4" strokeWidth={1.5} />
            Previous
          </button>
          <span className="text-sm text-gray-500 dark:text-gray-400">
            Page {currentPage} of {totalPages}
          </span>
          <button
            type="button"
            disabled={store.padOffset + PAD_LIMIT >= store.padsTotal}
            onClick={handleNext}
            className="flex items-center gap-1 rounded-lg px-3 py-1.5 text-sm font-medium text-gray-500 dark:text-gray-400 transition-colors hover:bg-gray-100 dark:hover:bg-gray-800 hover:text-black dark:hover:text-white disabled:cursor-not-allowed disabled:opacity-40"
          >
            Next
            <ChevronRight className="h-4 w-4" strokeWidth={1.5} />
          </button>
        </div>
      </div>

      {/* Create Pad Modal */}
      {showCreateModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
          <div className="w-full max-w-md rounded-lg border border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900 p-6">
            <div className="mb-4 flex items-center justify-between">
              <h3 className="text-lg font-semibold text-black dark:text-white">
                Create New Pad
              </h3>
              <button
                type="button"
                onClick={() => setShowCreateModal(false)}
                className="rounded-md p-1 text-gray-400 transition-colors hover:text-black dark:hover:text-white"
              >
                <X className="h-5 w-5" strokeWidth={1.5} />
              </button>
            </div>
            <input
              type="text"
              placeholder="Pad name"
              value={newPadName}
              onChange={(e) => setNewPadName(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleCreatePad()}
              autoFocus
              className="w-full rounded-lg border border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900 px-3 py-2 text-sm text-black dark:text-white placeholder-gray-400 dark:placeholder-gray-500 transition-colors focus:border-black dark:focus:border-white focus:outline-none"
            />
            <div className="mt-4 flex justify-end gap-2">
              <button
                type="button"
                onClick={() => setShowCreateModal(false)}
                className="rounded-lg border border-gray-200 dark:border-gray-700 px-4 py-2 text-sm font-medium text-gray-600 dark:text-gray-300 transition-colors hover:border-black dark:hover:border-white hover:text-black dark:hover:text-white"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={handleCreatePad}
                className="rounded-lg bg-black px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-800"
              >
                Create
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Preview Slide-over Panel */}
      {previewPad && (
        <>
          {/* Backdrop */}
          <div
            className="fixed inset-0 z-40 bg-black/30"
            onClick={closePreview}
          />
          {/* Panel */}
          <div className="fixed inset-y-0 right-0 z-50 flex w-[400px] flex-col border-l border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900 shadow-xl">
            <div className="flex items-center justify-between border-b border-gray-200 dark:border-gray-800 px-5 py-4">
              <h3 className="text-sm font-semibold text-black dark:text-white truncate">
                Preview: {previewPad}
              </h3>
              <button
                type="button"
                onClick={closePreview}
                className="rounded-md p-1 text-gray-400 transition-colors hover:text-black dark:hover:text-white"
              >
                <X className="h-5 w-5" strokeWidth={1.5} />
              </button>
            </div>
            <div className="flex-1 overflow-auto p-5">
              <textarea
                readOnly
                value={store.padPreview?.content ?? 'Loading...'}
                className="h-full w-full resize-none rounded-lg border border-gray-200 dark:border-gray-800 bg-gray-50 dark:bg-gray-950 p-3 font-mono text-xs text-black dark:text-white focus:outline-none"
              />
            </div>
          </div>
        </>
      )}
    </div>
  )
}

// -- Helpers ------------------------------------------------------------------

function formatLastEdited(ts: number | undefined): string {
  if (!ts) return 'Never'
  const d = new Date(ts)
  if (isNaN(d.getTime())) return 'Unknown'

  const now = Date.now()
  const diff = now - d.getTime()

  if (diff < 60_000) return 'Just now'
  if (diff < 3_600_000) return `${Math.floor(diff / 60_000)}m ago`
  if (diff < 86_400_000) return `${Math.floor(diff / 3_600_000)}h ago`

  const pad = (n: number) => String(n).padStart(2, '0')
  return `${pad(d.getDate())}.${pad(d.getMonth() + 1)}.${d.getFullYear()}`
}

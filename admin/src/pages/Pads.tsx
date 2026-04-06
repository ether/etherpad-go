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
    doRequest()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const handleSort = (key: string) => {
    if (store.padSort === key) {
      store.setPadAscending(!store.padAscending)
    } else {
      store.setPadSort(key)
      store.setPadAscending(true)
    }
    // Trigger after state update via effect
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

  const totalPages = store.padsTotal > 0 ? Math.ceil(store.padsTotal / PAD_LIMIT) : 1
  const currentPage = Math.floor(store.padOffset / PAD_LIMIT) + 1

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
        <h2 className="text-2xl font-bold text-gray-900 dark:text-white">
          Pads
        </h2>
        <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
          Manage and inspect collaborative pads
        </p>
      </div>

      {/* Toolbar */}
      <div className="mb-6 flex flex-col gap-3 sm:flex-row sm:items-center">
        <div className="flex flex-1 gap-2">
          <input
            type="text"
            placeholder="Search pads..."
            value={store.padSearch}
            onChange={(e) => store.setPadSearch(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && handleSearch()}
            className="flex-1 rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm text-gray-900 placeholder-gray-400 shadow-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none dark:border-gray-700 dark:bg-gray-900 dark:text-white dark:placeholder-gray-500"
          />
          <button
            type="button"
            onClick={handleSearch}
            className="rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white shadow-sm hover:bg-blue-700 transition-colors"
          >
            Apply
          </button>
        </div>
        <button
          type="button"
          onClick={() => setShowCreateModal(true)}
          className="flex items-center gap-2 rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 shadow-sm hover:bg-gray-50 transition-colors dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200 dark:hover:bg-gray-700"
        >
          <Plus className="h-4 w-4" />
          Create Pad
        </button>
      </div>

      {/* Table */}
      <div className="overflow-hidden rounded-xl border border-gray-200 bg-white shadow-sm dark:border-gray-800 dark:bg-gray-900">
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-gray-100 bg-gray-50 dark:border-gray-800 dark:bg-gray-900/50">
                {sortColumns.map(({ key, label }) => (
                  <th
                    key={key}
                    className="px-6 py-3 text-left text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400"
                  >
                    <button
                      type="button"
                      onClick={() => handleSort(key)}
                      className="flex items-center gap-1 hover:text-gray-900 dark:hover:text-white transition-colors"
                    >
                      {label}
                      <ArrowUpDown className="h-3.5 w-3.5" />
                      {store.padSort === key && (
                        <span className="text-blue-600 dark:text-blue-400">
                          {store.padAscending ? '\u2191' : '\u2193'}
                        </span>
                      )}
                    </button>
                  </th>
                ))}
                <th className="px-6 py-3 text-left text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400">
                  Actions
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100 dark:divide-gray-800">
              {store.pads.length === 0 ? (
                <tr>
                  <td
                    colSpan={5}
                    className="px-6 py-12 text-center text-sm text-gray-400"
                  >
                    No pads found.
                  </td>
                </tr>
              ) : (
                store.pads.map((pad) => (
                  <tr
                    key={pad.padName}
                    className="hover:bg-gray-50 dark:hover:bg-gray-800/50 transition-colors"
                  >
                    <td className="whitespace-nowrap px-6 py-3 font-medium text-gray-900 dark:text-white">
                      {pad.padName}
                    </td>
                    <td className="whitespace-nowrap px-6 py-3 text-gray-600 dark:text-gray-300">
                      {pad.userCount ?? 0}
                    </td>
                    <td className="whitespace-nowrap px-6 py-3 text-gray-600 dark:text-gray-300">
                      {formatLastEdited(pad.lastEdited)}
                    </td>
                    <td className="whitespace-nowrap px-6 py-3 text-gray-600 dark:text-gray-300">
                      {pad.revisionNumber ?? 0}
                    </td>
                    <td className="whitespace-nowrap px-6 py-3">
                      <div className="flex items-center gap-2">
                        <button
                          type="button"
                          onClick={() =>
                            window.open(`/p/${pad.padName}`, '_blank')
                          }
                          title="Open pad"
                          className="rounded-md p-1.5 text-gray-400 hover:bg-blue-50 hover:text-blue-600 transition-colors dark:hover:bg-blue-900/30"
                        >
                          <ExternalLink className="h-4 w-4" />
                        </button>
                        {confirmClean === pad.padName ? (
                          <div className="flex items-center gap-1">
                            <button
                              type="button"
                              onClick={() => handleCleanPad(pad.padName)}
                              className="rounded-md bg-yellow-100 px-2 py-1 text-xs font-medium text-yellow-800 hover:bg-yellow-200 transition-colors dark:bg-yellow-900/40 dark:text-yellow-300"
                            >
                              Confirm
                            </button>
                            <button
                              type="button"
                              onClick={() => setConfirmClean(null)}
                              className="rounded-md p-1 text-gray-400 hover:text-gray-600"
                            >
                              <X className="h-3.5 w-3.5" />
                            </button>
                          </div>
                        ) : (
                          <button
                            type="button"
                            onClick={() => setConfirmClean(pad.padName)}
                            title="Clean revisions"
                            className="rounded-md p-1.5 text-gray-400 hover:bg-yellow-50 hover:text-yellow-600 transition-colors dark:hover:bg-yellow-900/30"
                          >
                            <RotateCcw className="h-4 w-4" />
                          </button>
                        )}
                        {confirmDelete === pad.padName ? (
                          <div className="flex items-center gap-1">
                            <button
                              type="button"
                              onClick={() => handleDeletePad(pad.padName)}
                              className="rounded-md bg-red-100 px-2 py-1 text-xs font-medium text-red-800 hover:bg-red-200 transition-colors dark:bg-red-900/40 dark:text-red-300"
                            >
                              Confirm
                            </button>
                            <button
                              type="button"
                              onClick={() => setConfirmDelete(null)}
                              className="rounded-md p-1 text-gray-400 hover:text-gray-600"
                            >
                              <X className="h-3.5 w-3.5" />
                            </button>
                          </div>
                        ) : (
                          <button
                            type="button"
                            onClick={() => setConfirmDelete(pad.padName)}
                            title="Delete pad"
                            className="rounded-md p-1.5 text-gray-400 hover:bg-red-50 hover:text-red-600 transition-colors dark:hover:bg-red-900/30"
                          >
                            <Trash2 className="h-4 w-4" />
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
        <div className="flex items-center justify-between border-t border-gray-100 px-6 py-3 dark:border-gray-800">
          <button
            type="button"
            disabled={store.padOffset < PAD_LIMIT}
            onClick={handlePrev}
            className="flex items-center gap-1 rounded-lg px-3 py-1.5 text-sm font-medium text-gray-600 hover:bg-gray-100 disabled:opacity-40 disabled:cursor-not-allowed transition-colors dark:text-gray-300 dark:hover:bg-gray-800"
          >
            <ChevronLeft className="h-4 w-4" />
            Previous
          </button>
          <span className="text-sm text-gray-500 dark:text-gray-400">
            Page {currentPage} of {totalPages}
          </span>
          <button
            type="button"
            disabled={store.padOffset + PAD_LIMIT >= store.padsTotal}
            onClick={handleNext}
            className="flex items-center gap-1 rounded-lg px-3 py-1.5 text-sm font-medium text-gray-600 hover:bg-gray-100 disabled:opacity-40 disabled:cursor-not-allowed transition-colors dark:text-gray-300 dark:hover:bg-gray-800"
          >
            Next
            <ChevronRight className="h-4 w-4" />
          </button>
        </div>
      </div>

      {/* Create Pad Modal */}
      {showCreateModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
          <div className="w-full max-w-md rounded-xl bg-white p-6 shadow-xl dark:bg-gray-900">
            <div className="mb-4 flex items-center justify-between">
              <h3 className="text-lg font-semibold text-gray-900 dark:text-white">
                Create New Pad
              </h3>
              <button
                type="button"
                onClick={() => setShowCreateModal(false)}
                className="rounded-md p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
              >
                <X className="h-5 w-5" />
              </button>
            </div>
            <input
              type="text"
              placeholder="Pad name"
              value={newPadName}
              onChange={(e) => setNewPadName(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleCreatePad()}
              autoFocus
              className="w-full rounded-lg border border-gray-300 px-4 py-2 text-sm text-gray-900 placeholder-gray-400 focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none dark:border-gray-700 dark:bg-gray-800 dark:text-white dark:placeholder-gray-500"
            />
            <div className="mt-4 flex justify-end gap-2">
              <button
                type="button"
                onClick={() => setShowCreateModal(false)}
                className="rounded-lg px-4 py-2 text-sm font-medium text-gray-600 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-gray-800"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={handleCreatePad}
                className="rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 transition-colors"
              >
                Create
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

// ── Helpers ──────────────────────────────────────────────────────────────────

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

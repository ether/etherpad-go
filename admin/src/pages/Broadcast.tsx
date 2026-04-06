import { Send } from 'lucide-react'
import { useAdminStore } from '@/store'
import { useAdminActions } from '@/hooks/useAdminActions'

export function BroadcastPage() {
  const store = useAdminStore()
  const actions = useAdminActions()

  const handleSend = () => {
    const msg = store.shoutMessage.trim()
    if (!msg) return
    actions.sendBroadcast(msg, store.shoutSticky)
  }

  return (
    <div className="mx-auto max-w-4xl p-6 lg:p-8">
      {/* Header */}
      <div className="mb-8 flex items-center gap-3">
        <div>
          <h2 className="text-2xl font-bold text-gray-900 dark:text-white">
            Broadcast
          </h2>
          <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
            Send messages to all connected users
          </p>
        </div>
        <span className="ml-auto rounded-full bg-blue-100 px-3 py-1 text-xs font-semibold text-blue-700 dark:bg-blue-900/40 dark:text-blue-300">
          {store.totalUsers} users online
        </span>
      </div>

      {/* Compose */}
      <div className="mb-8 rounded-xl border border-gray-200 bg-white p-6 shadow-sm dark:border-gray-800 dark:bg-gray-900">
        <textarea
          value={store.shoutMessage}
          onChange={(e) => store.setShoutMessage(e.target.value)}
          placeholder="Message for all connected users..."
          rows={4}
          className="w-full resize-none rounded-lg border border-gray-300 bg-white px-4 py-3 text-sm text-gray-900 placeholder-gray-400 focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none dark:border-gray-700 dark:bg-gray-800 dark:text-white dark:placeholder-gray-500"
        />
        <div className="mt-4 flex items-center justify-between">
          <label className="flex items-center gap-2 text-sm text-gray-600 dark:text-gray-300 cursor-pointer select-none">
            <input
              type="checkbox"
              checked={store.shoutSticky}
              onChange={(e) => store.setShoutSticky(e.target.checked)}
              className="h-4 w-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
            />
            Sticky message
          </label>
          <button
            type="button"
            onClick={handleSend}
            disabled={!store.shoutMessage.trim()}
            className="flex items-center gap-2 rounded-lg bg-blue-600 px-5 py-2 text-sm font-medium text-white shadow-sm hover:bg-blue-700 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
          >
            <Send className="h-4 w-4" />
            Send
          </button>
        </div>
      </div>

      {/* Message History */}
      <h3 className="mb-4 text-base font-semibold text-gray-900 dark:text-white">
        Message History
      </h3>
      {store.shouts.length === 0 ? (
        <div className="rounded-xl border border-dashed border-gray-300 py-12 text-center dark:border-gray-700">
          <p className="text-sm text-gray-400">No messages sent yet.</p>
        </div>
      ) : (
        <div className="space-y-3">
          {store.shouts.map((shout, i) => (
            <div
              key={i}
              className="rounded-xl border border-gray-200 bg-white px-5 py-4 shadow-sm dark:border-gray-800 dark:bg-gray-900"
            >
              <div className="flex items-start justify-between gap-3">
                <p className="text-sm text-gray-900 dark:text-white">
                  {shout.message}
                </p>
                {shout.sticky && (
                  <span className="shrink-0 rounded-full bg-blue-100 px-2 py-0.5 text-xs font-medium text-blue-700 dark:bg-blue-900/40 dark:text-blue-400">
                    sticky
                  </span>
                )}
              </div>
              <p className="mt-1 text-xs text-gray-400">
                {formatTimestamp(shout.timestamp)}
              </p>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

function formatTimestamp(d: Date): string {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${pad(d.getDate())}.${pad(d.getMonth() + 1)}.${d.getFullYear()} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

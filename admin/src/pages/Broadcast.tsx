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
          <h2 className="text-2xl font-semibold text-black dark:text-white">Broadcast</h2>
          <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
            Send messages to all connected users
          </p>
        </div>
        <span className="ml-auto rounded-full border border-gray-200 dark:border-gray-700 px-3 py-1 text-xs font-medium text-gray-500 dark:text-gray-400">
          {store.totalUsers} users online
        </span>
      </div>

      {/* Compose */}
      <div className="mb-8 rounded-lg border border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900 p-5">
        <textarea
          value={store.shoutMessage}
          onChange={(e) => store.setShoutMessage(e.target.value)}
          placeholder="Message for all connected users..."
          rows={4}
          className="w-full resize-none rounded-lg border border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900 px-3 py-3 text-sm text-black dark:text-white placeholder-gray-400 dark:placeholder-gray-500 transition-colors focus:border-black dark:focus:border-white focus:outline-none"
        />
        <div className="mt-4 flex items-center justify-between">
          <label className="flex cursor-pointer select-none items-center gap-2 text-sm text-gray-500 dark:text-gray-400">
            <input
              type="checkbox"
              checked={store.shoutSticky}
              onChange={(e) => store.setShoutSticky(e.target.checked)}
              className="h-4 w-4 rounded border-gray-300 dark:border-gray-600 text-black focus:ring-black"
            />
            Sticky message
          </label>
          <button
            type="button"
            onClick={handleSend}
            disabled={!store.shoutMessage.trim()}
            className="flex items-center gap-2 rounded-lg bg-black px-5 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-800 disabled:cursor-not-allowed disabled:opacity-40"
          >
            <Send className="h-4 w-4" strokeWidth={1.5} />
            Send
          </button>
        </div>
      </div>

      {/* Message History */}
      <h3 className="mb-4 text-sm font-semibold text-black dark:text-white">
        Message History
      </h3>
      {store.shouts.length === 0 ? (
        <div className="rounded-lg border border-dashed border-gray-200 dark:border-gray-700 py-12 text-center">
          <p className="text-sm text-gray-400 dark:text-gray-500">No messages sent yet.</p>
        </div>
      ) : (
        <div className="space-y-3">
          {store.shouts.map((shout, i) => (
            <div
              key={i}
              className="rounded-lg border border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900 px-5 py-4"
            >
              <div className="flex items-start justify-between gap-3">
                <p className="text-sm text-black dark:text-white">{shout.message}</p>
                {shout.sticky && (
                  <span className="shrink-0 rounded-full border border-gray-200 dark:border-gray-700 px-2 py-0.5 text-xs font-medium text-gray-500 dark:text-gray-400">
                    sticky
                  </span>
                )}
              </div>
              <p className="mt-1 text-xs text-gray-400 dark:text-gray-500">
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

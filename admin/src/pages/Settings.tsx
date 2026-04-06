import { useEffect } from 'react'
import { RefreshCw, Save, RotateCcw } from 'lucide-react'
import { useAdminStore } from '@/store'
import { useAdminActions } from '@/hooks/useAdminActions'

export function SettingsPage() {
  const store = useAdminStore()
  const actions = useAdminActions()

  useEffect(() => {
    actions.loadSettings()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const handleSave = () => {
    actions.saveSettings(store.settings)
    store.setToast({ kind: 'success', message: 'Settings saved.' })
  }

  const handleReload = () => {
    actions.loadSettings()
  }

  const handleRestart = () => {
    actions.restartServer()
    store.setToast({ kind: 'success', message: 'Restart signal sent.' })
  }

  return (
    <div className="mx-auto max-w-5xl p-6 lg:p-8">
      {/* Header */}
      <div className="mb-8">
        <h2 className="text-2xl font-bold text-gray-900 dark:text-white">
          Settings
        </h2>
        <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
          View and edit the server configuration
        </p>
      </div>

      {/* Editor panel */}
      <div className="rounded-xl border border-gray-200 bg-white shadow-sm dark:border-gray-800 dark:bg-gray-900">
        {/* Toolbar */}
        <div className="flex items-center justify-between border-b border-gray-100 px-6 py-4 dark:border-gray-800">
          <h3 className="text-base font-semibold text-gray-900 dark:text-white">
            settings.json
          </h3>
          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={handleReload}
              className="flex items-center gap-1.5 rounded-lg border border-gray-300 px-3 py-1.5 text-sm font-medium text-gray-700 hover:bg-gray-50 transition-colors dark:border-gray-700 dark:text-gray-200 dark:hover:bg-gray-800"
            >
              <RefreshCw className="h-3.5 w-3.5" />
              Reload
            </button>
            <button
              type="button"
              onClick={handleSave}
              className="flex items-center gap-1.5 rounded-lg bg-blue-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-blue-700 transition-colors"
            >
              <Save className="h-3.5 w-3.5" />
              Save
            </button>
            <button
              type="button"
              onClick={handleRestart}
              className="flex items-center gap-1.5 rounded-lg border border-gray-300 px-3 py-1.5 text-sm font-medium text-gray-700 hover:bg-gray-50 transition-colors dark:border-gray-700 dark:text-gray-200 dark:hover:bg-gray-800"
            >
              <RotateCcw className="h-3.5 w-3.5" />
              Restart
            </button>
          </div>
        </div>

        {/* Editor */}
        <div className="p-6">
          <textarea
            value={store.settings}
            onChange={(e) =>
              store.dispatch({ type: 'SET_SETTINGS', payload: e.target.value })
            }
            spellCheck={false}
            className="h-[60vh] w-full resize-y rounded-lg border border-gray-300 bg-gray-50 px-4 py-3 font-mono text-sm text-gray-900 focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100"
          />
          <p className="mt-3 text-xs text-gray-400">
            Changes are sent directly to the server.
          </p>
        </div>
      </div>
    </div>
  )
}

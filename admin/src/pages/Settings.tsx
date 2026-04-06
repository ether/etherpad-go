import { useEffect, useMemo, useState, useCallback } from 'react'
import { RefreshCw, Save, RotateCcw, Code } from 'lucide-react'
import { useAdminStore } from '@/store'
import { useAdminActions } from '@/hooks/useAdminActions'

// ── Tab definitions ────────────────────────────────────────────────────────
const tabs = [
  { id: 'general', label: 'General' },
  { id: 'pad', label: 'Pad Options' },
  { id: 'database', label: 'Database' },
  { id: 'security', label: 'Security' },
  { id: 'sso', label: 'SSO / Auth' },
  { id: 'plugins', label: 'Plugins' },
  { id: 'advanced', label: 'Advanced' },
  { id: 'json', label: 'JSON' },
] as const

type TabId = (typeof tabs)[number]['id']

// ── Field types ────────────────────────────────────────────────────────────
type FieldDef = {
  key: string
  label: string
  type: 'text' | 'number' | 'boolean' | 'select' | 'textarea'
  options?: string[]
  description?: string
}

const generalFields: FieldDef[] = [
  { key: 'title', label: 'Title', type: 'text', description: 'Instance name shown in the browser tab' },
  { key: 'ip', label: 'IP', type: 'text', description: 'IP address to bind to' },
  { key: 'port', label: 'Port', type: 'text', description: 'Port to listen on' },
  { key: 'skinName', label: 'Skin', type: 'text', description: 'UI skin name' },
  { key: 'skinVariants', label: 'Skin Variants', type: 'text' },
  { key: 'enableDarkMode', label: 'Dark Mode', type: 'boolean' },
  { key: 'showRecentPads', label: 'Show Recent Pads', type: 'boolean' },
  { key: 'loglevel', label: 'Log Level', type: 'select', options: ['DEBUG', 'INFO', 'WARN', 'ERROR'] },
  { key: 'defaultPadText', label: 'Default Pad Text', type: 'textarea' },
]

const padFields: FieldDef[] = [
  { key: 'padOptions.ShowControls', label: 'Show Controls', type: 'boolean' },
  { key: 'padOptions.ShowChat', label: 'Show Chat', type: 'boolean' },
  { key: 'padOptions.ShowLineNumbers', label: 'Show Line Numbers', type: 'boolean' },
  { key: 'padOptions.UseMonospaceFont', label: 'Use Monospace Font', type: 'boolean' },
  { key: 'padOptions.NoColors', label: 'No Colors', type: 'boolean' },
  { key: 'padOptions.RTL', label: 'Right to Left', type: 'boolean' },
  { key: 'padOptions.AlwaysShowChat', label: 'Always Show Chat', type: 'boolean' },
  { key: 'padOptions.ChatAndUsers', label: 'Chat And Users', type: 'boolean' },
  { key: 'padOptions.Lang', label: 'Language', type: 'text' },
  { key: 'indentationOnNewLine', label: 'Indent on New Line', type: 'boolean' },
]

const dbFields: FieldDef[] = [
  { key: 'dbType', label: 'Database Type', type: 'select', options: ['memory', 'sqlite', 'postgres', 'mysql'], description: 'Requires restart' },
  { key: 'dbSettings.Host', label: 'Host', type: 'text' },
  { key: 'dbSettings.Port', label: 'Port', type: 'text' },
  { key: 'dbSettings.Database', label: 'Database', type: 'text' },
  { key: 'dbSettings.User', label: 'User', type: 'text' },
  { key: 'dbSettings.Password', label: 'Password', type: 'text' },
  { key: 'dbSettings.Filename', label: 'Filename (SQLite)', type: 'text' },
  { key: 'dbSettings.Charset', label: 'Charset', type: 'text' },
]

const securityFields: FieldDef[] = [
  { key: 'requireSession', label: 'Require Session', type: 'boolean' },
  { key: 'requireAuthentication', label: 'Require Authentication', type: 'boolean' },
  { key: 'requireAuthorization', label: 'Require Authorization', type: 'boolean' },
  { key: 'editOnly', label: 'Edit Only', type: 'boolean' },
  { key: 'trustProxy', label: 'Trust Proxy', type: 'boolean' },
  { key: 'disableIPLogging', label: 'Disable IP Logging', type: 'boolean' },
  { key: 'cookie.sameSite', label: 'Cookie SameSite', type: 'select', options: ['Lax', 'Strict', 'None'] },
  { key: 'cookie.sessionLifetime', label: 'Session Lifetime (ms)', type: 'number' },
  { key: 'cookie.sessionRefreshInterval', label: 'Session Refresh (ms)', type: 'number' },
]

const advancedFields: FieldDef[] = [
  { key: 'minify', label: 'Minify', type: 'boolean' },
  { key: 'maxAge', label: 'Max Cache Age (s)', type: 'number' },
  { key: 'loadTest', label: 'Load Test Mode', type: 'boolean' },
  { key: 'exposeVersion', label: 'Expose Version', type: 'boolean' },
  { key: 'lowerCasePadIds', label: 'Lowercase Pad IDs', type: 'boolean' },
  { key: 'suppressErrorsInPadText', label: 'Suppress Errors in Pad', type: 'boolean' },
  { key: 'enableMetrics', label: 'Enable Metrics', type: 'boolean' },
  { key: 'dumpOnUncleanExit', label: 'Dump on Unclean Exit', type: 'boolean' },
  { key: 'automaticReconnectionTimeout', label: 'Auto Reconnect Timeout (s)', type: 'number' },
  { key: 'importMaxFileSize', label: 'Import Max File Size (bytes)', type: 'number' },
  { key: 'socketIo.maxHttpBufferSize', label: 'Socket Max Buffer (bytes)', type: 'number' },
  { key: 'commitRateLimiting.duration', label: 'Rate Limit Duration (s)', type: 'number' },
  { key: 'commitRateLimiting.points', label: 'Rate Limit Points', type: 'number' },
  { key: 'cleanup.enabled', label: 'Revision Cleanup', type: 'boolean' },
  { key: 'cleanup.keepRevisions', label: 'Keep Revisions', type: 'number' },
]

// ── Nested object helpers ──────────────────────────────────────────────────
function getNestedValue(obj: any, path: string): any {
  return path.split('.').reduce((o, k) => o?.[k], obj)
}

function setNestedValue(obj: any, path: string, value: any): any {
  const clone = JSON.parse(JSON.stringify(obj))
  const keys = path.split('.')
  let target = clone
  for (let i = 0; i < keys.length - 1; i++) {
    if (target[keys[i]] === undefined) target[keys[i]] = {}
    target = target[keys[i]]
  }
  target[keys[keys.length - 1]] = value
  return clone
}

// ── Component ──────────────────────────────────────────────────────────────
export function SettingsPage() {
  const store = useAdminStore()
  const actions = useAdminActions()
  const [activeTab, setActiveTab] = useState<TabId>('general')

  useEffect(() => { actions.loadSettings() }, [])

  const parsed = useMemo(() => {
    try { return JSON.parse(store.settings) } catch { return null }
  }, [store.settings])

  const updateField = useCallback((key: string, value: any) => {
    if (!parsed) return
    const updated = setNestedValue(parsed, key, value)
    store.dispatch({ type: 'SET_SETTINGS', payload: JSON.stringify(updated, null, 2) })
  }, [parsed, store])

  const handleSave = () => {
    actions.saveSettings(store.settings)
    store.setToast({ kind: 'success', message: 'Settings saved.' })
  }

  const renderField = (f: FieldDef) => {
    const value = parsed ? getNestedValue(parsed, f.key) : ''
    return (
      <div key={f.key} className="flex flex-col gap-1.5 sm:flex-row sm:items-center sm:gap-4">
        <label className="w-48 shrink-0 text-sm font-medium text-gray-700 dark:text-gray-300">
          {f.label}
        </label>
        <div className="flex-1">
          {f.type === 'boolean' ? (
            <button
              type="button"
              onClick={() => updateField(f.key, !value)}
              className={`relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors ${value ? 'bg-black dark:bg-white' : 'bg-gray-200 dark:bg-gray-700'}`}
            >
              <span className={`pointer-events-none inline-block h-5 w-5 rounded-full bg-white dark:bg-gray-900 shadow transition-transform ${value ? 'translate-x-5' : 'translate-x-0'}`} />
            </button>
          ) : f.type === 'select' ? (
            <select
              value={value ?? ''}
              onChange={(e) => updateField(f.key, e.target.value)}
              className="w-full rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-900 px-3 py-2 text-sm text-black dark:text-white focus:border-black dark:focus:border-white focus:outline-none"
            >
              {f.options?.map((o) => <option key={o} value={o}>{o}</option>)}
            </select>
          ) : f.type === 'textarea' ? (
            <textarea
              value={value ?? ''}
              onChange={(e) => updateField(f.key, e.target.value)}
              rows={3}
              className="w-full resize-y rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-900 px-3 py-2 text-sm text-black dark:text-white focus:border-black dark:focus:border-white focus:outline-none"
            />
          ) : f.type === 'number' ? (
            <input
              type="number"
              value={value ?? ''}
              onChange={(e) => updateField(f.key, e.target.value === '' ? 0 : Number(e.target.value))}
              className="w-full rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-900 px-3 py-2 text-sm text-black dark:text-white focus:border-black dark:focus:border-white focus:outline-none"
            />
          ) : (
            <input
              type="text"
              value={value ?? ''}
              onChange={(e) => updateField(f.key, e.target.value)}
              className="w-full rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-900 px-3 py-2 text-sm text-black dark:text-white focus:border-black dark:focus:border-white focus:outline-none"
            />
          )}
          {f.description && <p className="mt-1 text-xs text-gray-400 dark:text-gray-500">{f.description}</p>}
        </div>
      </div>
    )
  }

  const renderFieldGroup = (fields: FieldDef[]) => (
    <div className="space-y-5">{fields.map(renderField)}</div>
  )

  const renderSSOTab = () => {
    if (!parsed) return null
    const sso = parsed.sso ?? {}
    const authMethod = parsed.authenticationMethod ?? 'none'
    return (
      <div className="space-y-5">
        <div className="flex flex-col gap-1.5 sm:flex-row sm:items-center sm:gap-4">
          <label className="w-48 shrink-0 text-sm font-medium text-gray-700 dark:text-gray-300">Auth Method</label>
          <select
            value={authMethod}
            onChange={(e) => updateField('authenticationMethod', e.target.value)}
            className="w-full rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-900 px-3 py-2 text-sm text-black dark:text-white focus:border-black dark:focus:border-white focus:outline-none"
          >
            <option value="sso">SSO (OAuth/OIDC)</option>
            <option value="apikey">API Key</option>
          </select>
        </div>
        <div className="flex flex-col gap-1.5 sm:flex-row sm:items-center sm:gap-4">
          <label className="w-48 shrink-0 text-sm font-medium text-gray-700 dark:text-gray-300">SSO Issuer</label>
          <input
            type="text"
            value={sso.issuer ?? ''}
            onChange={(e) => updateField('sso.issuer', e.target.value)}
            className="w-full rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-900 px-3 py-2 text-sm text-black dark:text-white focus:border-black dark:focus:border-white focus:outline-none"
          />
        </div>
        {(sso.clients ?? []).map((_: any, i: number) => (
          <div key={i} className="rounded-lg border border-gray-200 dark:border-gray-700 p-4">
            <p className="mb-3 text-xs font-semibold uppercase tracking-wider text-gray-400 dark:text-gray-500">Client {i + 1}</p>
            <div className="space-y-3">
              {['client_id', 'client_secret', 'type'].map((field) => (
                <div key={field} className="flex flex-col gap-1.5 sm:flex-row sm:items-center sm:gap-4">
                  <label className="w-40 shrink-0 text-sm text-gray-600 dark:text-gray-400">{field}</label>
                  <input
                    type="text"
                    value={getNestedValue(parsed, `sso.clients.${i}.${field}`) ?? ''}
                    onChange={(e) => updateField(`sso.clients.${i}.${field}`, e.target.value)}
                    className="w-full rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-900 px-3 py-2 text-sm text-black dark:text-white focus:border-black dark:focus:border-white focus:outline-none"
                  />
                </div>
              ))}
            </div>
          </div>
        ))}
      </div>
    )
  }

  const renderPluginsTab = () => {
    if (!parsed?.plugins) return <p className="text-sm text-gray-400">No plugins configured.</p>
    const plugins = parsed.plugins as Record<string, { enabled: boolean }>
    return (
      <div className="space-y-2">
        {Object.entries(plugins).sort(([a], [b]) => a.localeCompare(b)).map(([name, config]) => (
          <div key={name} className="flex items-center justify-between rounded-lg border border-gray-200 dark:border-gray-700 px-4 py-3">
            <span className="text-sm font-medium text-black dark:text-white">{name}</span>
            <button
              type="button"
              onClick={() => updateField(`plugins.${name}.enabled`, !config.enabled)}
              className={`relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors ${config.enabled ? 'bg-black dark:bg-white' : 'bg-gray-200 dark:bg-gray-700'}`}
            >
              <span className={`pointer-events-none inline-block h-5 w-5 rounded-full bg-white dark:bg-gray-900 shadow transition-transform ${config.enabled ? 'translate-x-5' : 'translate-x-0'}`} />
            </button>
          </div>
        ))}
      </div>
    )
  }

  const renderTabContent = () => {
    switch (activeTab) {
      case 'general': return renderFieldGroup(generalFields)
      case 'pad': return renderFieldGroup(padFields)
      case 'database': return renderFieldGroup(dbFields)
      case 'security': return renderFieldGroup(securityFields)
      case 'sso': return renderSSOTab()
      case 'plugins': return renderPluginsTab()
      case 'advanced': return renderFieldGroup(advancedFields)
      case 'json':
        return (
          <textarea
            value={store.settings}
            onChange={(e) => store.dispatch({ type: 'SET_SETTINGS', payload: e.target.value })}
            spellCheck={false}
            className="h-[60vh] w-full resize-y rounded-lg border border-gray-200 dark:border-gray-800 bg-gray-50 dark:bg-gray-950 px-4 py-3 font-mono text-sm text-black dark:text-white transition-colors focus:border-black dark:focus:border-white focus:outline-none"
          />
        )
    }
  }

  return (
    <div className="mx-auto max-w-5xl p-6 lg:p-8">
      {/* Header + actions */}
      <div className="mb-6 flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-semibold text-black dark:text-white">Settings</h2>
          <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">View and edit the server configuration</p>
        </div>
        <div className="flex items-center gap-2">
          <button type="button" onClick={() => actions.loadSettings()} className="flex items-center gap-1.5 rounded-lg border border-gray-200 dark:border-gray-700 px-3 py-1.5 text-sm font-medium text-gray-600 dark:text-gray-300 transition-colors hover:border-black dark:hover:border-white hover:text-black dark:hover:text-white">
            <RefreshCw className="h-3.5 w-3.5" strokeWidth={1.5} /> Reload
          </button>
          <button type="button" onClick={handleSave} className="flex items-center gap-1.5 rounded-lg bg-black px-3 py-1.5 text-sm font-medium text-white transition-colors hover:bg-gray-800">
            <Save className="h-3.5 w-3.5" strokeWidth={1.5} /> Save
          </button>
          <button type="button" onClick={() => { actions.restartServer(); store.setToast({ kind: 'success', message: 'Restart signal sent.' }) }} className="flex items-center gap-1.5 rounded-lg border border-gray-200 dark:border-gray-700 px-3 py-1.5 text-sm font-medium text-gray-600 dark:text-gray-300 transition-colors hover:border-black dark:hover:border-white hover:text-black dark:hover:text-white">
            <RotateCcw className="h-3.5 w-3.5" strokeWidth={1.5} /> Restart
          </button>
        </div>
      </div>

      {/* Tabs */}
      <div className="mb-6 flex gap-1 overflow-x-auto rounded-lg border border-gray-200 dark:border-gray-800 bg-gray-50 dark:bg-gray-900 p-1">
        {tabs.map((tab) => (
          <button
            key={tab.id}
            onClick={() => setActiveTab(tab.id)}
            className={`flex shrink-0 items-center gap-1.5 rounded-md px-3 py-1.5 text-sm font-medium transition-colors ${
              activeTab === tab.id
                ? 'bg-white dark:bg-gray-800 text-black dark:text-white shadow-sm'
                : 'text-gray-500 dark:text-gray-400 hover:text-black dark:hover:text-white'
            }`}
          >
            {tab.id === 'json' && <Code className="h-3.5 w-3.5" strokeWidth={1.5} />}
            {tab.label}
          </button>
        ))}
      </div>

      {/* Content */}
      <div className="rounded-lg border border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900 p-6">
        {parsed ? renderTabContent() : (
          <p className="text-sm text-gray-400 dark:text-gray-500">Loading settings...</p>
        )}
      </div>
    </div>
  )
}

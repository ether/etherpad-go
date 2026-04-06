import { Routes, Route, Navigate } from 'react-router-dom'
import { AdminProvider } from '@/store'
import { Sidebar } from '@/components/Sidebar'
import { Toast } from '@/components/Toast'
import { OverviewPage } from '@/pages/Overview'
import { PadsPage } from '@/pages/Pads'
import { BroadcastPage } from '@/pages/Broadcast'
import { SettingsPage } from '@/pages/Settings'
import { MonitoringPage } from '@/pages/Monitoring'
import { AdminSocketProvider } from '@/hooks/useAdminSocket'
import { useAuth } from '@/hooks/useAuth'

function AuthenticatedApp({ token }: { token: string }) {
  return (
    <AdminSocketProvider token={token}>
      <div className="flex h-screen bg-gray-50 dark:bg-gray-950">
        <Sidebar />
        <main className="flex-1 overflow-auto">
          <Routes>
            <Route path="/" element={<OverviewPage />} />
            <Route path="/monitoring" element={<MonitoringPage />} />
            <Route path="/pads" element={<PadsPage />} />
            <Route path="/broadcast" element={<BroadcastPage />} />
            <Route path="/settings" element={<SettingsPage />} />
            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </main>
        <Toast />
      </div>
    </AdminSocketProvider>
  )
}

export default function App() {
  const { token, loading, error } = useAuth()

  if (loading) {
    return (
      <div className="flex h-screen items-center justify-center bg-gray-50 dark:bg-gray-950">
        <div className="text-gray-500 dark:text-gray-400 text-lg">Authenticating...</div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="flex h-screen items-center justify-center bg-gray-50 dark:bg-gray-950">
        <div className="text-center">
          <div className="text-red-500 text-lg font-medium mb-2">Authentication Error</div>
          <div className="text-gray-500 dark:text-gray-400">{error}</div>
          <button
            onClick={() => { sessionStorage.clear(); window.location.reload() }}
            className="mt-4 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700"
          >
            Retry
          </button>
        </div>
      </div>
    )
  }

  if (token === null) return null

  return (
    <AdminProvider>
      <AuthenticatedApp token={token} />
    </AdminProvider>
  )
}

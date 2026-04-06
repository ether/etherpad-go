import { Routes, Route, Navigate } from 'react-router-dom'
import { AdminProvider } from '@/store'
import { Sidebar } from '@/components/Sidebar'
import { Toast } from '@/components/Toast'
import { OverviewPage } from '@/pages/Overview'
import { PadsPage } from '@/pages/Pads'
import { BroadcastPage } from '@/pages/Broadcast'
import { SettingsPage } from '@/pages/Settings'
import { AdminSocketProvider } from '@/hooks/useAdminSocket'

export default function App() {
  return (
    <AdminProvider>
      <AdminSocketProvider>
        <div className="flex h-screen bg-gray-50 dark:bg-gray-950">
          <Sidebar />
          <main className="flex-1 overflow-auto">
            <Routes>
              <Route path="/" element={<OverviewPage />} />
              <Route path="/pads" element={<PadsPage />} />
              <Route path="/broadcast" element={<BroadcastPage />} />
              <Route path="/settings" element={<SettingsPage />} />
              <Route path="*" element={<Navigate to="/" replace />} />
            </Routes>
          </main>
          <Toast />
        </div>
      </AdminSocketProvider>
    </AdminProvider>
  )
}

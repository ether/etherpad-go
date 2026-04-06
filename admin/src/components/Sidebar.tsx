import { NavLink } from 'react-router-dom'
import {
  LayoutDashboard,
  FileText,
  Megaphone,
  Settings,
  Wifi,
  WifiOff,
} from 'lucide-react'
import { useAdminStore } from '@/store'

const navItems = [
  { to: '/', label: 'Overview', icon: LayoutDashboard },
  { to: '/pads', label: 'Pads', icon: FileText },
  { to: '/broadcast', label: 'Broadcast', icon: Megaphone },
  { to: '/settings', label: 'Settings', icon: Settings },
] as const

export function Sidebar() {
  const { connected } = useAdminStore()

  return (
    <aside className="flex w-60 shrink-0 flex-col bg-gray-900 text-white">
      {/* Brand */}
      <div className="px-6 pt-8 pb-6">
        <p className="text-xs font-medium uppercase tracking-widest text-gray-400">
          Etherpad
        </p>
        <h1 className="mt-1 text-2xl font-bold">Admin</h1>
      </div>

      {/* Navigation */}
      <nav className="flex-1 space-y-1 px-3">
        {navItems.map(({ to, label, icon: Icon }) => (
          <NavLink
            key={to}
            to={to}
            end={to === '/'}
            className={({ isActive }) =>
              `flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium transition-colors ${
                isActive
                  ? 'bg-blue-600 text-white'
                  : 'text-gray-300 hover:bg-gray-800 hover:text-white'
              }`
            }
          >
            <Icon className="h-5 w-5 shrink-0" />
            {label}
          </NavLink>
        ))}
      </nav>

      {/* Connection status */}
      <div className="flex items-center gap-2 border-t border-gray-800 px-6 py-4">
        {connected ? (
          <>
            <Wifi className="h-4 w-4 text-green-400" />
            <span className="text-xs text-green-400">Connected</span>
          </>
        ) : (
          <>
            <WifiOff className="h-4 w-4 text-red-400" />
            <span className="text-xs text-red-400">Disconnected</span>
          </>
        )}
      </div>
    </aside>
  )
}

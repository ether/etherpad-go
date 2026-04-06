import { NavLink } from 'react-router-dom'
import {
  LayoutDashboard,
  Activity,
  FileText,
  Megaphone,
  Settings,
} from 'lucide-react'
import { useAdminStore } from '@/store'

const navItems = [
  { to: '/', label: 'Overview', icon: LayoutDashboard },
  { to: '/monitoring', label: 'Monitoring', icon: Activity },
  { to: '/pads', label: 'Pads', icon: FileText },
  { to: '/broadcast', label: 'Broadcast', icon: Megaphone },
  { to: '/settings', label: 'Settings', icon: Settings },
] as const

export function Sidebar() {
  const { connected } = useAdminStore()

  return (
    <aside className="flex w-60 shrink-0 flex-col bg-black text-white">
      {/* Brand */}
      <div className="border-b border-white/10 px-5 py-5">
        <p className="text-[13px] font-medium text-gray-400">
          Etherpad
        </p>
        <h1 className="mt-0.5 text-lg font-semibold tracking-tight text-white">
          Admin
        </h1>
      </div>

      {/* Navigation */}
      <nav className="flex-1 space-y-0.5 px-3 pt-4">
        {navItems.map(({ to, label, icon: Icon }) => (
          <NavLink
            key={to}
            to={to}
            end={to === '/'}
            className={({ isActive }) =>
              `flex items-center gap-3 rounded-lg px-3 py-2 text-[13px] font-medium transition-colors ${
                isActive
                  ? 'bg-white/10 text-white'
                  : 'text-gray-400 hover:bg-white/5 hover:text-gray-200'
              }`
            }
          >
            <Icon className="h-4 w-4 shrink-0" strokeWidth={1.5} />
            {label}
          </NavLink>
        ))}
      </nav>

      {/* Connection status */}
      <div className="flex items-center gap-2 border-t border-white/10 px-5 py-4">
        <span
          className={`h-2 w-2 rounded-full ${
            connected ? 'bg-green-500' : 'bg-red-500'
          }`}
        />
        <span className="text-xs text-gray-400">
          {connected ? 'Connected' : 'Disconnected'}
        </span>
      </div>
    </aside>
  )
}

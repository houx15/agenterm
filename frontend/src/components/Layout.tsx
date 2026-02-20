import { useEffect, useState } from 'react'
import { Outlet, useLocation } from 'react-router-dom'
import Sidebar from './Sidebar'
import { Menu } from './Lucide'
import { useAppContext } from '../App'

export default function Layout() {
  const app = useAppContext()
  const [sidebarOpen, setSidebarOpen] = useState(window.innerWidth >= 768)
  const location = useLocation()

  const titleByPath: Record<string, string> = {
    '/': 'Dashboard',
    '/sessions': 'Sessions',
    '/pm-chat': 'PM Chat',
    '/demand-pool': 'Demand Pool',
    '/settings': 'Settings',
  }
  const topbarTitle = location.pathname.startsWith('/projects/') ? 'Project Detail' : titleByPath[location.pathname] ?? location.pathname.slice(1)
  const topbarSubtitle =
    location.pathname === '/sessions'
      ? app.activeWindow ?? 'Select a session'
      : location.pathname === '/pm-chat'
        ? 'Plan, monitor, and steer execution'
        : location.pathname === '/demand-pool'
          ? 'Read-only demand overview (edit in PM Chat)'
        : app.connected
          ? 'Realtime sync online'
          : 'Realtime sync offline'

  useEffect(() => {
    const onResize = () => {
      if (window.innerWidth >= 768) {
        setSidebarOpen(true)
      }
    }
    window.addEventListener('resize', onResize)
    return () => window.removeEventListener('resize', onResize)
  }, [])

  const closeOnMobile = () => {
    if (window.innerWidth < 768) {
      setSidebarOpen(false)
    }
  }

  return (
    <div className="app-shell">
      <button
        aria-expanded={sidebarOpen}
        aria-label="Toggle navigation"
        className="mobile-sidebar-toggle"
        onClick={() => setSidebarOpen((v) => !v)}
        type="button"
      >
        <Menu size={16} />
      </button>

      <div className={`sidebar-container ${sidebarOpen ? 'open' : ''}`.trim()}>
        <Sidebar closeOnNavigate={closeOnMobile} />
      </div>
      <button
        aria-hidden={!sidebarOpen}
        aria-label="Close navigation"
        className={`mobile-sidebar-backdrop ${sidebarOpen ? 'open' : ''}`.trim()}
        onClick={closeOnMobile}
        tabIndex={sidebarOpen ? 0 : -1}
        type="button"
      />

      <main className="content-shell" onClick={closeOnMobile}>
        <header className="topbar">
          <div className={`connection-indicator ${app.connected ? '' : app.connectionStatus}`.trim()} />
          <strong>{topbarTitle}</strong>
          <span className="topbar-subtitle">{topbarSubtitle}</span>
        </header>
        <Outlet />
      </main>
    </div>
  )
}

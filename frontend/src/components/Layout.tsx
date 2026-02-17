import { useState } from 'react'
import { Outlet, useLocation } from 'react-router-dom'
import Sidebar from './Sidebar'
import { useAppContext } from '../App'

export default function Layout() {
  const app = useAppContext()
  const [sidebarOpen, setSidebarOpen] = useState(window.innerWidth >= 768)
  const location = useLocation()

  const closeOnMobile = () => {
    if (window.innerWidth < 768) {
      setSidebarOpen(false)
    }
  }

  return (
    <div className="app-shell">
      <button className="mobile-sidebar-toggle" onClick={() => setSidebarOpen((v) => !v)} type="button">
        menu
      </button>

      <div className={`sidebar-container ${sidebarOpen ? 'open' : ''}`.trim()}>
        <Sidebar
          windows={app.windows}
          activeWindow={app.activeWindow}
          unread={app.unreadByWindow}
          onSelectWindow={app.setActiveWindow}
          onNewWindow={() => app.send({ type: 'new_window', name: '' })}
          onKillWindow={(windowID) => app.send({ type: 'kill_window', window: windowID })}
          closeOnNavigate={closeOnMobile}
        />
      </div>

      <main className="content-shell" onClick={closeOnMobile}>
        <header className="topbar">
          <div className={`connection-indicator ${app.connected ? '' : app.connectionStatus}`.trim()} />
          <strong>{location.pathname === '/' ? 'Dashboard' : location.pathname.slice(1)}</strong>
          <span className="topbar-subtitle">{app.activeWindow ?? 'no active session'}</span>
        </header>
        <Outlet />
      </main>
    </div>
  )
}

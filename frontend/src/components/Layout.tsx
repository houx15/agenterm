import { useEffect, useState } from 'react'
import { Outlet, useLocation } from 'react-router-dom'
import Sidebar from './Sidebar'
import { ChevronLeft, ChevronRight, Menu } from './Lucide'
import { useAppContext } from '../App'

export default function Layout() {
  const app = useAppContext()
  const [isMobile, setIsMobile] = useState(window.innerWidth < 768)
  const [mobileSidebarOpen, setMobileSidebarOpen] = useState(false)
  const [desktopSidebarCollapsed, setDesktopSidebarCollapsed] = useState(false)
  const location = useLocation()

  const titleByPath: Record<string, string> = {
    '/': 'Workspace',
    '/stats': 'Dashboard',
    '/sessions': 'Session Console',
    '/pm-chat': 'Workspace',
    '/connect': 'Connect Mobile',
    '/settings': 'Agent Registry',
  }
  const topbarTitle = location.pathname.startsWith('/projects/') ? 'Project Detail' : titleByPath[location.pathname] ?? location.pathname.slice(1)
  const topbarSubtitle =
    location.pathname === '/sessions'
      ? app.activeWindow ?? 'Select a session'
      : location.pathname === '/pm-chat' || location.pathname === '/'
        ? 'Coordinate agents and keep execution flowing'
        : app.connected
          ? 'Realtime sync online'
          : 'Realtime sync offline'

  useEffect(() => {
    const onResize = () => {
      const nextIsMobile = window.innerWidth < 768
      setIsMobile(nextIsMobile)
      if (!nextIsMobile) {
        setMobileSidebarOpen(false)
      }
    }
    window.addEventListener('resize', onResize)
    return () => window.removeEventListener('resize', onResize)
  }, [])

  const closeOnMobile = () => {
    if (isMobile) {
      setMobileSidebarOpen(false)
    }
  }

  return (
    <div className={`app-shell ${!isMobile && desktopSidebarCollapsed ? 'sidebar-collapsed' : ''}`.trim()}>
      <button
        aria-expanded={mobileSidebarOpen}
        aria-label="Toggle navigation"
        className="mobile-sidebar-toggle"
        onClick={() => setMobileSidebarOpen((v) => !v)}
        type="button"
      >
        <Menu size={16} />
      </button>

      <div className={`sidebar-container ${mobileSidebarOpen ? 'open' : ''} ${!isMobile && desktopSidebarCollapsed ? 'collapsed' : ''}`.trim()}>
        <Sidebar closeOnNavigate={closeOnMobile} />
      </div>
      <button
        aria-hidden={!mobileSidebarOpen}
        aria-label="Close navigation"
        className={`mobile-sidebar-backdrop ${mobileSidebarOpen ? 'open' : ''}`.trim()}
        onClick={closeOnMobile}
        tabIndex={mobileSidebarOpen ? 0 : -1}
        type="button"
      />

      <main className="content-shell" onClick={closeOnMobile}>
        <header className="topbar">
          {!isMobile && (
            <button
              aria-label={desktopSidebarCollapsed ? 'Expand sidebar' : 'Collapse sidebar'}
              className="secondary-btn sidebar-collapse-toggle"
              onClick={() => setDesktopSidebarCollapsed((prev) => !prev)}
              type="button"
            >
              {desktopSidebarCollapsed ? <ChevronRight size={14} /> : <ChevronLeft size={14} />}
              <span className="sidebar-collapse-label">{desktopSidebarCollapsed ? 'Show' : 'Hide'}</span>
            </button>
          )}
          <div className={`connection-indicator ${app.connected ? '' : app.connectionStatus}`.trim()} />
          <strong>{topbarTitle}</strong>
          <span className="topbar-subtitle">{topbarSubtitle}</span>
        </header>
        <Outlet />
      </main>
    </div>
  )
}

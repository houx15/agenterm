import { NavLink } from 'react-router-dom'
import type { WindowInfo } from '../api/types'
import StatusDot from './StatusDot'

interface SidebarProps {
  windows: WindowInfo[]
  activeWindow: string | null
  unread: Record<string, number>
  onSelectWindow: (windowID: string) => void
  onNewWindow: () => void
  onKillWindow: (windowID: string) => void
  closeOnNavigate?: () => void
}

function splitSessionName(name: string) {
  const parts = name.split('-', 3)
  if (parts.length < 2) {
    return { model: name, rest: '' }
  }
  return {
    model: parts[0],
    rest: parts.slice(1).join('/'),
  }
}

export default function Sidebar({
  windows,
  activeWindow,
  unread,
  onSelectWindow,
  onNewWindow,
  onKillWindow,
  closeOnNavigate,
}: SidebarProps) {
  return (
    <aside className="sidebar-shell">
      <div className="sidebar-header">
        <h1>agenterm</h1>
        <button className="secondary-btn" onClick={onNewWindow} type="button">
          + new
        </button>
      </div>

      <nav className="nav-links" onClick={closeOnNavigate}>
        <NavLink to="/" className={({ isActive }) => `nav-link ${isActive ? 'active' : ''}`.trim()} end>
          Dashboard
        </NavLink>
        <NavLink to="/sessions" className={({ isActive }) => `nav-link ${isActive ? 'active' : ''}`.trim()}>
          Sessions
        </NavLink>
        <NavLink to="/settings" className={({ isActive }) => `nav-link ${isActive ? 'active' : ''}`.trim()}>
          Settings
        </NavLink>
      </nav>

      <div className="session-list">
        {windows.length === 0 && <div className="empty-text">no sessions yet</div>}

        {windows.map((win) => {
          const name = splitSessionName(win.name)
          const isActive = win.id === activeWindow
          return (
            <button
              key={win.id}
              className={`session-card ${isActive ? 'active' : ''}`.trim()}
              onClick={() => {
                onSelectWindow(win.id)
                closeOnNavigate?.()
              }}
              type="button"
            >
              <span
                className="kill-btn"
                onClick={(event) => {
                  event.stopPropagation()
                  onKillWindow(win.id)
                }}
                role="button"
                tabIndex={0}
              >
                x
              </span>

              <div className="session-header">
                <StatusDot status={win.status} />
                <span className="session-name">
                  <span className="model">{name.model}</span>
                  {name.rest && (
                    <>
                      <span className="separator">/</span>
                      <span className="rest">{name.rest}</span>
                    </>
                  )}
                </span>
              </div>

              {(unread[win.id] ?? 0) > 0 && <span className="unread-badge">{unread[win.id]}</span>}
            </button>
          )
        })}
      </div>
    </aside>
  )
}

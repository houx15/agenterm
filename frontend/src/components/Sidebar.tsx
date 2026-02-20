import { NavLink } from 'react-router-dom'
import { ClipboardList, House, MessageSquareText, Settings, SquareTerminal } from './Lucide'

interface SidebarProps {
  closeOnNavigate?: () => void
}

export default function Sidebar({ closeOnNavigate }: SidebarProps) {
  return (
    <aside className="sidebar-shell">
      <div className="sidebar-header">
        <h1>agenterm</h1>
        <span className="sidebar-badge">control</span>
      </div>

      <nav className="nav-links" onClick={closeOnNavigate}>
        <NavLink to="/" className={({ isActive }) => `nav-link ${isActive ? 'active' : ''}`.trim()} end>
          <House size={15} />
          <span>Dashboard</span>
        </NavLink>
        <NavLink to="/sessions" className={({ isActive }) => `nav-link ${isActive ? 'active' : ''}`.trim()}>
          <SquareTerminal size={15} />
          <span>Sessions</span>
        </NavLink>
        <NavLink to="/pm-chat" className={({ isActive }) => `nav-link ${isActive ? 'active' : ''}`.trim()}>
          <MessageSquareText size={15} />
          <span>PM Chat</span>
        </NavLink>
        <NavLink to="/demand-pool" className={({ isActive }) => `nav-link ${isActive ? 'active' : ''}`.trim()}>
          <ClipboardList size={15} />
          <span>Demand Pool</span>
        </NavLink>
        <NavLink to="/settings" className={({ isActive }) => `nav-link ${isActive ? 'active' : ''}`.trim()}>
          <Settings size={15} />
          <span>Settings</span>
        </NavLink>
      </nav>
    </aside>
  )
}

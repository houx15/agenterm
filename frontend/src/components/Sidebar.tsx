import { NavLink } from 'react-router-dom'
import { House, MessageSquareText, Settings, Smartphone, SquareTerminal } from './Lucide'

interface SidebarProps {
  closeOnNavigate?: () => void
}

export default function Sidebar({ closeOnNavigate }: SidebarProps) {
  return (
    <aside className="sidebar-shell">
      <div className="sidebar-header">
        <h1>agenterm</h1>
        <span className="sidebar-badge">vnext</span>
      </div>

      <nav className="nav-links" onClick={closeOnNavigate}>
        <NavLink to="/" className={({ isActive }) => `nav-link ${isActive ? 'active' : ''}`.trim()} end>
          <MessageSquareText size={15} />
          <span>Projects Workspace</span>
        </NavLink>
      </nav>

      <div className="sidebar-section-label">Utilities</div>
      <nav className="nav-links nav-links-secondary sidebar-utility-nav" onClick={closeOnNavigate}>
        <NavLink to="/stats" className={({ isActive }) => `nav-link ${isActive ? 'active' : ''}`.trim()}>
          <House size={15} />
          <span>Dashboard</span>
        </NavLink>
        <NavLink to="/sessions" className={({ isActive }) => `nav-link ${isActive ? 'active' : ''}`.trim()}>
          <SquareTerminal size={15} />
          <span>Session Console</span>
        </NavLink>
        <NavLink to="/connect" className={({ isActive }) => `nav-link ${isActive ? 'active' : ''}`.trim()}>
          <Smartphone size={15} />
          <span>Connect Mobile</span>
        </NavLink>
        <NavLink to="/settings" className={({ isActive }) => `nav-link ${isActive ? 'active' : ''}`.trim()}>
          <Settings size={15} />
          <span>Agent Registry</span>
        </NavLink>
      </nav>
    </aside>
  )
}

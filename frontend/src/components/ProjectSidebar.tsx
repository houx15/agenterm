import { useState, useEffect } from 'react'
import type { Project, Session } from '../api/types'
import { getWindowID } from '../api/types'
import { House, Settings, QrCode, Bot, TerminalIcon, ChevronDown, ChevronRight } from './Lucide'

interface ProjectSidebarProps {
  projects: Project[]
  activeProjectID: string
  onSelectProject: (id: string) => void
  sessions: Session[]
  unreadByWindow: Record<string, number>
  onSelectAgent: (session: Session) => void
  onNewProject: () => void
  onOpenHome: () => void
  onOpenConnect: () => void
  onOpenSettings: () => void
  collapsed: boolean
}

function isOrchestrator(session: Session): boolean {
  const role = (session.role || '').toLowerCase()
  return role.includes('orchestrator') || role.includes('pm')
}

function statusClass(status: string): string {
  const s = (status || '').toLowerCase()
  if (['working', 'running', 'executing', 'busy', 'active'].includes(s)) return 'working'
  if (['waiting', 'waiting_review', 'human_takeover', 'blocked', 'needs_input'].includes(s)) return 'waiting'
  if (['error', 'failed'].includes(s)) return 'error'
  return 'idle'
}

function sortSessions(sessions: Session[]): Session[] {
  return [...sessions].sort((a, b) => {
    const aOrch = isOrchestrator(a) ? 0 : 1
    const bOrch = isOrchestrator(b) ? 0 : 1
    if (aOrch !== bOrch) return aOrch - bOrch
    const aName = (a.role || a.agent_type || '').toLowerCase()
    const bName = (b.role || b.agent_type || '').toLowerCase()
    return aName.localeCompare(bName)
  })
}

export default function ProjectSidebar({
  projects,
  activeProjectID,
  onSelectProject,
  sessions,
  unreadByWindow,
  onSelectAgent,
  onNewProject,
  onOpenHome,
  onOpenConnect,
  onOpenSettings,
  collapsed,
}: ProjectSidebarProps) {
  const [expandedProjects, setExpandedProjects] = useState<Record<string, boolean>>({})

  // Auto-expand the active project
  useEffect(() => {
    if (activeProjectID) {
      setExpandedProjects((prev) => ({
        ...prev,
        [activeProjectID]: true,
      }))
    }
  }, [activeProjectID])

  function handleProjectClick(projectID: string) {
    onSelectProject(projectID)
    setExpandedProjects((prev) => ({
      ...prev,
      [projectID]: projectID === activeProjectID ? !prev[projectID] : true,
    }))
  }

  function getProjectSessions(projectID: string): Session[] {
    // Sessions are fetched per-project. When the sidebar receives them,
    // they belong to the active project. Filter by project_id if present,
    // otherwise show all sessions under the active project.
    return sessions.filter((s) => {
      const sid = (s as unknown as Record<string, unknown>).project_id as string | undefined
      if (sid) return sid === projectID
      // If no project_id on session, associate with active project
      return projectID === activeProjectID
    })
  }

  return (
    <aside className="sidebar">
      {/* 1. Brand header */}
      <div className="sidebar-header">
        <span className="sidebar-brand">agenTerm</span>
      </div>

      {/* 2. Project tree */}
      <div className="sidebar-tree">
        {projects.map((project) => {
          const isActive = project.id === activeProjectID
          const isExpanded = !!expandedProjects[project.id]
          const projectSessions = sortSessions(getProjectSessions(project.id))

          return (
            <div className="sidebar-project" key={project.id}>
              <div
                className={`sidebar-project-header${isActive ? ' active' : ''}`}
                onClick={() => handleProjectClick(project.id)}
              >
                {isExpanded ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
                <span style={{ flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                  {project.name}
                </span>
              </div>

              {isExpanded &&
                projectSessions.map((session) => {
                  const wid = getWindowID(session)
                  const unread = wid ? (unreadByWindow[wid] || 0) : 0
                  const agentName = session.role || session.agent_type
                  const orch = isOrchestrator(session)

                  return (
                    <div
                      className="sidebar-agent-item"
                      key={session.id}
                      onClick={() => onSelectAgent(session)}
                      style={{ cursor: 'pointer', alignItems: 'center' }}
                    >
                      <span className={`sidebar-status-dot ${statusClass(session.status)}`} />
                      {orch ? <Bot size={13} /> : <TerminalIcon size={13} />}
                      <span style={{ flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                        {agentName}
                      </span>
                      {unread > 0 && <span className="sidebar-unread-badge">{unread}</span>}
                    </div>
                  )
                })}
            </div>
          )
        })}
      </div>

      {/* 3. New Project button */}
      <div className="sidebar-new-project-btn" onClick={onNewProject}>
        + New Project
      </div>

      {/* 4. Utility nav */}
      <div className="sidebar-utilities">
        <div className="sidebar-utility-item" onClick={onOpenHome}>
          <House size={14} /> Home
        </div>
        <div className="sidebar-utility-item" onClick={onOpenConnect}>
          <QrCode size={14} /> Connect
        </div>
        <div className="sidebar-utility-item" onClick={onOpenSettings}>
          <Settings size={14} /> Agents
        </div>
      </div>
    </aside>
  )
}

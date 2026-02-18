import { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { listProjects, listProjectTasks, listSessions } from '../api/client'
import type { OrchestratorServerMessage, Project, Session, Task } from '../api/types'
import { useAppContext } from '../App'
import ChatPanel from '../components/ChatPanel'
import type { MessageTaskLink } from '../components/ChatMessage'
import { ChevronLeft, ChevronRight, FolderOpen, MessageSquareText } from '../components/Lucide'
import TaskDAG from '../components/TaskDAG'
import { useOrchestratorWS } from '../hooks/useOrchestratorWS'

function shouldRefreshFromOrchestratorEvent(event: OrchestratorServerMessage | null): boolean {
  if (!event) {
    return false
  }
  return event.type === 'tool_result' || event.type === 'done'
}

export default function PMChat() {
  const navigate = useNavigate()
  const app = useAppContext()
  const [, setSearchParams] = useSearchParams()

  const [projects, setProjects] = useState<Project[]>([])
  const [tasks, setTasks] = useState<Task[]>([])
  const [sessions, setSessions] = useState<Session[]>([])
  const [projectStats, setProjectStats] = useState<Record<string, { sessionCount: number; needsResponse: number }>>({})
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [detailCollapsed, setDetailCollapsed] = useState(false)
  const [showMobileProject, setShowMobileProject] = useState(false)

  const [projectID, setProjectID] = useState(() => new URLSearchParams(window.location.search).get('project') ?? '')

  const orchestrator = useOrchestratorWS(projectID)

  const loadProjectStats = useCallback(async (projectList: Project[]) => {
    if (projectList.length === 0) {
      setProjectStats({})
      return
    }
    const entries = await Promise.all(
      projectList.map(async (project) => {
        const sessionList = await listSessions<Session[]>({ projectID: project.id })
        const needsResponse = sessionList.filter((session) =>
          ['waiting', 'human_takeover', 'blocked', 'needs_input', 'reviewing'].includes((session.status || '').toLowerCase()),
        ).length
        return [project.id, { sessionCount: sessionList.length, needsResponse }] as const
      }),
    )
    setProjectStats(Object.fromEntries(entries))
  }, [])

  const loadProjectData = useCallback(async () => {
    if (!projectID) {
      setTasks([])
      setSessions([])
      setLoading(false)
      return
    }

    const [taskList, sessionList] = await Promise.all([
      listProjectTasks<Task[]>(projectID),
      listSessions<Session[]>({ projectID }),
    ])

    setTasks(taskList)
    setSessions(sessionList)
    setLoading(false)
  }, [projectID])

  const refreshAll = useCallback(async () => {
    setError('')
    try {
      const projectList = await listProjects<Project[]>()
      setProjects(projectList)
      await loadProjectStats(projectList)
      if (projectList.length === 0) {
        setProjectID('')
        return
      }
      const hasCurrent = Boolean(projectID && projectList.some((project) => project.id === projectID))
      const nextID = hasCurrent ? projectID : projectList[0].id
      if (nextID !== projectID) {
        setProjectID(nextID)
        setSearchParams({ project: nextID }, { replace: true })
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load projects')
      setLoading(false)
    }
  }, [loadProjectStats, projectID, setSearchParams])

  const refreshProjectData = useCallback(async () => {
    try {
      await loadProjectData()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load project context')
      setLoading(false)
    }
  }, [loadProjectData])

  useEffect(() => {
    void refreshAll()
  }, [refreshAll])

  useEffect(() => {
    void refreshProjectData()
  }, [refreshProjectData])

  useEffect(() => {
    if (!projectID) {
      return
    }

    const intervalID = window.setInterval(() => {
      void refreshProjectData()
    }, 5000)

    return () => window.clearInterval(intervalID)
  }, [projectID, refreshProjectData])

  useEffect(() => {
    const onResize = () => {
      if (window.innerWidth >= 900) {
        setShowMobileProject(false)
      }
    }
    window.addEventListener('resize', onResize)
    return () => window.removeEventListener('resize', onResize)
  }, [])

  useEffect(() => {
    if (shouldRefreshFromOrchestratorEvent(orchestrator.lastEvent)) {
      void refreshProjectData()
    }
  }, [orchestrator.lastEvent, refreshProjectData])

  useEffect(() => {
    if (!app.lastMessage || app.lastMessage.type !== 'project_event' || app.lastMessage.project_id !== projectID) {
      return
    }
    void refreshProjectData()
  }, [app.lastMessage, projectID, refreshProjectData])

  useEffect(() => {
    if (!projects.length) {
      return
    }
    const intervalID = window.setInterval(() => {
      void loadProjectStats(projects)
    }, 8000)
    return () => window.clearInterval(intervalID)
  }, [loadProjectStats, projects])

  const sessionsByTask = useMemo(() => {
    const mapping: Record<string, Session> = {}

    for (const session of sessions) {
      if (!session.task_id) {
        continue
      }
      mapping[session.task_id] = session
    }

    return mapping
  }, [sessions])

  const taskLinks = useMemo<MessageTaskLink[]>(() => {
    const links: MessageTaskLink[] = []

    for (const task of tasks) {
      links.push({ id: task.id, label: task.id })
      links.push({ id: task.id, label: task.title })
    }

    return links
  }, [tasks])

  const onSelectProject = (nextProjectID: string) => {
    setProjectID(nextProjectID)
    if (window.innerWidth < 900) {
      setShowMobileProject(true)
    }
    if (nextProjectID) {
      setSearchParams({ project: nextProjectID }, { replace: true })
    } else {
      setSearchParams({}, { replace: true })
    }
  }

  const openTaskSession = (taskID: string) => {
    const session = sessionsByTask[taskID]
    if (!session?.tmux_window_id) {
      return
    }
    app.setActiveWindow(session.tmux_window_id)
    navigate('/sessions')
  }

  const selectedProject = useMemo(() => projects.find((project) => project.id === projectID) ?? null, [projectID, projects])

  const tasksByStatus = useMemo(() => {
    const grouped: Record<string, number> = {}
    for (const task of tasks) {
      const key = (task.status || 'pending').toLowerCase()
      grouped[key] = (grouped[key] ?? 0) + 1
    }
    return Object.entries(grouped).sort(([a], [b]) => a.localeCompare(b))
  }, [tasks])

  const isMobile = window.innerWidth < 900

  return (
    <section className="pm-chat-page">
      <div className="pm-chat-layout-v2">
        <aside className={`pm-project-list ${isMobile && showMobileProject ? 'hidden-mobile' : ''}`.trim()}>
          <div className="pm-panel-header">
            <h3>PMs</h3>
            <small>{projects.length}</small>
          </div>
          <div className="pm-project-items">
            {projects.map((project) => {
              const stat = projectStats[project.id] ?? { sessionCount: 0, needsResponse: 0 }
              const active = project.id === projectID
              return (
                <button
                  className={`pm-project-item ${active ? 'active' : ''}`.trim()}
                  key={project.id}
                  onClick={() => onSelectProject(project.id)}
                  type="button"
                >
                  <span className="pm-project-item-name">{project.name}</span>
                  <span className="pm-project-item-meta">{stat.sessionCount} sessions</span>
                  {stat.needsResponse > 0 && <span className="pm-notification-badge">{stat.needsResponse}</span>}
                </button>
              )
            })}
            {projects.length === 0 && <p className="empty-view">No projects</p>}
          </div>
        </aside>

        <div className={`pm-project-content ${isMobile && !showMobileProject ? 'hidden-mobile' : ''}`.trim()}>
          <div className="pm-chat-header">
            {isMobile && (
              <button className="secondary-btn" onClick={() => setShowMobileProject(false)} type="button">
                <ChevronLeft size={14} />
                <span>Back</span>
              </button>
            )}
            <h2>{selectedProject?.name ?? 'PM Chat'}</h2>
          </div>

          {loading && <p className="empty-view">Loading PM chat context...</p>}
          {error && <p className="dashboard-error">{error}</p>}

          {!loading && !error && selectedProject && (
            <>
              <section className="pm-project-detail">
                <div className="pm-panel-header">
                  <h3>
                    <FolderOpen size={14} /> Project Detail
                  </h3>
                  <button className="secondary-btn" onClick={() => setDetailCollapsed((prev) => !prev)} type="button">
                    {detailCollapsed ? (
                      <>
                        <ChevronRight size={14} />
                        <span>Expand</span>
                      </>
                    ) : (
                      <>
                        <ChevronLeft size={14} />
                        <span>Collapse</span>
                      </>
                    )}
                  </button>
                </div>
                {!detailCollapsed && (
                  <div className="pm-project-detail-body">
                    <div className="pm-project-detail-meta">
                      <p>Repository: {selectedProject.repo_path}</p>
                      <p>Status: {selectedProject.status}</p>
                    </div>
                    <div className="pm-project-status-grid">
                      {tasksByStatus.map(([status, count]) => (
                        <div className="pm-status-card" key={status}>
                          <strong>{count}</strong>
                          <small>{status}</small>
                        </div>
                      ))}
                    </div>
                    <div className="pm-project-session-list">
                      <h4>Sessions</h4>
                      {sessions.length === 0 && <p className="empty-text">No sessions in this project yet.</p>}
                      {sessions.map((session) => (
                        <button
                          className="pm-project-session-item"
                          key={session.id}
                          onClick={() => {
                            if (!session.tmux_window_id) {
                              return
                            }
                            app.setActiveWindow(session.tmux_window_id)
                            navigate('/sessions')
                          }}
                          type="button"
                        >
                          <span>{session.agent_type}</span>
                          <small>{session.status}</small>
                        </button>
                      ))}
                    </div>
                  </div>
                )}
              </section>

              <div className="pm-chat-main">
                <TaskDAG tasks={tasks} sessionsByTask={sessionsByTask} onOpenTask={openTaskSession} />
                <ChatPanel
                  messages={orchestrator.messages}
                  taskLinks={taskLinks}
                  isStreaming={orchestrator.isStreaming}
                  connectionStatus={orchestrator.connectionStatus}
                  onSend={orchestrator.send}
                  onTaskClick={openTaskSession}
                />
              </div>
            </>
          )}

          {!loading && !error && !selectedProject && (
            <div className="empty-view">
              <MessageSquareText size={16} /> Select a project to open PM Chat
            </div>
          )}
        </div>
      </div>
    </section>
  )
}

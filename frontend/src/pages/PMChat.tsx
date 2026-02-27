import { useCallback, useEffect, useMemo, useRef, useState, type MouseEvent as ReactMouseEvent } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import {
  deleteProject,
  getOrchestratorReport,
  listOrchestratorExceptions,
  listProjects,
  listProjectTasks,
  listSessions,
  resolveOrchestratorException,
} from '../api/client'
import type {
  OrchestratorExceptionItem,
  OrchestratorExceptionListResponse,
  OrchestratorProgressReport,
  OrchestratorServerMessage,
  Project,
  Session,
  Task,
} from '../api/types'
import { useAppContext } from '../App'
import ChatPanel from '../components/ChatPanel'
import type { MessageTaskLink, SessionMessage } from '../components/ChatMessage'
import { ChevronDown, ChevronRight, FolderOpen, MessageSquareText } from '../components/Lucide'
import Modal from '../components/Modal'
import TaskDAG from '../components/TaskDAG'
import { useOrchestratorWS } from '../hooks/useOrchestratorWS'
import DemandPool from './DemandPool'

interface ProjectSessionStats {
  total: number
  working: number
  needsResponse: number
}

function shouldRefreshFromOrchestratorEvent(event: OrchestratorServerMessage | null): boolean {
  if (!event) {
    return false
  }
  return event.type === 'tool_result' || event.type === 'done'
}

function isWorkingSessionStatus(status: string): boolean {
  const normalized = (status || '').trim().toLowerCase()
  return ['working', 'running', 'executing', 'busy', 'active'].includes(normalized)
}

function needsResponseStatus(status: string): boolean {
  const normalized = (status || '').trim().toLowerCase()
  return ['waiting', 'waiting_review', 'human_takeover', 'blocked', 'needs_input', 'reviewing'].includes(normalized)
}

function isOrchestratorSession(session: Session): boolean {
  const role = (session.role || '').toLowerCase()
  const agent = (session.agent_type || '').toLowerCase()
  return role.includes('orchestrator') || role.includes('pm') || agent.includes('orchestrator')
}

export default function PMChat() {
  const navigate = useNavigate()
  const app = useAppContext()
  const [, setSearchParams] = useSearchParams()
  const [isMobile, setIsMobile] = useState(() => window.innerWidth < 900)

  const [projects, setProjects] = useState<Project[]>([])
  const [tasks, setTasks] = useState<Task[]>([])
  const [sessions, setSessions] = useState<Session[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  const [projectInfoCollapsed, setProjectInfoCollapsed] = useState(false)
  const [progressCollapsed, setProgressCollapsed] = useState(false)
  const [taskGroupCollapsed, setTaskGroupCollapsed] = useState(false)
  const [exceptionsCollapsed, setExceptionsCollapsed] = useState(false)

  const [showMobileInfo, setShowMobileInfo] = useState(false)
  const [demandPoolOpen, setDemandPoolOpen] = useState(false)
  const [deleteModalOpen, setDeleteModalOpen] = useState(false)
  const [deletingProject, setDeletingProject] = useState(false)
  const [deleteError, setDeleteError] = useState('')

  const [progressReport, setProgressReport] = useState<OrchestratorProgressReport | null>(null)
  const [reportUpdatedAt, setReportUpdatedAt] = useState<number | null>(null)
  const [reportLoading, setReportLoading] = useState(false)
  const [reportError, setReportError] = useState('')

  const [exceptions, setExceptions] = useState<OrchestratorExceptionItem[]>([])
  const [exceptionCounts, setExceptionCounts] = useState<{ total: number; open: number; resolved: number }>({ total: 0, open: 0, resolved: 0 })
  const [exceptionsLoading, setExceptionsLoading] = useState(false)
  const [exceptionsError, setExceptionsError] = useState('')

  const [projectSessionWindows, setProjectSessionWindows] = useState<Record<string, string[]>>({})
  const [projectSessionStats, setProjectSessionStats] = useState<Record<string, ProjectSessionStats>>({})
  const [projectID, setProjectID] = useState(() => new URLSearchParams(window.location.search).get('project') ?? '')
  const [leftPaneWidth, setLeftPaneWidth] = useState<number>(() => loadPanePreference().leftWidth)
  const [rightPaneWidth, setRightPaneWidth] = useState<number>(() => loadPanePreference().rightWidth)
  const [leftPaneCollapsed, setLeftPaneCollapsed] = useState<boolean>(() => loadPanePreference().leftCollapsed)
  const [rightPaneCollapsed, setRightPaneCollapsed] = useState<boolean>(() => loadPanePreference().rightCollapsed)
  const [resizingPane, setResizingPane] = useState<'left' | 'right' | null>(null)
  const dragOriginRef = useRef<{ x: number; width: number } | null>(null)

  const orchestrator = useOrchestratorWS(projectID)

  useEffect(() => {
    const onResize = () => setIsMobile(window.innerWidth < 900)
    window.addEventListener('resize', onResize)
    return () => window.removeEventListener('resize', onResize)
  }, [])

  useEffect(() => {
    savePanePreference({
      leftWidth: leftPaneWidth,
      rightWidth: rightPaneWidth,
      leftCollapsed: leftPaneCollapsed,
      rightCollapsed: rightPaneCollapsed,
    })
  }, [leftPaneCollapsed, leftPaneWidth, rightPaneCollapsed, rightPaneWidth])

  useEffect(() => {
    if (!resizingPane || isMobile) {
      return
    }
    const onMouseMove = (event: MouseEvent) => {
      const origin = dragOriginRef.current
      if (!origin) {
        return
      }
      const delta = event.clientX - origin.x
      if (resizingPane === 'left') {
        setLeftPaneWidth(clampPaneWidth(origin.width + delta, 240, 520))
      } else {
        setRightPaneWidth(clampPaneWidth(origin.width - delta, 280, 560))
      }
    }
    const onMouseUp = () => {
      dragOriginRef.current = null
      setResizingPane(null)
    }
    window.addEventListener('mousemove', onMouseMove)
    window.addEventListener('mouseup', onMouseUp)
    return () => {
      window.removeEventListener('mousemove', onMouseMove)
      window.removeEventListener('mouseup', onMouseUp)
    }
  }, [isMobile, resizingPane])

  const loadProjectData = useCallback(async () => {
    if (!projectID) {
      setTasks([])
      setSessions([])
      setLoading(false)
      return
    }
    const [taskList, sessionList] = await Promise.all([listProjectTasks<Task[]>(projectID), listSessions<Session[]>({ projectID })])
    setTasks(taskList)
    setSessions(sessionList)
    setLoading(false)
  }, [projectID])

  const refreshAll = useCallback(async () => {
    setError('')
    try {
      const projectList = await listProjects<Project[]>({ status: 'active' })
      setProjects(projectList)
      if (projectList.length === 0) {
        setProjectID('')
        setLoading(false)
        return
      }

      const hasCurrent = Boolean(projectID && projectList.some((project) => project.id === projectID))
      const nextID = hasCurrent ? projectID : projectList[0].id
      if (nextID !== projectID) {
        setProjectID(nextID)
        setSearchParams({ project: nextID }, { replace: true })
      }

      const projectSessions = await Promise.all(
        projectList.map(async (project) => ({
          projectID: project.id,
          sessions: await listSessions<Session[]>({ projectID: project.id }),
        })),
      )

      const windowsByProject: Record<string, string[]> = {}
      const statsByProject: Record<string, ProjectSessionStats> = {}
      for (const project of projectList) {
        windowsByProject[project.id] = []
        statsByProject[project.id] = { total: 0, working: 0, needsResponse: 0 }
      }

      for (const pair of projectSessions) {
        for (const session of pair.sessions ?? []) {
          statsByProject[pair.projectID].total += 1
          if (isWorkingSessionStatus(session.status)) {
            statsByProject[pair.projectID].working += 1
          }
          if (needsResponseStatus(session.status)) {
            statsByProject[pair.projectID].needsResponse += 1
          }
          if (session.tmux_window_id) {
            windowsByProject[pair.projectID].push(session.tmux_window_id)
          }
        }
      }

      setProjectSessionWindows(windowsByProject)
      setProjectSessionStats(statsByProject)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load projects')
      setLoading(false)
    }
  }, [projectID, setSearchParams])

  const refreshProjectData = useCallback(async () => {
    try {
      await loadProjectData()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load project context')
      setLoading(false)
    }
  }, [loadProjectData])

  const refreshExceptions = useCallback(async () => {
    if (!projectID) {
      setExceptions([])
      setExceptionCounts({ total: 0, open: 0, resolved: 0 })
      return
    }
    setExceptionsLoading(true)
    setExceptionsError('')
    try {
      const response = await listOrchestratorExceptions<OrchestratorExceptionListResponse>(projectID, 'open')
      setExceptions(response.items ?? [])
      setExceptionCounts(response.counts ?? { total: 0, open: 0, resolved: 0 })
    } catch (err) {
      setExceptionsError(err instanceof Error ? err.message : 'Failed to load exceptions')
    } finally {
      setExceptionsLoading(false)
    }
  }, [projectID])

  useEffect(() => {
    void refreshAll()
  }, [refreshAll])

  useEffect(() => {
    void refreshProjectData()
  }, [refreshProjectData])

  useEffect(() => {
    void refreshExceptions()
  }, [refreshExceptions])

  useEffect(() => {
    if (shouldRefreshFromOrchestratorEvent(orchestrator.lastEvent)) {
      void refreshProjectData()
      void refreshExceptions()
    }
  }, [orchestrator.lastEvent, refreshExceptions, refreshProjectData])

  useEffect(() => {
    if (!app.lastMessage || app.lastMessage.type !== 'project_event' || app.lastMessage.project_id !== projectID) {
      return
    }
    void refreshProjectData()
  }, [app.lastMessage, projectID, refreshProjectData])

  useEffect(() => {
    setProgressReport(null)
    setReportUpdatedAt(null)
    setReportError('')
    setReportLoading(false)
    setExceptions([])
    setExceptionCounts({ total: 0, open: 0, resolved: 0 })
    setExceptionsError('')
    setExceptionsLoading(false)
  }, [projectID])

  const selectedProject = useMemo(() => projects.find((project) => project.id === projectID) ?? null, [projectID, projects])

  const sessionsByTask = useMemo(() => {
    const mapping: Record<string, Session> = {}
    for (const session of sessions) {
      if (session.task_id) {
        mapping[session.task_id] = session
      }
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

  const taskByID = useMemo(() => {
    const mapping: Record<string, Task> = {}
    for (const task of tasks) {
      mapping[task.id] = task
    }
    return mapping
  }, [tasks])

  const projectUnreadCounts = useMemo<Record<string, number>>(() => {
    const unread: Record<string, number> = {}
    for (const [project, windows] of Object.entries(projectSessionWindows)) {
      unread[project] = windows.reduce((sum, windowID) => sum + (app.unreadByWindow[windowID] ?? 0), 0)
    }
    return unread
  }, [app.unreadByWindow, projectSessionWindows])

  const roadmapStages = useMemo(() => {
    const order = ['brainstorm', 'plan', 'build', 'test', 'summarize']
    const phase = (readAsString(progressReport?.phase) || 'plan').toLowerCase()
    const activeIndex = Math.max(order.indexOf(phase), 0)
    return order.map((stage, idx) => {
      const state = idx < activeIndex ? 'done' : idx === activeIndex ? 'active' : 'pending'
      return { stage, state }
    })
  }, [progressReport])

  const agentSessions = useMemo(() => {
    const list = [...sessions]
    list.sort((left, right) => {
      const leftOrchestrator = isOrchestratorSession(left) ? 1 : 0
      const rightOrchestrator = isOrchestratorSession(right) ? 1 : 0
      if (leftOrchestrator !== rightOrchestrator) {
        return rightOrchestrator - leftOrchestrator
      }
      return left.role.localeCompare(right.role)
    })
    return list
  }, [sessions])

  const chatMessages = useMemo<SessionMessage[]>(() => {
    if (!progressReport || !reportUpdatedAt) {
      return orchestrator.messages
    }
    return [
      ...orchestrator.messages,
      {
        id: `progress-report-${reportUpdatedAt}`,
        text: `Progress report (${new Date(reportUpdatedAt).toLocaleTimeString()}):\n${buildReportSummaryText(progressReport)}`,
        className: 'prompt',
        role: 'system',
        kind: 'text',
        timestamp: reportUpdatedAt,
      },
    ]
  }, [orchestrator.messages, progressReport, reportUpdatedAt])

  const tasksByStatus = useMemo(() => {
    const grouped: Record<string, number> = {}
    for (const task of tasks) {
      const key = (task.status || 'pending').toLowerCase()
      grouped[key] = (grouped[key] ?? 0) + 1
    }
    return Object.entries(grouped).sort(([a], [b]) => a.localeCompare(b))
  }, [tasks])

  const onSelectProject = (nextProjectID: string) => {
    setProjectID(nextProjectID)
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

  const openSession = (session: Session) => {
    if (session.tmux_window_id) {
      app.setActiveWindow(session.tmux_window_id)
    }
    navigate('/sessions')
  }

  const requestProgressReport = useCallback(async () => {
    if (!projectID) {
      return
    }
    setReportLoading(true)
    setReportError('')
    try {
      const report = await getOrchestratorReport<OrchestratorProgressReport>(projectID)
      setProgressReport(report)
      setReportUpdatedAt(Date.now())
    } catch (err) {
      setReportError(err instanceof Error ? err.message : 'Failed to get progress report')
    } finally {
      setReportLoading(false)
    }
  }, [projectID])

  const confirmDeleteProject = useCallback(async () => {
    if (!selectedProject) {
      return
    }
    setDeletingProject(true)
    setDeleteError('')
    try {
      await deleteProject(selectedProject.id)
      setDeleteModalOpen(false)
      await refreshAll()
      await refreshProjectData()
    } catch (err) {
      setDeleteError(err instanceof Error ? err.message : 'Failed to delete project')
    } finally {
      setDeletingProject(false)
    }
  }, [refreshAll, refreshProjectData, selectedProject])

  const resolveException = useCallback(
    async (exceptionID: string) => {
      if (!projectID || !exceptionID) {
        return
      }
      try {
        await resolveOrchestratorException(projectID, exceptionID, 'resolved')
        await refreshExceptions()
      } catch (err) {
        setExceptionsError(err instanceof Error ? err.message : 'Failed to resolve exception')
      }
    },
    [projectID, refreshExceptions],
  )

  const infoPanel = (
    <div className="pm-info-panel">
      <section className="pm-project-detail">
        <div className="pm-panel-header">
          <h3>
            <MessageSquareText size={14} /> Project Roadmap
          </h3>
        </div>
        <div className="pm-project-detail-body">
          <div className="pm-roadmap">
            {roadmapStages.map((item) => (
              <div className={`pm-roadmap-stage ${item.state}`.trim()} key={item.stage}>
                <span>{item.stage}</span>
              </div>
            ))}
          </div>
        </div>
      </section>

      <section className="pm-project-detail">
        <div className="pm-panel-header">
          <h3>
            <FolderOpen size={14} /> Project Details
          </h3>
          <button className="secondary-btn" onClick={() => setProjectInfoCollapsed((prev) => !prev)} type="button">
            {projectInfoCollapsed ? <ChevronRight size={14} /> : <ChevronDown size={14} />}
          </button>
        </div>
        {!projectInfoCollapsed && selectedProject && (
          <div className="pm-project-detail-body">
            <div className="pm-project-detail-meta">
              <p>Name: {selectedProject.name}</p>
              <p>Repository: {selectedProject.repo_path}</p>
              <p>Status: {selectedProject.status}</p>
              <p>Playbook: {selectedProject.playbook || 'auto'}</p>
            </div>
          </div>
        )}
      </section>

      <section className="pm-report-panel">
        <div className="pm-panel-header">
          <h3>Progress Report</h3>
          <div className="pm-header-actions">
            <button className="secondary-btn" onClick={() => void requestProgressReport()} disabled={reportLoading || !selectedProject} type="button">
              {reportLoading ? 'Loading…' : 'Refresh'}
            </button>
            <button className="secondary-btn" onClick={() => setProgressCollapsed((prev) => !prev)} type="button">
              {progressCollapsed ? <ChevronRight size={14} /> : <ChevronDown size={14} />}
            </button>
          </div>
        </div>
        {!progressCollapsed && (
          <div className="pm-report-panel-body">
            <small>{reportUpdatedAt ? `Updated ${new Date(reportUpdatedAt).toLocaleTimeString()}` : 'No report yet'}</small>
            {reportError && <p className="dashboard-error">{reportError}</p>}
            {!reportError && progressReport && <p>{buildReportSummaryText(progressReport)}</p>}
            {!reportError && !progressReport && <p className="empty-text">Press refresh to request a summary.</p>}
          </div>
        )}
      </section>

      <section className="pm-report-panel">
        <div className="pm-panel-header">
          <h3>Exception Inbox</h3>
          <div className="pm-header-actions">
            <button className="secondary-btn" onClick={() => void refreshExceptions()} disabled={exceptionsLoading || !selectedProject} type="button">
              {exceptionsLoading ? 'Loading…' : 'Refresh'}
            </button>
            <button className="secondary-btn" onClick={() => setExceptionsCollapsed((prev) => !prev)} type="button">
              {exceptionsCollapsed ? <ChevronRight size={14} /> : <ChevronDown size={14} />}
            </button>
          </div>
        </div>
        {!exceptionsCollapsed && (
          <div className="pm-report-panel-body">
            <small>
              Open {exceptionCounts.open} / Total {exceptionCounts.total}
            </small>
            {exceptionsError && <p className="dashboard-error">{exceptionsError}</p>}
            {!exceptionsError && exceptions.length === 0 && <p className="empty-text">No open exceptions.</p>}
            {!exceptionsError && exceptions.length > 0 && (
              <ul className="project-detail-task-list">
                {exceptions.slice(0, 6).map((item) => (
                  <li key={item.id}>
                    <div>
                      <strong>{item.category}</strong>
                      <p>{item.message}</p>
                    </div>
                    <button className="secondary-btn" onClick={() => void resolveException(item.id)} type="button">
                      Resolve
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </div>
        )}
      </section>

      <section className="pm-project-detail">
        <div className="pm-panel-header">
          <h3>Task Group</h3>
          <button className="secondary-btn" onClick={() => setTaskGroupCollapsed((prev) => !prev)} type="button">
            {taskGroupCollapsed ? <ChevronRight size={14} /> : <ChevronDown size={14} />}
          </button>
        </div>
        {!taskGroupCollapsed && (
          <div className="pm-project-detail-body">
            <div className="pm-project-status-grid">
              {tasksByStatus.map(([status, count]) => (
                <div className="pm-status-card" key={status}>
                  <strong>{count}</strong>
                  <small>{status}</small>
                </div>
              ))}
            </div>
            <TaskDAG tasks={tasks} sessionsByTask={sessionsByTask} onOpenTask={openTaskSession} />
          </div>
        )}
      </section>
    </div>
  )

  const startPaneResize = (pane: 'left' | 'right') => (event: ReactMouseEvent<HTMLDivElement>) => {
    if (isMobile) {
      return
    }
    event.preventDefault()
    dragOriginRef.current = {
      x: event.clientX,
      width: pane === 'left' ? leftPaneWidth : rightPaneWidth,
    }
    setResizingPane(pane)
  }

  const workspaceStyle = isMobile
    ? undefined
    : ({
        gridTemplateColumns: `${leftPaneCollapsed ? 0 : leftPaneWidth}px ${leftPaneCollapsed ? 0 : 8}px minmax(0, 1fr) ${
          rightPaneCollapsed ? 0 : 8
        }px ${rightPaneCollapsed ? 0 : rightPaneWidth}px`,
      } as const)

  return (
    <section className="pm-chat-page">
      <div className={`pm-workspace-layout ${resizingPane ? 'resizing' : ''}`.trim()} style={workspaceStyle}>
        {!isMobile && (
          <aside className={`pm-workspace-left ${leftPaneCollapsed ? 'collapsed' : ''}`.trim()}>
            <section className="pm-chat-project-list">
              <div className="pm-panel-header">
                <h3>Projects</h3>
                <small>{projects.length}</small>
              </div>
              <div className="pm-project-items">
                {projects.map((project) => {
                  const active = project.id === projectID
                  const unread = projectUnreadCounts[project.id] ?? 0
                  const stats = projectSessionStats[project.id] ?? { total: 0, working: 0, needsResponse: 0 }
                  return (
                    <button
                      className={`pm-project-item ${active ? 'active' : ''}`.trim()}
                      key={project.id}
                      onClick={() => onSelectProject(project.id)}
                      type="button"
                    >
                      <span className="pm-project-item-name">{project.name}</span>
                      <span className="pm-project-item-meta">
                        {project.status} · {stats.working}/{stats.total} working
                      </span>
                      {stats.needsResponse > 0 && <span className="pm-project-item-alert">needs {stats.needsResponse}</span>}
                      {unread > 0 && <span className="pm-notification-badge">{unread}</span>}
                    </button>
                  )
                })}
                {projects.length === 0 && <p className="empty-view">No projects</p>}
              </div>
            </section>

            <section className="pm-project-detail">
              <div className="pm-panel-header">
                <h3>Agents</h3>
                <button className="secondary-btn" onClick={() => navigate('/sessions')} type="button">
                  Open Sessions
                </button>
              </div>
              <div className="pm-project-items">
                {agentSessions.length === 0 && <p className="empty-view">No active agents.</p>}
                {agentSessions.map((session) => {
                  const task = session.task_id ? taskByID[session.task_id] : null
                  const hasNeedsResponse = needsResponseStatus(session.status)
                  return (
                    <button className="pm-project-session-item" key={session.id} onClick={() => openSession(session)} type="button">
                      <div className="pm-project-session-meta">
                        <strong>{session.role || session.agent_type}</strong>
                        <small>{task?.title || session.agent_type}</small>
                      </div>
                      <div className="pm-project-session-status">
                        <small>{session.status}</small>
                        {hasNeedsResponse ? <span className="pm-project-item-alert">needs reply</span> : null}
                      </div>
                    </button>
                  )
                })}
              </div>
            </section>
          </aside>
        )}
        {!isMobile && (
          <div
            aria-hidden={leftPaneCollapsed}
            className={`pm-pane-divider ${leftPaneCollapsed ? 'hidden' : ''}`.trim()}
            onMouseDown={startPaneResize('left')}
            role="separator"
          />
        )}

        <section className="pm-workspace-center">
          <div className="pm-chat-header">
            {isMobile && (
              <div className="project-selector">
                <span>Project</span>
                <select onChange={(event) => onSelectProject(event.target.value)} value={projectID}>
                  {projects.map((project) => (
                    <option key={project.id} value={project.id}>
                      {project.name}
                    </option>
                  ))}
                </select>
              </div>
            )}
            {selectedProject && (
              <div className="pm-pane-toggle">
                {!isMobile && (
                  <>
                    <button className="secondary-btn" onClick={() => setLeftPaneCollapsed((prev) => !prev)} type="button">
                      {leftPaneCollapsed ? 'Show Projects' : 'Hide Projects'}
                    </button>
                    <button className="secondary-btn" onClick={() => setRightPaneCollapsed((prev) => !prev)} type="button">
                      {rightPaneCollapsed ? 'Show Details' : 'Hide Details'}
                    </button>
                  </>
                )}
                <button className="secondary-btn" onClick={() => setDemandPoolOpen(true)} type="button">
                  Demand Pool
                </button>
                {isMobile && (
                  <button className="secondary-btn" onClick={() => setShowMobileInfo(true)} type="button">
                    Project Info
                  </button>
                )}
                <button className="secondary-btn danger-btn" onClick={() => setDeleteModalOpen(true)} type="button">
                  Delete Project
                </button>
              </div>
            )}
          </div>

          {loading && <p className="empty-view">Loading PM chat context...</p>}
          {error && <p className="dashboard-error">{error}</p>}

          {!loading && !error && selectedProject && (
            <ChatPanel
              messages={chatMessages}
              taskLinks={taskLinks}
              isStreaming={orchestrator.isStreaming}
              connectionStatus={orchestrator.connectionStatus}
              onSend={orchestrator.send}
              onTaskClick={openTaskSession}
              onReportProgress={requestProgressReport}
              isFetchingReport={reportLoading}
            />
          )}

          {!loading && !error && !selectedProject && (
            <div className="empty-view">
              <MessageSquareText size={16} /> Select a project to open PM Chat
            </div>
          )}
        </section>
        {!isMobile && (
          <div
            aria-hidden={rightPaneCollapsed}
            className={`pm-pane-divider ${rightPaneCollapsed ? 'hidden' : ''}`.trim()}
            onMouseDown={startPaneResize('right')}
            role="separator"
          />
        )}

        {!isMobile && <aside className={`pm-workspace-right ${rightPaneCollapsed ? 'collapsed' : ''}`.trim()}>{infoPanel}</aside>}
      </div>

      <Modal onClose={() => setShowMobileInfo(false)} open={showMobileInfo} title="Project Info">
        <div className="modal-form">
          {selectedProject ? infoPanel : <p className="empty-text">No project selected.</p>}
          <div className="settings-actions">
            <button className="secondary-btn" onClick={() => setShowMobileInfo(false)} type="button">
              Close
            </button>
          </div>
        </div>
      </Modal>

      <Modal onClose={() => setDemandPoolOpen(false)} open={demandPoolOpen} title="Demand Pool">
        <div className="modal-form">
          {selectedProject ? (
            <DemandPool embedded projectID={selectedProject.id} projectName={selectedProject.name} />
          ) : (
            <p className="empty-text">Select a project first.</p>
          )}
        </div>
      </Modal>

      <Modal onClose={() => setDeleteModalOpen(false)} open={deleteModalOpen} title="Delete Project">
        <div className="modal-form">
          <p className="empty-text">
            Delete project <strong>{selectedProject?.name ?? ''}</strong>? This will archive it and hide it from active workflow views.
          </p>
          {deleteError && <p className="dashboard-error">{deleteError}</p>}
          <div className="settings-actions">
            <button className="secondary-btn" disabled={deletingProject} onClick={() => setDeleteModalOpen(false)} type="button">
              Cancel
            </button>
            <button className="secondary-btn danger-btn" disabled={deletingProject} onClick={() => void confirmDeleteProject()} type="button">
              {deletingProject ? 'Deleting…' : 'Delete'}
            </button>
          </div>
        </div>
      </Modal>
    </section>
  )
}

interface PanePreference {
  leftWidth: number
  rightWidth: number
  leftCollapsed: boolean
  rightCollapsed: boolean
}

const panePreferenceStorageKey = 'agenterm:pm-workspace:pane:v1'

function clampPaneWidth(value: number, min: number, max: number): number {
  if (!Number.isFinite(value)) {
    return min
  }
  return Math.min(max, Math.max(min, Math.round(value)))
}

function loadPanePreference(): PanePreference {
  const fallback: PanePreference = {
    leftWidth: 300,
    rightWidth: 360,
    leftCollapsed: false,
    rightCollapsed: false,
  }
  if (typeof window === 'undefined') {
    return fallback
  }
  try {
    const raw = window.localStorage.getItem(panePreferenceStorageKey)
    if (!raw) {
      return fallback
    }
    const decoded = JSON.parse(raw) as Partial<PanePreference>
    return {
      leftWidth: clampPaneWidth(Number(decoded.leftWidth ?? fallback.leftWidth), 240, 520),
      rightWidth: clampPaneWidth(Number(decoded.rightWidth ?? fallback.rightWidth), 280, 560),
      leftCollapsed: Boolean(decoded.leftCollapsed),
      rightCollapsed: Boolean(decoded.rightCollapsed),
    }
  } catch {
    return fallback
  }
}

function savePanePreference(preference: PanePreference): void {
  if (typeof window === 'undefined') {
    return
  }
  try {
    window.localStorage.setItem(panePreferenceStorageKey, JSON.stringify(preference))
  } catch {
    // Ignore storage failures so workspace remains usable in restrictive browser modes.
  }
}

function readAsString(value: unknown): string {
  return typeof value === 'string' ? value : ''
}

function readAsNumber(value: unknown): number {
  if (typeof value === 'number' && Number.isFinite(value)) {
    return value
  }
  return 0
}

function readAsStringArray(value: unknown): string[] {
  if (!Array.isArray(value)) {
    return []
  }
  return value.filter((item): item is string => typeof item === 'string')
}

function buildReportSummaryText(report: OrchestratorProgressReport): string {
  const phase = readAsString(report.phase) || 'unknown'
  const queueDepth = readAsNumber(report.queue_depth)
  const activeSessions = readAsNumber(report.active_sessions)
  const pendingTasks = readAsNumber(report.pending_tasks)
  const completedTasks = readAsNumber(report.completed_tasks)
  const reviewState = readAsString(report.review_state) || 'not_started'
  const openReviewIssues = readAsNumber(report.open_review_issues_total)
  const blockers = readAsStringArray(report.blockers)

  const lines = [
    `phase=${phase}`,
    `queue=${queueDepth}`,
    `sessions_active=${activeSessions}`,
    `tasks_pending=${pendingTasks}`,
    `tasks_done=${completedTasks}`,
    `review=${reviewState}`,
    `open_review_issues=${openReviewIssues}`,
  ]
  if (blockers.length > 0) {
    lines.push(`blockers=${blockers.join('; ')}`)
  }
  return lines.join('\n')
}

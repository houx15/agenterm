import { useCallback, useEffect, useMemo, useState } from 'react'
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
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [projectInfoCollapsed, setProjectInfoCollapsed] = useState(false)
  const [progressCollapsed, setProgressCollapsed] = useState(false)
  const [taskGroupCollapsed, setTaskGroupCollapsed] = useState(false)
  const [showMobileInfo, setShowMobileInfo] = useState(false)
  const [progressReport, setProgressReport] = useState<OrchestratorProgressReport | null>(null)
  const [reportUpdatedAt, setReportUpdatedAt] = useState<number | null>(null)
  const [reportLoading, setReportLoading] = useState(false)
  const [reportError, setReportError] = useState('')
  const [exceptions, setExceptions] = useState<OrchestratorExceptionItem[]>([])
  const [exceptionCounts, setExceptionCounts] = useState<{ total: number; open: number; resolved: number }>({ total: 0, open: 0, resolved: 0 })
  const [exceptionsLoading, setExceptionsLoading] = useState(false)
  const [exceptionsError, setExceptionsError] = useState('')
  const [exceptionsCollapsed, setExceptionsCollapsed] = useState(false)
  const [activePane, setActivePane] = useState<'execution' | 'demand'>('execution')
  const [deleteModalOpen, setDeleteModalOpen] = useState(false)
  const [deletingProject, setDeletingProject] = useState(false)
  const [deleteError, setDeleteError] = useState('')

  const [projectID, setProjectID] = useState(() => new URLSearchParams(window.location.search).get('project') ?? '')

  const orchestrator = useOrchestratorWS(projectID)

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
  }, [projectID, setSearchParams])

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
  const chatMessages = useMemo<SessionMessage[]>(() => {
    if (!progressReport || !reportUpdatedAt) {
      return orchestrator.messages
    }
    const summary = buildReportSummaryText(progressReport)
    return [
      ...orchestrator.messages,
      {
        id: `progress-report-${reportUpdatedAt}`,
        text: `Progress report (${new Date(reportUpdatedAt).toLocaleTimeString()}):\n${summary}`,
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

  const isMobile = window.innerWidth < 900

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
    void refreshExceptions()
  }, [refreshExceptions])

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
            <FolderOpen size={14} /> Project Detail
          </h3>
          <button className="secondary-btn" onClick={() => setProjectInfoCollapsed((prev) => !prev)} type="button">
            {projectInfoCollapsed ? <ChevronRight size={14} /> : <ChevronDown size={14} />}
          </button>
        </div>
        {!projectInfoCollapsed && selectedProject && (
          <div className="pm-project-detail-body">
            <div className="pm-project-detail-meta">
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

  return (
    <section className="pm-chat-page">
      <div className="pm-chat-layout-v2 pm-chat-layout-shell">
        {!isMobile && (
          <aside className="pm-chat-project-list">
            <div className="pm-panel-header">
              <h3>Projects</h3>
              <small>{projects.length}</small>
            </div>
            <div className="pm-project-items">
              {projects.map((project) => {
                const active = project.id === projectID
                return (
                  <button
                    className={`pm-project-item ${active ? 'active' : ''}`.trim()}
                    key={project.id}
                    onClick={() => onSelectProject(project.id)}
                    type="button"
                  >
                    <span className="pm-project-item-name">{project.name}</span>
                    <span className="pm-project-item-meta">{project.status}</span>
                  </button>
                )
              })}
              {projects.length === 0 && <p className="empty-view">No projects</p>}
            </div>
          </aside>
        )}

        <div className="pm-chat-content-area">
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
                <button
                  className={`secondary-btn ${activePane === 'execution' ? 'active' : ''}`.trim()}
                  onClick={() => setActivePane('execution')}
                  type="button"
                >
                  Execution
                </button>
                <button
                  className={`secondary-btn ${activePane === 'demand' ? 'active' : ''}`.trim()}
                  onClick={() => setActivePane('demand')}
                  type="button"
                >
                  Demand Pool
                </button>
                <button className="secondary-btn danger-btn" onClick={() => setDeleteModalOpen(true)} type="button">
                  Delete Project
                </button>
              </div>
            )}
            {activePane === 'execution' && isMobile && selectedProject && (
              <button className="secondary-btn" onClick={() => setShowMobileInfo(true)} type="button">
                <span>Project Info</span>
              </button>
            )}
          </div>

          {loading && <p className="empty-view">Loading PM chat context...</p>}
          {error && <p className="dashboard-error">{error}</p>}

          {!loading && !error && selectedProject && activePane === 'execution' && (
            <div className="pm-execution-layout">
              {!isMobile && infoPanel}
              <div className="pm-chat-only">
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
              </div>
            </div>
          )}

          {!loading && !error && selectedProject && activePane === 'demand' && (
            <DemandPool embedded projectID={selectedProject.id} projectName={selectedProject.name} />
          )}

          {!loading && !error && !selectedProject && (
            <div className="empty-view">
              <MessageSquareText size={16} /> Select a project to open PM Chat
            </div>
          )}
        </div>
      </div>

      <Modal onClose={() => setShowMobileInfo(false)} open={showMobileInfo} title="Project Info">
        <div className="modal-form">
          {selectedProject ? infoPanel : <p className="empty-text">No project selected.</p>}
          <div className="settings-actions">
            <button className="secondary-btn" onClick={() => setShowMobileInfo(false)} type="button">
              <span>Close</span>
            </button>
          </div>
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

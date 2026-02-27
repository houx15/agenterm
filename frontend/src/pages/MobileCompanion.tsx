import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { AlertTriangle, BellRing, CheckCircle2, ClipboardList, LoaderCircle, MessageSquareText, RefreshCw } from 'lucide-react'
import { getOrchestratorReport, listOrchestratorExceptions, listProjectTasks, listProjects, listSessions, resolveOrchestratorException } from '../api/client'
import type { OrchestratorExceptionItem, OrchestratorProgressReport, Project, Session, Task } from '../api/types'
import type { SessionMessage } from '../components/ChatMessage'
import { useOrchestratorWS } from '../hooks/useOrchestratorWS'
import { useAppContext } from '../App'

type MobileTab = 'projects' | 'approvals' | 'reports' | 'alerts'

function readAsNumber(value: unknown): number {
  if (typeof value === 'number' && Number.isFinite(value)) {
    return value
  }
  if (typeof value === 'string') {
    const parsed = Number(value)
    if (Number.isFinite(parsed)) {
      return parsed
    }
  }
  return 0
}

function readAsString(value: unknown): string {
  return typeof value === 'string' ? value.trim() : ''
}

function readAsStringArray(value: unknown): string[] {
  if (!Array.isArray(value)) {
    return []
  }
  return value.map((item) => (typeof item === 'string' ? item.trim() : '')).filter((item) => Boolean(item))
}

function looksLikeApprovalReply(text: string): boolean {
  const trimmed = text.trim().toLowerCase()
  if (!trimmed) {
    return false
  }
  const tokens = ['confirm', 'approved', 'approve', 'cancel', 'reject', 'modify', '同意', '确认', '取消', '拒绝', '修改']
  return tokens.some((token) => trimmed.includes(token))
}

function resolvePendingConfirmations(messages: SessionMessage[]): SessionMessage[] {
  const queue: SessionMessage[] = []
  for (const message of messages) {
    if (message.status === 'confirmation' && !message.isUser) {
      queue.push(message)
      continue
    }
    if (message.isUser && queue.length > 0 && looksLikeApprovalReply(message.text)) {
      queue.shift()
    }
  }
  return [...queue].reverse()
}

function formatTime(value: number): string {
  if (!value) {
    return '--'
  }
  return new Date(value).toLocaleTimeString()
}

export default function MobileCompanion() {
  const app = useAppContext()
  const [tab, setTab] = useState<MobileTab>('approvals')
  const [projects, setProjects] = useState<Project[]>([])
  const [selectedProjectID, setSelectedProjectID] = useState<string>('')
  const [report, setReport] = useState<OrchestratorProgressReport | null>(null)
  const [exceptions, setExceptions] = useState<OrchestratorExceptionItem[]>([])
  const [tasks, setTasks] = useState<Task[]>([])
  const [sessions, setSessions] = useState<Session[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [updatedAt, setUpdatedAt] = useState(0)
  const [resolvingExceptionID, setResolvingExceptionID] = useState('')
  const prevAlertCountRef = useRef(0)

  const orchestrator = useOrchestratorWS(selectedProjectID)

  const selectedProject = useMemo(
    () => projects.find((project) => project.id === selectedProjectID) ?? null,
    [projects, selectedProjectID],
  )

  const refreshProjects = useCallback(async () => {
    const items = await listProjects<Project[]>()
    setProjects(items)
    setSelectedProjectID((current) => {
      if (current && items.some((item) => item.id === current)) {
        return current
      }
      return items[0]?.id ?? ''
    })
  }, [])

  const refreshSelectedProject = useCallback(async () => {
    if (!selectedProjectID) {
      setReport(null)
      setExceptions([])
      setTasks([])
      setSessions([])
      return
    }
    setLoading(true)
    setError('')
    try {
      const [nextReport, nextExceptions, nextTasks, nextSessions] = await Promise.all([
        getOrchestratorReport<OrchestratorProgressReport>(selectedProjectID),
        listOrchestratorExceptions<{ items: OrchestratorExceptionItem[] }>(selectedProjectID, 'open'),
        listProjectTasks<Task[]>(selectedProjectID),
        listSessions<Session[]>({ projectID: selectedProjectID }),
      ])
      setReport(nextReport)
      setExceptions(nextExceptions.items ?? [])
      setTasks(nextTasks)
      setSessions(nextSessions)
      setUpdatedAt(Date.now())
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to refresh project status')
    } finally {
      setLoading(false)
    }
  }, [selectedProjectID])

  useEffect(() => {
    void refreshProjects()
  }, [refreshProjects])

  useEffect(() => {
    void refreshSelectedProject()
  }, [refreshSelectedProject])

  useEffect(() => {
    if (!selectedProjectID) {
      return
    }
    const timer = window.setInterval(() => {
      void refreshSelectedProject()
    }, 15000)
    return () => window.clearInterval(timer)
  }, [refreshSelectedProject, selectedProjectID])

  useEffect(() => {
    if (app.lastMessage?.type !== 'project_event') {
      return
    }
    if (app.lastMessage.project_id !== selectedProjectID) {
      return
    }
    void refreshSelectedProject()
  }, [app.lastMessage, refreshSelectedProject, selectedProjectID])

  const pendingApprovals = useMemo(() => resolvePendingConfirmations(orchestrator.messages), [orchestrator.messages])

  const blockerAlerts = useMemo(() => {
    const blockers = readAsStringArray(report?.blockers)
    const waitingSessions = sessions.filter((session) => {
      const status = readAsString(session.status).toLowerCase()
      return status === 'waiting_review' || status === 'human_takeover' || status === 'blocked'
    })
    return {
      blockers,
      waitingSessions,
    }
  }, [report, sessions])

  const totalAlertCount = pendingApprovals.length + exceptions.length + blockerAlerts.blockers.length + blockerAlerts.waitingSessions.length

  useEffect(() => {
    const base = 'agenterm mobile'
    document.title = totalAlertCount > 0 ? `(${totalAlertCount}) ${base}` : base
  }, [totalAlertCount])

  useEffect(() => {
    if (!('Notification' in window)) {
      return
    }
    if (Notification.permission === 'default') {
      void Notification.requestPermission()
    }
  }, [])

  useEffect(() => {
    const previous = prevAlertCountRef.current
    if (totalAlertCount <= previous) {
      prevAlertCountRef.current = totalAlertCount
      return
    }
    if (!document.hidden) {
      prevAlertCountRef.current = totalAlertCount
      return
    }
    if (!('Notification' in window) || Notification.permission !== 'granted') {
      prevAlertCountRef.current = totalAlertCount
      return
    }
    const projectName = selectedProject?.name ?? 'current project'
    const body = `${totalAlertCount} pending approvals or alerts in ${projectName}.`
    // Browser notifications are in-app only and respect local permission.
    new Notification('agenterm action required', { body })
    prevAlertCountRef.current = totalAlertCount
  }, [selectedProject?.name, totalAlertCount])

  const sendQuickReply = useCallback(
    (message: string) => {
      const ok = orchestrator.send(message)
      if (!ok) {
        setError('Orchestrator socket is offline. Reconnect and try again.')
      }
    },
    [orchestrator],
  )

  const resolveException = useCallback(
    async (exceptionID: string) => {
      if (!selectedProjectID) {
        return
      }
      setResolvingExceptionID(exceptionID)
      try {
        await resolveOrchestratorException(selectedProjectID, exceptionID, 'resolved')
        await refreshSelectedProject()
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to resolve exception')
      } finally {
        setResolvingExceptionID('')
      }
    },
    [refreshSelectedProject, selectedProjectID],
  )

  const recentAssistantMessages = useMemo(() => {
    return orchestrator.messages
      .filter((message) => !message.isUser && (message.discussion || message.text))
      .slice(-8)
      .reverse()
  }, [orchestrator.messages])

  return (
    <div className="mobile-companion-shell">
      <header className="mobile-companion-header">
        <div>
          <strong>Mobile Companion</strong>
          <small>{selectedProject ? selectedProject.name : 'No project selected'}</small>
        </div>
        <div className="mobile-header-actions">
          <a className="secondary-btn" href="/">
            Desktop
          </a>
          <button className="secondary-btn" disabled={loading} onClick={() => void refreshSelectedProject()} type="button">
            {loading ? <LoaderCircle size={14} className="spin" /> : <RefreshCw size={14} />}
            <span>{loading ? 'Syncing' : 'Sync'}</span>
          </button>
        </div>
      </header>

      <section className="mobile-project-picker">
        {projects.map((project) => (
          <button
            key={project.id}
            className={`mobile-project-chip ${project.id === selectedProjectID ? 'active' : ''}`.trim()}
            onClick={() => setSelectedProjectID(project.id)}
            type="button"
          >
            <span>{project.name}</span>
            <small>{project.status}</small>
          </button>
        ))}
        {projects.length === 0 && <p className="empty-text">No projects found.</p>}
      </section>

      {error && <p className="dashboard-error">{error}</p>}

      <section className="mobile-tab-content">
        {tab === 'projects' && (
          <div className="mobile-panel-list">
            <article className="mobile-card">
              <h3>Project Status</h3>
              <p>{selectedProject ? selectedProject.repo_path : 'Select a project to view details.'}</p>
              <ul className="mobile-kv-list">
                <li>
                  <span>Phase</span>
                  <strong>{readAsString(report?.phase) || 'unknown'}</strong>
                </li>
                <li>
                  <span>Pending Tasks</span>
                  <strong>{readAsNumber(report?.pending_tasks)}</strong>
                </li>
                <li>
                  <span>Completed Tasks</span>
                  <strong>{readAsNumber(report?.completed_tasks)}</strong>
                </li>
                <li>
                  <span>Active Sessions</span>
                  <strong>{readAsNumber(report?.active_sessions)}</strong>
                </li>
              </ul>
            </article>
            <article className="mobile-card">
              <h3>Session Snapshot</h3>
              {sessions.length === 0 && <p className="empty-text">No active sessions.</p>}
              {sessions.length > 0 && (
                <ul className="mobile-event-list">
                  {sessions.slice(0, 8).map((session) => (
                    <li key={session.id}>
                      <strong>{session.role || session.agent_type}</strong>
                      <span>{session.status}</span>
                    </li>
                  ))}
                </ul>
              )}
            </article>
          </div>
        )}

        {tab === 'approvals' && (
          <div className="mobile-panel-list">
            <article className="mobile-card">
              <h3>Pending Confirmations</h3>
              {pendingApprovals.length === 0 && <p className="empty-text">No pending confirmation prompts.</p>}
              {pendingApprovals.length > 0 && (
                <ul className="mobile-event-list">
                  {pendingApprovals.map((message) => (
                    <li key={message.id ?? String(message.timestamp)}>
                      <p>{message.confirmationPrompt || message.discussion || message.text}</p>
                      <small>{formatTime(message.timestamp)}</small>
                    </li>
                  ))}
                </ul>
              )}
              <div className="mobile-action-row">
                <button className="primary-btn" onClick={() => sendQuickReply('Approved. Execute now.')} type="button">
                  Confirm
                </button>
                <button className="secondary-btn" onClick={() => sendQuickReply('Modify plan before execution.')} type="button">
                  Modify
                </button>
                <button className="secondary-btn danger-btn" onClick={() => sendQuickReply('Cancel execution for now.')} type="button">
                  Cancel
                </button>
              </div>
            </article>

            <article className="mobile-card">
              <h3>Exception Inbox</h3>
              {exceptions.length === 0 && <p className="empty-text">No open exceptions.</p>}
              {exceptions.length > 0 && (
                <ul className="mobile-event-list">
                  {exceptions.map((item) => (
                    <li key={item.id}>
                      <p>{item.message}</p>
                      <small>
                        {item.category} · {item.severity}
                      </small>
                      <button
                        className="secondary-btn"
                        disabled={resolvingExceptionID === item.id}
                        onClick={() => void resolveException(item.id)}
                        type="button"
                      >
                        {resolvingExceptionID === item.id ? 'Resolving…' : 'Mark resolved'}
                      </button>
                    </li>
                  ))}
                </ul>
              )}
            </article>
          </div>
        )}

        {tab === 'reports' && (
          <div className="mobile-panel-list">
            <article className="mobile-card">
              <h3>Latest Report</h3>
              <p>
                Updated {updatedAt ? new Date(updatedAt).toLocaleTimeString() : '--'} · Review state {readAsString(report?.review_state) || 'not_started'}
              </p>
              <ul className="mobile-kv-list">
                <li>
                  <span>Queue Depth</span>
                  <strong>{readAsNumber(report?.queue_depth)}</strong>
                </li>
                <li>
                  <span>Open Review Issues</span>
                  <strong>{readAsNumber(report?.open_review_issues_total)}</strong>
                </li>
                <li>
                  <span>Finalize Ready</span>
                  <strong>{String(Boolean(report?.finalize_ready))}</strong>
                </li>
              </ul>
            </article>
            <article className="mobile-card">
              <h3>PM Timeline</h3>
              {recentAssistantMessages.length === 0 && <p className="empty-text">No timeline events yet.</p>}
              {recentAssistantMessages.length > 0 && (
                <ul className="mobile-event-list">
                  {recentAssistantMessages.map((message) => (
                    <li key={message.id ?? String(message.timestamp)}>
                      <p>{message.discussion || message.text}</p>
                      <small>{formatTime(message.timestamp)}</small>
                    </li>
                  ))}
                </ul>
              )}
            </article>
          </div>
        )}

        {tab === 'alerts' && (
          <div className="mobile-panel-list">
            <article className="mobile-card">
              <h3>Blockers</h3>
              {blockerAlerts.blockers.length === 0 && <p className="empty-text">No blocker text in current report.</p>}
              {blockerAlerts.blockers.length > 0 && (
                <ul className="mobile-event-list">
                  {blockerAlerts.blockers.map((item, index) => (
                    <li key={`${item}-${index}`}>{item}</li>
                  ))}
                </ul>
              )}
            </article>
            <article className="mobile-card">
              <h3>Needs Attention</h3>
              {blockerAlerts.waitingSessions.length === 0 && <p className="empty-text">No sessions waiting for response.</p>}
              {blockerAlerts.waitingSessions.length > 0 && (
                <ul className="mobile-event-list">
                  {blockerAlerts.waitingSessions.map((session) => (
                    <li key={session.id}>
                      <strong>{session.role || session.agent_type}</strong>
                      <span>{session.status}</span>
                    </li>
                  ))}
                </ul>
              )}
              {tasks.length > 0 && (
                <p className="mobile-footnote">
                  Total tasks {tasks.length}. Open exceptions {exceptions.length}. Pending confirmations {pendingApprovals.length}.
                </p>
              )}
            </article>
          </div>
        )}
      </section>

      <nav className="mobile-tab-nav">
        <button className={tab === 'projects' ? 'active' : ''} onClick={() => setTab('projects')} type="button">
          <ClipboardList size={14} />
          <span>Projects</span>
        </button>
        <button className={tab === 'approvals' ? 'active' : ''} onClick={() => setTab('approvals')} type="button">
          <CheckCircle2 size={14} />
          <span>Approvals</span>
          {pendingApprovals.length > 0 && <em>{pendingApprovals.length}</em>}
        </button>
        <button className={tab === 'reports' ? 'active' : ''} onClick={() => setTab('reports')} type="button">
          <MessageSquareText size={14} />
          <span>Reports</span>
        </button>
        <button className={tab === 'alerts' ? 'active' : ''} onClick={() => setTab('alerts')} type="button">
          {totalAlertCount > 0 ? <BellRing size={14} /> : <AlertTriangle size={14} />}
          <span>Alerts</span>
          {totalAlertCount > 0 && <em>{totalAlertCount}</em>}
        </button>
      </nav>
    </div>
  )
}

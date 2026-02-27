import { useCallback, useEffect, useMemo, useState } from 'react'
import { useAppContext } from '../App'
import {
  listProjects,
  listSessions,
  listProjectTasks,
  getOrchestratorReport,
  listOrchestratorExceptions,
  resolveOrchestratorException,
  getSessionOutput,
  deleteProject,
} from '../api/client'
import type {
  Project,
  Session,
  Task,
  OrchestratorProgressReport,
  OrchestratorExceptionItem,
  OrchestratorExceptionListResponse,
  ServerMessage,
} from '../api/types'
import { useOrchestratorWS } from '../hooks/useOrchestratorWS'
import { PanelRight, PanelLeft, Moon, Sun } from './Lucide'
import ProjectSidebar from './ProjectSidebar'
import TerminalGrid from './TerminalGrid'
import OrchestratorPanel from './OrchestratorPanel'
import SettingsModal from './SettingsModal'
import HomeView from './HomeView'
import ConnectModal from './ConnectModal'
import CreateProjectFlow from './CreateProjectFlow'
import Modal from './Modal'

// ---------------------------------------------------------------------------
// Helpers (migrated from PMChat.tsx / Sessions.tsx)
// ---------------------------------------------------------------------------

const terminalReplayStorageKey = 'agenterm:terminal:replay:v1'
const terminalReplayMaxChars = 120000

function isOrchestratorSession(session: Session): boolean {
  const role = (session.role || '').toLowerCase()
  const agent = (session.agent_type || '').toLowerCase()
  return role.includes('orchestrator') || role.includes('pm') || agent.includes('orchestrator')
}

function shouldRefreshFromOrchestratorEvent(
  event: { type: string } | null,
): boolean {
  if (!event) return false
  return event.type === 'tool_result' || event.type === 'done'
}

function loadTerminalReplay(): Record<string, string> {
  if (typeof window === 'undefined') return {}
  try {
    const raw = window.localStorage.getItem(terminalReplayStorageKey)
    if (!raw) return {}
    const parsed = JSON.parse(raw) as Record<string, unknown>
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return {}
    const out: Record<string, string> = {}
    for (const [key, value] of Object.entries(parsed)) {
      if (typeof value !== 'string' || !key.trim()) continue
      out[key] = value.slice(-terminalReplayMaxChars)
    }
    return out
  } catch {
    return {}
  }
}

function saveTerminalReplay(buffers: Record<string, string>): void {
  if (typeof window === 'undefined') return
  const compacted: Record<string, string> = {}
  for (const [key, value] of Object.entries(buffers)) {
    if (!key.trim() || !value) continue
    compacted[key] = value.slice(-terminalReplayMaxChars)
  }
  window.localStorage.setItem(terminalReplayStorageKey, JSON.stringify(compacted))
}

// ---------------------------------------------------------------------------
// Workspace Component
// ---------------------------------------------------------------------------

export default function Workspace() {
  const app = useAppContext()

  // ---- UI state -----------------------------------------------------------
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false)
  const [panelOpen, setPanelOpen] = useState(true)
  const [theme, setTheme] = useState<'dark' | 'light'>(() => {
    const stored = localStorage.getItem('agenterm:theme')
    if (stored === 'light' || stored === 'dark') return stored
    return 'dark'
  })

  // ---- Project state ------------------------------------------------------
  const [projects, setProjects] = useState<Project[]>([])
  const [activeProjectID, setActiveProjectID] = useState('')
  const [tasks, setTasks] = useState<Task[]>([])
  const [sessions, setSessions] = useState<Session[]>([])
  const [loading, setLoading] = useState(true)

  // ---- Terminal state (from Sessions.tsx) ----------------------------------
  const [rawBuffers, setRawBuffers] = useState<Record<string, string>>(() => loadTerminalReplay())
  const [activeWindowID, setActiveWindowID] = useState<string | null>(null)

  // ---- Orchestrator state (from PMChat.tsx) --------------------------------
  const [progressReport, setProgressReport] = useState<OrchestratorProgressReport | null>(null)
  const [reportUpdatedAt, setReportUpdatedAt] = useState<number | null>(null)
  const [reportLoading, setReportLoading] = useState(false)
  const [exceptions, setExceptions] = useState<OrchestratorExceptionItem[]>([])
  const [exceptionCounts, setExceptionCounts] = useState({ total: 0, open: 0, resolved: 0 })

  // ---- Modal state --------------------------------------------------------
  const [settingsOpen, setSettingsOpen] = useState(false)
  const [homeOpen, setHomeOpen] = useState(false)
  const [connectOpen, setConnectOpen] = useState(false)
  const [createProjectOpen, setCreateProjectOpen] = useState(false)
  const [deleteModalOpen, setDeleteModalOpen] = useState(false)
  const [deletingProject, setDeletingProject] = useState(false)
  const [deleteError, setDeleteError] = useState('')

  // ---- Orchestrator WS ----------------------------------------------------
  const orchestrator = useOrchestratorWS(activeProjectID)

  // ---- Project session windows (for unread counts) ------------------------
  const [projectSessionWindows, setProjectSessionWindows] = useState<Record<string, string[]>>({})

  // =========================================================================
  // Theme persistence
  // =========================================================================
  useEffect(() => {
    document.documentElement.setAttribute('data-theme', theme)
    localStorage.setItem('agenterm:theme', theme)
  }, [theme])

  // =========================================================================
  // Data fetching - refreshAll (from PMChat.tsx)
  // =========================================================================
  const refreshAll = useCallback(async () => {
    try {
      const projectList = await listProjects<Project[]>({ status: 'active' })
      setProjects(projectList)
      if (projectList.length === 0) {
        setActiveProjectID('')
        setLoading(false)
        return
      }

      const hasCurrent = Boolean(activeProjectID && projectList.some((p) => p.id === activeProjectID))
      const nextID = hasCurrent ? activeProjectID : projectList[0].id
      if (nextID !== activeProjectID) {
        setActiveProjectID(nextID)
      }

      // Collect window IDs per project for unread counting
      const projectSessions = await Promise.all(
        projectList.map(async (project) => ({
          projectID: project.id,
          sessions: await listSessions<Session[]>({ projectID: project.id }),
        })),
      )

      const windowsByProject: Record<string, string[]> = {}
      for (const project of projectList) {
        windowsByProject[project.id] = []
      }
      for (const pair of projectSessions) {
        for (const session of pair.sessions ?? []) {
          if (session.tmux_window_id) {
            windowsByProject[pair.projectID].push(session.tmux_window_id)
          }
        }
      }
      setProjectSessionWindows(windowsByProject)
    } catch {
      setLoading(false)
    }
  }, [activeProjectID])

  // =========================================================================
  // Data fetching - loadProjectData (from PMChat.tsx)
  // =========================================================================
  const loadProjectData = useCallback(async () => {
    if (!activeProjectID) {
      setTasks([])
      setSessions([])
      setLoading(false)
      return
    }
    try {
      const [taskList, sessionList] = await Promise.all([
        listProjectTasks<Task[]>(activeProjectID),
        listSessions<Session[]>({ projectID: activeProjectID }),
      ])
      setTasks(taskList)
      setSessions(sessionList)
    } catch {
      // keep workspace usable even if fetch fails
    } finally {
      setLoading(false)
    }
  }, [activeProjectID])

  // =========================================================================
  // Data fetching - refreshExceptions (from PMChat.tsx)
  // =========================================================================
  const refreshExceptions = useCallback(async () => {
    if (!activeProjectID) {
      setExceptions([])
      setExceptionCounts({ total: 0, open: 0, resolved: 0 })
      return
    }
    try {
      const response = await listOrchestratorExceptions<OrchestratorExceptionListResponse>(activeProjectID, 'open')
      setExceptions(response.items ?? [])
      setExceptionCounts(response.counts ?? { total: 0, open: 0, resolved: 0 })
    } catch {
      // keep workspace usable
    }
  }, [activeProjectID])

  // =========================================================================
  // Request orchestrator progress report (from PMChat.tsx)
  // =========================================================================
  const requestProgressReport = useCallback(async () => {
    if (!activeProjectID) return
    setReportLoading(true)
    try {
      const report = await getOrchestratorReport<OrchestratorProgressReport>(activeProjectID)
      setProgressReport(report)
      setReportUpdatedAt(Date.now())
    } catch {
      // keep workspace usable
    } finally {
      setReportLoading(false)
    }
  }, [activeProjectID])

  // =========================================================================
  // Resolve exception (from PMChat.tsx)
  // =========================================================================
  const resolveException = useCallback(
    async (exceptionID: string) => {
      if (!activeProjectID || !exceptionID) return
      try {
        await resolveOrchestratorException(activeProjectID, exceptionID, 'resolved')
        await refreshExceptions()
      } catch {
        // no-op
      }
    },
    [activeProjectID, refreshExceptions],
  )

  // =========================================================================
  // Delete project (from PMChat.tsx)
  // =========================================================================
  const confirmDeleteProject = useCallback(async () => {
    const project = projects.find((p) => p.id === activeProjectID)
    if (!project) return
    setDeletingProject(true)
    setDeleteError('')
    try {
      await deleteProject(project.id)
      setDeleteModalOpen(false)
      await refreshAll()
    } catch (err) {
      setDeleteError(err instanceof Error ? err.message : 'Failed to delete project')
    } finally {
      setDeletingProject(false)
    }
  }, [activeProjectID, projects, refreshAll])

  // =========================================================================
  // Terminal server message handler (from Sessions.tsx)
  // =========================================================================
  const handleServerMessage = useCallback((message: ServerMessage) => {
    if (message.type === 'terminal_data') {
      setRawBuffers((prev) => {
        const current = prev[message.window] ?? ''
        const next = current + (message.text ?? '')
        return {
          ...prev,
          [message.window]: next.length > terminalReplayMaxChars
            ? next.slice(next.length - terminalReplayMaxChars)
            : next,
        }
      })
    }
  }, [])

  // =========================================================================
  // Bootstrap terminal snapshot (from Sessions.tsx)
  // =========================================================================
  const refreshWindowSnapshot = useCallback(async (sessionID: string, windowID: string) => {
    if (!sessionID?.trim() || !windowID?.trim()) return
    try {
      const lines = await getSessionOutput<Array<{ text: string }>>(sessionID, 1200)
      const snapshot = lines.map((line) => line.text ?? '').join('\n')
      setRawBuffers((prev) => {
        const existing = prev[windowID] ?? ''
        if (existing.length >= snapshot.length && existing.startsWith(snapshot)) {
          return prev
        }
        return { ...prev, [windowID]: snapshot.slice(-terminalReplayMaxChars) }
      })
    } catch {
      // keep terminal usable even if snapshot bootstrap fails
    }
  }, [])

  // =========================================================================
  // Effects: data loading
  // =========================================================================

  // Initial load + periodic refresh
  useEffect(() => {
    void refreshAll()
  }, [refreshAll])

  // Load project data when active project changes
  useEffect(() => {
    void loadProjectData()
  }, [loadProjectData])

  // Load exceptions when active project changes
  useEffect(() => {
    void refreshExceptions()
  }, [refreshExceptions])

  // Reset orchestrator state on project change
  useEffect(() => {
    setProgressReport(null)
    setReportUpdatedAt(null)
    setReportLoading(false)
    setExceptions([])
    setExceptionCounts({ total: 0, open: 0, resolved: 0 })
  }, [activeProjectID])

  // Refresh on orchestrator events
  useEffect(() => {
    if (shouldRefreshFromOrchestratorEvent(orchestrator.lastEvent)) {
      void loadProjectData()
      void refreshExceptions()
    }
  }, [orchestrator.lastEvent, loadProjectData, refreshExceptions])

  // Refresh on project_event WebSocket messages
  useEffect(() => {
    if (
      !app.lastMessage ||
      app.lastMessage.type !== 'project_event' ||
      app.lastMessage.project_id !== activeProjectID
    ) {
      return
    }
    void loadProjectData()
  }, [app.lastMessage, activeProjectID, loadProjectData])

  // =========================================================================
  // Effects: terminal
  // =========================================================================

  // Handle terminal_data messages from main WebSocket
  useEffect(() => {
    if (!app.lastMessage) return
    handleServerMessage(app.lastMessage)
  }, [app.lastMessage, handleServerMessage])

  // Persist terminal buffers to localStorage (debounced)
  useEffect(() => {
    const timer = window.setTimeout(() => {
      saveTerminalReplay(rawBuffers)
    }, 250)
    return () => window.clearTimeout(timer)
  }, [rawBuffers])

  // Bootstrap terminal snapshot when connection is established
  useEffect(() => {
    if (app.connectionStatus !== 'connected') return
    // Snapshot all sessions with window IDs
    for (const session of sessions) {
      if (session.tmux_window_id && session.id) {
        void refreshWindowSnapshot(session.id, session.tmux_window_id)
      }
    }
  }, [app.connectionStatus, sessions, refreshWindowSnapshot])

  // =========================================================================
  // Computed values
  // =========================================================================

  const selectedProject = useMemo(
    () => projects.find((p) => p.id === activeProjectID) ?? null,
    [activeProjectID, projects],
  )

  const agentSessions = useMemo(() => {
    return sessions.filter((s) => !isOrchestratorSession(s))
  }, [sessions])

  const orchestratorSession = useMemo(() => {
    return sessions.find((s) => isOrchestratorSession(s)) ?? null
  }, [sessions])

  const projectUnreadCounts = useMemo<Record<string, number>>(() => {
    const unread: Record<string, number> = {}
    for (const [project, windows] of Object.entries(projectSessionWindows)) {
      unread[project] = windows.reduce(
        (sum, windowID) => sum + (app.unreadByWindow[windowID] ?? 0),
        0,
      )
    }
    return unread
  }, [app.unreadByWindow, projectSessionWindows])

  // =========================================================================
  // Render
  // =========================================================================

  return (
    <>
      <div
        className={`workspace ${panelOpen ? 'panel-open' : ''} ${sidebarCollapsed ? 'sidebar-collapsed' : ''}`.trim()}
      >
        {/* Left sidebar */}
        <ProjectSidebar
          projects={projects}
          activeProjectID={activeProjectID}
          onSelectProject={setActiveProjectID}
          sessions={sessions}
          unreadByWindow={app.unreadByWindow}
          onSelectAgent={(session: Session) => {
            if (session.tmux_window_id) {
              setActiveWindowID(session.tmux_window_id)
              app.setActiveWindow(session.tmux_window_id)
            }
          }}
          onNewProject={() => setCreateProjectOpen(true)}
          onOpenHome={() => setHomeOpen(true)}
          onOpenConnect={() => setConnectOpen(true)}
          onOpenSettings={() => setSettingsOpen(true)}
          collapsed={sidebarCollapsed}
        />

        {/* Center: terminals or home */}
        <main
          style={{
            display: 'flex',
            flexDirection: 'column',
            overflow: 'hidden',
            position: 'relative',
          }}
        >
          {/* Toolbar */}
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: '6px',
              padding: '4px 8px',
              background: 'var(--bg-surface)',
              borderBottom: '1px solid var(--border-subtle)',
              flexShrink: 0,
            }}
          >
            <button
              className="btn btn-ghost btn-icon"
              onClick={() => setSidebarCollapsed((prev) => !prev)}
              type="button"
            >
              <PanelLeft size={14} />
            </button>
            <button
              className="btn btn-ghost btn-icon"
              onClick={() => setTheme((prev) => (prev === 'dark' ? 'light' : 'dark'))}
              type="button"
            >
              {theme === 'dark' ? <Sun size={14} /> : <Moon size={14} />}
            </button>
            <span style={{ flex: 1 }} />
            {selectedProject && (
              <span style={{ fontSize: '12px', color: 'var(--text-secondary)' }}>
                {selectedProject.name}
              </span>
            )}
            <button
              className="btn btn-ghost btn-icon"
              onClick={() => setPanelOpen((prev) => !prev)}
              type="button"
            >
              <PanelRight size={14} />
            </button>
          </div>

          {/* Terminal grid or empty state */}
          {selectedProject && agentSessions.length > 0 ? (
            <TerminalGrid
              sessions={agentSessions}
              rawBuffers={rawBuffers}
              activeWindowID={activeWindowID}
              onTerminalInput={(windowID: string, sessionID: string, keys: string) => {
                app.send({
                  type: 'terminal_input',
                  session_id: sessionID,
                  window: windowID,
                  keys,
                })
              }}
              onTerminalResize={(windowID: string, sessionID: string, cols: number, rows: number) => {
                app.send({
                  type: 'terminal_resize',
                  session_id: sessionID,
                  window: windowID,
                  cols,
                  rows,
                })
              }}
              onClosePane={(windowID: string) => {
                const win = app.windows.find((w) => w.id === windowID)
                if (win) {
                  app.send({
                    type: 'kill_window',
                    session_id: win.session_id,
                    window: windowID,
                  })
                }
              }}
              onFocusPane={(windowID: string) => {
                setActiveWindowID(windowID)
                app.setActiveWindow(windowID)
              }}
            />
          ) : selectedProject ? (
            <div className="empty-state">No active agent sessions</div>
          ) : (
            <HomeView
              projects={projects}
              onSelectProject={(id: string) => setActiveProjectID(id)}
              onCreateProject={() => setCreateProjectOpen(true)}
            />
          )}
        </main>

        {/* Right panel: orchestrator */}
        {panelOpen && selectedProject && (
          <OrchestratorPanel
            project={selectedProject}
            projectID={activeProjectID}
            tasks={tasks}
            sessions={sessions}
            open={panelOpen}
            onClose={() => setPanelOpen(false)}
            onOpenTaskSession={(taskID: string) => {
              const session = sessions.find((s) => s.task_id === taskID)
              if (session?.tmux_window_id) {
                setActiveWindowID(session.tmux_window_id)
                app.setActiveWindow(session.tmux_window_id)
              }
            }}
            onOpenDemandPool={() => {
              /* demand pool is inline in panel */
            }}
          />
        )}
      </div>

      {/* Modals */}
      <SettingsModal open={settingsOpen} onClose={() => setSettingsOpen(false)} />
      <ConnectModal open={connectOpen} onClose={() => setConnectOpen(false)} />
      {createProjectOpen && (
        <CreateProjectFlow
          open={createProjectOpen}
          onClose={() => setCreateProjectOpen(false)}
          onCreated={() => {
            setCreateProjectOpen(false)
            void refreshAll()
          }}
        />
      )}
      <Modal
        open={deleteModalOpen}
        title="Delete Project"
        onClose={() => setDeleteModalOpen(false)}
      >
        <div className="modal-form">
          <p className="empty-text">
            Delete project <strong>{selectedProject?.name ?? ''}</strong>? This will archive it and
            hide it from active workflow views.
          </p>
          {deleteError && <p className="dashboard-error">{deleteError}</p>}
          <div className="settings-actions">
            <button
              className="secondary-btn"
              disabled={deletingProject}
              onClick={() => setDeleteModalOpen(false)}
              type="button"
            >
              Cancel
            </button>
            <button
              className="secondary-btn danger-btn"
              disabled={deletingProject}
              onClick={() => void confirmDeleteProject()}
              type="button"
            >
              {deletingProject ? 'Deleting...' : 'Delete'}
            </button>
          </div>
        </div>
      </Modal>
    </>
  )
}

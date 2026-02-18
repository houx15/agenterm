import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  createProject,
  listAgents,
  listPlaybooks,
  listProjects,
  listProjectTasks,
  listSessions,
  updateProjectOrchestrator,
} from '../api/client'
import type {
  AgentConfig,
  OutputMessage,
  Playbook,
  Project,
  ProjectOrchestratorProfile,
  ServerMessage,
  Session,
  Task,
  WindowsMessage,
} from '../api/types'
import { useAppContext } from '../App'
import ActivityFeed, { type DashboardActivity } from '../components/ActivityFeed'
import CreateProjectModal from '../components/CreateProjectModal'
import { FolderPlus, MessageSquareText } from '../components/Lucide'

interface ProjectSummary {
  project: Project
  tasks: Task[]
  doneTasks: number
  sessionCount: number
  workingSessionCount: number
}

function normalizeStatus(status: string): string {
  return (status || '').trim().toLowerCase()
}

function isDoneTask(status: string): boolean {
  return ['done', 'completed', 'success'].includes(normalizeStatus(status))
}

function isActiveProject(status: string): boolean {
  return !['inactive', 'archived', 'completed', 'done', 'paused', 'closed'].includes(normalizeStatus(status))
}

function isWorkingSession(status: string): boolean {
  return ['working', 'running', 'executing', 'active', 'busy'].includes(normalizeStatus(status))
}

function needsResponseSession(status: string): boolean {
  return ['waiting', 'human_takeover', 'blocked', 'needs_input', 'reviewing'].includes(normalizeStatus(status))
}

function isIdleSession(status: string): boolean {
  return ['idle', 'disconnected', 'sleeping', 'paused'].includes(normalizeStatus(status))
}

function isBusyAgentSession(status: string): boolean {
  return !['idle', 'disconnected', 'sleeping', 'paused', 'completed', 'failed', 'stopped', 'terminated', 'closed', 'dead'].includes(
    normalizeStatus(status),
  )
}

function isSessionStoppedStatus(status: string): boolean {
  return ['idle', 'disconnected', 'completed', 'stopped', 'terminated'].includes(normalizeStatus(status))
}

function buildActivityFromData(projects: Project[], tasksByProject: Record<string, Task[]>, sessions: Session[]): DashboardActivity[] {
  const items: DashboardActivity[] = []

  for (const session of sessions) {
    const normalizedStatus = normalizeStatus(session.status)
    const isStopped = isSessionStoppedStatus(session.status)

    items.push({
      id: `session-${session.id}-${session.last_activity_at || session.created_at}`,
      timestamp: session.last_activity_at || session.created_at,
      text: isStopped
        ? `${session.agent_type} session ${session.id.slice(0, 6)} moved to ${normalizedStatus || 'idle'}`
        : `${session.agent_type} session started (${session.role})`,
    })
  }

  for (const project of projects) {
    const projectTasks = tasksByProject[project.id] ?? []
    for (const task of projectTasks) {
      const normalizedTaskStatus = normalizeStatus(task.status) || 'unknown'
      items.push({
        id: `task-status-${task.id}-${task.updated_at}`,
        timestamp: task.updated_at,
        text: `${task.title} is ${normalizedTaskStatus} in ${project.name}`,
      })
    }
  }

  return items
    .sort((a, b) => new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime())
    .slice(0, 12)
}

function buildLiveActivityFromMessage(lastMessage: ServerMessage | null): DashboardActivity | null {
  if (!lastMessage) {
    return null
  }

  const now = new Date().toISOString()
  if (lastMessage.type === 'status') {
    const nextStatus = normalizeStatus(String(lastMessage.status ?? 'unknown'))
    const stopText = isSessionStoppedStatus(nextStatus) ? 'stopped/idle' : 'started/running'
    return {
      id: `live-status-${String(lastMessage.window ?? 'unknown')}-${Date.now()}`,
      timestamp: now,
      text: `Session ${String(lastMessage.window ?? '')} ${stopText} (${nextStatus || 'unknown'})`,
    }
  }

  if (lastMessage.type === 'windows') {
    const count = Array.isArray(lastMessage.list) ? lastMessage.list.length : 0
    return {
      id: `live-windows-${Date.now()}`,
      timestamp: now,
      text: `Session list updated (${count} windows)`,
    }
  }

  if (lastMessage.type === 'output' && typeof lastMessage.text === 'string' && lastMessage.text.trim()) {
    const trimmed = lastMessage.text.trim()

    if (/\b(git\s+commit|commit)\b/i.test(trimmed)) {
      return {
        id: `live-commit-${Date.now()}`,
        timestamp: now,
        text: `Commit activity detected in ${String(lastMessage.window ?? 'session')}`,
      }
    }

    return {
      id: `live-output-${Date.now()}`,
      timestamp: now,
      text: `New session output in ${String(lastMessage.window ?? 'session')}`,
    }
  }

  return null
}

export default function Dashboard() {
  const app = useAppContext()
  const navigate = useNavigate()

  const [projects, setProjects] = useState<Project[]>([])
  const [sessions, setSessions] = useState<Session[]>([])
  const [agents, setAgents] = useState<AgentConfig[]>([])
  const [tasksByProject, setTasksByProject] = useState<Record<string, Task[]>>({})
  const [activity, setActivity] = useState<DashboardActivity[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [createProjectOpen, setCreateProjectOpen] = useState(false)
  const [creatingProject, setCreatingProject] = useState(false)
  const [playbooks, setPlaybooks] = useState<Playbook[]>([])
  const [modelOptions, setModelOptions] = useState<string[]>([])

  const syncTimerRef = useRef<number | null>(null)

  const loadDashboard = useCallback(async () => {
    setError('')

    try {
      const [projectsData, sessionsData] = await Promise.all([listProjects<Project[]>(), listSessions<Session[]>()])

      const taskPairs = await Promise.all(
        projectsData.map(async (project) => ({
          projectID: project.id,
          tasks: await listProjectTasks<Task[]>(project.id),
        })),
      )

      const nextTasksByProject: Record<string, Task[]> = {}
      for (const pair of taskPairs) {
        nextTasksByProject[pair.projectID] = pair.tasks
      }

      setProjects(projectsData)
      setSessions(sessionsData)
      setTasksByProject(nextTasksByProject)
      setActivity(buildActivityFromData(projectsData, nextTasksByProject, sessionsData))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load dashboard data')
    } finally {
      setLoading(false)
    }
  }, [])

  const scheduleSync = useCallback(() => {
    if (syncTimerRef.current !== null) {
      window.clearTimeout(syncTimerRef.current)
    }

    syncTimerRef.current = window.setTimeout(() => {
      syncTimerRef.current = null
      void loadDashboard()
    }, 800)
  }, [loadDashboard])

  useEffect(() => {
    return () => {
      if (syncTimerRef.current !== null) {
        window.clearTimeout(syncTimerRef.current)
      }
    }
  }, [])

  useEffect(() => {
    void loadDashboard()
  }, [loadDashboard])

  useEffect(() => {
    const intervalID = window.setInterval(() => {
      void loadDashboard()
    }, 30000)
    return () => window.clearInterval(intervalID)
  }, [loadDashboard])

  useEffect(() => {
    let cancelled = false

    async function loadCreateOptions() {
      try {
        const [agentsData, playbooksData] = await Promise.all([listAgents<AgentConfig[]>(), listPlaybooks<Playbook[]>()])
        if (cancelled) {
          return
        }
        setAgents(agentsData)
        setPlaybooks(playbooksData)
        const models = Array.from(new Set(agentsData.map((agent) => (agent.model || '').trim()).filter(Boolean))).sort((a, b) =>
          a.localeCompare(b),
        )
        setModelOptions(models)
      } catch {
        // non-blocking for dashboard rendering
      }
    }

    void loadCreateOptions()
    return () => {
      cancelled = true
    }
  }, [])

  useEffect(() => {
    const live = buildLiveActivityFromMessage(app.lastMessage)
    if (live) {
      setActivity((prev) => [live, ...prev].slice(0, 12))
    }

    if (!app.lastMessage) {
      return
    }

    if (app.lastMessage.type === 'status') {
      const nextStatus = app.lastMessage.status
      const windowID = app.lastMessage.window
      const now = new Date().toISOString()
      setSessions((prev) =>
        prev.map((session) => {
          if (session.tmux_window_id !== windowID) {
            return session
          }
          return {
            ...session,
            status: nextStatus,
            last_activity_at: now,
          }
        }),
      )
      scheduleSync()
      return
    }

    if (app.lastMessage.type === 'windows') {
      const windowsMessage = app.lastMessage as WindowsMessage
      const statusByWindow: Record<string, string> = {}
      for (const item of windowsMessage.list) {
        statusByWindow[item.id] = item.status
      }

      const now = new Date().toISOString()
      setSessions((prev) =>
        prev.map((session) => {
          if (!session.tmux_window_id) {
            return session
          }
          const nextStatus = statusByWindow[session.tmux_window_id]
          if (!nextStatus || nextStatus === session.status) {
            return session
          }
          return {
            ...session,
            status: nextStatus,
            last_activity_at: now,
          }
        }),
      )

      scheduleSync()
      return
    }

    if (app.lastMessage.type === 'output') {
      const message = app.lastMessage as OutputMessage
      if (!message.window) {
        return
      }

      const now = new Date().toISOString()
      setSessions((prev) =>
        prev.map((session) => {
          if (session.tmux_window_id !== message.window) {
            return session
          }
          return {
            ...session,
            last_activity_at: now,
          }
        }),
      )
    }
  }, [app.lastMessage, scheduleSync])

  const allWindows = useMemo(() => app.windows, [app.windows])

  const projectByTaskID = useMemo(() => {
    const map: Record<string, string> = {}
    for (const project of projects) {
      for (const task of tasksByProject[project.id] ?? []) {
        map[task.id] = project.name
      }
    }
    return map
  }, [projects, tasksByProject])

  const windowProjectNameByID = useMemo(() => {
    const mapping: Record<string, string> = {}
    for (const session of sessions) {
      if (!session.tmux_window_id || !session.task_id) {
        continue
      }
      mapping[session.tmux_window_id] = projectByTaskID[session.task_id] ?? 'Unassigned'
    }
    return mapping
  }, [projectByTaskID, sessions])

  const projectSummaries = useMemo<ProjectSummary[]>(() => {
    return projects.map((project) => {
      const tasks = tasksByProject[project.id] ?? []
      const doneTasks = tasks.filter((task) => isDoneTask(task.status)).length
      const projectWindows = allWindows.filter((windowItem) => windowProjectNameByID[windowItem.id] === project.name)

      return {
        project,
        tasks,
        doneTasks,
        sessionCount: projectWindows.length,
        workingSessionCount: projectWindows.filter((windowItem) => isWorkingSession(windowItem.status)).length,
      }
    })
  }, [allWindows, projects, tasksByProject, windowProjectNameByID])

  const sortedProjectSummaries = useMemo(
    () =>
      [...projectSummaries].sort((a, b) => {
        const aActive = isActiveProject(a.project.status)
        const bActive = isActiveProject(b.project.status)
        if (aActive !== bActive) {
          return aActive ? -1 : 1
        }
        return a.project.name.localeCompare(b.project.name)
      }),
    [projectSummaries],
  )

  const sessionStatusSummary = useMemo(() => {
    const total = allWindows.length
    const working = allWindows.filter((windowItem) => isWorkingSession(windowItem.status)).length
    const needsResponse = allWindows.filter((windowItem) => needsResponseSession(windowItem.status)).length
    const idle = allWindows.filter((windowItem) => isIdleSession(windowItem.status)).length
    return { total, working, needsResponse, idle }
  }, [allWindows])

  const agentTeamSummary = useMemo(() => {
    const byType: Array<[string, { capacity: number; assigned: number; orchestrator: number; idle: number; overflow: number }]> = []

    let totalCapacity = 0
    let totalBusy = 0
    let totalOrchestrator = 0
    let totalAssigned = 0

    for (const agent of agents) {
      const capacity = Math.max(1, agent.max_parallel_agents ?? 1)
      const busySessions = sessions.filter((session) => session.agent_type === agent.id && isBusyAgentSession(session.status))
      const orchestrator = busySessions.filter((session) => normalizeStatus(session.role) === 'orchestrator').length
      const assigned = busySessions.filter((session) => normalizeStatus(session.role) !== 'orchestrator').length
      const busy = assigned + orchestrator
      const overflow = Math.max(0, busy - capacity)
      const idle = Math.max(0, capacity - busy)

      totalCapacity += capacity
      totalBusy += Math.min(capacity, busy)
      totalAssigned += assigned
      totalOrchestrator += orchestrator

      byType.push([agent.id, { capacity, assigned, orchestrator, idle, overflow }])
    }

    const workingRatio = totalCapacity > 0 ? Math.round((totalBusy / totalCapacity) * 100) : 0

    return {
      configuredAgents: agents.length,
      totalCapacity,
      totalBusy,
      totalAssigned,
      totalOrchestrator,
      totalIdle: Math.max(0, totalCapacity - totalBusy),
      workingRatio,
      byType: byType.sort(([a], [b]) => a.localeCompare(b)),
    }
  }, [agents, sessions])

  const handleCreateProject = () => {
    setCreateProjectOpen(true)
  }

  const submitCreateProject = async (values: {
    name: string
    repoPath: string
    orchestratorAgentID: string
    orchestratorProvider: string
    playbook: string
    orchestratorModel: string
    workers: number
  }) => {
    setCreatingProject(true)
    setError('')
    try {
      const created = await createProject<Project>({
        name: values.name,
        repo_path: values.repoPath,
        playbook: values.playbook || undefined,
        status: 'active',
      })

      if (values.orchestratorModel || values.workers > 0) {
        try {
          await updateProjectOrchestrator<ProjectOrchestratorProfile>(created.id, {
            default_provider: values.orchestratorProvider || undefined,
            default_model: values.orchestratorModel || undefined,
            max_parallel: values.workers,
          })
        } catch (patchErr) {
          setError(
            patchErr instanceof Error
              ? `Project created, but orchestrator settings were not applied: ${patchErr.message}`
              : 'Project created, but orchestrator settings were not applied',
          )
        }
      }

      await loadDashboard()
      setCreateProjectOpen(false)
      setActivity((prev) => [
        {
          id: `project-created-${Date.now()}`,
          timestamp: new Date().toISOString(),
          text: `${values.name} created`,
        },
        ...prev,
      ])
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create project')
    } finally {
      setCreatingProject(false)
    }
  }

  return (
    <section className="page-block dashboard-page">
      <div className="dashboard-hero">
        <h2>Dashboard</h2>
        <div className="dashboard-hero-actions">
          <button className="secondary-btn" onClick={() => navigate('/pm-chat')} type="button">
            <MessageSquareText size={14} />
            <span>Open PM Chat</span>
          </button>
          <button className="primary-btn" onClick={handleCreateProject} type="button">
            <FolderPlus size={14} />
            <span>New Project</span>
          </button>
        </div>
      </div>

      {error && <p className="dashboard-error">{error}</p>}
      {loading && <p className="empty-text">Loading dashboard...</p>}

      {!loading && projectSummaries.length === 0 && (
        <div className="dashboard-empty-state">
          <h3>Welcome to AgenTerm</h3>
          <p>Create your first project to start orchestrating tasks and sessions.</p>
          <button className="primary-btn" onClick={handleCreateProject} type="button">
            Create Project
          </button>
        </div>
      )}

      {!loading && projectSummaries.length > 0 && (
        <>
          <section className="dashboard-section">
            <div className="dashboard-section-title">
              <h3>Project List</h3>
            </div>
            <div className="dashboard-project-list">
              <div className="dashboard-project-list-head">
                <span>Project</span>
                <span>Working Dir</span>
                <span>Status</span>
                <span>Sessions</span>
                <span>Tasks</span>
              </div>
              {sortedProjectSummaries.map((summary) => (
                <button
                  className="dashboard-project-list-row"
                  key={summary.project.id}
                  onClick={() => navigate(`/pm-chat?project=${encodeURIComponent(summary.project.id)}`)}
                  type="button"
                >
                  <strong>{summary.project.name}</strong>
                  <span>{summary.project.repo_path}</span>
                  <span>{summary.project.status}</span>
                  <span>
                    {summary.workingSessionCount}/{summary.sessionCount} working
                  </span>
                  <span>
                    {summary.doneTasks}/{summary.tasks.length} done
                  </span>
                </button>
              ))}
            </div>
          </section>

          <section className="dashboard-section">
            <div className="dashboard-section-title">
              <h3>Agent Team</h3>
            </div>
            <div className="dashboard-resource-grid">
              <article className="dashboard-resource-card">
                <small>Configured Agents</small>
                <strong>{agentTeamSummary.configuredAgents}</strong>
              </article>
              <article className="dashboard-resource-card">
                <small>Total Capacity</small>
                <strong>{agentTeamSummary.totalCapacity}</strong>
              </article>
              <article className="dashboard-resource-card">
                <small>Assigned</small>
                <strong>{agentTeamSummary.totalAssigned}</strong>
              </article>
              <article className="dashboard-resource-card">
                <small>Orchestrator</small>
                <strong>{agentTeamSummary.totalOrchestrator}</strong>
              </article>
              <article className="dashboard-resource-card">
                <small>Idle</small>
                <strong>{agentTeamSummary.totalIdle}</strong>
              </article>
              <article className="dashboard-resource-card">
                <small>Working Ratio</small>
                <strong>{agentTeamSummary.workingRatio}%</strong>
              </article>
              <article className="dashboard-resource-card">
                <small>Socket</small>
                <strong>{app.connected ? 'Connected' : 'Offline'}</strong>
              </article>
            </div>
            {agentTeamSummary.byType.length > 0 && (
              <div className="dashboard-agent-breakdown">
                {agentTeamSummary.byType.map(([agentType, stat]) => (
                  <article className="dashboard-agent-card" key={agentType}>
                    <strong>{agentType}</strong>
                    <small>{stat.capacity} capacity</small>
                    <small>{stat.assigned} assigned</small>
                    <small>{stat.orchestrator} orchestrator</small>
                    <small>{stat.idle} idle</small>
                    {stat.overflow > 0 && <small>+{stat.overflow} overflow</small>}
                  </article>
                ))}
              </div>
            )}
          </section>

          <section className="dashboard-section">
            <div className="dashboard-section-title">
              <h3>Session Status</h3>
            </div>
            <div className="dashboard-resource-grid">
              <article className="dashboard-resource-card">
                <small>Total Sessions</small>
                <strong>{sessionStatusSummary.total}</strong>
              </article>
              <article className="dashboard-resource-card">
                <small>Working</small>
                <strong>{sessionStatusSummary.working}</strong>
              </article>
              <article className="dashboard-resource-card">
                <small>Needs Response</small>
                <strong>{sessionStatusSummary.needsResponse}</strong>
              </article>
              <article className="dashboard-resource-card">
                <small>Idle</small>
                <strong>{sessionStatusSummary.idle}</strong>
              </article>
            </div>
          </section>

          <section className="dashboard-section">
            <div className="dashboard-section-title">
              <h3>Recent Activity</h3>
            </div>
            <ActivityFeed items={activity} />
          </section>
        </>
      )}

      <CreateProjectModal
        agents={agents}
        busy={creatingProject}
        modelOptions={modelOptions}
        onClose={() => setCreateProjectOpen(false)}
        onSubmit={submitCreateProject}
        open={createProjectOpen}
        playbooks={playbooks}
      />
    </section>
  )
}

import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { createProject, listProjects, listProjectTasks, listSessions } from '../api/client'
import type { OutputMessage, Project, ServerMessage, Session, Task, WindowsMessage } from '../api/types'
import { useAppContext } from '../App'
import ActivityFeed, { type DashboardActivity } from '../components/ActivityFeed'
import ProjectCard from '../components/ProjectCard'
import SessionGrid from '../components/SessionGrid'

interface ProjectSummary {
  project: Project
  tasks: Task[]
  doneTasks: number
  activeAgents: number
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

function isActiveSession(status: string): boolean {
  return !['completed', 'stopped', 'terminated'].includes(normalizeStatus(status))
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
  const [tasksByProject, setTasksByProject] = useState<Record<string, Task[]>>({})
  const [activity, setActivity] = useState<DashboardActivity[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [showInactiveProjects, setShowInactiveProjects] = useState(false)

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

  const activeSessions = useMemo(() => sessions.filter((session) => isActiveSession(session.status)), [sessions])

  const projectByTaskID = useMemo(() => {
    const map: Record<string, string> = {}
    for (const project of projects) {
      for (const task of tasksByProject[project.id] ?? []) {
        map[task.id] = project.name
      }
    }
    return map
  }, [projects, tasksByProject])

  const projectSummaries = useMemo<ProjectSummary[]>(() => {
    return projects.map((project) => {
      const tasks = tasksByProject[project.id] ?? []
      const doneTasks = tasks.filter((task) => isDoneTask(task.status)).length

      const activeAgents = activeSessions.filter((session) => {
        if (!session.task_id) {
          return false
        }
        return tasks.some((task) => task.id === session.task_id)
      }).length

      return {
        project,
        tasks,
        doneTasks,
        activeAgents,
      }
    })
  }, [activeSessions, projects, tasksByProject])

  const activeProjectSummaries = useMemo(
    () => projectSummaries.filter((summary) => isActiveProject(summary.project.status)),
    [projectSummaries],
  )

  const inactiveProjectSummaries = useMemo(
    () => projectSummaries.filter((summary) => !isActiveProject(summary.project.status)),
    [projectSummaries],
  )

  const groupedSessions = useMemo(() => {
    const grouped: Record<string, { session: Session; projectName: string }[]> = {}

    for (const session of activeSessions) {
      const projectName = session.task_id ? (projectByTaskID[session.task_id] ?? 'Unassigned') : 'Unassigned'
      if (!grouped[projectName]) {
        grouped[projectName] = []
      }
      grouped[projectName].push({ session, projectName })
    }

    return grouped
  }, [activeSessions, projectByTaskID])

  const resourceSummary = useMemo(() => {
    const allTasks = Object.values(tasksByProject).flat()
    const doneTasks = allTasks.filter((task) => isDoneTask(task.status)).length
    const runningSessions = activeSessions.filter((session) => ['running', 'working'].includes(normalizeStatus(session.status))).length
    const idleSessions = activeSessions.filter((session) => ['idle', 'waiting', 'human_takeover'].includes(normalizeStatus(session.status))).length
    const taskCompletionRate = allTasks.length > 0 ? Math.round((doneTasks / allTasks.length) * 100) : 0

    return {
      totalTasks: allTasks.length,
      doneTasks,
      activeSessions: activeSessions.length,
      runningSessions,
      idleSessions,
      taskCompletionRate,
    }
  }, [activeSessions, tasksByProject])

  const handleCreateProject = async () => {
    const name = window.prompt('Project name')?.trim()
    if (!name) {
      return
    }
    const repoPath = window.prompt('Repository path (absolute)')?.trim()
    if (!repoPath) {
      return
    }

    try {
      await createProject<Project>({ name, repo_path: repoPath, status: 'active' })
      await loadDashboard()
      setActivity((prev) => [
        {
          id: `project-created-${Date.now()}`,
          timestamp: new Date().toISOString(),
          text: `${name} created`,
        },
        ...prev,
      ])
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create project')
    }
  }

  return (
    <section className="page-block dashboard-page">
      <h2>Dashboard</h2>

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
              <h3>Projects ({activeProjectSummaries.length} active)</h3>
            </div>
            <div className="dashboard-project-grid">
              {activeProjectSummaries.map((summary) => (
                <ProjectCard
                  activeAgents={summary.activeAgents}
                  doneTasks={summary.doneTasks}
                  key={summary.project.id}
                  onClick={() => navigate(`/projects/${summary.project.id}`)}
                  project={summary.project}
                  totalTasks={summary.tasks.length}
                />
              ))}
              <ProjectCard isNewCard onClick={handleCreateProject} />
            </div>

            {inactiveProjectSummaries.length > 0 && (
              <div className="dashboard-inactive-projects">
                <button className="secondary-btn" onClick={() => setShowInactiveProjects((prev) => !prev)} type="button">
                  {showInactiveProjects ? 'Hide' : 'Show'} inactive projects ({inactiveProjectSummaries.length})
                </button>

                {showInactiveProjects && (
                  <div className="dashboard-project-grid dashboard-project-grid-inactive">
                    {inactiveProjectSummaries.map((summary) => (
                      <ProjectCard
                        activeAgents={summary.activeAgents}
                        doneTasks={summary.doneTasks}
                        key={summary.project.id}
                        onClick={() => navigate(`/projects/${summary.project.id}`)}
                        project={summary.project}
                        totalTasks={summary.tasks.length}
                      />
                    ))}
                  </div>
                )}
              </div>
            )}
          </section>

          <section className="dashboard-section">
            <div className="dashboard-section-title">
              <h3>Resource Summary</h3>
            </div>
            <div className="dashboard-resource-grid">
              <article className="dashboard-resource-card">
                <small>Active Sessions</small>
                <strong>{resourceSummary.activeSessions}</strong>
              </article>
              <article className="dashboard-resource-card">
                <small>Running Sessions</small>
                <strong>{resourceSummary.runningSessions}</strong>
              </article>
              <article className="dashboard-resource-card">
                <small>Idle Sessions</small>
                <strong>{resourceSummary.idleSessions}</strong>
              </article>
              <article className="dashboard-resource-card">
                <small>Tasks Done</small>
                <strong>
                  {resourceSummary.doneTasks}/{resourceSummary.totalTasks}
                </strong>
              </article>
              <article className="dashboard-resource-card">
                <small>Completion</small>
                <strong>{resourceSummary.taskCompletionRate}%</strong>
              </article>
              <article className="dashboard-resource-card">
                <small>Socket</small>
                <strong>{app.connected ? 'Connected' : 'Offline'}</strong>
              </article>
            </div>
          </section>

          <section className="dashboard-section">
            <div className="dashboard-section-title">
              <h3>Active Sessions</h3>
            </div>
            <SessionGrid
              groupedSessions={groupedSessions}
              onSessionClick={(session) => {
                if (session.tmux_window_id) {
                  app.setActiveWindow(session.tmux_window_id)
                }
                navigate('/sessions')
              }}
            />
          </section>

          <section className="dashboard-section">
            <div className="dashboard-section-title">
              <h3>Recent Activity</h3>
            </div>
            <ActivityFeed items={activity} />
          </section>
        </>
      )}
    </section>
  )
}

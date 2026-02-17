import { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { createProject, listProjects, listProjectTasks, listSessions } from '../api/client'
import type { Project, Session, Task } from '../api/types'
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

function isDoneTask(status: string): boolean {
  return ['done', 'completed', 'success'].includes((status || '').toLowerCase())
}

function buildActivityFromData(projects: Project[], tasksByProject: Record<string, Task[]>, sessions: Session[]): DashboardActivity[] {
  const items: DashboardActivity[] = []

  for (const session of sessions) {
    items.push({
      id: `session-${session.id}`,
      timestamp: session.created_at,
      text: `${session.agent_type} session started (${session.role})`,
    })
  }

  for (const project of projects) {
    const projectTasks = tasksByProject[project.id] ?? []
    for (const task of projectTasks) {
      if (isDoneTask(task.status)) {
        items.push({
          id: `task-done-${task.id}`,
          timestamp: task.updated_at,
          text: `${task.title} completed in ${project.name}`,
        })
      }
    }
  }

  return items
    .sort((a, b) => new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime())
    .slice(0, 12)
}

function buildLiveActivityFromMessage(lastMessage: { type: string; [key: string]: unknown } | null): DashboardActivity | null {
  if (!lastMessage) {
    return null
  }

  const now = new Date().toISOString()
  if (lastMessage.type === 'status') {
    return {
      id: `live-status-${String(lastMessage.window ?? 'unknown')}-${Date.now()}`,
      timestamp: now,
      text: `Session ${String(lastMessage.window ?? '')} changed to ${String(lastMessage.status ?? 'unknown')}`,
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
  const [searchParams, setSearchParams] = useSearchParams()

  const [projects, setProjects] = useState<Project[]>([])
  const [sessions, setSessions] = useState<Session[]>([])
  const [tasksByProject, setTasksByProject] = useState<Record<string, Task[]>>({})
  const [activity, setActivity] = useState<DashboardActivity[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  const selectedProjectID = searchParams.get('project')

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
    const live = buildLiveActivityFromMessage(app.lastMessage as { type: string; [key: string]: unknown } | null)
    if (!live) {
      return
    }

    setActivity((prev) => [live, ...prev].slice(0, 12))
  }, [app.lastMessage])

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

      const activeAgents = sessions.filter((session) => {
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
  }, [projects, sessions, tasksByProject])

  const groupedSessions = useMemo(() => {
    const grouped: Record<string, { session: Session; projectName: string }[]> = {}

    for (const session of sessions) {
      const projectName = session.task_id ? (projectByTaskID[session.task_id] ?? 'Unassigned') : 'Unassigned'
      if (!grouped[projectName]) {
        grouped[projectName] = []
      }
      grouped[projectName].push({ session, projectName })
    }

    return grouped
  }, [projectByTaskID, sessions])

  const selectedSummary = projectSummaries.find((summary) => summary.project.id === selectedProjectID)

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
              <h3>Projects ({projectSummaries.length} active)</h3>
            </div>
            <div className="dashboard-project-grid">
              {projectSummaries.map((summary) => (
                <ProjectCard
                  activeAgents={summary.activeAgents}
                  doneTasks={summary.doneTasks}
                  key={summary.project.id}
                  onClick={() => setSearchParams({ project: summary.project.id })}
                  project={summary.project}
                  totalTasks={summary.tasks.length}
                />
              ))}
              <ProjectCard isNewCard onClick={handleCreateProject} />
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

          {selectedSummary && (
            <section className="dashboard-section">
              <div className="dashboard-section-title">
                <h3>{selectedSummary.project.name} details</h3>
              </div>
              <div className="dashboard-project-detail">
                <p>Status: {selectedSummary.project.status}</p>
                <p>
                  Tasks: {selectedSummary.doneTasks}/{selectedSummary.tasks.length} done
                </p>
                <ul>
                  {selectedSummary.tasks.slice(0, 6).map((task) => (
                    <li key={task.id}>{task.title}</li>
                  ))}
                </ul>
              </div>
            </section>
          )}
        </>
      )}
    </section>
  )
}

import { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { listProjects, listProjectTasks, listSessions } from '../api/client'
import type { Project, Session, Task } from '../api/types'

function normalizeStatus(status: string): string {
  return (status || '').trim().toLowerCase()
}

function isDoneTask(status: string): boolean {
  return ['done', 'completed', 'success'].includes(normalizeStatus(status))
}

function isActiveSession(status: string): boolean {
  return !['completed', 'stopped', 'terminated'].includes(normalizeStatus(status))
}

export default function ProjectDetail() {
  const navigate = useNavigate()
  const { projectId = '' } = useParams<{ projectId: string }>()

  const [project, setProject] = useState<Project | null>(null)
  const [tasks, setTasks] = useState<Task[]>([])
  const [sessions, setSessions] = useState<Session[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  const loadProjectDetail = useCallback(async () => {
    if (!projectId) {
      setError('Missing project id')
      setLoading(false)
      return
    }

    setError('')

    try {
      const [projectsData, sessionsData] = await Promise.all([listProjects<Project[]>(), listSessions<Session[]>()])
      const target = projectsData.find((item) => item.id === projectId)
      if (!target) {
        setError('Project not found')
        setLoading(false)
        return
      }

      const tasksData = await listProjectTasks<Task[]>(projectId)

      setProject(target)
      setTasks(tasksData)
      setSessions(sessionsData)
      setLoading(false)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load project details')
      setLoading(false)
    }
  }, [projectId])

  useEffect(() => {
    void loadProjectDetail()
  }, [loadProjectDetail])

  const doneTasks = useMemo(() => tasks.filter((task) => isDoneTask(task.status)).length, [tasks])

  const activeAgents = useMemo(() => {
    const taskIDSet = new Set(tasks.map((task) => task.id))
    return sessions.filter((session) => session.task_id && taskIDSet.has(session.task_id) && isActiveSession(session.status)).length
  }, [sessions, tasks])

  return (
    <section className="page-block project-detail-page">
      <div className="project-detail-header">
        <button className="secondary-btn" onClick={() => navigate('/')} type="button">
          Back to Dashboard
        </button>
      </div>

      {loading && <p className="empty-text">Loading project details...</p>}
      {error && <p className="dashboard-error">{error}</p>}

      {!loading && !error && project && (
        <>
          <section className="dashboard-section">
            <div className="dashboard-section-title">
              <h3>{project.name}</h3>
            </div>
            <div className="project-detail-meta">
              <p>Status: {project.status}</p>
              <p>Repository: {project.repo_path}</p>
              <p>
                Progress: {doneTasks}/{tasks.length} done
              </p>
              <p>Active Agents: {activeAgents}</p>
            </div>
          </section>

          <section className="dashboard-section">
            <div className="dashboard-section-title">
              <h3>Tasks</h3>
            </div>
            {tasks.length === 0 && <p className="empty-text">No tasks yet.</p>}
            {tasks.length > 0 && (
              <ul className="project-detail-task-list">
                {tasks.map((task) => (
                  <li key={task.id}>
                    <strong>{task.title}</strong>
                    <small>{task.status}</small>
                  </li>
                ))}
              </ul>
            )}
          </section>
        </>
      )}
    </section>
  )
}

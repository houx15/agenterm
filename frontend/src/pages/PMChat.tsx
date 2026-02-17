import { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { listProjects, listProjectTasks, listSessions } from '../api/client'
import type { OrchestratorServerMessage, Project, Session, Task } from '../api/types'
import { useAppContext } from '../App'
import ChatPanel from '../components/ChatPanel'
import type { MessageTaskLink } from '../components/ChatMessage'
import ProjectSelector from '../components/ProjectSelector'
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
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  const [projectID, setProjectID] = useState(() => new URLSearchParams(window.location.search).get('project') ?? '')

  const orchestrator = useOrchestratorWS(projectID)

  const loadProjects = useCallback(async () => {
    const projectList = await listProjects<Project[]>()
    setProjects(projectList)

    if (projectList.length === 0) {
      setProjectID('')
      return
    }

    const current = projectID
    const hasCurrent = Boolean(current && projectList.some((project) => project.id === current))
    const nextID = hasCurrent ? current : projectList[0].id

    if (nextID !== projectID) {
      setProjectID(nextID)
      setSearchParams({ project: nextID }, { replace: true })
    }
  }, [projectID, setSearchParams])

  const loadProjectData = useCallback(async () => {
    if (!projectID) {
      setTasks([])
      setSessions([])
      setLoading(false)
      return
    }

    const [taskList, sessionList] = await Promise.all([listProjectTasks<Task[]>(projectID), listSessions<Session[]>()])

    setTasks(taskList)
    setSessions(sessionList)
    setLoading(false)
  }, [projectID])

  const refreshAll = useCallback(async () => {
    setError('')
    try {
      await loadProjects()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load projects')
      setLoading(false)
    }
  }, [loadProjects])

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

  return (
    <section className="pm-chat-page">
      <div className="pm-chat-header">
        <h2>PM Chat</h2>
        <ProjectSelector projects={projects} value={projectID} onChange={onSelectProject} />
      </div>

      {loading && <p className="empty-view">Loading PM chat context...</p>}
      {error && <p className="dashboard-error">{error}</p>}

      {!loading && !error && (
        <div className="pm-chat-layout">
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
      )}
    </section>
  )
}

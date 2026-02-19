import { useEffect, useMemo, useState } from 'react'
import { listProjects, listProjectTasks, listSessions } from '../api/client'
import type { Project, ServerMessage, Session, Task } from '../api/types'
import { Plus, Square } from '../components/Lucide'
import Terminal from '../components/Terminal'
import { useAppContext } from '../App'

type GroupMode = 'project' | 'status'

function normalizeStatus(status: string): string {
  return (status || 'unknown').trim().toLowerCase()
}

export default function Sessions() {
  const app = useAppContext()
  const [rawBuffers, setRawBuffers] = useState<Record<string, string>>({})
  const [inputValue, setInputValue] = useState('')
  const [groupMode, setGroupMode] = useState<GroupMode>('project')
  const [projects, setProjects] = useState<Project[]>([])
  const [tasksByProject, setTasksByProject] = useState<Record<string, Task[]>>({})
  const [dbSessions, setDbSessions] = useState<Session[]>([])
  const [sendError, setSendError] = useState('')
  const rawHistory = app.activeWindow ? (rawBuffers[app.activeWindow] ?? '') : ''
  const activeWindowInfo = useMemo(() => app.windows.find((win) => win.id === app.activeWindow) ?? null, [app.activeWindow, app.windows])

  useEffect(() => {
    if (!app.lastMessage) {
      return
    }

    handleServerMessage(app.lastMessage)
  }, [app.lastMessage])

  useEffect(() => {
    let cancelled = false

    async function loadContext() {
      try {
        const [projectList, sessionList] = await Promise.all([listProjects<Project[]>(), listSessions<Session[]>()])
        const taskPairs = await Promise.all(
          projectList.map(async (project) => ({
            projectID: project.id,
            tasks: await listProjectTasks<Task[]>(project.id),
          })),
        )
        if (cancelled) {
          return
        }
        const taskMap: Record<string, Task[]> = {}
        for (const pair of taskPairs) {
          taskMap[pair.projectID] = pair.tasks
        }
        setProjects(projectList)
        setTasksByProject(taskMap)
        setDbSessions(sessionList)
      } catch {
        // keep session terminal usable even if metadata fetch fails
      }
    }

    void loadContext()
    const intervalID = window.setInterval(() => {
      void loadContext()
    }, 10000)

    return () => {
      cancelled = true
      window.clearInterval(intervalID)
    }
  }, [])

  const sendInput = (text: string) => {
    if (!app.activeWindow || !text) {
      return
    }
    const ok = app.send({
      type: 'terminal_input',
      session_id: activeWindowInfo?.session_id,
      window: app.activeWindow,
      keys: text,
    })
    if (!ok) {
      setSendError('Socket disconnected. Reconnect and try again.')
      return
    }
    setSendError('')
  }

  function handleServerMessage(message: ServerMessage) {
    if (message.type === 'error') {
      setSendError(message.message || 'Terminal command failed.')
      return
    }
    switch (message.type) {
      case 'terminal_data': {
        setRawBuffers((prev) => {
          const current = prev[message.window] ?? ''
          const next = current + (message.text ?? '')
          return {
            ...prev,
            [message.window]: next.length > 300000 ? next.slice(next.length - 300000) : next,
          }
        })
        return
      }
      default:
        return
    }
  }

  const taskToProjectName = useMemo(() => {
    const map: Record<string, string> = {}
    for (const project of projects) {
      for (const task of tasksByProject[project.id] ?? []) {
        map[task.id] = project.name
      }
    }
    return map
  }, [projects, tasksByProject])

  const projectByWindow = useMemo(() => {
    const mapping: Record<string, string> = {}
    for (const session of dbSessions) {
      if (!session.tmux_window_id) {
        continue
      }
      mapping[session.tmux_window_id] = session.task_id ? (taskToProjectName[session.task_id] ?? 'Unassigned') : 'Unassigned'
    }
    return mapping
  }, [dbSessions, taskToProjectName])

  const groupedWindows = useMemo(() => {
    const groups: Record<string, typeof app.windows> = {}
    for (const win of app.windows) {
      const key = groupMode === 'project' ? projectByWindow[win.id] ?? 'Unassigned' : normalizeStatus(win.status)
      if (!groups[key]) {
        groups[key] = []
      }
      groups[key].push(win)
    }
    return Object.entries(groups).sort(([a], [b]) => a.localeCompare(b))
  }, [app.windows, groupMode, projectByWindow])

  const createSession = () => {
    const timestamp = new Date().toISOString().replace(/[-:TZ.]/g, '').slice(0, 14)
    const ok = app.send({ type: 'new_session', name: `session-${timestamp}` })
    if (!ok) {
      setSendError('Socket disconnected. Reconnect and try again.')
      return
    }
    setSendError('')
  }

  return (
    <section className="sessions-grid">
      <aside className="sessions-panel">
        <div className="sessions-panel-head">
          <h3>Active Sessions</h3>
          <div className="sessions-panel-actions">
            <button className="secondary-btn" onClick={createSession} type="button">
              <Plus size={14} />
              <span>New Session</span>
            </button>
          </div>
        </div>
        <div className="sessions-group-toggle">
          <button
            className={`secondary-btn ${groupMode === 'project' ? 'active' : ''}`.trim()}
            onClick={() => setGroupMode('project')}
            type="button"
          >
            By Project
          </button>
          <button
            className={`secondary-btn ${groupMode === 'status' ? 'active' : ''}`.trim()}
            onClick={() => setGroupMode('status')}
            type="button"
          >
            By Status
          </button>
        </div>
        {groupedWindows.map(([groupName, windows]) => (
          <div className="sessions-group" key={groupName}>
            <h4>{groupName}</h4>
            {windows.map((win) => (
              <div className={`session-row ${app.activeWindow === win.id ? 'active' : ''}`.trim()} key={win.id}>
                <button className="session-row-main" onClick={() => app.setActiveWindow(win.id)} type="button">
                  <span>{win.name}</span>
                  <small>{win.session_id || 'default'} · {win.status}</small>
                </button>
                <button
                  className="session-row-end action-btn danger"
                  onClick={() => app.send({ type: 'kill_window', session_id: win.session_id, window: win.id })}
                  title="End session"
                  type="button"
                >
                  <Square size={12} />
                </button>
              </div>
            ))}
          </div>
        ))}
      </aside>

      <section className="viewer-panel">
        <div className="viewer-toolbar">
          <strong>{activeWindowInfo ? `${activeWindowInfo.name} (${activeWindowInfo.session_id || 'default'})` : 'Select a session'}</strong>
          <small className="empty-text">xterm.js</small>
        </div>

        {!app.activeWindow && <div className="empty-view">Select a session to start</div>}

        {app.activeWindow && (
          <Terminal
            sessionId={app.activeWindow}
            history={rawHistory}
            onInput={(keys) =>
              app.send({
                type: 'terminal_input',
                session_id: activeWindowInfo?.session_id,
                window: app.activeWindow!,
                keys,
              })
            }
            onResize={(cols, rows) =>
              app.send({
                type: 'terminal_resize',
                session_id: activeWindowInfo?.session_id,
                window: app.activeWindow!,
                cols,
                rows,
              })
            }
          />
        )}

        <div className="input-row">
          <textarea
            value={inputValue}
            onChange={(event) => setInputValue(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === 'Tab') {
                event.preventDefault()
                sendInput('\t')
              }
              if (event.key === 'Enter' && !event.shiftKey) {
                event.preventDefault()
                if (inputValue.trim()) {
                  sendInput(`${inputValue}\n`)
                  setInputValue('')
                }
              }
            }}
            placeholder="Type command..."
          />
          <button
            className="primary-btn"
            onClick={() => {
              if (!inputValue.trim()) {
                return
              }
              sendInput(`${inputValue}\n`)
              setInputValue('')
            }}
            type="button"
          >
            Send
          </button>
        </div>
        {sendError && <p className="dashboard-error session-send-error">{sendError}</p>}
      </section>
    </section>
  )
}

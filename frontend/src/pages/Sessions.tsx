import { useEffect, useMemo, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { getSessionOutput, listProjects, listProjectTasks, listSessions } from '../api/client'
import type { Project, ServerMessage, Session, Task } from '../api/types'
import { ChevronLeft, Plus, Square } from '../components/Lucide'
import Terminal from '../components/Terminal'
import { useAppContext } from '../App'

type GroupMode = 'project' | 'status'

function normalizeStatus(status: string): string {
  return (status || 'unknown').trim().toLowerCase()
}

export default function Sessions() {
  const navigate = useNavigate()
  const { windowId } = useParams<{ windowId?: string }>()
  const app = useAppContext()
  const [rawBuffers, setRawBuffers] = useState<Record<string, string>>({})
  const [inputValue, setInputValue] = useState('')
  const [searchValue, setSearchValue] = useState('')
  const [statusFilter, setStatusFilter] = useState('')
  const [groupMode, setGroupMode] = useState<GroupMode>('project')
  const [projects, setProjects] = useState<Project[]>([])
  const [tasksByProject, setTasksByProject] = useState<Record<string, Task[]>>({})
  const [dbSessions, setDbSessions] = useState<Session[]>([])
  const [sendError, setSendError] = useState('')
  const [isMobile, setIsMobile] = useState<boolean>(() => (typeof window !== 'undefined' ? window.innerWidth <= 900 : false))
  const selectedWindowID = useMemo(() => {
    if (isMobile && windowId) {
      return windowId
    }
    return app.activeWindow
  }, [app.activeWindow, isMobile, windowId])
  const rawHistory = selectedWindowID ? (rawBuffers[selectedWindowID] ?? '') : ''
  const activeWindowInfo = useMemo(() => app.windows.find((win) => win.id === selectedWindowID) ?? null, [selectedWindowID, app.windows])
  const maxBufferChars = 120000

  useEffect(() => {
    const onResize = () => {
      setIsMobile(window.innerWidth <= 900)
    }
    window.addEventListener('resize', onResize)
    return () => window.removeEventListener('resize', onResize)
  }, [])

  useEffect(() => {
    if (!selectedWindowID) {
      return
    }
    if (app.activeWindow !== selectedWindowID) {
      app.setActiveWindow(selectedWindowID)
    }
  }, [app.activeWindow, app.setActiveWindow, selectedWindowID])

  useEffect(() => {
    const sessionID = activeWindowInfo?.session_id ?? ''
    app.send({
      type: 'subscribe',
      session_id: sessionID,
      window: '',
      keys: '',
    })
    return () => {
      app.send({
        type: 'subscribe',
        session_id: '',
        window: '',
        keys: '',
      })
    }
  }, [activeWindowInfo?.session_id, app.send])

  useEffect(() => {
    if (!app.lastMessage) {
      return
    }

    handleServerMessage(app.lastMessage)
  }, [app.lastMessage])

  useEffect(() => {
    let cancelled = false
    const sessionID = activeWindowInfo?.session_id?.trim()
    const windowID = selectedWindowID?.trim()
    if (!sessionID || !windowID) {
      return () => {
        cancelled = true
      }
    }

    void (async () => {
      try {
        const lines = await getSessionOutput<Array<{ text: string }>>(sessionID, 500)
        if (cancelled) {
          return
        }
        const snapshot = lines.map((line) => line.text ?? '').join('\n')
        setRawBuffers((prev) => {
          const existing = prev[windowID] ?? ''
          if (existing.length >= snapshot.length && existing.startsWith(snapshot)) {
            return prev
          }
          return { ...prev, [windowID]: snapshot }
        })
      } catch {
        // keep terminal usable even if snapshot bootstrap fails
      }
    })()

    return () => {
      cancelled = true
    }
  }, [activeWindowInfo?.session_id, selectedWindowID])

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
            [message.window]: next.length > maxBufferChars ? next.slice(next.length - maxBufferChars) : next,
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

  const filteredWindows = useMemo(() => {
    const statusNeedle = normalizeStatus(statusFilter)
    const query = searchValue.trim().toLowerCase()
    return app.windows.filter((win) => {
      if (statusNeedle && normalizeStatus(win.status) !== statusNeedle) {
        return false
      }
      if (!query) {
        return true
      }
      const projectName = (projectByWindow[win.id] ?? 'Unassigned').toLowerCase()
      return win.name.toLowerCase().includes(query) || projectName.includes(query) || normalizeStatus(win.status).includes(query)
    })
  }, [app.windows, projectByWindow, searchValue, statusFilter])
  const statusOptions = useMemo(() => {
    const set = new Set<string>()
    for (const win of app.windows) {
      set.add(normalizeStatus(win.status))
    }
    return Array.from(set).sort()
  }, [app.windows])

  const createSession = () => {
    const timestamp = new Date().toISOString().replace(/[-:TZ.]/g, '').slice(0, 14)
    const ok = app.send({ type: 'new_session', name: `session-${timestamp}` })
    if (!ok) {
      setSendError('Socket disconnected. Reconnect and try again.')
      return
    }
    setSendError('')
  }

  const openWindow = (windowID: string) => {
    app.setActiveWindow(windowID)
    if (isMobile) {
      navigate(`/sessions/${encodeURIComponent(windowID)}`)
    }
  }

  const renderViewer = () => (
    <>
      <div className="viewer-toolbar">
        <strong>{activeWindowInfo ? `${activeWindowInfo.name} (${activeWindowInfo.session_id || 'default'})` : 'Select a session'}</strong>
        <small className="empty-text">xterm.js</small>
      </div>

      {!selectedWindowID && <div className="empty-view">Select a session to start</div>}

      {selectedWindowID && (
        <Terminal
          sessionId={selectedWindowID}
          history={rawHistory}
          onInput={(keys) =>
            app.send({
              type: 'terminal_input',
              session_id: activeWindowInfo?.session_id,
              window: selectedWindowID,
              keys,
            })
          }
          onResize={(cols, rows) =>
            app.send({
              type: 'terminal_resize',
              session_id: activeWindowInfo?.session_id,
              window: selectedWindowID,
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
    </>
  )

  if (isMobile && !windowId) {
    return (
      <section className="sessions-mobile-list-page">
        <div className="sessions-panel-head">
          <h3>Sessions</h3>
          <div className="sessions-panel-actions">
            <button className="secondary-btn" onClick={createSession} type="button">
              <Plus size={14} />
              <span>New Session</span>
            </button>
          </div>
        </div>
        <div className="sessions-mobile-filters">
          <input placeholder="Search session / project / status" value={searchValue} onChange={(event) => setSearchValue(event.target.value)} />
          <select value={statusFilter} onChange={(event) => setStatusFilter(event.target.value)}>
            <option value="">All status</option>
            {statusOptions.map((status) => (
              <option key={status} value={status}>
                {status}
              </option>
            ))}
          </select>
        </div>
        <div className="sessions-mobile-list">
          {filteredWindows.map((win) => (
            <div className="session-row" key={win.id}>
              <button className="session-row-main" onClick={() => openWindow(win.id)} type="button">
                <span>{win.name}</span>
                <small>{projectByWindow[win.id] ?? 'Unassigned'} · {win.status}</small>
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
          {filteredWindows.length === 0 && <p className="empty-text">No sessions matched filters.</p>}
        </div>
      </section>
    )
  }

  if (isMobile && windowId) {
    return (
      <section className="sessions-mobile-viewer-page">
        <div className="sessions-mobile-viewer-head">
          <button className="secondary-btn" onClick={() => navigate('/sessions')} type="button">
            <ChevronLeft size={14} />
            <span>Back</span>
          </button>
          <button
            className="action-btn danger"
            onClick={() => {
              if (selectedWindowID) {
                app.send({ type: 'kill_window', session_id: activeWindowInfo?.session_id, window: selectedWindowID })
                navigate('/sessions')
              }
            }}
            type="button"
          >
            <Square size={12} />
            <span>End</span>
          </button>
        </div>
        <section className="viewer-panel">{renderViewer()}</section>
      </section>
    )
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
                <button className="session-row-main" onClick={() => openWindow(win.id)} type="button">
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

      <section className="viewer-panel">{renderViewer()}</section>
    </section>
  )
}
